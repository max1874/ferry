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
- Status: local candidate pending full gates, push, Mac mini rebuild, and deployed browser replay.
