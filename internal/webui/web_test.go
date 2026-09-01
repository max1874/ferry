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

func TestMessagesUseCompactUserBubbles(t *testing.T) {
	handler := Handler()

	app := httptest.NewRecorder()
	handler.ServeHTTP(app, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if app.Code != http.StatusOK {
		t.Fatalf("app status = %d", app.Code)
	}
	javascript := app.Body.String()
	for _, contract := range []string{
		`body.className = "message-body"`,
		`sourceIcon.className = "message-source-icon"`,
		`article.classList.toggle("is-current-device", message.is_current_device === true)`,
		`article.append(head, body)`,
	} {
		if !strings.Contains(javascript, contract) {
			t.Errorf("app.js does not contain bubble contract %q", contract)
		}
	}
	if strings.Contains(javascript, `avatar.className = "avatar"`) {
		t.Error("message renderer still creates the old avatar column")
	}

	stylesheet := httptest.NewRecorder()
	handler.ServeHTTP(stylesheet, httptest.NewRequest(http.MethodGet, "/app.css", nil))
	if stylesheet.Code != http.StatusOK {
		t.Fatalf("stylesheet status = %d", stylesheet.Code)
	}
	css := stylesheet.Body.String()
	for _, contract := range []string{
		`.message { display: flex; flex-direction: column; align-items: flex-start; }`,
		`.message.is-current-device { align-items: flex-end; }`,
		`.message-head { display: flex; align-items: center; gap: 5px; margin: 0 9px 6px;`,
		`.message-body { max-width: 90%;`,
		`.message.is-current-device .message-body { border-radius: 24px 24px 8px; }`,
		`.message-source-icon .device-icon { width: 13px;`,
	} {
		if !strings.Contains(css, contract) {
			t.Errorf("stylesheet does not contain bubble contract %q", contract)
		}
	}
	if strings.Contains(css, `grid-template-columns: 34px minmax(0, 1fr)`) {
		t.Error("stylesheet still contains the old avatar-column layout")
	}
}

func TestAuthenticatedGrokGeometryOwnsThePageShell(t *testing.T) {
	handler := Handler()
	stylesheet := httptest.NewRecorder()
	handler.ServeHTTP(stylesheet, httptest.NewRequest(http.MethodGet, "/app.css", nil))
	if stylesheet.Code != http.StatusOK {
		t.Fatalf("stylesheet status = %d", stylesheet.Code)
	}

	css := stylesheet.Body.String()
	for _, contract := range []string{
		`.sidebar {`,
		`width: 257px;`,
		`.conversation { width: calc(100% - 257px);`,
		`.messages { width: min(704px, 100%);`,
		`.composer { width: min(752px, 100%); min-height: 60px;`,
		`border-radius: 999px;`,
		`--base: #050505;`,
	} {
		if !strings.Contains(css, contract) {
			t.Errorf("stylesheet does not contain authenticated Grok geometry %q", contract)
		}
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	html := page.Body.String()
	for _, contract := range []string{`id="sidebar"`, `>Timeline<`, `>Devices<`, `>Ferry Server<`} {
		if !strings.Contains(html, contract) {
			t.Errorf("page does not contain real Ferry sidebar contract %q", contract)
		}
	}
	row := strings.Index(html, `<div class="composer-row">`)
	text := strings.Index(html, `<textarea id="text"`)
	if row < 0 || text < row {
		t.Error("composer is not the authenticated Grok-style single row")
	}
}

func TestStatusAndSettingsButtonsKeepTruthfulStyling(t *testing.T) {
	handler := Handler()
	stylesheet := httptest.NewRecorder()
	handler.ServeHTTP(stylesheet, httptest.NewRequest(http.MethodGet, "/app.css", nil))
	if stylesheet.Code != http.StatusOK {
		t.Fatalf("stylesheet status = %d", stylesheet.Code)
	}

	css := stylesheet.Body.String()
	for _, contract := range []string{
		`.connection::before { width: 6px; height: 6px; content: ""; border-radius: 50%; background: currentColor; }`,
		`.secondary {`,
		`background: var(--raised);`,
		`border: 1px solid var(--border);`,
	} {
		if !strings.Contains(css, contract) {
			t.Errorf("stylesheet does not contain truthful control contract %q", contract)
		}
	}
	if strings.Contains(css, `.connection::before`) && strings.Contains(css, `background: #41a65a`) {
		t.Error("connection indicator must not stay green while its text can say Offline")
	}
	if strings.Contains(css, `.device-button, .secondary`) {
		t.Error("settings button must not share the transparent sidebar-device styling")
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

func TestComposerAcceptsPastedImagesWithoutInterceptingText(t *testing.T) {
	handler := Handler()
	app := httptest.NewRecorder()
	handler.ServeHTTP(app, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if app.Code != http.StatusOK {
		t.Fatalf("app status = %d", app.Code)
	}

	javascript := app.Body.String()
	for _, contract := range []string{
		`let pastedAttachment = null`,
		`return pastedAttachment || photoInput.files[0] || fileInput.files[0] || null`,
		"function clearAttachment() {\n  pastedAttachment = null;",
		`textInput.addEventListener("paste", (event) => {`,
		`const image = pastedImage(event)`,
		`if (!image) return`,
		`event.preventDefault()`,
		`pastedAttachment = image`,
	} {
		if !strings.Contains(javascript, contract) {
			t.Errorf("app.js does not contain pasted-image contract %q", contract)
		}
	}
	pasteStart := strings.Index(javascript, `textInput.addEventListener("paste", (event) => {`)
	if pasteStart < 0 {
		t.Fatal("paste handler start is missing")
	}
	pasteEnd := strings.Index(javascript[pasteStart:], `textInput.addEventListener("keydown"`)
	if pasteEnd < 0 {
		t.Fatal("paste handler end is missing")
	}
	pasteHandler := javascript[pasteStart : pasteStart+pasteEnd]
	ignored := strings.Index(pasteHandler, `if (!image) return;`)
	intercepted := strings.Index(pasteHandler, `event.preventDefault();`)
	if ignored < 0 || intercepted < 0 || ignored > intercepted {
		t.Error("non-image clipboard data is intercepted before it can use normal text paste")
	}
}

func TestComposerUsesCompactImageAndFileAttachmentChips(t *testing.T) {
	handler := Handler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, contract := range []string{`id="file-preview"`, `class="file-kind-icon"`, `id="remove-file"`} {
		if !strings.Contains(page.Body.String(), contract) {
			t.Errorf("page does not contain attachment-chip contract %q", contract)
		}
	}

	app := httptest.NewRecorder()
	handler.ServeHTTP(app, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	for _, contract := range []string{`URL.revokeObjectURL(previewURL)`, `URL.createObjectURL(selected)`, `fileChip.classList.toggle("is-image", isImage)`} {
		if !strings.Contains(app.Body.String(), contract) {
			t.Errorf("app.js does not contain attachment-preview contract %q", contract)
		}
	}

	stylesheet := httptest.NewRecorder()
	handler.ServeHTTP(stylesheet, httptest.NewRequest(http.MethodGet, "/app.css", nil))
	for _, contract := range []string{`.file-chip { position: relative; width: fit-content;`, `.file-preview { display: none; width: 34px; height: 34px;`, `.file-chip.is-image { width: 40px; padding: 2px;`, `.file-chip.is-image:hover button`} {
		if !strings.Contains(stylesheet.Body.String(), contract) {
			t.Errorf("stylesheet does not contain attachment-chip contract %q", contract)
		}
	}
}

func TestCSSKeepsCompatibilityFallbacks(t *testing.T) {
	handler := Handler()
	stylesheet := httptest.NewRecorder()
	handler.ServeHTTP(stylesheet, httptest.NewRequest(http.MethodGet, "/app.css", nil))
	if stylesheet.Code != http.StatusOK {
		t.Fatalf("stylesheet status = %d", stylesheet.Code)
	}

	css := stylesheet.Body.String()
	if strings.Contains(css, ":has(") {
		t.Error("empty-timeline layout must not depend on :has() support")
	}
	for _, contract := range []string{
		`.welcome:not([hidden]) ~ .composer-shell`,
		`background: var(--raised); background: color-mix`,
		`border: 1px solid #dcaaa6; border-color: color-mix`,
	} {
		if !strings.Contains(css, contract) {
			t.Errorf("stylesheet does not contain compatibility contract %q", contract)
		}
	}
}

func TestComposerFocusStaysOnRoundedContainer(t *testing.T) {
	handler := Handler()
	stylesheet := httptest.NewRecorder()
	handler.ServeHTTP(stylesheet, httptest.NewRequest(http.MethodGet, "/app.css", nil))
	if stylesheet.Code != http.StatusOK {
		t.Fatalf("stylesheet status = %d", stylesheet.Code)
	}

	css := stylesheet.Body.String()
	if strings.Contains(css, `button:focus-visible, input:focus-visible, textarea:focus-visible`) {
		t.Error("textarea inherited the rectangular global focus outline")
	}
	for _, contract := range []string{
		`.composer:focus-within { border-color: var(--focus-border); }`,
		`textarea:focus-visible { outline: none; }`,
	} {
		if !strings.Contains(css, contract) {
			t.Errorf("stylesheet does not contain composer focus contract %q", contract)
		}
	}
}

func TestComposerRecomputesTextHeightAfterViewportChanges(t *testing.T) {
	handler := Handler()
	app := httptest.NewRecorder()
	handler.ServeHTTP(app, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if app.Code != http.StatusOK {
		t.Fatalf("app status = %d", app.Code)
	}
	if !strings.Contains(app.Body.String(), `window.addEventListener("resize", resizeComposer)`) {
		t.Error("composer does not recompute textarea height after a viewport change")
	}
}

func TestWebUsesLicensedDeviceIconsInsteadOfInitials(t *testing.T) {
	handler := Handler()
	app := httptest.NewRecorder()
	handler.ServeHTTP(app, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if app.Code != http.StatusOK {
		t.Fatalf("app status = %d", app.Code)
	}
	javascript := app.Body.String()
	for _, contract := range []string{
		`X-Ferry-Device-Kind`,
		`message.sender_kind`,
		`createDeviceIcon(device.kind)`,
		`iphone:`, `ipad:`, `mac:`, `android:`, `windows:`, `browser:`,
	} {
		if !strings.Contains(javascript, contract) {
			t.Errorf("app.js does not contain device icon contract %q", contract)
		}
	}
	if strings.Contains(javascript, `sender_name.slice(0, 1)`) {
		t.Error("message avatar still renders the sender initial")
	}

	license := httptest.NewRecorder()
	handler.ServeHTTP(license, httptest.NewRequest(http.MethodGet, "/tabler-icons-LICENSE.txt", nil))
	if license.Code != http.StatusOK || !strings.Contains(license.Body.String(), "Copyright (c) 2020-2026 Paweł Kuna") || !strings.Contains(license.Body.String(), "MIT License") {
		t.Fatalf("Tabler license status = %d, body = %q", license.Code, license.Body.String())
	}
}
