package ferry

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestHTTPTextContractAndCursor(t *testing.T) {
	handler := newTestHandler(t)

	created := requestJSON(t, handler, http.MethodPost, "/api/v1/messages/text", `{"text":"hello ferry"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var message Message
	decodeResponse(t, created, &message)
	assertJSONKeys(t, created.Body.Bytes(), "created text", []string{"created_at", "id", "is_current_device", "kind", "sender_kind", "sender_name", "sequence", "text"})
	if message.Kind != KindText || message.Text == nil || *message.Text != "hello ferry" || message.File != nil {
		t.Fatalf("created message = %#v", message)
	}

	listed := requestJSON(t, handler, http.MethodGet, "/api/v1/messages?after=0&limit=1", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listed.Code, listed.Body.String())
	}
	var list struct {
		Messages   []Message `json:"messages"`
		NextCursor int64     `json:"next_cursor"`
	}
	decodeResponse(t, listed, &list)
	if len(list.Messages) != 1 || list.Messages[0].ID != message.ID || list.NextCursor != message.Sequence {
		t.Fatalf("list = %#v", list)
	}

	empty := requestJSON(t, handler, http.MethodGet, "/api/v1/messages?after="+strconv.FormatInt(message.Sequence, 10), "")
	decodeResponse(t, empty, &list)
	if len(list.Messages) != 0 || list.NextCursor != message.Sequence {
		t.Fatalf("empty cursor response = %#v", list)
	}
}

func TestHTTPRejectsTextBoundaryAndUnknownFields(t *testing.T) {
	handler := newTestHandler(t)
	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{name: "whitespace", body: `{"text":"  "}`, status: 400, code: "invalid_request"},
		{name: "over maximum", body: `{"text":"` + strings.Repeat("x", MaxTextBytes+1) + `"}`, status: 413, code: "payload_too_large"},
		{name: "unknown field", body: `{"text":"hello","admin":true}`, status: 400, code: "invalid_request"},
		{name: "old sender field", body: `{"sender_name":"Admin","text":"hello"}`, status: 400, code: "invalid_request"},
		{name: "duplicate field", body: `{"text":"first","text":"second"}`, status: 400, code: "invalid_request"},
		{name: "equivalent duplicate field", body: `{"text":"first","te\u0078t":"second"}`, status: 400, code: "invalid_request"},
		{name: "multiple documents", body: `{"text":"hello"}{}`, status: 400, code: "invalid_request"},
		{name: "lone high surrogate", body: `{"text":"\ud800"}`, status: 400, code: "invalid_request"},
		{name: "lone low surrogate", body: `{"text":"\udc00"}`, status: 400, code: "invalid_request"},
		{name: "invalid utf8", body: `{"text":"` + string([]byte{0xff}) + `"}`, status: 400, code: "invalid_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestJSON(t, handler, http.MethodPost, "/api/v1/messages/text", test.body)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.status, response.Body.String())
			}
			if got := errorCode(t, response); got != test.code {
				t.Fatalf("error code = %q, want %q", got, test.code)
			}
		})
	}
}

func TestHTTPAcceptsPairedSurrogate(t *testing.T) {
	handler := newTestHandler(t)
	response := requestJSON(t, handler, http.MethodPost, "/api/v1/messages/text", `{"text":"\ud83d\ude80"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var message Message
	decodeResponse(t, response, &message)
	if message.Text == nil || *message.Text != "🚀" {
		t.Fatalf("text = %#v, want rocket", message.Text)
	}
}

func TestHTTPTextRequiresJSONContentType(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/messages/text", strings.NewReader(`{"text":"hello"}`))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType || errorCode(t, response) != "unsupported_media_type" {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRequestBoundaryRejectsCrossOriginWrite(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/messages/text", strings.NewReader(`{"text":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || errorCode(t, response) != "cross_origin_denied" {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRequestBoundaryRejectsCrossSchemeWrite(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/messages/text", strings.NewReader(`{"text":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://127.0.0.1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || errorCode(t, response) != "cross_origin_denied" {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRequestBoundaryRejectsMultipleOrigins(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/messages/text", strings.NewReader(`{"text":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("Origin", "http://127.0.0.1")
	request.Header.Add("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || errorCode(t, response) != "cross_origin_denied" {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRequestBoundaryRejectsNonLoopbackHost(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "http://evil.example/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMisdirectedRequest || errorCode(t, response) != "invalid_host" {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHTTPFileContractDownloadAndTraversalName(t *testing.T) {
	handler := newTestHandler(t)
	response := multipartRequest(t, handler, `../../hello.txt`, []byte("ferry file\n"), nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body.String())
	}
	var message Message
	decodeResponse(t, response, &message)
	assertJSONKeys(t, response.Body.Bytes(), "created file", []string{"created_at", "file", "id", "is_current_device", "kind", "sender_kind", "sender_name", "sequence"})
	var rawFileMessage map[string]any
	decodeResponse(t, response, &rawFileMessage)
	fileObject, ok := rawFileMessage["file"].(map[string]any)
	if !ok {
		t.Fatalf("file field = %#v", rawFileMessage["file"])
	}
	assertMapKeys(t, fileObject, "file info", []string{"download_url", "media_type", "name", "size"})
	if message.Kind != KindFile || message.Text != nil || message.File == nil || message.File.Name != "hello.txt" || message.File.Size != 11 {
		t.Fatalf("file message = %#v", message)
	}

	download := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+message.File.DownloadURL, nil)
	handler.ServeHTTP(download, request)
	if download.Code != http.StatusOK || download.Body.String() != "ferry file\n" {
		t.Fatalf("download status = %d, body = %q", download.Code, download.Body.String())
	}
	if disposition := download.Header().Get("Content-Disposition"); !strings.Contains(disposition, "hello.txt") || strings.Contains(disposition, "..") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
}

func TestHTTPFileEndpointReturnsNotFoundForTextMessage(t *testing.T) {
	handler := newTestHandler(t)
	created := requestJSON(t, handler, http.MethodPost, "/api/v1/messages/text", `{"text":"hello"}`)
	var message Message
	decodeResponse(t, created, &message)
	response := requestJSON(t, handler, http.MethodGet, "/api/v1/files/"+message.ID, "")
	if response.Code != http.StatusNotFound || errorCode(t, response) != "not_found" {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHTTPRejectsUnexpectedMultipartField(t *testing.T) {
	handler := newTestHandler(t)
	for _, extra := range []map[string]string{{"unexpected": "value"}, {"sender_name": "Admin"}} {
		response := multipartRequest(t, handler, "hello.txt", []byte("hello"), extra)
		if response.Code != http.StatusBadRequest || errorCode(t, response) != "invalid_request" {
			t.Fatalf("extra = %v, status = %d, body = %s", extra, response.Code, response.Body.String())
		}
	}
}

func TestHTTPRejectsFileOverMaximumWithoutCreatingMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("streams the 64 MiB contract boundary")
	}
	handler := newTestHandler(t)
	body, contentType := streamingMultipart(t, "large.bin", MaxFileBytes+1)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/messages/file", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge || errorCode(t, response) != "payload_too_large" {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	list := requestJSON(t, handler, http.MethodGet, "/api/v1/messages", "")
	var payload struct {
		Messages []Message `json:"messages"`
	}
	decodeResponse(t, list, &payload)
	if len(payload.Messages) != 0 {
		t.Fatalf("oversize upload created messages: %#v", payload.Messages)
	}
}

func TestStaticWebAndSecurityHeadersShareHandler(t *testing.T) {
	handler := newTestHandler(t)
	response := requestJSON(t, handler, http.MethodGet, "/", "")
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "<title>Ferry</title>") || !strings.Contains(body, `id="composer"`) {
		t.Fatalf("web response status = %d, body = %s", response.Code, response.Body.String())
	}
	expectedPolicy := "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"
	if response.Header().Get("Content-Security-Policy") != expectedPolicy || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers = %v", response.Header())
	}
}

// A device has to trust the certificate before it can join, so the CA file is
// served outside the device session — but only the public certificate, and only
// when this server actually manages a CA.
func TestCACertificateIsServedWithoutASession(t *testing.T) {
	store := openTestStore(t)
	certificate := []byte("-----BEGIN CERTIFICATE-----\nferry\n-----END CERTIFICATE-----\n")
	handler := NewHandler(store, HandlerOptions{Logger: log.New(io.Discard, "", 0), CACertificate: certificate})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost/ferry-ca.crt", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Body.String() != string(certificate) {
		t.Fatalf("body = %q", response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/x-x509-ca-cert" {
		t.Fatalf("content type = %q", contentType)
	}

	withoutCA := NewHandler(openTestStore(t), HandlerOptions{Logger: log.New(io.Discard, "", 0)})
	response = httptest.NewRecorder()
	withoutCA.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost/ferry-ca.crt", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status without a managed CA = %d, body = %s", response.Code, response.Body.String())
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	store := openTestStore(t)
	token, hash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateDevice(t.Context(), "Web", hash); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, HandlerOptions{Logger: log.New(io.Discard, "", 0)})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Authorization")) == 0 {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		handler.ServeHTTP(w, r)
	})
}

func requestJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://127.0.0.1"+path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func multipartRequest(t *testing.T, handler http.Handler, name string, contents []byte, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range extra {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/messages/file", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func streamingMultipart(t *testing.T, name string, size int64) (io.Reader, string) {
	t.Helper()
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	go func() {
		defer writer.Close()
		part, err := multipartWriter.CreateFormFile("file", name)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		if _, err := io.CopyN(part, zeroReader{}, size); err != nil {
			writer.CloseWithError(err)
			return
		}
		if err := multipartWriter.Close(); err != nil {
			writer.CloseWithError(err)
		}
	}()
	return reader, multipartWriter.FormDataContentType()
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 0
	}
	return len(buffer), nil
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
		t.Fatalf("decode %q: %v", response.Body.String(), err)
	}
}

func errorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeResponse(t, response, &payload)
	return payload.Error.Code
}

func assertJSONKeys(t *testing.T, body []byte, name string, expected []string) {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	assertMapKeys(t, object, name, expected)
}

func assertMapKeys(t *testing.T, object map[string]any, name string, expected []string) {
	t.Helper()
	actual := make([]string, 0, len(object))
	for key := range object {
		actual = append(actual, key)
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if strings.Join(actual, ",") != strings.Join(expected, ",") {
		t.Fatalf("%s keys = %v, want %v", name, actual, expected)
	}
}
