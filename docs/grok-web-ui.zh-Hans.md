# 参考 Grok 的 Web UI 交付

> [English](grok-web-ui.md) | 简体中文

## S0 — 范围卡

- **REQUESTED（Max）**：`ui 有点丑，开一个浏览器 1:1 复制 grok 的 ui`。
- **完成标准**：本地运行的 Ferry Web 在桌面与手机浏览器中采用 Grok 当前首页的空间结构、尺寸节奏、圆角、明暗配色与贴底输入体验，同时保留 Ferry 的消息、文件、设备和密码功能。
- **不做**：不修改 iOS/Android；不改 Server/API/认证语义；不复制 Grok 商标、名称、文案或专有图形；不增加依赖、主题账户体系或新持久化状态。
- **INFERRED（不纳入）**：手动主题切换、侧边栏、搜索、Imagine、模型选择、登录/注册。
- **REPO_REQUIRED**：保留 Ferry 图标与设备图标；照片/文件分入口；错误状态可观察；真实浏览器桌面/手机旅程；相关 Go/Web 检查、diff check、对抗式自审和完整文件复核。
- **深度**：full；这是一个横跨接入页、空时间线、已有消息、composer、附件菜单、设备设置与响应式布局的整体视觉重做，但不改变外部合约。
- **预算**：最多 3 个生产文件、净新增不超过 450 行、0 个新持久机制；1 个过程文件、最多 180 行。
- **执行预算**：45 分钟；最多 2 次无效浏览器验收尝试；外部动作仅限对公开 Grok UI 的只读检查与最终 `origin/main` push。
- **边界计划**：真实本地 Ferry Server + browser-harness；桌面 1383×997 与手机 390×844 分别检查空状态、已有消息、附件菜单和设备弹窗。API 测试只作支持证据，不能替代页面验收。
- **扩张触发**：需要后端/API 变化、第四个生产文件、新依赖、Grok 专有资产、或超出预算时立即停止并回报。

范围由 Max 在 2026-08-31 以 `go` 确认。

## S1 — 设计决策

**Observed**：Grok 当前的公开 Web 页面使用无边框的全视口画布、左上角只有图标的品牌、右上角轻量的操作、居中的大号品牌、约 760 px 宽的胶囊形 composer、接近纯黑的深色模式，以及贴底的手机端 composer。Ferry 当前使用带边框的顶栏、卡片式的接入面板、窄的 ChatGPT 式消息流，以及始终贴底的 composer。

候选方案：

1. **Selected — 保留 DOM/API 状态机，替换 HTML 展示和 CSS**。既有的 `hidden` 状态仍是权威；空时间线把同一个 composer 移到 Grok 的居中位置，有消息的时间线再把它放回底部。没有新状态或依赖。
2. **Rejected — 只改 CSS，markup 不变**。不借助脆弱的生成内容，就做不出相同的品牌/composer 层级和可访问的图标按钮。
3. **Rejected — 用前端框架重建**。它会重复一套已经工作的请求/会话/渲染状态机，并超出批准的机制预算。

反例：如果已有消息到来后 composer 仍然停在消息流中间，选定的设计就不够。验证必须观察到一次真实发送后，composer 从空状态的中心移动到时间线底部。

约束性门禁：`go test ./...`、`go vet ./...`、`node --check internal/webui/assets/app.js`、`scripts/check-repo.sh`、`git diff --check`，以及真实浏览器的桌面/手机旅程。

## S2 — 冻结的交付检查清单

1. **REQUESTED** — 桌面空状态符合检查过的 Grok 空间体系：角落只有图标的品牌、轻量控件、居中的 Ferry 标识、宽胶囊 composer；浏览器截图和计算出的矩形。
2. **REQUESTED** — 手机空状态和有消息状态在 390×844 下保持可用，顶部控件考虑安全区，第一条消息之后 composer 贴底；真实浏览器发送旅程和截图。
3. **DESIGN_NECESSARY** — 文字发送、Photos/Files 选择入口、可见的发送中/错误状态、文件下载卡片、设备列表、撤销和密码设置保持既有 DOM/API 合约；Go 资源测试、JS 语法和浏览器交互。
4. **REPO_REQUIRED** — 明暗系统主题、焦点可见性、减少动效、Ferry/设备图标和响应式弹窗保持可访问；两种配色下的 DOM 检查和浏览器截图。
5. **REPO_REQUIRED** — 默认 commit/push 前，完成改动文件完整评审、作者 1–13 项自审、仓库门禁和最终 HEAD 浏览器重放。

冻结旅程：

- 桌面 1383×997，空 Server：页面显示居中的 Ferry 标识和 composer；输入 `desktop journey` 并提交；消息带发送者元信息出现，composer 贴到底部。
- 桌面，有消息的 Server：打开附件菜单；Photos 和 Files 两个入口都可见；打开 Devices；当前设备和密码设置可见；不做修改就关闭。
- 手机 390×844：重新加载有消息的时间线；顶部操作放得下，消息内容不横向裁切，composer 在底部安全区之上仍可触达。
- 深色模式：用 `prefers-color-scheme: dark` 重放有消息的手机页面；背景、composer、菜单/弹窗和文字保持清晰可读。

## 已被取代的匿名参照交付

第一次交付、它的兼容性修复和 composer 焦点修复保留在 commit `3b59eee` 和 `24bc4a9` 里；登录后的 Grok 证据推翻了页面结构之后，它们的详细证据在这里做了压缩。长期的回归仍由 `TestCSSKeepsCompatibilityFallbacks` 和 `TestComposerFocusStaysOnRoundedContainer` 保证。

## 实际聊天页重做 — S0 范围卡

- **REQUESTED（Max）**：`你觉和 grok 不像呢`，随后要求 `go` 继续处理。
- **完成标准**：部署在 Mac mini 的 Ferry 已有消息页，第一眼呈现 Grok 实际聊天页的信息层级：右侧发送气泡、无头像内容流、弱化元信息、窄正文列与贴底 composer；文字、文件、设备和密码功能保持可用。
- **不做**：不复制 Grok 商标、文案或专有资产；不伪造 AI 回复、模型选择、搜索、会话列表或账号功能；不修改 Server/API、认证、iOS、Android 或持久化数据。
- **INFERRED（不纳入）**：完整左侧会话栏、登录态 Grok 的隐藏功能、主题切换与消息编辑。
- **REPO_REQUIRED**：保留 Ferry/设备图标、照片与文件入口、可观察错误；真实桌面/手机浏览器旅程；相关测试、diff check、完整文件复核与对抗式自审。
- **深度**：full；已有消息、文件卡片、空状态、composer、设备面板与响应式布局需要作为一个页面系统重做，但无外部合约变化。
- **预算**：最多 3 个生产文件、净新增不超过 350 行、0 个新持久机制；沿用本过程文件，总行数不超过 220 行。
- **执行预算**：60 分钟；最多 2 次无效浏览器验收；外部动作限 Grok 匿名 UI 对照、`origin/main` push 和既有 Mac mini Ferry 容器部署。
- **边界计划**：真实本地 Ferry Server 上验证桌面空/有消息、文件卡、设备面板与 390×844 手机布局，再在 Mac mini 部署后的真实入口复验。匿名 Grok 登录墙后的内容不作臆测。
- **扩张触发**：需要 API/数据模型变化、第四个生产文件、新依赖、专有资产或超预算时立即停止。

只读证据：Grok 匿名状态下的桌面和手机聊天页截图在 `/private/tmp/grok-live-chat-result.png` 和 `/private/tmp/grok-live-chat-mobile.png`；录制 `grok-actual-chat-study`（18 帧）。登录墙挡住了对助手回复的检查，所以只有直接观察到的用户气泡、内容卡片、字体、间距和响应式行为进入设计。

范围卡展示之后，由 Max 在 2026-08-31 以 `go` 确认。

### 实际聊天页重做 — S1 决策

**Observed 旧语义**：`renderMessage` 创建一个 34 px 的设备头像，再在文字或文件上方放发送者/时间；CSS 把每一项都布局成左对齐的两列行。这就是为什么首页外壳换了之后，页面读起来仍像传统的聊天软件。

1. **Selected — 复用当前 DOM/API 状态机，改动消息 markup 的 class 和展示层**。每个 Ferry 事件仍是用户某台设备发出的消息，所以它的内容变成 Grok 式右对齐的用户气泡，下方是紧凑的设备/时间来源信息。空状态、composer、附件和设置保持既有的所有者和 ID。
2. **Rejected — 加一个类似 Grok 的侧边栏和会话历史**。Ferry 只有一条共享时间线，假的侧边栏会加入没有产品含义的控件。
3. **Rejected — 按发送者名字/类型识别「我的」消息并分两侧渲染**。API 有意快照了名字/类型，但不暴露发送者设备 ID；这种启发式会把两台同名设备分错。

反例：如果真实发送后头像列、加粗的发送者标题或整行左对齐的行仍然存在，选定的设计就没有解决反馈的不像问题。约束性门禁仍是 Web 资源测试、JS 解析、`go test ./...`、`go vet ./...`、`scripts/check-repo.sh`、`git diff --check`，以及真实桌面/手机浏览器旅程。状态：**由已确认的范围决定**；没有新机制或外部合约。

### 实际聊天页重做 — S2 冻结检查清单

1. **REQUESTED** — 真实桌面发送渲染出右对齐的圆角消息气泡，没有头像列或加粗的发送者标题；浏览器 DOM/样式断言和截图。
2. **REQUESTED** — 文件消息使用同样紧凑的气泡语言，同时保留文件名、大小和下载行为；静态合约加真实有消息页面的检查。
3. **DESIGN_NECESSARY** — 设备类型/名字/时间作为低调的来源信息保留在每条消息下方，避免之前要求的图标工作消失；对 SVG 和元信息的 DOM 断言。
4. **REPO_REQUIRED** — 空状态、composer 焦点、Photos/Files、设备/密码面板、明暗模式和 390×844 布局保持可用；既有测试加真实浏览器旅程。
5. **REPO_REQUIRED** — 完整仓库门禁、作者攻击/自审、完整文件全新一轮、最终 HEAD 的 Mac mini 浏览器重放，然后 commit/push/部署。

冻结旅程：桌面有消息的时间线检查气泡几何和元信息；一条新的文字发送通过真实 API 呈现为这种形状；附件菜单和 Devices 弹窗保持可操作；390×844 明暗两种有消息页面没有横向溢出，composer 可触达。预期的失败见证：恢复 `.message { grid-template-columns: 34px ... }` 或追加 `.avatar` 必须让新的资源回归测试失败。

### 实际聊天页重做 — S3 构建与旅程

- 被替换的语义：旧的左侧头像列加加粗发送者标题，变成了右对齐的内容气泡，下方是设备 SVG/名字/时间来源信息；没有改动 API、状态字段、依赖或持久化机制。
- 真实本地 Server（`127.0.0.1:42831`）：文字 `actual chat journey` 渲染在 `{x:924.98,w:162.52,right:1087.5}` 的气泡里，没有 `.avatar`，带 Mac SVG 元信息；composer 保持 `{w:760,bottom:959}`，横向溢出为 0。
- 真实文件 input 上传了 `ferry-ui-sample.txt`；渲染出文件标题、`19 B`、`.message-body`，并保留下载按钮。观察到了 Photos/Files 菜单、Devices 弹窗、当前设备和 `No password is required.`。
- 390×844 明/暗：气泡结束于 x=374，composer `{x:8,w:374,bottom:815}`，溢出 0。一条 360 字符的消息换行到 307.88 px 且不溢出；聚焦的 textarea outline 为 `none`，composer 圆角保持 26 px。
- Kill probe：停掉真实 Server 后，连接状态变为 `Offline`，状态文字显示 `Failed to fetch`，指示灯变为中性的 `rgb(112,112,112)`；重启后恢复 `Local` 并清除错误。录制：`ferry-actual-chat-local`（21）、`ferry-actual-chat-final-local`（10）、`ferry-actual-chat-edge`（7）。

构建者七类攻击记录：

1. 双裁判 — Web 资源测试和渲染后的 Chromium 都拒绝了头像列形状，并观察到气泡/设备来源信息。
2. 极值 — 零条消息、文字/文件消息、360 字符、桌面和 390 px 手机都渲染了；溢出保持 0。
3. 等价拼写 — 不适用：没有改动解析器、归一化或协议。
4. 默认值 — 明/暗两种系统默认都渲染了；没有新存储的主题或字段缺省行为。
5. 旁路 — 走查了空/有消息时间线、文件 input、附件菜单、Devices/密码面板和 Offline/恢复。
6. 策略门禁 — `TestMessagesUseCompactUserBubbles` 拒绝恢复头像/网格布局；`TestStatusAndSettingsButtonsKeepTruthfulStyling` 固化了两个自己发现的回归。
7. 不自我认证 — 真实 Go Server、真实 Chromium 发送/上传和实际 DOM 几何是验收见证；单元测试是支持证据。

### 实际聊天页重做 — S4 作者自审

1. 耦合状态 — 没有改动状态/定时器/缓存；`rg renderMessage|welcome.hidden|composerShell.hidden` 列出了既有 producer，只改变了渲染出的 DOM 顺序/class。
2. 失败路径 — 没有改动 I/O 分支；真实 Server 停止/恢复保留了可见的 Offline/错误，并回到 Local。
3. 未改动的消费者 — 完整读了 `app.js`；`loadMessages` 是 `renderMessage` 唯一的调用方，所有 JS 使用的 HTML ID 都还在；JS 解析和 Go 测试通过。
4. 合约面 — `git diff -- api internal/ferry ios android cmd` 为空；没有 API/schema/DB/env/客户端合约改动。
5. 原始复现 — 当前桌面截图有右侧气泡，没有头像/加粗发送者行；旧的 grid 字符串已不存在，并被测试禁止。
6. 当前 HEAD 旅程 — CSS 修复之后，在真实 Server 上重新做了桌面/手机/弹窗重放；最终本地证据列在 S3。
7. 机制区分 — `.message-body` 的几何加 `avatar:false` 见证了选定的布局；恢复旧的 grid/头像字符串会让回归测试失败。
8. 回归扫描 — 修复了发现的两个回归：Offline 状态误显示为绿色、Save 按钮透明；浏览器测得 Offline 为中性色，次要按钮不透明且有边框。
9. 规模/边界 — 零状态和一条 360 字符的手机消息通过；文件大小上限和存储行为不变。
10. 合约攻击 — 上面的七类攻击记录把解析器/协议攻击界定为不适用，其余给出了具体的 UI 探针。
11. 断言 producer — `rg connectionElement.textContent` 列出了 Connect/Local/Offline/password 这些 producer；指示灯现在继承各自状态文字的颜色，而不是断言在线。
12. 被推翻的发现 — 旧的设备按钮/次要按钮共用选择器的问题对两个消费者都重放了；`.secondary` 现在恢复了凸起的背景/边框，`.device-button` 保持极简。
13. 轮次上限 — 这是作者预筛，不是独立评审；收口仍然是有限的 S2 检查清单加完整文件全新一轮和部署旅程。

### 实际聊天页重做 — S5/S6 评审状态

- **S5 作者全新一轮**（非独立）：重读了最终完整的 `index.html`、`app.css`、`app.js`、`web_test.go`、旧实现和本设计。S4 发现的两个需求路径上的回归已修复并固化；修复后的完整文件和真实旅程没有剩余的作者已知 P0/P1。仓库门禁通过：`go test ./...`、`go vet ./...`、JS 解析、`scripts/check-repo.sh` 和 `git diff --check`。
- **S6 不适用** — 没有改动 API/协议/schema/鉴权/数据完整性合约。
- 部署前状态：`local_candidate`；基线 `24bc4a9`；评审来源 `author fresh pass`；语义失效 0；计划/当前生产范围 `3 files / -1 net line / 0 persistent mechanisms`。

### 实际聊天页重做 — S7 部署收口

- 对象 `3b59eee` 已推送到 `origin/main`，归档到既有的 Mac mini 部署目录，构建为 `ferry:local` 镜像 `sha256:5394083373c3512b0b732c3e1884ee3831117630fe0860b3aa58f5b8e47e7200`，重启时没有删除命名数据卷。`http://10.0.0.2:42817/` 返回 200。
- 部署后的桌面浏览器加载了保留下来的 5 条历史消息：全部是右对齐的 `.message-body`，0 个 `.avatar`，5 行设备 SVG 来源信息；原有的 Android 文件渲染在 `{right:1087.5,w:491.63}` 的气泡里，composer 为 `{w:760,bottom:959}`，连接状态 `Local`，溢出 0。
- 部署后的 Devices 显示保留下来的 8 台设备、`Current device: Mac browser` 和 `No password is required.`。手机 390×844 明/暗模式下文件气泡为 `{x:66.13,w:307.88,right:374}`，composer `{x:8,w:374,bottom:815}`，溢出 0。录制：`ferry-actual-chat-macmini`（10 帧）。
- 检查清单收口：S2 第 1–4 项有最终对象的浏览器证据加长期资源测试；第 5 项有完整门禁、作者全新一轮来源、push 和线上部署。不适用合约类 S6 门禁。旅程脚本：已删除，记录见上。
- 最终状态：`shipped`；确切的生产对象 `3b59eee`；评审轮次 1 次作者全新一轮；语义失效 0；计划/当前生产范围 `3 files / -1 net line / 0 persistent mechanisms`。

## 登录后参照的修正

Max 指出匿名/登录墙状态下的证据不够，并完成了 Grok 登录交接。录制 `grok-authenticated-study`（21 帧）替换了之前的视觉假设：桌面侧边栏/主区域为 257/1126 px；空状态 composer 为 `{x:444,y:413.5,w:752,h:60,radius:160}`；推广行为 `{x:452,y:505.5,w:736,h:66,radius:16}`；聊天内容宽 704 px；用户气泡为 `{w:633.59,h:63,radius:24px 24px 8px}`；body 背景为 `#050505`。

**S1 重新打开 — 语义失效 1。** 选定的「无侧边栏 + 两行 composer」设计与登录后的 Grok 矛盾，已移除。替换方案：一个 257 px 的 Ferry 侧边栏，只包含真实的 Ferry 目的地（Timeline、Devices 和当前 Server 标识），704 px 的消息列，既有的单行 752×60 composer，以及一个有意义的空状态 Server 信息行。不做假的搜索/历史/模型/AI 功能。

**S2 替换检查**：桌面 1383×997 必须测得侧边栏 257、composer 752×60、消息列 704；已有消息保留设备来源信息和文件下载；手机使用紧凑窄栏且不溢出；附件、Devices/密码、焦点、Offline/恢复和明暗模式检查仍保留。生产预算仍为 3 个文件 / ≤350 净行 / 0 个持久机制；用户授权是最初的 1:1 需求加上已完成的登录浏览器交接。

### 登录后修正 — S3/S4 证据

- 真实空 Server 在 1383×997 下测得侧边栏/主区域 `257/1126`，欢迎语 y `321.5`，composer `{x:444,y:413.5,w:752,h:60}`，信息行 `{x:452,y:505.5,w:736,h:66}`，焦点 outline `none`，溢出 0。截图：`/private/tmp/ferry-authenticated-grok-empty-final.png`。
- 真实有消息的 Server 测得消息列 `{x:468,w:704}`，右侧气泡带设备来源信息且无头像；Photos/Files 和 Devices/密码这些旁路入口都能打开。手机 390×844 测得窄栏/主区域 `56/334`，composer `{x:64,y:755,w:318,h:60}`，溢出 0。
- 强制明/暗模式得到可读的白色/`#050505` 画布。Kill probe 把 Local 变为可见的 Offline/`Failed to fetch`；重启后恢复 Local 并清除错误。
- 构建者攻击发现了 viewport 状态残留：手机端 textarea 高度在切换到桌面尺寸后仍保留，让 composer 变成 62 px。`resizeComposer` 现在在 resize 时运行；长期测试加一次手机→桌面的浏览器重放让两种状态都保持 60 px。

作者对抗式评审，第 4 轮（作者全新一轮，非独立）：

1. 耦合状态 — 新的 `sidebar.hidden` 只有 `showAccess/showApp` 两个 producer；resize 监听调用既有的幂等高度派生；`rg` 列出了所有 producer。
2. 失败路径 — 没有改动 fetch 分支；真实停止/恢复暴露了 Offline/错误，并回到 Local。
3. 未改动的消费者 — 重读了完整的 `index.html`、`app.css`、`app.js` 和 `web_test.go`；所有 JS 使用的 ID 和调用方都还在；JS 解析和 Go 测试通过。
4. 合约面 — 没有 API/schema/DB/env/客户端合约改动；只有展示和静态测试。
5. 原始复现 — 登录后参照的矩形现在完全一致；矩形焦点 outline 仍然不存在。
6. 当前 HEAD 旅程 — 最后一次 JS 修复之后，重放了空状态、有消息、菜单/弹窗、明暗模式和手机→桌面 resize。
7. 机制区分 — 手机端先测得 textarea 42/composer 60；切到桌面尺寸后测得 textarea 40/composer 60，证明是重新计算而不是 min-height 兜底。
8. 回归扫描 — 作者发现并修复了 62 px 的 resize 回归；390 px 和 1383 px 溢出保持 0。
9. 规模/边界 — 零条和有消息的时间线都通过；既有的长文本换行/文件名省略和负载行为不变。
10. 合约攻击 — 双裁判是内嵌资源测试加真实 Chromium；解析器/归一化/策略攻击不适用，因为这些边界没有改动。
11. 断言 producer — 在信任 CSS/视觉状态之前，grep 了 `sidebar.hidden`、`welcome.hidden`、`composerShell.hidden`、连接文字和状态错误的 producer。
12. 被推翻的发现 — 之前「无侧边栏」和两行 composer 的结论已失效，但它们关于假功能、响应式和真实状态的问题都对替换方案重新检查过。
13. 轮次上限 — 这明确是作者预筛；有限的门禁是替换后的 S2 检查清单、完整文件阅读、机器门禁和部署后的浏览器重放。

### 登录后修正 — S5/S6 状态

- **SHIP candidate，作者全新一轮**：一轮修复之后没有已知的 P0/P1/P2。最终工作树上需要再次运行 `go test ./...`、`go vet ./...`、JS 解析、仓库策略和 diff 检查。不适用合约类 S6 门禁。

### 登录后修正 — S7 部署收口

- 对象 `3158cea` 已推送，在 Mac mini 上构建为镜像 `sha256:70d8ef9a9494d10cf364b53c1300f3268fa029c8bdb12df13f6d7aa82ef1539d`，用既有数据卷重启；`http://10.0.0.2:42817/` 返回 200。
- 线上桌面测得侧边栏/主区域 `257/1126`，消息列 704，composer `{x:444,y:899,w:752,h:60}`，保留 5 条消息，Local，溢出 0。线上手机测得窄栏/主区域 `56/334`，composer `{x:64,y:755,w:318,h:60}`，右侧气泡结束于 x=378，溢出 0。
- 线上 Devices 显示保留下来的 8 台设备、`Current device: Mac browser` 和 `No password is required.`。录制：`ferry-authenticated-grok-rebuild`（52 帧）。最终状态：`shipped`；一次登录后参照导致的语义失效；评审来源：作者全新一轮，按用户决定不用 subagent。

## 当前设备消息对齐

- **需求**：`本机发的在右边，其他发的在左边`。完成标准是精确的设备身份，而不是按名字/类型猜；没有身份信息的旧消息保持在左边。
- **范围**：Server 持久化/API 投影加 Web 渲染；iOS/Android 的视觉对齐不在这次 Web 修正之内。适用合约深度，因为消息响应新增了 `is_current_device`；预算是 5 个生产文件、≤100 净行和一个可空 SQLite 列，没有新服务/依赖。
- **决策**：在既有的已鉴权创建入口持久化可空的 `sender_device_id`，保持私有，list/create handler 只投影出与请求上下文相关的 `is_current_device`。反例：两台同名为 `iPhone` 且类型相同的设备，仍必须渲染在相反的两侧。
- **冻结检查**：旧 DB 迁移后旧消息都算他人发送；已鉴权的文字/文件创建保留发送者身份；list 响应因请求设备不同而不同；Web 把严格的布尔 true 映射到右侧，其他任何值映射到左侧；两个真实浏览器身份发送消息，并在桌面/手机宽度下观察到相反的两侧。
- 状态：`shipped`；实现和作者门禁通过；为保持长期的不用 subagent 决定，Max 在 2026-08-31 明确豁免了合约类独立验证者。

对齐证据与对抗式收口：

- **S3 行为**：两个真实浏览器身份，名字/类型都是 `Mac Web/browser`，分别发送 A/B；每次重新加载都把自己的消息放在右侧、对方的放在左侧。在 390 px 下右侧结束于 x=378，左侧开始于 x=68，溢出 0。已鉴权的 curl 返回 `Cache-Control: no-store`、上下文相关的布尔值，且没有 `sender_device_id`。
- **S3 变异**：临时把 ID 相等换成发送者名字相等，会让 `TestMessageCurrentDeviceProjectionUsesIdentityNotName` 失败；恢复 ID 相等后通过。
- **攻击 1–3**：双裁判是 Store/API 测试加 Chromium；演练了旧 NULL、文字、文件、同名对端和手机；名字/类型的等价拼写无法把不同的 ID 合并。
- **攻击 4–7**：缺失/旧数据值通过严格的 `=== true` 失败到左侧；列出了直接的旧数据路径以及已鉴权的文字/文件/list/打开文件路径；按名字比较的变异失败关闭；两个鉴权 token 和一个 no-store 响应演练了身份/缓存边界。

作者对抗式评审（最终工作树全新一轮，非独立）：

1. 耦合状态 — 持久化的发送者 ID 有 create/scan 两类 producer；上下文布尔值只有 list/create 响应这些 producer；`rg` 列出了两者。
2. 失败路径 — 旧 NULL 保持可读/在左侧；格式错误的非 NULL ID 失败关闭；两者都有 Store 测试。
3. 未改动的消费者 — 重读了完整的 Go/Web 文件；新增的 JSON 字段对当前 iOS/Android 解码器仍可忽略。
4. 合约面 — 确切的响应 key 包含该布尔值、不含原始 ID；同名身份测试证明了这条边界。
5. 原始复现 — 已鉴权的 A/B 桌面矩形证明了自己在右/对方在左。
6. 当前 HEAD 旅程 — 最后一次缓存修复之后，重放了 A/B 和 390 px 旅程。
7. 机制区分 — 只换设备 token 时，同样两条消息会交换左右；基于名字的变异会失败。
8. 回归扫描 — 作者发现并用 `no-store` 修复了上下文相关响应的缓存泄漏；手机溢出保持 0。
9. 规模/边界 — 旧数据、文字、文件、同名和分页路径都通过；Ferry 只有一个共享的局域网空间，没有租户划分。
10. 合约攻击 — 所需的七类攻击都连同机器/浏览器证据记录在上面。
11. 断言 producer — 在信任测试之前，grep 了所有 `sender_device_id` 和 `is_current_device` 的读写。
12. 被推翻的发现 — 旧的 Web 全部靠右默认值被有意反转；旧消息保持在左侧，因为身份无从得知。
13. 轮次上限 — 作者 S5 为绿，没有已知的 P0/P1/P2；Max 在看到与不用 subagent 的冲突后明确豁免了 S6。

### 当前设备对齐 — S7 部署收口

- 对象 `c572b98` 已推送，并在 Mac mini 上部署为镜像 `sha256:1daba82f8096757c33975e41d42ecadb153c682c08a3d250d3ef2b86a6065b18`；容器在保留的 `ferry_ferry-data:/data` 数据卷上运行，`http://10.0.0.2:42817/healthz` 返回 200。
- 线上 Chromium 保留了五条历史消息，然后发送 `deployed right-side check c572b98`；它的气泡是当前设备/右侧 `{x:893.25,right:1172}`，溢出 0。一个临时的同类型对端观察到 `is_current_device:false`、`Cache-Control:no-store`，且没有原始发送者 ID，随后被撤销。录制：`ferry-current-device-alignment-macmini`（7 帧）；截图：`/private/tmp/ferry-current-device-alignment-macmini.png`。
