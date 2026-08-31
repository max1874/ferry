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

## Post-deploy composer focus repair

User screenshot on 2026-08-31 exposed a rectangular outline inside the rounded composer. Browser reproduction measured textarea `outline: rgb(119,119,119) solid 2px`; the global `textarea:focus-visible` selector had higher specificity than the later base `textarea { outline:0 }` declaration. The repair removes textarea from the global rule, explicitly suppresses its own outline, and keeps focus feedback on the rounded composer border. `TestComposerFocusStaysOnRoundedContainer` fossilizes the exact regression.

Author review (pass 3, no subagent by user decision):

1. Coupled state — no JS/runtime state changed; only CSS focus selectors and theme variables; `git diff` is the producer list.
2. Failure paths — N/A: no I/O/async path; unsupported variables fall back only to the existing border color.
3. Unchanged consumers — button/input focus rings remain; textarea focus is represented by `.composer:focus-within`; CSS asset test passed.
4. Contract surfaces — no HTML/API/schema/env change; only CSS and its embedded-resource test.
5. Original reproduction — before: `solid 2px`; after: `outlineStyle=none`, active element still `#text`, rounded composer border `rgb(189,189,189)`.
6. Current-HEAD journey — fresh local Server, real browser focus at desktop and 390×844; both screenshots show no inner rectangle.
7. Mechanism discrimination — `#text` remains focused while only its outline disappears, excluding loss-of-focus as a false pass.
8. Regression scan — keyboard focus visibility could regress; computed composer border changes from base `#e4e4e4` to focus `#bdbdbd` and retains 32 px radius.
9. Scale/edge — desktop and mobile focused states both have overflow 0; text sizing/upload limits are untouched.
10. Contract attacks — dual judges are CSS test + rendered computed style; other protocol patterns are N/A because no protocol changed.
11. Predicate producers — `focus-visible` and `focus-within` occurrences were fully inspected in `app.css`; no alternate textarea outline producer remains.
12. Reversed finding — the prior generic accessibility rule was not universally safe; button/input rings remain while composer owns textarea focus.
13. Pass limit — finite gates are the named regression test, full repo gates, and real-browser reproduction; no claim of reviewer silence.

Evidence: `go test ./...`, `go vet ./...`, JS parse, repo policy and diff check passed; recording `ferry-composer-focus-fixed` (5 frames), screenshots `/private/tmp/ferry-focus-fixed.png` and `/private/tmp/ferry-focus-fixed-mobile.png`.

## Actual-chat redesign — S0 scope card

- **REQUESTED（Max）**：`你觉和 grok 不像呢`，随后要求 `go` 继续处理。
- **Done**：部署在 Mac mini 的 Ferry 已有消息页，第一眼呈现 Grok 实际聊天页的信息层级：右侧发送气泡、无头像内容流、弱化元信息、窄正文列与贴底 composer；文字、文件、设备和密码功能保持可用。
- **Non-goals**：不复制 Grok 商标、文案或专有资产；不伪造 AI 回复、模型选择、搜索、会话列表或账号功能；不修改 Server/API、认证、iOS、Android 或持久化数据。
- **INFERRED（不纳入）**：完整左侧会话栏、登录态 Grok 的隐藏功能、主题切换与消息编辑。
- **REPO_REQUIRED**：保留 Ferry/device icon、照片与文件入口、可观察错误；真实桌面/手机浏览器 journey；相关测试、diff check、完整文件复核与对抗式自审。
- **Depth**：full；已有消息、文件卡片、空状态、composer、设备面板与响应式布局需要作为一个页面系统重做，但无外部合同变化。
- **Budget**：最多 3 个生产文件、净新增不超过 350 行、0 个新持久机制；沿用本过程文件，总行数不超过 220 行。
- **Execution budget**：60 分钟；最多 2 次无效浏览器验收；外部动作限 Grok 匿名 UI 对照、`origin/main` push 和既有 Mac mini Ferry 容器部署。
- **Boundary plan**：真实本地 Ferry Server 上验证桌面空/有消息、文件卡、设备面板与 390×844 手机布局，再在 Mac mini 部署后的真实入口复验。匿名 Grok 登录墙后的内容不作臆测。
- **Expansion triggers**：需要 API/数据模型变化、第四个生产文件、新依赖、专有资产或超预算时立即停止。

Read-only evidence: Grok anonymous desktop and mobile chat state captured in `/private/tmp/grok-live-chat-result.png` and `/private/tmp/grok-live-chat-mobile.png`; recording `grok-actual-chat-study` (18 frames). The login wall blocked assistant-response inspection, so only directly observed user-bubble, content-card, typography, spacing and responsive behavior enter the design.

Scope confirmed by Max's `go` on 2026-08-31 after the card was displayed.

### Actual-chat redesign — S1 decision

**Observed old semantics**：`renderMessage` creates a 34 px device avatar, then sender/time above text or file; CSS lays every item out as a left-aligned two-column row. This is why the page still reads as a conventional messenger even after the homepage shell changed.

1. **Selected — reuse the current DOM/API state machine, change message markup classes and the presentation layer**. Every Ferry event remains a message from one of the user's devices, so its content becomes a right-aligned Grok-style user bubble with compact device/time provenance below it. Empty state, composer, attachment and settings keep their existing owners and IDs.
2. **Rejected — add a Grok-like sidebar and conversation history**. Ferry has one shared timeline, so a fake sidebar would add controls with no product meaning.
3. **Rejected — identify “my” messages by sender name/kind and render two sides**. The API intentionally snapshots name/kind but exposes no sender device ID; the heuristic can misclassify two identically named devices.

Counterexample: if the avatar column, bold sender heading or full-width left-aligned row remains after a real send, the selected design has not solved the reported mismatch. Governing gates remain Web asset tests, JS parse, `go test ./...`, `go vet ./...`, `scripts/check-repo.sh`, `git diff --check`, and real desktop/mobile browser journeys. Status: **Decided by the confirmed scope**; no new mechanism or external contract.

### Actual-chat redesign — S2 frozen checklist

1. **REQUESTED** — a real desktop send renders a right-aligned rounded message bubble, with no avatar column or bold sender header; browser DOM/style assertion and screenshot.
2. **REQUESTED** — file messages use the same compact bubble language while filename, size and download behavior remain; static contract plus real populated-page inspection.
3. **DESIGN_NECESSARY** — device kind/name/time remain visible as quiet provenance below each message, preventing the requested icon work from disappearing; DOM assertion for SVG plus metadata.
4. **REPO_REQUIRED** — empty state, composer focus, Photos/Files, device/password panel, light/dark and 390×844 layout remain usable; existing tests plus real browser journeys.
5. **REPO_REQUIRED** — full repository gates, author attack/self-review, complete-file fresh pass, final-HEAD Mac mini browser replay, then commit/push/deploy.

Frozen journeys: desktop populated timeline checks bubble geometry and metadata; a new text send moves through the real API into that shape; attachment menu and Devices dialog remain operable; 390×844 populated light/dark pages have no horizontal overflow and keep the composer reachable. Expected failure witness: restoring `.message { grid-template-columns: 34px ... }` or appending `.avatar` must fail the new asset regression.

### Actual-chat redesign — S3 build and journeys

- Replaced semantics: the old left avatar column plus bold sender heading became a right-aligned content bubble with device SVG/name/time provenance below it; no API, state field, dependency or persistent mechanism changed.
- Real local Server (`127.0.0.1:42831`): text `actual chat journey` rendered in a `{x:924.98,w:162.52,right:1087.5}` bubble, no `.avatar`, with Mac SVG metadata; composer remained `{w:760,bottom:959}` and horizontal overflow was 0.
- Real file input uploaded `ferry-ui-sample.txt`; rendered file title, `19 B`, `.message-body`, and retained the download button. Photos/Files menu, Devices dialog, current device and `No password is required.` were observed.
- 390×844 light/dark: bubble ended at x=374, composer `{x:8,w:374,bottom:815}`, overflow 0. A 360-character message wrapped to 307.88 px without overflow; focused textarea outline was `none`, rounded composer remained 26 px.
- Kill probe: stopping the real Server changed connection to `Offline`, status to visible `Failed to fetch`, and the indicator to neutral `rgb(112,112,112)`; restarting recovered `Local` and cleared the error. Recordings: `ferry-actual-chat-local` (21), `ferry-actual-chat-final-local` (10), `ferry-actual-chat-edge` (7).

Builder seven-pattern record:

1. Two judges — Web asset tests and rendered Chromium both rejected the avatar-column shape and observed bubble/device provenance.
2. Extremes — zero messages, text/file messages, 360 characters, desktop and 390 px mobile all rendered; overflow stayed 0.
3. Equivalent spellings — N/A: no parser, normalization or protocol changed.
4. Defaults — light/dark system defaults both rendered; no new stored theme or omitted-field behavior.
5. Side doors — empty/populated timeline, file input, attachment menu, Devices/password panel and Offline/recovery were walked.
6. Policy gate — `TestMessagesUseCompactUserBubbles` rejects restored avatar/grid layout; `TestStatusAndSettingsButtonsKeepTruthfulStyling` fossilizes the two self-found regressions.
7. No self-certification — real Go Server, real Chromium send/upload and actual DOM geometry are the acceptance witnesses; unit tests are supporting evidence.

### Actual-chat redesign — S4 author self-review

1. Coupled state — no state/timer/cache changed; `rg renderMessage|welcome.hidden|composerShell.hidden` enumerated the existing producers, and only rendered DOM order/classes changed.
2. Failure paths — no I/O branch changed; real Server kill/recovery preserved visible Offline/error and returned to Local.
3. Unchanged consumers — complete `app.js` read; `loadMessages` is the only `renderMessage` caller, and every JS-owned HTML ID remains; JS parse and Go tests passed.
4. Contract surfaces — `git diff -- api internal/ferry ios android cmd` was empty; no API/schema/DB/env/client contract changed.
5. Original reproduction — current desktop screenshot has a right bubble and no avatar/bold sender row; the exact old grid string is absent and test-forbidden.
6. Current-HEAD journey — CSS repair was followed by a fresh real-Server desktop/mobile/dialog replay; final local evidence is listed in S3.
7. Mechanism discrimination — `.message-body` geometry plus `avatar:false` witnesses the selected layout; restoring the old grid/avatar strings fails the regression test.
8. Regression scan — fixed two found regressions: false-green Offline status and transparent Save button; browser measured neutral Offline and an opaque bordered secondary button.
9. Scale/edge — zero state and a 360-character mobile message passed; file size limits and storage behavior are unchanged.
10. Contract attacks — the seven-pattern record above scopes parser/protocol attacks N/A and gives concrete UI probes for the rest.
11. Predicate producers — `rg connectionElement.textContent` enumerated Connect/Local/Offline/password producers; the indicator now inherits each status text color rather than asserting online.
12. Reversed findings — the old shared device/secondary selector question was replayed against both consumers; `.secondary` now restores raised background/border while `.device-button` remains minimal.
13. Pass limit — this is an author pre-filter, not an independent review; closure remains the finite S2 checklist plus complete-file fresh pass and deployed journey.

### Actual-chat redesign — S5/S6 review status

- **S5 author fresh pass** (not independent): complete final `index.html`, `app.css`, `app.js`, `web_test.go`, old implementation and this design were reread. Two requested-path regressions found in S4 were fixed and fossilized; the repaired full files and real journeys have no remaining author-known P0/P1. Repository gates passed: `go test ./...`, `go vet ./...`, JS parse, `scripts/check-repo.sh`, and `git diff --check`.
- **S6 N/A** — no API/protocol/schema/auth/data-integrity contract changed.
- Status before deployment: `local_candidate`; base `24bc4a9`; review provenance `author fresh pass`; semantic invalidations 0; planned/current production scope `3 files / -1 net line / 0 persistent mechanisms`.

### Actual-chat redesign — S7 deployed closure

- Subject `3b59eee` was pushed to `origin/main`, archived to the existing Mac mini deployment directory, built as `ferry:local` image `sha256:5394083373c3512b0b732c3e1884ee3831117630fe0860b3aa58f5b8e47e7200`, and restarted without removing the named data volume. `http://10.0.0.2:42817/` returned 200.
- Deployed desktop browser loaded 5 retained historical messages: all had right-aligned `.message-body`, 0 `.avatar`, and 5 device SVG provenance rows; the existing Android file rendered in a `{right:1087.5,w:491.63}` bubble, composer was `{w:760,bottom:959}`, connection `Local`, overflow 0.
- Deployed Devices showed 8 retained devices, `Current device: Mac browser`, and `No password is required.` Mobile 390×844 light/dark rendered the file bubble at `{x:66.13,w:307.88,right:374}`, composer `{x:8,w:374,bottom:815}`, overflow 0. Recording: `ferry-actual-chat-macmini` (10 frames).
- Checklist closure: S2 items 1–4 have final-subject browser evidence plus permanent asset tests; item 5 has full gates, author fresh-pass provenance, push and live deployment. No contract-class S6 gate applies. Journey script: deleted, transcript above.
- Final status: `shipped`; exact production subject `3b59eee`; review rounds 1 author fresh pass; semantic invalidations 0; planned/current production scope `3 files / -1 net line / 0 persistent mechanisms`.
