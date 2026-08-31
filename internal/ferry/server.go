package ferry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/max1874/ferry/internal/webui"
)

const (
	maxJSONBody       = 512 << 10
	multipartOverhead = 1 << 20
)

type server struct {
	store    *Store
	logger   *log.Logger
	accessMu sync.Mutex
}

type HandlerOptions struct {
	AllowLANHosts bool
	Logger        *log.Logger
}

func NewHandler(store *Store, options HandlerOptions) http.Handler {
	if options.Logger == nil {
		options.Logger = log.Default()
	}
	s := &server{store: store, logger: options.Logger}
	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/session", s.session)
	api.HandleFunc("GET /api/v1/messages", s.listMessages)
	api.HandleFunc("POST /api/v1/messages/text", s.createText)
	api.HandleFunc("POST /api/v1/messages/file", s.createFile)
	api.HandleFunc("GET /api/v1/files/{message_id}", s.downloadFile)
	api.HandleFunc("GET /api/v1/devices", s.listDevices)
	api.HandleFunc("DELETE /api/v1/devices/{device_id}", s.deleteDevice)
	api.HandleFunc("GET /api/v1/settings/access", s.accessSettings)
	api.HandleFunc("PUT /api/v1/settings/access", s.updateAccessSettings)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/v1/access", s.accessStatus)
	mux.HandleFunc("POST /api/v1/access/join", s.join)
	mux.Handle("/api/v1/", s.requireDevice(api))
	mux.Handle("/", webui.Handler())
	return requestBoundary(securityHeaders(mux), options.AllowLANHosts)
}

func requestBoundary(next http.Handler, allowLANHosts bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAllowedHost(r.Host, allowLANHosts) {
			writeError(w, http.StatusMisdirectedRequest, "invalid_host", "Host must be localhost or an allowed IP address")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !sameOriginOrNative(r) {
			writeError(w, http.StatusForbidden, "cross_origin_denied", "cross-origin writes are not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAllowedHost(hostPort string, allowLANHosts bool) bool {
	host := hostPort
	if parsedHost, _, err := net.SplitHostPort(hostPort); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	if address == nil {
		return false
	}
	return address.IsLoopback() || allowLANHosts && (address.IsPrivate() || address.IsLinkLocalUnicast())
}

func sameOriginOrNative(r *http.Request) bool {
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return true
	}
	if len(origins) != 1 {
		return false
	}
	parsed, err := url.Parse(origins[0])
	expectedScheme := "http"
	if r.TLS != nil {
		expectedScheme = "https"
	}
	return err == nil && parsed.Scheme == expectedScheme && parsed.Host != "" && strings.EqualFold(parsed.Host, r.Host)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type deviceContextKey struct{}

func (s *server) requireDevice(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := requestDeviceToken(r)
		if !ok {
			s.unauthorized(w)
			return
		}
		hash, ok := hashDeviceToken(token)
		if !ok {
			s.unauthorized(w)
			return
		}
		device, err := s.store.AuthenticateDevice(r.Context(), hash)
		if errors.Is(err, ErrNotFound) {
			s.unauthorized(w)
			return
		}
		if err != nil {
			s.internalError(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), deviceContextKey{}, device)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestDeviceToken(r *http.Request) (string, bool) {
	authorizationValues := r.Header.Values("Authorization")
	if len(authorizationValues) != 1 {
		return "", false
	}
	scheme, token, found := strings.Cut(authorizationValues[0], " ")
	token = strings.TrimLeft(token, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return token, token != "" && !strings.ContainsAny(token, " \t\r\n,")
}

func currentDevice(r *http.Request) Device {
	return r.Context().Value(deviceContextKey{}).(Device)
}

func (s *server) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "unauthorized", "connect this device to Ferry")
}

func (s *server) session(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]Device{"device": currentDevice(r)})
}

type accessJoinRequest struct {
	DeviceName strictString
	Password   strictString
}

func (s *server) accessStatus(w http.ResponseWriter, r *http.Request) {
	verifier, err := s.store.AccessPassword(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]bool{"password_required": verifier != nil})
}

func (s *server) join(w http.ResponseWriter, r *http.Request) {
	if !hasMediaType(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	request, err := decodeAccessJoinRequest(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain only device_name and password")
		return
	}
	name, err := normalizeSenderName(string(request.DeviceName))
	if err != nil {
		s.domainError(w, err)
		return
	}
	deviceKind, err := requestDeviceKind(r, name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "X-Ferry-Device-Kind must be iphone, ipad, mac, android, windows, or browser")
		return
	}
	password := string(request.Password)
	if password != "" && !validAccessPassword(password) {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("password must be valid UTF-8 and at most %d bytes", maxAccessPasswordBytes))
		return
	}
	s.accessMu.Lock()
	defer s.accessMu.Unlock()
	verifier, err := s.store.AccessPassword(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	if verifier != nil && !verifyAccessPassword(password, *verifier) {
		writeError(w, http.StatusUnauthorized, "invalid_password", "password is incorrect")
		return
	}
	token, tokenHash, err := newDeviceToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	device, err := s.store.CreateDeviceWithKind(r.Context(), name, deviceKind, tokenHash)
	if err != nil {
		s.domainError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]any{"device": device, "token": token})
}

func requestDeviceKind(r *http.Request, deviceName string) (DeviceKind, error) {
	values := r.Header.Values("X-Ferry-Device-Kind")
	if len(values) == 0 {
		return inferDeviceKind(deviceName), nil
	}
	if len(values) != 1 || values[0] == "" || strings.TrimSpace(values[0]) != values[0] || strings.Contains(values[0], ",") {
		return "", ErrInvalid
	}
	return normalizeDeviceKind(values[0], deviceName)
}

func (s *server) accessSettings(w http.ResponseWriter, r *http.Request) {
	s.accessStatus(w, r)
}

type accessSettingsRequest struct {
	Password strictString
}

func (s *server) updateAccessSettings(w http.ResponseWriter, r *http.Request) {
	if !hasMediaType(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	request, err := decodeAccessSettingsRequest(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain only password")
		return
	}
	password := string(request.Password)
	var verifier *AccessPasswordVerifier
	if password != "" {
		derived, err := newAccessPasswordVerifier(password)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		verifier = &derived
	}
	s.accessMu.Lock()
	err = s.store.SetAccessPasswordForDevice(r.Context(), currentDevice(r).ID, verifier)
	s.accessMu.Unlock()
	if errors.Is(err, ErrUnauthorized) {
		s.unauthorized(w)
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]bool{"password_required": verifier != nil})
}

func (s *server) listDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.store.ListDevices(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]Device{"devices": devices})
}

func (s *server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("device_id")
	err := s.store.DeleteDevice(r.Context(), currentDevice(r).ID, id)
	if errors.Is(err, ErrUnauthorized) {
		s.unauthorized(w)
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "device was not found")
		return
	}
	if errors.Is(err, ErrInvalid) {
		writeError(w, http.StatusConflict, "cannot_revoke_device", "the current or last device cannot be revoked")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listMessages(w http.ResponseWriter, r *http.Request) {
	after, err := parseNonNegativeInt64(r.URL.Query().Get("after"), 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "after must be a non-negative integer")
		return
	}
	limit, err := parseBoundedInt(r.URL.Query().Get("limit"), 100, 1, 200)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "limit must be an integer from 1 to 200")
		return
	}
	messages, nextCursor, err := s.store.ListMessages(r.Context(), after, limit)
	if err != nil {
		s.internalError(w, err)
		return
	}
	deviceID := currentDevice(r).ID
	for index := range messages {
		messages[index].IsCurrentDevice = messages[index].SenderDeviceID != "" && messages[index].SenderDeviceID == deviceID
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"messages":    messages,
		"next_cursor": nextCursor,
	})
}

func (s *server) createText(w http.ResponseWriter, r *http.Request) {
	if !hasMediaType(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	request, err := decodeTextRequest(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain only text")
		return
	}
	message, err := s.store.CreateTextForDevice(r.Context(), currentDevice(r), string(request.Text))
	if errors.Is(err, ErrUnauthorized) {
		s.unauthorized(w)
		return
	}
	if err != nil {
		s.domainError(w, err)
		return
	}
	message.IsCurrentDevice = true
	writeJSON(w, http.StatusCreated, message)
}

func (s *server) createFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxFileBytes+multipartOverhead)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "file upload is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "request must be multipart form data")
		return
	}
	defer r.MultipartForm.RemoveAll()
	if !validMultipartShape(r) {
		writeError(w, http.StatusBadRequest, "invalid_request", "multipart body must contain exactly one file")
		return
	}
	source, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "file is required")
		return
	}
	defer source.Close()
	mediaType := header.Header.Get("Content-Type")
	message, err := s.store.CreateFileForDevice(r.Context(), currentDevice(r), header.Filename, mediaType, source)
	if errors.Is(err, ErrUnauthorized) {
		s.unauthorized(w)
		return
	}
	if err != nil {
		s.domainError(w, err)
		return
	}
	message.IsCurrentDevice = true
	writeJSON(w, http.StatusCreated, message)
}

func validMultipartShape(r *http.Request) bool {
	form := r.MultipartForm
	if form == nil || len(form.Value) != 0 || len(form.File) != 1 {
		return false
	}
	return len(form.File["file"]) == 1
}

func (s *server) downloadFile(w http.ResponseWriter, r *http.Request) {
	message, file, err := s.store.OpenFile(r.Context(), r.PathValue("message_id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "file message was not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", message.File.MediaType)
	w.Header().Set("Content-Length", strconv.FormatInt(message.File.Size, 10))
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": message.File.Name})
	w.Header().Set("Content-Disposition", disposition)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, file); err != nil {
		s.logger.Printf("stream file %s: %v", message.ID, err)
	}
}

func (s *server) domainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", err.Error())
	case errors.Is(err, ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		s.internalError(w, err)
	}
}

func (s *server) internalError(w http.ResponseWriter, err error) {
	s.logger.Printf("request failed: %v", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "the server could not complete the request")
}

func parseBoundedInt(value string, fallback, minimum, maximum int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("integer is outside bounds")
	}
	return parsed, nil
}

func parseNonNegativeInt64(value string, fallback int64) (int64, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("integer is outside bounds")
	}
	return parsed, nil
}

func hasMediaType(value, expected string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == expected
}

type textRequest struct {
	Text strictString
}

func decodeTextRequest(source io.Reader) (textRequest, error) {
	var request textRequest
	err := decodeStrictObject(source, map[string]func(*json.Decoder) error{
		"text": func(decoder *json.Decoder) error { return decoder.Decode(&request.Text) },
	})
	return request, err
}

func decodeAccessJoinRequest(source io.Reader) (accessJoinRequest, error) {
	var request accessJoinRequest
	err := decodeStrictObject(source, map[string]func(*json.Decoder) error{
		"device_name": func(decoder *json.Decoder) error { return decoder.Decode(&request.DeviceName) },
		"password":    func(decoder *json.Decoder) error { return decoder.Decode(&request.Password) },
	})
	return request, err
}

func decodeAccessSettingsRequest(source io.Reader) (accessSettingsRequest, error) {
	var request accessSettingsRequest
	err := decodeStrictObject(source, map[string]func(*json.Decoder) error{
		"password": func(decoder *json.Decoder) error { return decoder.Decode(&request.Password) },
	})
	return request, err
}

func decodeStrictObject(source io.Reader, fields map[string]func(*json.Decoder) error) error {
	decoder := json.NewDecoder(source)
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return fmt.Errorf("request must be a JSON object")
	}
	seen := make(map[string]bool, len(fields))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return fmt.Errorf("request contains a duplicate field")
		}
		seen[key] = true
		decode, exists := fields[key]
		if !exists {
			return fmt.Errorf("request contains unknown field %q", key)
		}
		if err := decode(decoder); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(seen) != len(fields) {
		return fmt.Errorf("request does not contain the exact required fields")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

type strictString string

func (s *strictString) UnmarshalJSON(data []byte) error {
	if !validJSONStringEncoding(data) {
		return fmt.Errorf("JSON string contains invalid Unicode")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = strictString(value)
	return nil
}

func validJSONStringEncoding(data []byte) bool {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return false
	}
	for index := 1; index < len(data)-1; {
		if data[index] != '\\' {
			_, size := utf8.DecodeRune(data[index : len(data)-1])
			if size == 1 && data[index] >= utf8.RuneSelf {
				return false
			}
			index += size
			continue
		}
		if index+1 >= len(data)-1 || data[index+1] != 'u' {
			index += 2
			continue
		}
		code, ok := parseHex4(data, index+2)
		if !ok || code >= 0xdc00 && code <= 0xdfff {
			return false
		}
		if code >= 0xd800 && code <= 0xdbff {
			low, paired := parseHex4(data, index+8)
			if index+7 >= len(data)-1 || data[index+6] != '\\' || data[index+7] != 'u' || !paired || low < 0xdc00 || low > 0xdfff {
				return false
			}
			index += 12
			continue
		}
		index += 6
	}
	return true
}

func parseHex4(data []byte, start int) (uint64, bool) {
	if start+4 > len(data)-1 {
		return 0, false
	}
	value, err := strconv.ParseUint(string(data[start:start+4]), 16, 16)
	return value, err == nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": strings.TrimSpace(message),
		},
	})
}
