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

## Superseded anonymous-reference delivery

The first delivery, its compatibility repair and the composer-focus repair remain in commits `3b59eee` and `24bc4a9`; their detailed evidence was compacted here after authenticated Grok evidence invalidated the page structure. The durable regressions remain enforced by `TestCSSKeepsCompatibilityFallbacks` and `TestComposerFocusStaysOnRoundedContainer`.

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

## Authenticated-reference correction

Max identified that anonymous/login-wall evidence was insufficient and completed the Grok login handoff. Recording `grok-authenticated-study` (21 frames) replaces the earlier visual assumptions: desktop sidebar/main are 257/1126 px; empty composer is `{x:444,y:413.5,w:752,h:60,radius:160}`; promo row is `{x:452,y:505.5,w:736,h:66,radius:16}`; chat content is 704 px; a user bubble is `{w:633.59,h:63,radius:24px 24px 8px}`; body background is `#050505`.

**S1 reopened — semantic invalidation 1.** The selected “no sidebar + two-row composer” design contradicts authenticated Grok and is removed. Replacement: a 257 px Ferry sidebar containing only real Ferry destinations (Timeline, Devices and current-server identity), a 704 px message column, the existing one-row 752×60 composer, and a meaningful empty-state server row. No fake search/history/model/AI functions.

**S2 replacement checks**：desktop at 1383×997 must measure sidebar 257, composer 752×60 and message column 704; existing messages retain device provenance and file download; mobile uses a compact rail without overflow; attachment, Devices/password, focus, Offline/recovery and light/dark checks remain. Production budget remains 3 files / ≤350 net lines / 0 persistent mechanisms; user authorization is the original 1:1 request plus the completed authenticated-browser handoff.

### Authenticated correction — S3/S4 evidence

- Real empty Server at 1383×997 measured sidebar/main `257/1126`, welcome y `321.5`, composer `{x:444,y:413.5,w:752,h:60}`, info row `{x:452,y:505.5,w:736,h:66}`, focus outline `none`, and overflow 0. Screenshot: `/private/tmp/ferry-authenticated-grok-empty-final.png`.
- Real populated Server measured message column `{x:468,w:704}`, right bubbles with device provenance and no avatar; Photos/Files and Devices/password side doors opened. Mobile 390×844 measured rail/main `56/334`, composer `{x:64,y:755,w:318,h:60}`, overflow 0.
- Forced light/dark produced readable white/`#050505` canvases. Kill probe changed Local to visible Offline/`Failed to fetch`; restarting recovered Local and cleared the error.
- Builder attack found viewport-state residue: mobile textarea height survived a desktop resize and made the composer 62 px. `resizeComposer` now runs on resize; the permanent test plus a mobile→desktop browser replay hold both states at 60 px.

Author adversarial review, pass 4 (author fresh pass, not independent):

1. Coupled state — new `sidebar.hidden` has only `showAccess/showApp` producers; the resize listener calls the existing idempotent height derivation; `rg` enumerated all producers.
2. Failure paths — no fetch branch changed; real kill/recovery exposed Offline/error and returned to Local.
3. Unchanged consumers — complete `index.html`, `app.css`, `app.js` and `web_test.go` reread; all JS-owned IDs and callers remain; JS parse and Go tests pass.
4. Contract surfaces — no API/schema/DB/env/client contract changed; presentation and static tests only.
5. Original reproduction — authenticated-reference rectangles now match exactly; the rectangular focus outline remains absent.
6. Current-HEAD journey — empty, populated, menu/dialog, light/dark and mobile→desktop resize were replayed after the final JS repair.
7. Mechanism discrimination — mobile first measured textarea 42/composer 60; desktop resize then measured textarea 40/composer 60, proving recalculation rather than min-height fallback.
8. Regression scan — author found and fixed the 62 px resize regression; 390 px and 1383 px overflow remained 0.
9. Scale/edge — zero and populated timelines passed; existing long-text wrapping/file ellipsis and payload behavior are unchanged.
10. Contract attacks — dual judges are embedded-resource tests plus real Chromium; parser/normalization/policy attacks are N/A because those boundaries did not change.
11. Predicate producers — `sidebar.hidden`, `welcome.hidden`, `composerShell.hidden`, connection text and status error producers were grepped before trusting CSS/visual state.
12. Reversed findings — prior “no sidebar” and two-row conclusions were invalidated, but their fake-function, responsive and truthful-status questions were rechecked against the replacement.
13. Pass limit — this is explicitly an author pre-filter; the finite gate is the replacement S2 checklist, complete-file read, machine gates and deployed browser replay.

### Authenticated correction — S5/S6 status

- **SHIP candidate, author fresh pass**: no known P0/P1/P2 after one repair round. `go test ./...`, `go vet ./...`, JS parse, repo policy and diff checks are required again at the final tree. No contract-class S6 gate applies.
