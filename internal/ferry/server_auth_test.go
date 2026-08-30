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

func TestHTTPPairAuthenticateGenerateAndRevokeJourney(t *testing.T) {
	handler, store, pairing := newRawTestApp(t, false)

	unauthorized := requestJSON(t, handler, http.MethodGet, "/api/v1/messages", "")
	if unauthorized.Code != http.StatusUnauthorized || errorCode(t, unauthorized) != "unauthorized" {
		t.Fatalf("unauthorized status = %d, body = %s", unauthorized.Code, unauthorized.Body.String())
	}

	bootstrap, err := pairing.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	firstClaim := requestJSON(t, handler, http.MethodPost, "/api/v1/pairing/claim", `{"code":"`+bootstrap.Code+`","device_name":" Max Mac "}`)
	first := decodeClaim(t, firstClaim)
	if firstClaim.Code != http.StatusCreated || first.Device.Name != "Max Mac" || first.Token == "" {
		t.Fatalf("first claim status = %d, payload = %#v", firstClaim.Code, first)
	}
	if firstClaim.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("claim cache control = %q", firstClaim.Header().Get("Cache-Control"))
	}
	if len(firstClaim.Result().Cookies()) != 0 {
		t.Fatalf("claim leaked a cross-port cookie: %v", firstClaim.Result().Cookies())
	}
	session := authenticatedRequest(t, handler, http.MethodGet, "/api/v1/session", "", first.Token)
	if session.Code != http.StatusOK {
		t.Fatalf("Bearer session status = %d, body = %s", session.Code, session.Body.String())
	}
	assertJSONKeys(t, session.Body.Bytes(), "session", []string{"device"})
	lowercaseBearer := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/v1/session", nil)
	lowercaseBearer.Header.Set("Authorization", "bearer  "+first.Token)
	lowercaseResponse := httptest.NewRecorder()
	handler.ServeHTTP(lowercaseResponse, lowercaseBearer)
	if lowercaseResponse.Code != http.StatusOK {
		t.Fatalf("case-insensitive Bearer status = %d", lowercaseResponse.Code)
	}

	replay := requestJSON(t, handler, http.MethodPost, "/api/v1/pairing/claim", `{"code":"`+bootstrap.Code+`","device_name":"Replay"}`)
	if replay.Code != http.StatusUnauthorized || errorCode(t, replay) != "invalid_pairing_code" {
		t.Fatalf("replay status = %d, body = %s", replay.Code, replay.Body.String())
	}

	codeResponse := authenticatedRequest(t, handler, http.MethodPost, "/api/v1/pairing/codes", "", first.Token)
	if codeResponse.Code != http.StatusCreated {
		t.Fatalf("create pairing code status = %d, body = %s", codeResponse.Code, codeResponse.Body.String())
	}
	if codeResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("pairing code cache control = %q", codeResponse.Header().Get("Cache-Control"))
	}
	var secondCode PairingCode
	decodeResponse(t, codeResponse, &secondCode)
	assertJSONKeys(t, codeResponse.Body.Bytes(), "pairing code", []string{"code", "expires_at"})
	secondClaim := requestJSON(t, handler, http.MethodPost, "/api/v1/pairing/claim", `{"code":"`+secondCode.Code+`","device_name":"iPhone"}`)
	second := decodeClaim(t, secondClaim)
	if secondClaim.Code != http.StatusCreated || second.Device.Name != "iPhone" {
		t.Fatalf("second claim status = %d, payload = %#v", secondClaim.Code, second)
	}
	staleInviteResponse := authenticatedRequest(t, handler, http.MethodPost, "/api/v1/pairing/codes", "", second.Token)
	var staleInvite PairingCode
	decodeResponse(t, staleInviteResponse, &staleInvite)
	if staleInviteResponse.Code != http.StatusCreated {
		t.Fatalf("second device code status = %d, body = %s", staleInviteResponse.Code, staleInviteResponse.Body.String())
	}

	spoof := authenticatedRequest(t, handler, http.MethodPost, "/api/v1/messages/text", `{"sender_name":"Admin","text":"spoof"}`, first.Token)
	if spoof.Code != http.StatusBadRequest || errorCode(t, spoof) != "invalid_request" {
		t.Fatalf("spoof status = %d, body = %s", spoof.Code, spoof.Body.String())
	}
	created := authenticatedRequest(t, handler, http.MethodPost, "/api/v1/messages/text", `{"text":"from mac"}`, first.Token)
	var message Message
	decodeResponse(t, created, &message)
	if created.Code != http.StatusCreated || message.SenderName != "Max Mac" {
		t.Fatalf("created status = %d, message = %#v", created.Code, message)
	}

	devicesResponse := authenticatedRequest(t, handler, http.MethodGet, "/api/v1/devices", "", first.Token)
	var devices struct {
		Devices []Device `json:"devices"`
	}
	decodeResponse(t, devicesResponse, &devices)
	assertJSONKeys(t, devicesResponse.Body.Bytes(), "device list", []string{"devices"})
	if len(devices.Devices) != 2 {
		t.Fatalf("devices = %#v", devices.Devices)
	}

	badHeaderWithCookie := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/v1/session", nil)
	badHeaderWithCookie.Header.Set("Authorization", "Bearer wrong")
	badHeaderWithCookie.AddCookie(&http.Cookie{Name: "ferry_device", Value: first.Token})
	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, badHeaderWithCookie)
	if badResponse.Code != http.StatusUnauthorized {
		t.Fatalf("bad bearer fell back to cookie: status = %d", badResponse.Code)
	}

	revoked := authenticatedRequest(t, handler, http.MethodDelete, "/api/v1/devices/"+second.Device.ID, "", first.Token)
	if revoked.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, body = %s", revoked.Code, revoked.Body.String())
	}
	staleRequest := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/pairing/codes", nil)
	staleRequest = staleRequest.WithContext(context.WithValue(staleRequest.Context(), deviceContextKey{}, second.Device))
	staleResponse := httptest.NewRecorder()
	(&server{store: store, pairing: pairing, logger: log.New(io.Discard, "", 0)}).createPairingCode(staleResponse, staleRequest)
	if staleResponse.Code != http.StatusUnauthorized {
		t.Fatalf("stale authenticated request generated a code: status = %d, body = %s", staleResponse.Code, staleResponse.Body.String())
	}
	repairAfterRevoke := requestJSON(t, handler, http.MethodPost, "/api/v1/pairing/claim", `{"code":"`+staleInvite.Code+`","device_name":"Stolen again"}`)
	if repairAfterRevoke.Code != http.StatusUnauthorized || errorCode(t, repairAfterRevoke) != "invalid_pairing_code" {
		t.Fatalf("revoked device's code status = %d, body = %s", repairAfterRevoke.Code, repairAfterRevoke.Body.String())
	}
	secondAfterRevoke := authenticatedRequest(t, handler, http.MethodGet, "/api/v1/messages", "", second.Token)
	if secondAfterRevoke.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status = %d", secondAfterRevoke.Code)
	}
	firstStillWorks := authenticatedRequest(t, handler, http.MethodGet, "/api/v1/messages", "", first.Token)
	if firstStillWorks.Code != http.StatusOK {
		t.Fatalf("first device after revoke status = %d", firstStillWorks.Code)
	}
	selfRevoke := authenticatedRequest(t, handler, http.MethodDelete, "/api/v1/devices/"+first.Device.ID, "", first.Token)
	if selfRevoke.Code != http.StatusConflict || errorCode(t, selfRevoke) != "cannot_revoke_device" {
		t.Fatalf("self revoke status = %d, body = %s", selfRevoke.Code, selfRevoke.Body.String())
	}
}

func TestPairingClaimDatabaseFailureRollsBackCode(t *testing.T) {
	handler, _, pairing := newRawTestApp(t, false)
	code, err := pairing.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"code":"` + code.Code + `","device_name":"Retry"}`
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/pairing/claim", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request.WithContext(ctx))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("canceled claim status = %d, body = %s", response.Code, response.Body.String())
	}
	retry := requestJSON(t, handler, http.MethodPost, "/api/v1/pairing/claim", body)
	if retry.Code != http.StatusCreated {
		t.Fatalf("retry after failed insert status = %d, body = %s", retry.Code, retry.Body.String())
	}
}

func TestStaleDeviceContextCannotDeleteAnotherDevice(t *testing.T) {
	_, store, pairing := newRawTestApp(t, false)
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
	first := create("First")
	second := create("Second")
	create("Third")
	if err := store.DeleteDevice(t.Context(), second.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodDelete, "http://127.0.0.1/api/v1/devices/"+second.ID, nil)
	request.SetPathValue("device_id", second.ID)
	request = request.WithContext(context.WithValue(request.Context(), deviceContextKey{}, first))
	response := httptest.NewRecorder()
	(&server{store: store, pairing: pairing, logger: log.New(io.Discard, "", 0)}).deleteDevice(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("stale delete status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestStaleDeviceContextCannotCreateText(t *testing.T) {
	_, store, pairing := newRawTestApp(t, false)
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
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/messages/text", strings.NewReader(`{"text":"must not land"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), deviceContextKey{}, revoked))
	response := httptest.NewRecorder()
	(&server{store: store, pairing: pairing, logger: log.New(io.Discard, "", 0)}).createText(response, request)
	if response.Code != http.StatusUnauthorized || errorCode(t, response) != "unauthorized" {
		t.Fatalf("stale create text status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestEveryProtectedAPIRouteRejectsUnpairedRequests(t *testing.T) {
	handler, _, _ := newRawTestApp(t, false)
	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/session"},
		{http.MethodGet, "/api/v1/messages"},
		{http.MethodPost, "/api/v1/messages/text"},
		{http.MethodPost, "/api/v1/messages/file"},
		{http.MethodGet, "/api/v1/files/0123456789abcdef0123456789abcdef"},
		{http.MethodPost, "/api/v1/pairing/codes"},
		{http.MethodGet, "/api/v1/devices"},
		{http.MethodDelete, "/api/v1/devices/0123456789abcdef0123456789abcdef"},
		{http.MethodGet, "/api/v1/unknown"},
	} {
		t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
			response := requestJSON(t, handler, endpoint.method, endpoint.path, "")
			if response.Code != http.StatusUnauthorized || errorCode(t, response) != "unauthorized" {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
	for _, path := range []string{"/", "/app.js", "/healthz"} {
		response := requestJSON(t, handler, http.MethodGet, path, "")
		if response.Code != http.StatusOK {
			t.Fatalf("public path %s status = %d", path, response.Code)
		}
	}
}

func TestWebClientOnlyPersistsDeviceTokenAfterSuccessfulClaim(t *testing.T) {
	handler, _, _ := newRawTestApp(t, false)
	response := requestJSON(t, handler, http.MethodGet, "/app.js", "")
	if response.Code != http.StatusOK {
		t.Fatalf("app.js status = %d", response.Code)
	}
	if strings.Contains(response.Body.String(), `removeItem("ferry_device_token")`) {
		t.Fatal("a stale tab can delete a newer tab's origin-shared device token")
	}
	if count := strings.Count(response.Body.String(), "storeToken("); count != 2 {
		t.Fatalf("storeToken occurrences = %d, want definition plus successful claim only", count)
	}
	if !strings.Contains(response.Body.String(), "storageError = storeToken(payload.token)") {
		t.Fatal("successful pairing claim does not persist its newly issued token")
	}
}

func TestHTTPAuthRejectsDuplicateCredentialsAndMalformedPairingJSON(t *testing.T) {
	handler, _, pairing := newRawTestApp(t, false)
	code, err := pairing.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"code":"` + code.Code + `","device_name":"A","device_name":"B"}`,
		`{"code":"` + code.Code + `","device_name":"A","device\u005fname":"B"}`,
		`{"code":"` + code.Code + `","device_name":"A","role":"admin"}`,
	} {
		response := requestJSON(t, handler, http.MethodPost, "/api/v1/pairing/claim", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, response.Code)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/v1/messages", nil)
	request.Header.Add("Authorization", "Bearer first")
	request.Header.Add("Authorization", "Bearer second")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("duplicate authorization status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/v1/messages", nil)
	request.AddCookie(&http.Cookie{Name: "ferry_device", Value: "legacy-secret"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("legacy cookie authenticated without a Bearer token: status = %d", response.Code)
	}
}

func TestCrossOriginPairingWriteIsRejectedWithoutConsumingCode(t *testing.T) {
	handler, _, pairing := newRawTestApp(t, false)
	code, err := pairing.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"code":"` + code.Code + `","device_name":"Mac"}`
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/pairing/claim", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || errorCode(t, response) != "cross_origin_denied" {
		t.Fatalf("cross-origin status = %d, body = %s", response.Code, response.Body.String())
	}
	accepted := requestJSON(t, handler, http.MethodPost, "/api/v1/pairing/claim", body)
	if accepted.Code != http.StatusCreated {
		t.Fatalf("same-origin claim after rejection status = %d, body = %s", accepted.Code, accepted.Body.String())
	}
}

func TestLANHostBoundaryRequiresExplicitModeAndPrivateAddress(t *testing.T) {
	loopbackOnly, _, _ := newRawTestApp(t, false)
	lan, _, _ := newRawTestApp(t, true)
	for _, test := range []struct {
		name    string
		handler http.Handler
		host    string
		status  int
	}{
		{name: "private denied by default", handler: loopbackOnly, host: "192.168.1.20:8080", status: 421},
		{name: "private allowed in lan", handler: lan, host: "192.168.1.20:8080", status: 200},
		{name: "link local allowed in lan", handler: lan, host: "169.254.1.2:8080", status: 200},
		{name: "public denied in lan", handler: lan, host: "8.8.8.8:8080", status: 421},
		{name: "hostname denied in lan", handler: lan, host: "evil.example:8080", status: 421},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/healthz", nil)
			request.Host = test.host
			response := httptest.NewRecorder()
			test.handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
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
		assertJSONKeys(t, response.Body.Bytes(), "pairing claim", []string{"device", "token"})
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

func newRawTestApp(t *testing.T, allowLAN bool) (http.Handler, *Store, *PairingManager) {
	t.Helper()
	store := openTestStore(t)
	pairing := NewPairingManager()
	handler := NewHandler(store, HandlerOptions{Pairing: pairing, AllowLANHosts: allowLAN, Logger: log.New(io.Discard, "", 0)})
	return handler, store, pairing
}
