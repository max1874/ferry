# Web 附件选择器

> [English](web-attachment-picker.md) | 简体中文

## 范围卡

- **REQUESTED — Max，2026-08-30**：Web 的 `+` 不能默认只进入文件选择；手机用户需要清楚、直接地从相册选择媒体。
- **完成标准**：点击 `+` 显示“Photos”和“Files”两个入口；Photos 只请求图片/视频，Files 保留任意文件；两者选中后复用现有附件预览与发送流程；菜单支持点击外部和 Escape 关闭。
- **不做**：不主动启动相机、不增加多选、不改 Server API、不修改 iOS 原生附件入口。
- **深度**：focused。
- **预算**：`index.html`、`app.css`、`app.js` 三个生产文件和直接相关的测试；无依赖、无持久化、无协议变更。
- **边界证明**：Web handler 合约测试，加真实移动 viewport 浏览器旅程，分别证明 Photos 与 Files 路由到不同 input。

## 证据与决策

- **Observed**：原实现的 `+` 是一个直接包裹无 `accept` 属性文件 input 的 label，因此只有通用文件选择语义。
- **Decided**：`+` 先打开 Ferry 自己的二选一菜单；Photos 使用 `accept="image/*,video/*"`，Files 不设置 `accept`。
- **Decided**：不设置 `capture`；用户要求的是相册入口，不能把它强制变成相机入口。

## 交付检查清单

1. `+` 显示 Photos / Files，ARIA 展开状态同步。
2. Photos 同步触发媒体 input，Files 同步触发通用 input。
3. 两种选择只保留一个当前附件，并复用原发送与清除流程。
4. 点击外部、Escape、进入 access 状态都会关闭菜单。
5. Go tests、race、vet、JavaScript 语法、Docker build 与 `git diff --check` 通过。
6. 当前 HEAD 和测试服务器部署均通过真实浏览器旅程。
7. 推送前完成 13 项对抗式自审与一次独立 focused review。

## 验证

- 完整 Go 测试、race 测试、vet、JavaScript 语法和 `git diff --check` 在当前工作树上通过。
- 修复无障碍问题后，最终当前工作树的 Docker 镜像构建通过，为 `sha256:ca7b08…`。
- 390×844 移动浏览器旅程：Photos 有 `accept=image/*,video/*`；Files 没有限制；坐标点击得到计数 `{photo: 1, file: 1}`，没有交叉触发；通过 Photos 选择的真实 PNG 渲染出附件 chip，发送成功，随后两个 input 都被清空；Escape 和点击外部都关闭了选择器。录制：`<browser-harness recordings>/ferry-web-attachment-picker-head3`（11 帧，仓库外）。

## 对抗式自审

结论：**ship candidate，待独立评审和部署证明**。第 1 轮发现并修复了一个 ARIA menu 模式不匹配，以及 `showAccess(clearCredential=false)` 中菜单残留打开的路径；第 2 轮没有新的生产问题。

1. 耦合状态 — `attachmentMenu.hidden`、`aria-expanded`、两个 input 的值和 composer 派生状态，都只有一个修改方/helper 或显式的 change 监听；`rg -n 'selectedAttachment|clearAttachment|setAttachmentMenuOpen|photoInput|fileInput|attachmentMenu' internal/webui/assets/app.js` 列出了所有读写方。
2. 失败路径 — 取消选择器不触发 change，保留之前的选择；空 input 得到 `null`；发送失败通过既有 catch 路径保留已选 input；当前 HEAD 的浏览器证明了成功后的清理。
3. 未改动的代码 — 同一个 `rg` 找到了以前所有 `fileInput` 的消费者：access 重置、composer 派生、提交和移除；它们在需要的地方都改用了共享的附件 helper。
4. 合约面 — `git diff --stat` 仅限内嵌 Web HTML/CSS/JS、一个 Web 测试和本文件；Server 路由、schema、数据库和环境变量都没变。
5. 原始复现 — 当前 HEAD 的移动浏览器不再从 `+` 直接打开选择器；它会先可见地打开 Photos / Files（`menu.hidden=false`，焦点在 `choose-photos`）。
6. 当前 HEAD 旅程 — 录制 `ferry-web-attachment-picker-head3` 读取的是实际选中的文件名、发送后的消息标题和正常的最终状态，而不只是 HTTP 状态码。
7. 机制区分 — 在同一个渲染页面里，覆盖原生 input 的 click 方法，先观察到 Photos `{photo:1,file:0}`，再观察到 Files `{photo:1,file:1}`；不带限制的文件 input 无法满足 Photos-only 的见证。
8. 回归扫描 — 候选回归是丢失任意文件发送；Files 保留空 `accept`，路由计数只增加了 `file`，既有完整 Go/race 测试通过。
9. 规模/边界 — 未选择时仍是普通文字模式；input 不带 `multiple`，保证只选一个；上传大小和 Server 隔离没有改动。证据：浏览器选了一张照片，另一个 input 计数为零。
10. 合约级攻击 — 不适用：没有改动网络、持久化、授权或跨客户端合约。
11. 断言 producer — 菜单 hidden/expanded 和两个文件计数的所有 producer 都由第 1 项的 grep 列出；初始 HTML 状态是 hidden/false，运行时每次菜单切换都走 `setAttachmentMenuOpen`。
12. 被推翻的发现 — 去掉 ARIA `menu/menuitem` 后保留了普通按钮的无障碍语义；把关闭动作挪到清除凭据的分支之外，覆盖了 `rg -n 'showAccess\\(' internal/webui/assets/app.js` 列出的每个 `showAccess` 调用方。
13. 轮次上限 — 作者评审不自我认证；独立聚焦评审和测试服务器当前部署的重放仍是显式的检查清单门禁。

## 独立聚焦评审

- 第 1 轮：**not ship**，一个 P2。关闭选项组会隐藏获得焦点的选项，让 `document.activeElement` 落到 `body` 上，所以键盘和辅助技术用户在取消系统选择器之后会丢失位置。
- 修复：Photos / Files 的每个 handler 现在都会先把焦点还给可见的 `+` 按钮，再同步触发原生 input。当前 HEAD 的浏览器重放在两个分支之后都观察到 `active=attach`，并保持 `{photo:1,file:1}` 的路由。
- 第 2 轮：**SHIP**，没有剩余的 P0/P1/P2。独立录制：`<browser-harness recordings>/ferry-attachment-independent-rereview`（仓库外）。

## 部署

- 源码 commit `30372f0` 已推送到 `origin/main`。
- 测试服务器重建并在 `http://192.168.1.20:42817` 运行镜像 `sha256:b83c6c…`；Compose 重建了容器/网络，没有删除命名数据卷，线上时间线保留了历史的文字和文件消息。
- 直接的局域网检查返回 `{"status":"ok"}`，返回的 HTML 包含 Photos 按钮和 `accept="image/*,video/*"` 合约。
- 线上 390×844 浏览器重放观察到两个内联 SVG 图标，先 Photos-only 再 Files-only 的路由 `{photo:1,file:1}`，两次选择后焦点都回到 `attach`，没有交叉触发。录制：`<browser-harness recordings>/ferry-web-attachment-picker-test-server`（6 帧，仓库外）。
- 已知验证边界：移动 Chromium 模拟证明了 Ferry 的响应式 UI、DOM 合约和路由；真实的 iOS 系统相册面板仍需要用户在实体 iPhone 上点按测试。

## 剪贴板图片粘贴 — 2026-09-01

- 需求：“输入框不支持直接粘贴图片？” 完成标准是剪贴板里的图片变成既有的单个附件 chip，并走既有的文件发送路径；普通的粘贴文字仍然是文字。原生 App、多图粘贴和新的上传机制不在范围内。
- 聚焦设计：一个可空的内存变量 `pastedAttachment` 在 `selectedAttachment` 处与两个既有的选择器 input 汇合；每条既有的清除/成功路径都会清掉它，选择器选中文件会替换它。只有真正的 `image/*` 文件才会阻止浏览器默认的粘贴。
- 冻结检查：原生 Cmd-V 粘贴图片显示 `image.png`，禁用文字模式并启用 Send；发送产生一条文件消息并清掉 chip；原生 Cmd-V 粘贴文字插入完全相同的文字且不出现 chip；既有的 Photos/Files 替换和移除行为保持不变。
- 本地最终工作树的 Chromium 使用真实系统剪贴板和原生 Paste 命令：PNG 得到 `{chip:true,name:image.png,textDisabled:true,sendDisabled:false}`，发送后显示为可见的 `image.png` 文件消息；随后纯文字得到 `{chip:false,text:"plain clipboard text"}`。录制：`ferry-paste-image-local`（9 帧）。
- 作者对抗式一轮：列出了所有附件 producer；取消保留之前的附件，选择器选择替换粘贴，移除/成功会清掉它，非图片粘贴在 `preventDefault` 之前就返回，只取第一张图片符合 Ferry 既有的单文件合约，Server 的上限/错误保持共用。没有协议/schema/鉴权改动；按 Max 的长期决定不用 subagent。
- 已发布：源码 `36527dd`；完整 Go tests/vet、JS 语法、仓库策略和 diff 检查通过。测试服务器以保留的 `ferry_ferry-data:/data` 运行镜像 `sha256:5e5118848901e902216929c47ae974889988cc458235deda2adb9c51d27c5ef9`；`/healthz` 返回 200。部署后的 Chromium 把 `deployed-paste.png` 放进了 chip，启用了 Send，移除后恢复为空的文字模式，没有发出测试消息。录制：`ferry-paste-image-test-server`（5 帧）。

## Grok 风格的附件展示 — 2026-09-01

- 截图反馈之后的需求：查看真实 Grok 的图片/文件附件状态，替换 Ferry 占满整行的文件名条。范围是 Web 附件展示和图片预览所需的窄 CSP 来源；上传/API/原生客户端不变。
- 登录后的 Grok 观察（未发送）：图片 chip 为 40×40，内含 34×34 的 object-cover 预览，内圆角 9 px，悬停时出现移除控件；文件 chip 是按内容宽度的 40 px 胶囊，带 20 px 文档图标、截断的文件名和 24 px 的移除按钮。两者都位于输入行之上，让 752 px 的 composer 高 112 px。录制：`grok-attachment-reference`（6 帧）。
- 决策：Ferry 使用相同的几何尺寸和按类型区分的展示，触屏上移除控件始终可见并适当缩放。附件身份每次变化都会 revoke object URL；大图不会复制成 base64。
- 根因/合约：第一次真实渲染暴露出 CSP 的 `img-src` 拦截了 `blob:`。策略现在只对图片放行 `blob:`，而 script/style/connect/object/base/frame/form 指令仍由 `TestStaticWebAndSecurityHeadersShareHandler` 逐字节固定。
- 本地最终工作树旅程：图片 `{chip:40×40,preview:34×34,natural:1794×364,composer:752×111}`；普通文件 `{chip:248.45×40,remove:24×24,composer:752×111}`；390 px 下文件视图保持在 composer x=64…382 之内，溢出为 0。记录：`ferry-grok-attachment-style-local`（8 帧）。
- 冻结收口：图片和文件选择、粘贴图片、移除、发送成功后的清理、桌面/手机几何、CSP 精确测试、完整仓库门禁、push 和测试服务器重放。只有作者评审，按 Max 的长期决定不用 subagent。
- 已发布：源码 `c186afc`；测试服务器以保留的 `ferry_ferry-data:/data` 运行镜像 `sha256:ae5f4d37183971f9a7fd93bc1c48d86d88e5c27a0d85a28ca5556b913aae416b`，CSP 精确一致，`/healthz` 为 200。线上图片测得 `40×40`/预览 `34×34`，线上文件测得 `248.45×40`/移除 `24×24`；两者都让 752 px 的 composer 高 111 px 且溢出为 0，之后未发送就移除。录制：`ferry-grok-attachment-style-test-server`（6 帧）。

## 可读图片预览修正 — 2026-09-01

- 真实使用反馈之后的需求：附件相关工作必须实际做视觉走查；40×40 的方块不能把一张宽截图变成看不清的小白点。
- 根因：照搬 Grok 的固定方块使用 `object-fit: cover`，极端宽高比会丢掉几乎所有有用像素。修改规则之前，同一张 1658×350 截图分别粘贴到登录后的 Grok 和已部署的 Ferry 里对照。
- 决策：图片预览保持完整宽高比，缩放到 220×96 以内；图片 chip 按实际预览定尺寸，普通文件 chip 和上传/API 路径不变。
- 失败行为：如果文件声称是图片媒体类型但无法渲染，待发送附件回退为普通文件名 chip，而不是塌成一个空的图片框；更换附件会重置这个回退状态。
- 冻结旅程：粘贴反馈的截图，检查预览几何和内容，移除，再粘贴，发送，检查生成的时间线条目，并在 390×844 下重复待发送状态且不溢出。

### 最终工作树证据与对抗式评审

- 登录后的 Grok 对照：上传了反馈的那张 PNG，它在 Grok 的 40×40 附件位里渲染。录制：`ferry-attachment-grok-reference`（3 帧）。
- Ferry 原始复现：同一张 PNG 在 40×40 chip 里渲染成 34×34 的方块；752 px 的 composer 变为 111 px 高。录制：`ferry-attachment-bug-repro`（3 帧）。
- 当前候选版本：浏览器解码的 1730×382 图片渲染为 220×48.57，chip 为 228×56.57，关闭控件可见，752 px 的 composer 高 127.57 px。在 390×844 下同一个 chip 保持在 318 px 的 composer 之内，溢出为 0。删除后恢复为空的文字模式；第二次粘贴发送成功，产生预期的 50.5 KB 时间线条目。录制：`ferry-readable-image-preview-final-subject`（8 帧）。

1. 耦合状态 — `previewAttachment`、`previewURL` 和 `previewFailed` 都归 `updateComposer` 所有；`rg` 除了图片的 error 监听之外没有找到其他 producer，而这个监听会比较 `currentSrc` 和当前的 object URL，所以被替换掉的旧图片发来的过期错误不会污染新的预览。
2. 失败路径 — 一个故意损坏的 `.png` 回退为 `file-chip`，文件名可见且 Send 可用；随后移除清掉了它。上传/发送失败仍通过未改动的 catch 路径保留已选附件。
3. 未改动的消费者 — 重读了 `selectedAttachment`、access 重置、成功提交、选择器 change 和移除这些调用方；CSS 不改变它们的状态转换。
4. 合约面 — `git diff --stat` 只改动内嵌 Web CSS/JS、它的静态测试和本文件；API、数据库、CSP、鉴权和原生客户端都没变。
5. 原始复现 — 用户提供的同一张宽 PNG 在改动前后各重放一次；看不清的方块变成了保持宽高比的 220 px 预览。
6. 当前 HEAD 旅程 — `ferry-readable-image-preview-final-subject` 在最后一次加入过期错误防护之后运行，观察到损坏回退、渲染几何、删除、再粘贴、发送完成，以及实际的时间线标题/大小/状态。
7. 机制区分 — 渲染宽度从固定的 34 变成 220，同时原始宽高比 `1730/382` 与渲染宽高比 `220/48.57` 一致；损坏的图片走了非图片回退，而不是满足预览检查。
8. 回归扫描 — 普通的 `README.md` 选择仍是 154.02×40 的文件名 chip，带 24×24 的移除控件，溢出为 0。录制：`ferry-readable-image-file-regression-final`（4 帧）。
9. 规模/边界 — 走查了反馈里极宽的截图和损坏图片的情况；390 px viewport 下 chip 边界为 77…305，在 composer 64…382 之内，溢出为 0。
10. 合约级攻击 — 不适用：没有改动外部协议、持久化、授权或普遍性边界。
11. 断言 producer — `rg -n 'previewAttachment|previewURL|previewFailed|fileChip|filePreview' internal/webui/assets/app.js` 列出了选择变化时的重置、错误 producer、渲染消费者和清理。
12. 被推翻的发现 — 放大预览带出了一个新问题：损坏图片会塌掉；现在它回退到既有的普通文件展示，普通文件也单独重放过。
13. 轮次上限 — 只有作者完整一轮；按 Max 的长期决定不用 subagent。测试服务器部署和线上浏览器重放提供了所需的用户界面收口。

### 部署收口

- 源码 `571df86` 已推送到 `origin/main`；测试服务器以保留的数据卷运行镜像 `sha256:a484a4401d5e9404f0341df60f15d1f2ebf1162136f534d8c7923cba0922455f`，`/healthz` 返回 `{"status":"ok"}`。
- 在 `http://192.168.1.20:42817` 上，反馈的那张 PNG 再次测得预览 220×48.57、chip 228×56.57、移除控件可见，桌面溢出为 0。移除后文字输入恢复焦点，Send 被禁用。
- 在部署环境 390×844 下，chip 保持在 x=77…305，位于 composer x=64…382 之内，溢出为 0。再次粘贴并发送，产生一条真实的 50.5 KB 时间线条目，文件名一致，待发送 chip 被清掉，恢复正常的隐私状态提示。录制：`ferry-readable-image-preview-test-server-final`（6 帧）。

状态：已发布，并在部署的测试服务器实例上做过视觉走查。

## 附件 composer 圆角修正 — 2026-09-01

- 最终截图评审之后的需求：带附件而变高的 composer 不能沿用纯文字时 `999px` 的胶囊圆角，鼓成一个很大的空胶囊。
- 根因：预览尺寸修复让 composer 高度从约 60 px 变成 132 px，但无条件的圆角仍是 `999px`；溢出检查通过了，轮廓在视觉上却仍然不对。
- 决策：纯文字 composer 保持胶囊形；只要选了附件，就派生出 `has-attachment` class 并使用 32 px 的面板圆角。清除或发送附件后，通过同一个 `updateComposer` 状态派生去掉这个 class。
- 冻结旅程：在桌面和手机上粘贴反馈的截图，检查圆角和轮廓，移除并证明胶囊形恢复，然后在部署的测试服务器上再次粘贴并成功发送。

### 本地候选证据与对抗式评审

- 用反馈的 PNG 在部署环境上的原始复现测得 composer `752×132.19`、圆角 `999px`；录制 `ferry-attachment-radius-bug-repro`（3 帧）。
- 最终本地桌面测得相同高度、圆角 `32px`；在 390×844 下面板为 `318×132.19`，预览在面板之内，溢出为 0。得到的截图经过了肉眼检查，而不是只凭几何数据接受。录制：`ferry-attachment-radius-local-final`（4 帧）。
- 移除附件后得到 class `composer`、高度 60、圆角 `999px`；再粘贴使用 `32px`，发送成功，产生预期的时间线条目，随后恢复 60 px 胶囊和正常状态。录制：`ferry-attachment-radius-local-send`（4 帧）。

1. 耦合状态 — `has-attachment` 只在 `updateComposer` 中由 `selectedAttachment` 派生；不存在第二个附件标志。
2. 失败路径 — 发送失败保留已选附件，因此保留面板圆角；移除/成功/access 重置会清除选择并重新派生出胶囊形。
3. 未改动的消费者 — 列出了每个 `updateComposer` 调用方；没有一个绕过选择派生。
4. 合约面 — 只有 CSS class；没有 API、存储、鉴权、CSP 或原生客户端改动。
5. 原始复现 — 同一张反馈的 PNG 从 999 px 的跑道形轮廓变成 32 px 的面板。
6. 当前 HEAD 旅程 — 桌面/手机粘贴、视觉截图、移除、再粘贴和发送都在最后一次 class/测试修改之后运行。
7. 机制区分 — 有附件时得到 `has-attachment/32px`；同一页面清除后得到无 class/999px。
8. 回归扫描 — 移除和发送之后，纯文字 composer 都回到恰好 60 px 和原有的胶囊圆角。
9. 规模/边界 — 390 px viewport 下 318 px 面板保持在 x=64…382 之内，横向溢出为 0。
10. 合约级攻击 — 不适用：没有改动边界合约。
11. 断言 producer — `rg -n 'has-attachment|updateComposer\\('` 找到了唯一的 class producer 和每个重新计算的调用方。
12. 被推翻的发现 — 预览尺寸仍然成立；漏掉的问题是父容器的轮廓，现在它独立于子元素边界单独检查。
13. 轮次上限 — 只有作者完整一轮；按 Max 的长期决定不用 subagent。最终的测试服务器重放提供用户界面收口。

### 部署收口

- 源码 `040c300` 已推送到 `origin/main`；测试服务器以保留的数据卷运行镜像 `sha256:0c5183aa08ff1f832231b47da74d031f3ca8b42a65041868cbf9f0c7ccb101ce`，`/healthz` 返回 `{"status":"ok"}`。
- 用反馈的 PNG 对部署后的桌面和 390×844 截图做了肉眼检查。附件状态测得圆角 32 px；手机面板为 `318×132.19`，溢出为 0。
- 移除后恢复 class `composer`、高度 60、圆角 999 px 和文字焦点。再粘贴使用 32 px，一条真实的 64.2 KB 消息发送成功，完成后恢复正常的胶囊形和状态。录制：`ferry-attachment-radius-test-server-final`（6 帧）。

状态：已发布，并在部署的测试服务器实例上做过视觉走查。
