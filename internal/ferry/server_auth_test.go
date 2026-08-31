package ferry

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPPasswordlessPasswordAndRevocationJourney(t *testing.T) {
	handler, store := newRawTestApp(t, false)

	status := requestJSON(t, handler, http.MethodGet, "/api/v1/access", "")
	assertStatus(t, status, http.StatusOK)
	assertBodyContains(t, status, `"password_required":false`)

	firstResponse := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", `{"device_name":" Max Mac ","password":""}`)
	first := decodeClaim(t, firstResponse)
	if firstResponse.Code != http.StatusCreated || first.Device.Name != "Max Mac" || first.Token == "" {
		t.Fatalf("first join status = %d, payload = %#v", firstResponse.Code, first)
	}
	if firstResponse.Header().Get("Cache-Control") != "no-store" || len(firstResponse.Result().Cookies()) != 0 {
		t.Fatalf("join response was cacheable or set a cookie")
	}

	enable := authenticatedRequest(t, handler, http.MethodPut, "/api/v1/settings/access", `{"password":"correct horse"}`, first.Token)
	assertStatus(t, enable, http.StatusOK)
	assertBodyContains(t, enable, `"password_required":true`)
	if strings.Contains(enable.Body.String(), "correct horse") {
		t.Fatal("settings response exposed the password")
	}

	before, err := store.DeviceCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"", "wrong", " correct horse", "correct horse "} {
		response := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", `{"device_name":"Rejected","password":"`+password+`"}`)
		if response.Code != http.StatusUnauthorized || errorCode(t, response) != "invalid_password" {
			t.Fatalf("password %q status = %d, body = %s", password, response.Code, response.Body.String())
		}
	}
	after, err := store.DeviceCount(t.Context())
	if err != nil || before != after {
		t.Fatalf("rejected joins changed device count: %d -> %d, err %v", before, after, err)
	}

	secondResponse := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", `{"device_name":"iPhone","password":"correct horse"}`)
	second := decodeClaim(t, secondResponse)
	assertStatus(t, secondResponse, http.StatusCreated)

	firstStillWorks := authenticatedRequest(t, handler, http.MethodGet, "/api/v1/session", "", first.Token)
	assertStatus(t, firstStillWorks, http.StatusOK)

	change := authenticatedRequest(t, handler, http.MethodPut, "/api/v1/settings/access", `{"password":"new secret"}`, second.Token)
	assertStatus(t, change, http.StatusOK)
	oldPassword := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", `{"device_name":"Old","password":"correct horse"}`)
	assertStatus(t, oldPassword, http.StatusUnauthorized)
	newPassword := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", `{"device_name":"New","password":"new secret"}`)
	assertStatus(t, newPassword, http.StatusCreated)

	disable := authenticatedRequest(t, handler, http.MethodPut, "/api/v1/settings/access", `{"password":""}`, first.Token)
	assertStatus(t, disable, http.StatusOK)
	passwordlessAgain := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", `{"device_name":"Browser","password":""}`)
	assertStatus(t, passwordlessAgain, http.StatusCreated)

	revoked := authenticatedRequest(t, handler, http.MethodDelete, "/api/v1/devices/"+second.Device.ID, "", first.Token)
	assertStatus(t, revoked, http.StatusNoContent)
	secondAfterRevoke := authenticatedRequest(t, handler, http.MethodGet, "/api/v1/messages", "", second.Token)
	assertStatus(t, secondAfterRevoke, http.StatusUnauthorized)
}

func TestStaleDeviceCannotChangeAccessSettings(t *testing.T) {
	_, store := newRawTestApp(t, false)
	create := func(name string) Device {
		t.Helper()
		_, hash, err := newDeviceToken()
		if err != nil {
			t.Fatal(err)
		}
		device, err := store.CreateDevice(t.Context(), name, hash)
		if err != nil {
			t.Fatal(err)
		}
		return device
	}
	revoked := create("Revoked")
	owner := create("Owner")
	if err := store.DeleteDevice(t.Context(), owner.ID, revoked.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "http://127.0.0.1/api/v1/settings/access", strings.NewReader(`{"password":"stolen"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), deviceContextKey{}, revoked))
	response := httptest.NewRecorder()
	(&server{store: store, logger: log.New(io.Discard, "", 0)}).updateAccessSettings(response, request)
	if response.Code != http.StatusUnauthorized || errorCode(t, response) != "unauthorized" {
		t.Fatalf("stale settings status = %d, body = %s", response.Code, response.Body.String())
	}
	verifier, err := store.AccessPassword(t.Context())
	if err != nil || verifier != nil {
		t.Fatalf("stale request changed setting: %#v, %v", verifier, err)
	}
}

func TestAccessJSONIsStrictAndSettingsAreProtected(t *testing.T) {
	handler, _ := newRawTestApp(t, false)
	for _, body := range []string{
		`{"device_name":"A"}`,
		`{"device_name":"A","password":"","password":"again"}`,
		`{"device_name":"A","password":"","role":"admin"}`,
		`{"device_name":"A","device\u005fname":"B","password":""}`,
		`{"device_name":"A","password":"` + strings.Repeat("x", maxAccessPasswordBytes+1) + `"}`,
	} {
		response := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, response.Code)
		}
	}
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/session"}, {http.MethodGet, "/api/v1/messages"},
		{http.MethodPost, "/api/v1/messages/text"}, {http.MethodPost, "/api/v1/messages/file"},
		{http.MethodGet, "/api/v1/files/0123456789abcdef0123456789abcdef"},
		{http.MethodGet, "/api/v1/devices"}, {http.MethodDelete, "/api/v1/devices/0123456789abcdef0123456789abcdef"},
		{http.MethodGet, "/api/v1/settings/access"}, {http.MethodPut, "/api/v1/settings/access"},
		{http.MethodGet, "/api/v1/unknown"},
	} {
		response := requestJSON(t, handler, endpoint.method, endpoint.path, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d", endpoint.method, endpoint.path, response.Code)
		}
	}
}

func TestJoinDeviceKindHeaderIsOptionalStrictAndCopiedToMessages(t *testing.T) {
	handler, store := newRawTestApp(t, false)
	join := func(name string, values ...string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/access/join", strings.NewReader(`{"device_name":"`+name+`","password":""}`))
		request.Header.Set("Content-Type", "application/json")
		for _, value := range values {
			request.Header.Add("X-Ferry-Device-Kind", value)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	inferredResponse := join("iPhone 17 Pro")
	inferred := decodeClaim(t, inferredResponse)
	if inferredResponse.Code != http.StatusCreated || inferred.Device.Kind != DeviceKindIPhone {
		t.Fatalf("inferred join status = %d, device = %#v", inferredResponse.Code, inferred.Device)
	}

	explicitResponse := join("Kitchen Display", "mac")
	explicit := decodeClaim(t, explicitResponse)
	if explicitResponse.Code != http.StatusCreated || explicit.Device.Kind != DeviceKindMac {
		t.Fatalf("explicit join status = %d, device = %#v", explicitResponse.Code, explicit.Device)
	}
	created := authenticatedRequest(t, handler, http.MethodPost, "/api/v1/messages/text", `{"text":"from explicit mac"}`, explicit.Token)
	var message Message
	decodeResponse(t, created, &message)
	if created.Code != http.StatusCreated || message.SenderKind != DeviceKindMac {
		t.Fatalf("created status = %d, message = %#v", created.Code, message)
	}

	before, err := store.DeviceCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range [][]string{{""}, {" mac"}, {"car"}, {"MAC"}, {"mac,browser"}, {"mac", "browser"}} {
		response := join("Rejected", values...)
		if response.Code != http.StatusBadRequest || errorCode(t, response) != "invalid_request" {
			t.Fatalf("header values %q status = %d, body = %s", values, response.Code, response.Body.String())
		}
	}
	after, err := store.DeviceCount(t.Context())
	if err != nil || after != before {
		t.Fatalf("invalid device kinds changed count: %d -> %d, err %v", before, after, err)
	}
}

func TestMessageCurrentDeviceProjectionUsesIdentityNotName(t *testing.T) {
	handler, _ := newRawTestApp(t, false)
	join := func() claimResponse {
		t.Helper()
		response := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", `{"device_name":"Same Mac","password":""}`)
		assertStatus(t, response, http.StatusCreated)
		return decodeClaim(t, response)
	}
	first := join()
	second := join()
	if first.Device.ID == second.Device.ID || first.Device.Name != second.Device.Name || first.Device.Kind != second.Device.Kind {
		t.Fatalf("test devices do not isolate identity: first=%#v second=%#v", first.Device, second.Device)
	}

	created := authenticatedRequest(t, handler, http.MethodPost, "/api/v1/messages/text", `{"text":"identity decides alignment"}`, first.Token)
	var createdMessage Message
	decodeResponse(t, created, &createdMessage)
	if created.Code != http.StatusCreated || !createdMessage.IsCurrentDevice || strings.Contains(created.Body.String(), "sender_device_id") {
		t.Fatalf("created message status=%d message=%#v body=%s", created.Code, createdMessage, created.Body.String())
	}

	listFor := func(token string) Message {
		t.Helper()
		response := authenticatedRequest(t, handler, http.MethodGet, "/api/v1/messages", "", token)
		var page struct {
			Messages []Message `json:"messages"`
		}
		decodeResponse(t, response, &page)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || len(page.Messages) != 1 || strings.Contains(response.Body.String(), "sender_device_id") {
			t.Fatalf("list status=%d messages=%#v body=%s", response.Code, page.Messages, response.Body.String())
		}
		return page.Messages[0]
	}
	if message := listFor(first.Token); !message.IsCurrentDevice {
		t.Fatalf("sender saw its message as foreign: %#v", message)
	}
	if message := listFor(second.Token); message.IsCurrentDevice {
		t.Fatalf("same-name peer saw foreign message as current: %#v", message)
	}
}

func TestCrossOriginJoinIsRejected(t *testing.T) {
	handler, _ := newRawTestApp(t, false)
	body := `{"device_name":"Mac","password":""}`
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/access/join", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || errorCode(t, response) != "cross_origin_denied" {
		t.Fatalf("cross-origin status = %d, body = %s", response.Code, response.Body.String())
	}
	accepted := requestJSON(t, handler, http.MethodPost, "/api/v1/access/join", body)
	assertStatus(t, accepted, http.StatusCreated)
}

func TestLANHostBoundaryRequiresExplicitModeAndPrivateAddress(t *testing.T) {
	loopbackOnly, _ := newRawTestApp(t, false)
	lan, _ := newRawTestApp(t, true)
	for _, test := range []struct {
		name    string
		handler http.Handler
		host    string
		status  int
	}{
		{"private denied by default", loopbackOnly, "192.168.1.20:8080", 421},
		{"private allowed in lan", lan, "192.168.1.20:8080", 200},
		{"link local allowed in lan", lan, "169.254.1.2:8080", 200},
		{"public denied in lan", lan, "8.8.8.8:8080", 421},
		{"hostname denied in lan", lan, "evil.example:8080", 421},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/healthz", nil)
			request.Host = test.host
			response := httptest.NewRecorder()
			test.handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
}

func TestWebClientStoresOnlySuccessfulJoinToken(t *testing.T) {
	handler, _ := newRawTestApp(t, false)
	response := requestJSON(t, handler, http.MethodGet, "/app.js", "")
	assertStatus(t, response, http.StatusOK)
	if strings.Contains(response.Body.String(), `removeItem("ferry_device_token")`) {
		t.Fatal("stale tab can delete a newer token")
	}
	if count := strings.Count(response.Body.String(), "storeToken("); count != 2 {
		t.Fatalf("storeToken occurrences = %d", count)
	}
}

type claimResponse struct {
	Device Device `json:"device"`
	Token  string `json:"token"`
}

func decodeClaim(t *testing.T, response *httptest.ResponseRecorder) claimResponse {
	t.Helper()
	var claim claimResponse
	if response.Code == http.StatusCreated {
		decodeResponse(t, response, &claim)
		assertJSONKeys(t, response.Body.Bytes(), "access join", []string{"device", "token"})
	}
	return claim
}

func authenticatedRequest(t *testing.T, handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://127.0.0.1"+path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func newRawTestApp(t *testing.T, allowLAN bool) (http.Handler, *Store) {
	t.Helper()
	store := openTestStore(t)
	return NewHandler(store, HandlerOptions{AllowLANHosts: allowLAN, Logger: log.New(io.Discard, "", 0)}), store
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, want, response.Body.String())
	}
}

func assertBodyContains(t *testing.T, response *httptest.ResponseRecorder, value string) {
	t.Helper()
	if !strings.Contains(response.Body.String(), value) {
		t.Fatalf("body %s does not contain %s", response.Body.String(), value)
	}
}
