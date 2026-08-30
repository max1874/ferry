package webui

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIconAndReferencesItFromPage(t *testing.T) {
	handler := Handler()

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("page status = %d", page.Code)
	}
	pageHTML := page.Body.String()
	for _, reference := range []string{"/ferry-icon-64.png", "/ferry-icon-256.png", `rel="icon"`, `rel="apple-touch-icon"`} {
		if !strings.Contains(pageHTML, reference) {
			t.Errorf("page does not reference %q", reference)
		}
	}
	if count := strings.Count(pageHTML, `class="brand-mark" src="/ferry-icon-64.png"`); count != 1 {
		t.Errorf("brand icon count = %d", count)
	}
	if count := strings.Count(pageHTML, `class="welcome-mark" src="/ferry-icon-256.png"`); count != 2 {
		t.Errorf("welcome icon count = %d", count)
	}
	if strings.Contains(pageHTML, `class="brand-mark" aria-hidden="true">F<`) ||
		strings.Contains(pageHTML, `class="welcome-mark" aria-hidden="true">F<`) {
		t.Error("page still contains a letter icon placeholder")
	}

	for path, expectedSize := range map[string]int{"/ferry-icon-64.png": 64, "/ferry-icon-256.png": 256} {
		icon := httptest.NewRecorder()
		handler.ServeHTTP(icon, httptest.NewRequest(http.MethodGet, path, nil))
		if icon.Code != http.StatusOK {
			t.Errorf("%s status = %d", path, icon.Code)
			continue
		}
		if contentType := icon.Header().Get("Content-Type"); contentType != "image/png" {
			t.Errorf("%s Content-Type = %q", path, contentType)
		}
		if !bytes.HasPrefix(icon.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
			t.Errorf("%s response is not PNG data", path)
			continue
		}
		config, err := png.DecodeConfig(bytes.NewReader(icon.Body.Bytes()))
		if err != nil {
			t.Errorf("decode %s: %v", path, err)
		} else if config.Width != expectedSize || config.Height != expectedSize {
			t.Errorf("%s dimensions = %dx%d, want %dx%d", path, config.Width, config.Height, expectedSize, expectedSize)
		}
	}
}

func TestPageOffersSeparatePhotoAndFilePickers(t *testing.T) {
	handler := Handler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("page status = %d", page.Code)
	}

	html := page.Body.String()
	for _, contract := range []string{
		`id="attach"`,
		`aria-controls="attachment-menu"`,
		`id="choose-photos"`,
		`id="photo" accept="image/*,video/*"`,
		`id="choose-files"`,
		`id="file" hidden`,
	} {
		if !strings.Contains(html, contract) {
			t.Errorf("page does not contain attachment contract %q", contract)
		}
	}
	if strings.Contains(html, `id="file" accept=`) {
		t.Error("general file picker must remain unrestricted")
	}
}
