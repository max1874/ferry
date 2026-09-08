# 跨设备剪贴板同步

## 三句话说明

Ferry 把别的设备发来的最新一条内容写进本机剪贴板，并提供一个按钮把本机剪贴板发出去。它默认关闭，只在应用处于前台时生效，因为三个平台都不允许后台读取剪贴板。如果边界设计错误，最直接的后果是用户正在使用的剪贴板被悄悄覆盖，或者应用在用户不知情时读走了密码。

## 问题与证据

1. **Decided（Max，2026-09-08）**：把跨设备剪贴板自动同步纳入范围。此前 `docs/product-core.md` 把它列为不做。

2. **Observed**：真正的后台自动读取在目标平台上不存在。

   - Android `targetSdk = 35`（`android/app/build.gradle.kts:27`）。Android 10 起只有前台应用或当前输入法能读剪贴板；Android 12 起读取会向用户显示「Ferry 粘贴了你的剪贴板」提示。
   - iOS deployment target 26（`project.pbxproj:155`）。iOS 从不允许后台访问 `UIPasteboard`；iOS 16 起程序化读取其他应用复制的内容会弹系统授权弹窗，除非使用系统粘贴控件。
   - 浏览器 `navigator.clipboard` 只在 secure context 下存在，且写入需要页面获得焦点。

3. **Observed（2026-09-08 实测，Chrome 152）**：在自签 HTTPS 的局域网页面上，无用户手势写入剪贴板成功，文字和 PNG 都进了 macOS 系统剪贴板。证据见 `docs/tls-lan.md`。

4. **Observed（同日实测）**：同一页面上 `clipboard-write` 权限为 `granted`，`clipboard-read` 为 `prompt`。写入不需要用户批准，读取需要。

## 决策

- **Decided**：下行（收到 → 写入本机剪贴板）自动；上行（本机剪贴板 → 发出）需要用户按一下。这不是产品偏好，是上面第 4 条证据的直接结果：写入平台允许自动，读取平台要求用户在场。
- **Decided**：默认关闭，由用户在设置里开启。`docs/product-core.md` 要求任何自动同步能力必须显式开启。
- **Decided**：不改 API 合约。`docs/product-core.md` 已把「用户复制文字」定义为核心动作之一，自动写剪贴板是把这一步自动化，不是新的消息种类。增加 `origin: "clipboard"` 字段要改 OpenAPI、SQLite schema 和四端模型，为一个当前不存在的区分付协议成本。
- **Decided**：只写入本次刷新中 `sequence` 最大且非本设备发送的那一条。
- **Decided**：连接后的第一页不写剪贴板。

## 数据与 I/O 保证

| 规则 | 机制 | 反例与检查 |
| --- | --- | --- |
| 断线重连不会连续覆盖剪贴板 | 每次刷新只取 `sequence` 最大的一条 | 离线期间积压 20 条，回来后剪贴板只被写一次 |
| 首次连接不覆盖用户正拿着的内容 | 第一页只设基线，不写入 | 连接时服务器返回历史消息，剪贴板不变 |
| 不复制自己发出去的内容 | 按 `is_current_device` 过滤，不比较设备名 | 两台设备同名时仍能正确区分 |
| 两台设备不互相回传同一段文字 | 记录上次写入或发出的文本，相同则不再写 | A 写入 T 后，T 再次到达不产生第二次写入 |
| 后台不写剪贴板 | 前台状态是写入前的硬条件，并在网络返回后复查 | 请求在飞行中时应用进入后台，返回后不得写入 |
| 会话失效后旧回调不写剪贴板 | 与消息刷新共用 generation gate | 撤销设备后到达的旧响应不写剪贴板 |
| 写入卡住不会拖垮功能 | 每次写入设超时 | 浏览器窗口失去系统焦点时写入永不 settle，超时后报错并恢复 |
| 不自动读取剪贴板 | 三端都只在用户按下按钮时读 | 页面获得焦点不触发读取，不弹权限框 |

## 各端实现差异

| | 下行 | 上行 |
| --- | --- | --- |
| Web | `clipboard.writeText` / `write([ClipboardItem])`；非 PNG 图片先经 canvas 转码，因为 Chromium 只写 `image/png` | 按钮触发 `clipboard.read()`，首次弹一次浏览器权限框 |
| iOS | `UIPasteboard.general.string` / `.image`，仅在 `scenePhase == .active` | `PasteButton` 系统粘贴控件，用户点一下，不弹权限弹窗 |
| Android | `ClipData.newPlainText`，仅文字 | 按钮读取 `primaryClip`；Android 12 起系统会提示用户 |

## 已知边界

- **Android 收到图片时不写剪贴板**。把图片交给 Android 剪贴板需要通过 content provider 发布，并依赖系统把读取授权传给粘贴方。这条路径没有在真机上验证过，所以没有实现，而不是实现一个没有证据的版本。Android 往外发图片不需要这条路径，现在就能用。
- **Web 端需要 HTTPS**。局域网 HTTP 页面上 `navigator.clipboard` 不存在，设置项会显示原因并保持禁用。见 `docs/tls-lan.md`。
- **浏览器窗口不在系统最前时写入不会返回**。Chromium 会把写入挂起到窗口重新获得焦点，既不 resolve 也不 reject，因此每次写入都有超时兜底。
- **不做剪贴板历史**。`docs/product-core.md` 的核心动作里没有这一条。

## 验收

前置期望：一台开启 `-lan -tls` 的 Ferry Server，四端都已加入并打开剪贴板同步。

1. Mac Chrome 复制一段文字 → 网页点发送剪贴板 → iPhone 打开 Ferry → 在备忘录里直接粘贴出同一段文字。
2. Mac 截图进剪贴板 → 网页 `Cmd+V` 贴进 composer 发送 → Windows Chrome 打开 Ferry → 在画图里粘贴出同一张图。
3. 关闭开关后，收到消息剪贴板不变。
4. 离线十分钟后回到 Ferry，收到 5 条积压，剪贴板只被最后一条覆盖一次。
5. Android 收到文字消息后剪贴板更新；收到图片消息时剪贴板不变。
