# Grok-inspired Web UI delivery

## S0 — Scope card

- **REQUESTED（Max）**：`ui 有点丑，开一个浏览器 1:1 复制 grok 的 ui`。
- **Done**：本地运行的 Ferry Web 在桌面与手机浏览器中采用 Grok 当前首页的空间结构、尺寸节奏、圆角、明暗配色与贴底输入体验，同时保留 Ferry 的消息、文件、设备和密码功能。
- **Non-goals**：不修改 iOS/Android；不改 Server/API/认证语义；不复制 Grok 商标、名称、文案或专有图形；不增加依赖、主题账户体系或新持久化状态。
- **INFERRED（不纳入）**：手动主题切换、侧边栏、搜索、Imagine、模型选择、登录/注册。
- **REPO_REQUIRED**：保留 Ferry 图标与设备图标；照片/文件分入口；错误状态可观察；真实浏览器桌面/手机 journey；相关 Go/Web 检查、diff check、对抗式自审和完整文件复核。
- **Depth**：full；这是一个横跨接入页、空时间线、已有消息、composer、附件菜单、设备设置与响应式布局的整体视觉重做，但不改变外部合同。
- **Budget**：最多 3 个生产文件、净新增不超过 450 行、0 个新持久机制；1 个过程文件、最多 180 行。
- **Execution budget**：45 分钟；最多 2 次无效浏览器验收尝试；外部动作仅公开 Grok UI 只读检查与最终 `origin/main` push。
- **Boundary plan**：真实本地 Ferry Server + browser-harness；桌面 1383×997 与手机 390×844 分别检查空状态、已有消息、附件菜单和设备弹窗。API 测试只作支持证据，不能替代页面验收。
- **Expansion triggers**：需要后端/API 变化、第四个生产文件、新依赖、Grok 专有资产、或超出预算时立即停止并回报。

Scope confirmed by Max's `go` on 2026-08-31.

## S1 — Design decision

**Observed**：Grok's current public Web page uses a borderless full-viewport canvas, icon-only top-left brand, lightweight top-right actions, a large centered brand, a roughly 760 px pill composer, near-black dark mode, and a bottom-pinned mobile composer. Ferry currently uses a bordered top bar, a card-like access panel, a narrow ChatGPT-style feed, and a permanently bottom-pinned composer.

Candidates:

1. **Selected — retain the DOM/API state machine and replace HTML presentation plus CSS**. Existing `hidden` state remains authoritative; the empty timeline moves the same composer to Grok's central position, while a populated timeline returns it to the bottom. No new state or dependency.
2. **Rejected — CSS-only with unchanged markup**. It cannot produce the same brand/composer hierarchy or accessible icon buttons without brittle generated content.
3. **Rejected — rebuild with a frontend framework**. It duplicates the working request/session/render state machine and exceeds the approved mechanism budget.

Counterexample: if an existing message arrives and the composer remains centered over the feed, the selected design is insufficient. Verification must observe the composer move from the empty-state center to the timeline bottom after a real send.

Governing gates: `go test ./...`, `go vet ./...`, `node --check internal/webui/assets/app.js`, `scripts/check-repo.sh`, `git diff --check`, and real browser desktop/mobile journeys.

## S2 — Frozen ship checklist

1. **REQUESTED** — desktop empty state matches the inspected Grok spatial system: icon-only corner brand, light controls, centered Ferry identity, wide pill composer; browser screenshot and computed rectangles.
2. **REQUESTED** — mobile empty and populated states remain usable at 390×844, with safe-area-aware top controls and bottom composer after the first message; real browser send journey and screenshots.
3. **DESIGN_NECESSARY** — text send, Photos/Files chooser entry points, visible sending/error status, file download card, device list, revoke and password settings retain their existing DOM/API contracts; Go asset tests, JS syntax and browser interactions.
4. **REPO_REQUIRED** — light/dark system themes, focus visibility, reduced motion, Ferry/device icons and responsive dialogs remain accessible; DOM inspection and browser screenshots in both color schemes.
5. **REPO_REQUIRED** — full changed-file review, author 1–13 self-review, repository gates and final-HEAD browser replay pass before default commit/push.

Frozen journeys:

- Desktop 1383×997, empty Server: page shows centered Ferry identity and composer; type `desktop journey`, submit; the message appears with sender metadata and composer is pinned at the bottom.
- Desktop populated Server: open attachment menu; both Photos and Files entries are visible; open Devices; current device and password setting are visible; close without mutation.
- Mobile 390×844: reload the populated timeline; top actions fit, message content does not clip horizontally, composer remains reachable above the bottom safe area.
- Dark mode: replay the populated mobile page with `prefers-color-scheme: dark`; background, composer, menu/dialog and text remain legible.

## S3 — Implementation and journey evidence

- Replaced only the Web presentation layer: `index.html` retains every JS-owned ID and adds accessible inline controls; `app.css` supplies the Grok-derived layout without dependencies or persistent state. The static-handler test now probes stable `<title>` plus composer identity instead of deleted display copy.
- Final-HEAD desktop empty, 1383×997: browser reported page width 1383, centered composer `{x:311.5,y:469.5,w:760,h:66}`, welcome `{y:320.5,h:94.5}`, and overflow 0. Evidence: `/private/tmp/ferry-grok-final-desktop-empty.png`.
- Final-HEAD desktop send replay: typed and submitted `final head journey`; rendered response body contained exactly that text, welcome became hidden, composer moved to `{y:893,bottom:959}`.
- Desktop side doors: clicking `+` exposed `Photos` and `Files`; Devices showed `Current device: Mac Web`, one `Mac Web` device, and `Access password`. Evidence: `/private/tmp/ferry-grok-desktop-menu.png` and `/private/tmp/ferry-grok-desktop-devices.png`.
- Final-HEAD mobile populated, 390×844: overflow 0, message `{x:16,w:358}`, composer `{x:8,y:755,w:374,bottom:815}`, device control `{x:338,w:40}`. Evidence: `/private/tmp/ferry-grok-final-mobile-light.png` and `/private/tmp/ferry-grok-final-mobile-dark.png`.
- Mobile empty/dark on a second fresh Server: welcome visible `{y:272,h:76.5}`, composer `{y:757,bottom:815}`, overflow 0. Dark computed colors were body `rgb(5,5,5)`, composer `rgb(17,17,17)`, text `rgb(242,242,242)`. Evidence: `/private/tmp/ferry-grok-mobile-empty-dark.png` and `/private/tmp/ferry-grok-mobile-dark.png`.
- Final trace: `ferry-grok-ui-final-head` (11 frames), after the compatibility repair, plus earlier empty/mobile traces under the browser-harness recording directory. Three invalid harness invocations occurred (stale current tab, wrong helper name, incorrect CDP argument form), exceeding the execution sub-budget by one; none exercised or invalidated product behavior.

## S4 — Builder attack record

Seven patterns:

1. **Two judges** — Go asset/server tests and a real Chromium journey both saw the same retained IDs/icons; `go test ./...` passed and browser send/menu/dialog content matched the DOM contracts.
2. **Extremes** — zero-message fresh Server and populated Server were both replayed; 390 px mobile and 1383 px desktop had zero horizontal overflow; existing `.message-text` uses `overflow-wrap:anywhere` and `.file-title` ellipsis.
3. **Equivalent spellings** — N/A: no parser, protocol, or normalization rule changed; this diff changes presentation copy and CSS only.
4. **Defaults** — system light and forced `prefers-color-scheme:dark` were both rendered; no manual theme state or omitted-field default was added.
5. **Side doors** — access form, empty timeline, populated timeline, attachment menu, Devices dialog and mobile bottom sheet were inspected; no JS-owned route or ID was removed.
6. **Policy gate** — icon/picker contracts are enforced by `internal/webui/web_test.go`; static shell and headers by `TestStaticWebAndSecurityHeadersShareHandler`; repo policy by `scripts/check-repo.sh`.
7. **No self-certification** — the browser used a real Go Server and read the actual rendered message/device/password content; API/unit green was not used as the UI witness. Fresh-context verdict is recorded separately below.

Author self-review, items 1–13:

1. Coupled state: no timer/flag/cache/state field changed; grep shows existing `welcome.hidden` producers at app.js lines 121/224 and `composerShell.hidden` at 160/177; CSS consumes only those authoritative flags.
2. Failure paths: no async/error path changed; existing visible `.footnote.error` and access error surfaces remain in full `app.js`; browser status remained readable.
3. Unchanged consumers: full `app.js` and `web_test.go` read; all queried IDs/classes remain, proven by `node --check` and `go test ./...`.
4. Contract surfaces: no API/schema/DB/env change; `git diff --name-only` contains only HTML, CSS, one static handler test, and this record.
5. Original reproduction: old bordered/narrow layout was replaced; desktop and mobile screenshots show the requested borderless canvas, central/bottom pill composer and minimal controls.
6. Current-HEAD journeys: no production edit occurred after the two recorded real-Server replays; actual text/menu/device/password content and computed rectangles are listed in S3.
7. Mechanism discrimination: empty center placement activates only while `.welcome:not([hidden])`; the same journey hid welcome and moved composer from y=469.5 to y=893, excluding a permanently centered fallback.
8. Regression scan: candidate was small-screen clipping; 390×844 probe returned `scrollWidth-innerWidth=0`, reachable 42 px controls, and a sheet entirely within the viewport.
9. Scale/edge: empty and non-empty states passed; long messages/files remain guarded by existing wrapping/ellipsis; maximum payload behavior is unchanged and covered by Go tests.
10. Contract attacks: all seven patterns above have concrete probes or scoped N/A.
11. Predicate producers: all `hidden` producers for welcome/composer/device/menu and dialog open/close were grepped and reconciled with selectors; no synthetic producer exists.
12. Reversed findings: initial Go failure identified a copy-pinned page probe and fresh review identified `color-mix()`/`:has()` compatibility gaps. The probe now uses title+composer; every color mix has a plain fallback; `:has()` count is zero; normal sibling selection owns centering; `TestCSSKeepsCompatibilityFallbacks` fossilizes both classes.
13. Pass limit: this author pass is only a pre-filter; the finite checklist is S2 and final green requires the fresh-context verdict below.

## S5 — Complete-file review and gates

- Author pass 2 re-read the adversarial checklist after the repair, complete current `index.html`, `app.css`, `server_test.go`, coupled `app.js` and `web_test.go`, plus the base diff. Delta is 243 additions/235 deletions across two production assets and two test files; no new runtime mechanism or dependency.
- Passed at repaired final tree: `GOCACHE=/private/tmp/ferry-go-cache go test ./...`, matching `go vet ./...`, `node --check internal/webui/assets/app.js`, `scripts/check-repo.sh`, and `git diff --check`. (`GOCACHE` avoided a sandbox-denied user cache, not a test failure.)
- Plain brief: this touches only Ferry Web presentation and its static-page test. It gives the empty and active chat the Grok-derived spatial hierarchy while keeping Ferry functions and brand. If wrong, users would see clipped mobile controls, a composer over messages, or missing file/device/password paths—the frozen journeys explicitly test those failures.

## S6 — Fresh-context verdict

- Round 1: **NOT SHIP**. Independent review found missing plain fallbacks for `color-mix()`, a `:has()` dependency for empty-state centering, and no permanent regression gate.
- Repair: added plain backgrounds/border before every enhancement, nested composer under the existing welcome/messages state so `.welcome:not([hidden]) ~ .composer-shell` needs no `:has()`, and added `TestCSSKeepsCompatibilityFallbacks`.
- Round 2: **SHIP**. Independent full-file re-review verified every fallback, the sibling-state mechanism, unchanged JS contracts and the new regression test; its `go test`, `go vet`, JS parse and diff checks passed independently. Residual non-blocker: composer/status now live inside `main[aria-live=polite]`, so future screen-reader testing should watch announcement cadence.

Status: `shipped-ready`; subject: working tree based on `24db715`; all S2 gates closed; author review round: 2; fresh review rounds: NOT SHIP → repair → SHIP; browser invalid invocations: 3 (one over execution sub-budget, disclosed above).
