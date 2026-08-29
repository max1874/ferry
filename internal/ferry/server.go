package ferry

import (
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
	"unicode/utf8"

	"github.com/max1874/ferry/internal/webui"
)

const (
	maxJSONBody       = 512 << 10
	multipartOverhead = 1 << 20
)

type server struct {
	store  *Store
	logger *log.Logger
}

func NewHandler(store *Store, logger *log.Logger) http.Handler {
	if logger == nil {
		logger = log.Default()
	}
	s := &server{store: store, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/v1/messages", s.listMessages)
	mux.HandleFunc("POST /api/v1/messages/text", s.createText)
	mux.HandleFunc("POST /api/v1/messages/file", s.createFile)
	mux.HandleFunc("GET /api/v1/files/{message_id}", s.downloadFile)
	mux.Handle("/", webui.Handler())
	return requestBoundary(securityHeaders(mux))
}

func requestBoundary(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			writeError(w, http.StatusMisdirectedRequest, "invalid_host", "this milestone only accepts loopback hosts")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !sameOriginOrNative(r) {
			writeError(w, http.StatusForbidden, "cross_origin_denied", "cross-origin writes are not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(hostPort string) bool {
	host := hostPort
	if parsedHost, _, err := net.SplitHostPort(hostPort); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func sameOriginOrNative(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
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
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain only sender_name and text")
		return
	}
	message, err := s.store.CreateText(r.Context(), string(request.SenderName), string(request.Text))
	if err != nil {
		s.domainError(w, err)
		return
	}
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
		writeError(w, http.StatusBadRequest, "invalid_request", "multipart body must contain exactly one sender_name and one file")
		return
	}
	source, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "file is required")
		return
	}
	defer source.Close()
	mediaType := header.Header.Get("Content-Type")
	message, err := s.store.CreateFile(r.Context(), r.MultipartForm.Value["sender_name"][0], header.Filename, mediaType, source)
	if err != nil {
		s.domainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, message)
}

func validMultipartShape(r *http.Request) bool {
	form := r.MultipartForm
	if form == nil || len(form.Value) != 1 || len(form.File) != 1 {
		return false
	}
	return len(form.Value["sender_name"]) == 1 && len(form.File["file"]) == 1
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
	SenderName strictString
	Text       strictString
}

func decodeTextRequest(source io.Reader) (textRequest, error) {
	decoder := json.NewDecoder(source)
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return textRequest{}, fmt.Errorf("request must be a JSON object")
	}
	var request textRequest
	seen := make(map[string]bool, 2)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return textRequest{}, err
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return textRequest{}, fmt.Errorf("request contains a duplicate field")
		}
		seen[key] = true
		switch key {
		case "sender_name":
			err = decoder.Decode(&request.SenderName)
		case "text":
			err = decoder.Decode(&request.Text)
		default:
			return textRequest{}, fmt.Errorf("request contains unknown field %q", key)
		}
		if err != nil {
			return textRequest{}, err
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seen["sender_name"] || !seen["text"] {
		return textRequest{}, fmt.Errorf("request must contain sender_name and text")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return textRequest{}, fmt.Errorf("multiple JSON values")
		}
		return textRequest{}, err
	}
	return request, nil
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
