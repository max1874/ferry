# Web attachment picker

## Scope card

- **REQUESTED — Max, 2026-08-30**: Web 的 `+` 不能默认只进入文件选择；手机用户需要清楚、直接地从相册选择媒体。
- **Done**: 点击 `+` 显示“Photos”和“Files”两个入口；Photos 只请求图片/视频，Files 保留任意文件；两者选中后复用现有附件预览与发送流程；菜单支持点击外部和 Escape 关闭。
- **Non-goals**: 不主动启动相机、不增加多选、不改 Server API、不修改 iOS 原生附件入口。
- **Depth**: focused。
- **Budget**: `index.html`、`app.css`、`app.js` 三个 production 文件和直接相关测试；无依赖、无持久化、无协议变更。
- **Boundary proof**: Web handler contract test，加真实移动 viewport 浏览器旅程，分别证明 Photos 与 Files 路由到不同 input。

## Evidence and decision

- **Observed**: 原实现的 `+` 是一个直接包裹无 `accept` 属性文件 input 的 label，因此只有通用文件选择语义。
- **Decided**: `+` 先打开 Ferry 自己的二选一菜单；Photos 使用 `accept="image/*,video/*"`，Files 不设置 `accept`。
- **Decided**: 不设置 `capture`；用户要求的是相册入口，不能把它强制变成相机入口。

## Ship checklist

1. `+` 显示 Photos / Files，ARIA 展开状态同步。
2. Photos 同步触发媒体 input，Files 同步触发通用 input。
3. 两种选择只保留一个当前附件，并复用原发送与清除流程。
4. 点击外部、Escape、进入 access 状态都会关闭菜单。
5. Go tests、race、vet、JavaScript syntax、Docker build 与 `git diff --check` 通过。
6. 当前 HEAD 和 macmini 部署均通过真实浏览器旅程。
7. 推送前完成 13 项对抗式自审与一次独立 focused review。

## Verification

- Full Go suite, race suite, vet, JavaScript syntax and `git diff --check` passed on the current worktree.
- Final current-worktree Docker image build passed as `sha256:ca7b08…` after the accessibility fix.
- Mobile browser journey at 390×844: Photos has `accept=image/*,video/*`; Files has no restriction; coordinate clicks produced counters `{photo: 1, file: 1}` with no cross-trigger; a real PNG selected through Photos rendered the attachment chip, sent successfully, then cleared both inputs; Escape and outside click closed the picker. Recording: `/Users/max/.config/browser-harness/agent-workspace/recordings/ferry-web-attachment-picker-head3` (11 frames, repository-external).

## Adversarial self-review

Conclusion: **ship candidate after independent review and deployment proof**. Pass 1 found and fixed an ARIA menu-pattern mismatch and a stale-open-menu path in `showAccess(clearCredential=false)`; pass 2 found no new production finding.

1. Coupled state — `attachmentMenu.hidden`, `aria-expanded`, two input values and composer derived state have one mutator/helper or explicit change listeners; `rg -n 'selectedAttachment|clearAttachment|setAttachmentMenuOpen|photoInput|fileInput|attachmentMenu' internal/webui/assets/app.js` enumerated every reader/writer.
2. Failure paths — picker cancellation emits no change and preserves the prior selection; empty inputs yield `null`; send failure preserves the selected input through the existing catch path; current-head browser proved successful cleanup.
3. Unchanged code — the same `rg` found all former `fileInput` consumers at access reset, composer derivation, submit and remove; each now uses the shared attachment helper where required.
4. Contract surfaces — `git diff --stat` is limited to embedded Web HTML/CSS/JS, one Web test and this document; Server route, schema, database and environment are unchanged.
5. Original reproduction — current-head mobile browser no longer opens a picker directly from `+`; it visibly opens Photos / Files first (`menu.hidden=false`, focus `choose-photos`).
6. Current-HEAD journey — recording `ferry-web-attachment-picker-head3` read actual selected filename, post-send message title and normal final status, not HTTP status alone.
7. Mechanism discrimination — in one rendered page, overridden native input click methods observed Photos `{photo:1,file:0}` then Files `{photo:1,file:1}`; the unrestricted file input could not satisfy the Photos-only witness.
8. Regression scan — candidate regression was losing arbitrary-file send; Files retained empty `accept`, route counter incremented only `file`, and the existing full Go/race suite passed.
9. Scale/edge — zero selection remains normal text mode; a single selection is enforced by inputs without `multiple`; upload size and server isolation were not changed. Evidence: browser selected one photo with the other input at count zero.
10. Contract-level attacks — N/A: no network, persistence, authorization or cross-client contract changed.
11. Predicate producers — all producers of menu hidden/expanded and both file counts are enumerated by the item 1 grep; initial HTML state is hidden/false and every runtime menu transition uses `setAttachmentMenuOpen`.
12. Reversed findings — removing ARIA `menu/menuitem` retained normal button accessibility; moving close outside the credential-clear branch covers every `showAccess` caller listed by `rg -n 'showAccess\\(' internal/webui/assets/app.js`.
13. Pass limit — author review does not self-certify; independent focused review and macmini current-deployment replay remain explicit checklist gates.

## Independent focused review

- Pass 1: **not ship**, one P2. Closing the choice group hid the focused choice and left `document.activeElement` on `body`, so keyboard and assistive-technology users could lose their place after cancelling the system picker.
- Resolution: each Photos / Files handler now restores focus to the visible `+` button before synchronously triggering its native input. Current-head browser replay observed `active=attach` after both branches while preserving `{photo:1,file:1}` routing.
- Pass 2: **SHIP**, no remaining P0/P1/P2. Independent recording: `/Users/max/.config/browser-harness/agent-workspace/recordings/ferry-attachment-independent-rereview` (repository-external).

## Deployment

- Source commit `30372f0` was pushed to `origin/main`.
- macmini rebuilt and runs image `sha256:b83c6c…` at `http://10.0.0.2:42817`; Compose recreated the container/network without removing the named data volume, and the live timeline retained its historical text and file messages.
- Direct LAN checks returned `{"status":"ok"}` and served HTML containing the Photos button and `accept="image/*,video/*"` contract.
- Live 390×844 browser replay observed two inline SVG icons, Photos-only then Files-only routing `{photo:1,file:1}`, focus restored to `attach` after both choices, and no cross-trigger. Recording: `/Users/max/.config/browser-harness/agent-workspace/recordings/ferry-web-attachment-picker-macmini` (6 frames, repository-external).
- Known verification boundary: mobile Chromium emulation proves Ferry's responsive UI, DOM contract and routing; the actual iOS system photo-library sheet still requires the user's physical-iPhone tap test.

## Clipboard image paste — 2026-09-01

- Requested: “输入框不支持直接粘贴图片？” Done is image clipboard data becoming the existing single attachment chip and using the existing file-send path; ordinary pasted text remains text. Native apps, multi-image paste, and a new upload mechanism are excluded.
- Focused design: one nullable in-memory `pastedAttachment` joins the two existing picker inputs at `selectedAttachment`; every existing clear/success path clears it, and choosing a picker file replaces it. Only an actual `image/*` file prevents the browser's default paste.
- Frozen checks: native Cmd-V image shows `image.png`, disables text mode and enables Send; sending produces a file message and clears the chip; native Cmd-V text inserts exact text without a chip; existing Photos/Files replacement and remove behavior remain.
- Local final-tree Chromium used the real system clipboard and native Paste command: a PNG produced `{chip:true,name:image.png,textDisabled:true,sendDisabled:false}`, sent as a visible `image.png` file message, then plain text produced `{chip:false,text:"plain clipboard text"}`. Recording: `ferry-paste-image-local` (9 frames).
- Author adversarial pass: all attachment producers were enumerated; cancellation preserves the prior attachment, picker selection replaces paste, remove/success clear it, non-image paste returns before `preventDefault`, first image only matches Ferry's existing single-file contract, and Server limits/errors remain shared. No protocol/schema/auth change; no subagent per Max's standing decision.
- Shipped: source `36527dd`; full Go tests/vet, JS syntax, repository policy and diff checks passed. Mac mini runs image `sha256:5e5118848901e902216929c47ae974889988cc458235deda2adb9c51d27c5ef9` with retained `ferry_ferry-data:/data`; `/healthz` returned 200. Deployed Chromium accepted `deployed-paste.png` into the chip, enabled Send, and remove restored empty text mode without posting a test message. Recording: `ferry-paste-image-macmini` (5 frames).

## Grok-style attachment presentation — 2026-09-01

- Requested after screenshot feedback: inspect real Grok image/file attachment states and replace Ferry's full-width filename bar. Scope is Web attachment presentation and the narrow image-preview CSP source; upload/API/native clients are unchanged.
- Authenticated Grok observation, without sending: its image chip is 40×40 with a 34×34 object-cover preview, 9 px inner radius and hover remove control; its file chip is a content-width 40 px pill with a 20 px document icon, truncated name and 24 px remove button. Both sit above the input row and make the 752 px composer 112 px high. Recording: `grok-attachment-reference` (6 frames).
- Decision: Ferry uses the same geometry and type-specific presentation, with an always discoverable scaled remove control on touch. Object URLs are revoked whenever attachment identity changes; large images are not copied into base64.
- Root cause/contract: the first real render exposed CSP blocking `blob:` in `img-src`. The policy now permits `blob:` for images only while script/style/connect/object/base/frame/form directives remain byte-for-byte pinned by `TestStaticWebAndSecurityHeadersShareHandler`.
- Local final-tree journey: image `{chip:40×40,preview:34×34,natural:1794×364,composer:752×111}`; ordinary file `{chip:248.45×40,remove:24×24,composer:752×111}`; 390 px file view stayed inside composer x=64…382 with overflow 0. Record: `ferry-grok-attachment-style-local` (8 frames).
- Frozen closure: image and file selection, pasted image, remove, successful send cleanup, desktop/mobile geometry, CSP exact test, full repository gates, push and Mac mini replay. Author review only and no subagent per Max's standing decision.
- Shipped: source `c186afc`; Mac mini image `sha256:ae5f4d37183971f9a7fd93bc1c48d86d88e5c27a0d85a28ca5556b913aae416b` runs with retained `ferry_ferry-data:/data`, exact CSP and `/healthz` 200. Live image measured `40×40`/preview `34×34` and live file `248.45×40`/remove `24×24`; both made the 752 px composer 111 px high with overflow 0, then were removed without sending. Recording: `ferry-grok-attachment-style-macmini` (6 frames).

## Readable image preview correction — 2026-09-01

- Requested after real-use feedback: attachment work must be exercised visually; the 40×40 square must not turn a wide screenshot into an unreadable white speck.
- Root cause: the Grok-derived fixed square uses `object-fit: cover`, so extreme aspect ratios discard nearly all useful pixels. The same 1658×350 screenshot was pasted into authenticated Grok and the deployed Ferry before changing the rule.
- Decision: image previews preserve their full aspect ratio and fit within 220×96; the image chip sizes to the actual preview, while ordinary file chips and the upload/API path remain unchanged.
- Failure behavior: if a file claims an image media type but cannot render, the pending attachment falls back to the ordinary filename chip instead of collapsing into an empty image box; changing the attachment resets that fallback.
- Frozen journey: paste the reported screenshot, inspect preview geometry/content, remove it, paste again, send it, inspect the resulting timeline item, and repeat the pending state at 390×844 without overflow.

### Final-tree evidence and adversarial review

- Authenticated Grok reference: the exact reported PNG was uploaded and rendered in Grok's 40×40 attachment slot. Recording: `ferry-attachment-grok-reference` (3 frames).
- Original Ferry reproduction: the exact PNG rendered as a 34×34 square inside a 40×40 chip; the 752 px composer became 111 px high. Recording: `ferry-attachment-bug-repro` (3 frames).
- Current candidate: the 1730×382 browser-decoded image renders at 220×48.57, its chip is 228×56.57, the close control is visible, and the 752 px composer is 127.57 px high. At 390×844 the same chip stays within the 318 px composer with overflow 0. Delete restores empty text mode; a second paste sends successfully and produces the expected 50.5 KB timeline item. Recording: `ferry-readable-image-preview-final-subject` (8 frames).

1. Coupled state — `previewAttachment`, `previewURL` and `previewFailed` are all owned by `updateComposer`; `rg` found no other producer except the image error listener, which compares `currentSrc` with the current object URL so a stale error from a replaced image cannot poison the new preview.
2. Failure paths — a deliberately corrupt `.png` fell back to `file-chip` with its filename visible and Send enabled; remove then cleared it. Upload/send failures still retain the selected attachment through the unchanged catch path.
3. Unchanged consumers — `selectedAttachment`, access reset, successful submit, picker change and remove callers were reread; CSS does not alter their state transitions.
4. Contract surfaces — `git diff --stat` changes only embedded Web CSS/JS, its static test and this document; API, database, CSP, auth and native clients are unchanged.
5. Original reproduction — the same user-provided wide PNG was replayed before and after the change; the unreadable square became a full-aspect 220 px preview.
6. Current-HEAD journey — `ferry-readable-image-preview-final-subject` ran after the final stale-error guard and observed corrupt fallback, rendered geometry, delete, repaste, send completion and the actual timeline title/size/status.
7. Mechanism discrimination — the rendered width changed from fixed 34 to 220 while natural aspect `1730/382` matched rendered aspect `220/48.57`; a corrupt image took the non-image fallback instead of satisfying the preview check.
8. Regression scan — an ordinary `README.md` selection remained a 154.02×40 filename chip with a 24×24 remove control and overflow 0. Recording: `ferry-readable-image-file-regression-final` (4 frames).
9. Scale/edge — the reported extreme-wide screenshot and corrupt-image case were exercised; the 390 px viewport retained 77…305 chip bounds inside composer 64…382 with overflow 0.
10. Contract-level attacks — N/A: no external protocol, persistence, authorization or universal boundary changed.
11. Predicate producers — `rg -n 'previewAttachment|previewURL|previewFailed|fileChip|filePreview' internal/webui/assets/app.js` enumerates selection-change reset, error producer, rendering consumers and cleanup.
12. Reversed findings — enlarging the preview raised corrupt-image collapse as a new question; it now falls back to the existing ordinary-file presentation, and ordinary files were separately replayed.
13. Pass limit — author full pass only; no subagent per Max's standing decision. Closure still requires final-image Mac mini deployment and live browser replay.

Status: local candidate; machine gates and final-tree browser journeys passed.
