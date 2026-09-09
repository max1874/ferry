# 剪贴板：为什么不做自动同步

## 三句话说明

跨设备剪贴板自动同步在 2026-09-08 实现过一次，覆盖 Web、iOS、Android 三端，随后连同为它引入的局域网 TLS 一起整体撤回。撤回的原因不是没做完，是三个平台都要求应用处于前台且获得焦点才能读写剪贴板，因此「自动」只在用户已经盯着 Ferry 的时候才生效，而那正是最不需要它的时刻。现在 Ferry 不主动碰任何设备的剪贴板：内容进 Ferry 用系统自带的粘贴，出 Ferry 用每条消息上的复制控件。

本文件存在的目的是让下一次提案不必重跑同样的测量。

## 问题与证据

1. **Decided（Max，2026-09-08）**：把跨设备剪贴板自动同步纳入范围，下行自动写入、上行一键发送、默认关闭。

2. **Observed（2026-09-08 实测，Chrome 152）**：局域网 HTTP 页面上 `navigator.clipboard` 不存在。

   ```
   {"origin":"http://192.168.110.45:8099","isSecureContext":false,
    "hasClipboardObj":"undefined","hasWrite":"undefined","hasClipboardItem":"undefined"}
   ```

   W3C Secure Contexts 只把 `https://`、`localhost`、`127.0.0.1`、`file://` 列为可信来源，私有 IP 段不在其中。这是来源本身的性质，不是浏览器版本问题。为解锁 Web 端，当时引入了自签本地 CA 与 `-tls`。

3. **Observed（2026-09-08 实测）**：换成自签 HTTPS 后 `isSecureContext` 变为 `true`，无用户手势写入剪贴板成功，macOS 系统剪贴板确实收到了浏览器生成的 PNG。技术路径是通的。

4. **Observed（2026-09-08 实测，4 次尝试）**：浏览器窗口不是操作系统最前窗口时，`navigator.clipboard.writeText` 既不 resolve 也不 reject，Chromium 把写入挂起到窗口重新获得焦点。`document.hasFocus()` 返回 `true` 也不足以判断。当时的缓解措施是给每次写入加超时兜底。

   ```
   {"status":"Ferry could not reach this clipboard: the clipboard did not respond",
    "messages":2,"hasFocus":true}
   ```

5. **Observed**：iOS 从不允许后台访问 `UIPasteboard`；Android 10 起只有前台应用或当前输入法能读剪贴板，Android 12 起读取会向用户显示提示。三端一致。

6. **Observed（2026-09-09 实测，Chrome 152）**：在**纯 HTTP** 的局域网页面上，带用户手势的 `document.execCommand('copy')` 可用，返回 `true`，内容确实进了 macOS 系统剪贴板。

   ```
   页面 http://10.0.0.2:42817   isSecureContext: false   navigator.clipboard: undefined
   点击后 navigator.userActivation.isActive: true
   document.execCommand('copy') -> true
   pbpaste -> FERRY-EXECCOMMAND-GESTURE-TEST-20260909
   ```

   同步的 `execCommand` 不会像异步 Clipboard API 那样被推迟，因此证据 4 的失败模式在这条路径上不存在。

## 决策

- **Decided（Max，2026-09-09）**：删除三端全部剪贴板自动化，删除为它引入的局域网 TLS。

  推理：由证据 4 与 5，任何平台上的自动写入都要求 Ferry 在前台且最前。用户需要剪贴板里有东西，是为了粘贴到**别的**应用里，所以流程必然是「把 Ferry 切到最前 → 切到目标应用 → 粘贴」。相比每条消息一个复制按钮的「切到最前 → 点复制 → 切走 → 粘贴」，自动只省了一次点击。代价是三端各一套状态机（回环守卫、首页守卫、generation gate、写入超时兜底）、一个不可修复且不可见的失败模式，以及 Web 端「必须 HTTPS」的耦合——因为只有免手势写入才需要 secure context。

- **Decided（Max，2026-09-09）**：放弃跨设备剪贴板互通，每台设备只处理自己的剪贴板。这一条明确放弃了「Mac 浏览器复制图片、Windows 浏览器直接粘贴」的场景；把图片写进浏览器剪贴板必须用 `ClipboardItem`，那只在 secure context 下存在。用户在权衡「每台设备装一次 CA 证书」之后选择放弃该场景。

- **Decided**：复制控件走 `execCommand('copy')`（证据 6），不走 `navigator.clipboard`。这样 Web 端在纯 HTTP 下可用，且不需要 secure context、不需要权限、不需要超时兜底。

- **Decided**：复制控件只出现在文本消息上。文件消息无法通过 `execCommand` 放进剪贴板。

## 数据与 I/O 保证

| 规则 | 机制 | 反例与检查 |
| --- | --- | --- |
| Ferry 不在用户未操作时读剪贴板 | 三端都没有读取剪贴板的代码路径 | 全仓库搜索不到 `clipboard.read`、`primaryClip` 读取、`UIPasteboard.general.string` 的读取方 |
| Ferry 不在用户未操作时写剪贴板 | 写入只发生在复制控件的点击处理里 | 收到新消息不改变剪贴板 |
| 写入不会卡住 | `execCommand` 同步返回，点击本身证明窗口在最前 | 返回 `false` 时按钮显示 `Press Ctrl+C`，不静默失败 |
| 不需要 secure context | 不使用 `navigator.clipboard` | 纯 HTTP 局域网页面上复制按钮可用 |

## 已知边界

- **文件消息没有复制按钮**。`execCommand` 只能放纯文本。要复制图片必须回到 `ClipboardItem`，那会重新引入 HTTPS 依赖。
- **`execCommand('copy')` 已被标记为废弃**。目前所有目标浏览器仍然支持。如果将来被移除，Web 端复制要么改用 `navigator.clipboard.writeText`（需要 secure context），要么退回让用户手动选中后 Ctrl/Cmd+C。
- **Android 的复制按钮在 Android 12 及以上不会触发系统提示**，因为提示只针对读取。写入不需要权限。

## 验收

1. 纯 HTTP 局域网页面上，点文本消息的 Copy，按钮变 Copied，系统剪贴板收到该消息原文。
2. 收到新消息时剪贴板不变。
3. iOS 与 Android 上点 Copy 后，在其他应用里能粘贴出同一段文字。
4. 文件消息上没有 Copy 控件。
