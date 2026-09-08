# 局域网 TLS

## 三句话说明

Ferry Server 可以用 `-tls` 在局域网上提供 HTTPS，证书由它自己维护的本地 CA 签发。这样做的唯一目的是让浏览器把 Ferry 页面当作 secure context，从而开放剪贴板 API；跨设备剪贴板同步完全依赖这一点。如果证书边界设计错误，最直接的后果是用户在手机上装了一个能冒充任意网站的根证书。

## 问题与证据

1. **Observed（2026-09-08 实测）**：Ferry 以 `-lan` 监听 `http://192.168.110.45:8099` 时，Chrome 152 中 `window.isSecureContext === false`，且 `navigator.clipboard` 为 `undefined`——不是调用失败，是对象不存在。

   ```
   {"origin":"http://192.168.110.45:8099","isSecureContext":false,
    "hasClipboardObj":"undefined","hasWrite":"undefined","hasClipboardItem":"undefined"}
   ```

2. **Observed（同日实测）**：退回 `document.execCommand('copy')` 也不可用，无用户手势时返回 `false`。

   ```
   {"execCommandCopy":false,"hasUserActivation":false}
   ```

3. **Observed**：W3C Secure Contexts 把 `https://`、`localhost`、`127.0.0.1`、`file://` 列为可信来源，私有 IP 段不在其中。所以问题不是浏览器版本，是来源本身。

4. **Observed（同日实测）**：换成自签 HTTPS 后，即使用户只是在 Chrome 拦截页点了「Proceed to ... (unsafe)」，`isSecureContext` 就变成 `true`，剪贴板读写 API 全部出现，`ClipboardItem.supports('image/png')` 为 `true`。

   ```
   {"isSecureContext":true,"clipboard":"object","write":"function",
    "readText":"function","ClipboardItem":"function","supportsPng":true}
   ```

5. **Observed（同日实测）**：在该页面上无用户手势写入剪贴板成功，macOS 系统剪贴板确实收到了浏览器生成的 PNG。

   ```
   {"hasFocus":true,"writeText":"ok","writeImage":"ok"}
   $ osascript -e 'clipboard info'
   «class PNGf», 173, «class AVIF», 513, ...
   ```

6. **Decided（Max，2026-09-08）**：为解锁 Web 端剪贴板同步，Ferry 引入局域网自签 HTTPS。不做公网部署，不申请公共证书。

## 候选方案

### A. 单张自签叶子证书，用户直接信任这张证书

- 优点：信任范围最窄，只对证书里列出的名字生效；泄露私钥不能冒充别的站点。
- 代价：DHCP 换 IP 后证书不再匹配，用户要在每台设备上重新信任一次。
- **Rejected**：Ferry 有四类客户端，重新信任的代价乘以设备数，会持续骚扰用户。

### B. 本地 CA + 叶子证书，用户信任 CA 一次

- **Recommended**：CA 十年有效，用户只信任它一次；叶子证书按当前地址签发，地址变了重签叶子，CA 不动。
- 代价：CA 私钥是真正的机密。持有它的人可以对所有信任过它的设备冒充证书里允许的名字。
- 缓解：CA 带 critical name constraints，只允许 loopback 与私有、链路本地地址段，DNS 只允许 `localhost`；再加 serverAuth EKU 限制。泄露的后果被压在私有地址空间内，不能冒充公网站点。

### C. 让用户改浏览器 flag（`unsafely-treat-insecure-origin-as-secure`）

- 优点：不写任何证书代码。
- 代价：每台电脑手动改浏览器开关，Safari 没有对应选项，iOS/Android 原生端完全不适用。
- **Rejected**：不能作为公开发布项目的部署方式。

## 决策

- **Decided**：采用候选 B。CA 与叶子证书都存放在 `<data-dir>/tls/`，私钥文件权限 `0600`，目录 `0700`。
- **Decided**：CA 有效期 10 年，叶子证书 398 天。398 天是 Apple 对 TLS 服务端证书的上限；即使本地信任的根不受该限制约束，贴着上限走可以少一个平台差异。
- **Decided**：证书只覆盖 listener 真正绑定的地址、`-published-host`，以及 loopback 身份。不枚举本机其他网卡——listener 只绑一个地址，其余地址签了也连不上，只会把这台机器的 VPN、容器和虚拟机网段公开给每个打开页面的人。
- **Decided**：`GET /ferry-ca.crt` 不鉴权返回 CA 公钥证书。设备必须先信任证书才能干净地完成握手并加入，所以这个入口不能放在设备会话之后。只返回公钥证书，CA 私钥永远不出 data directory。
- **Decided**：`-tls` 是可选开关，不带时行为与之前完全一致，不破坏已有客户端。
- **Decided**：`-tls-cert` 与 `-tls-key` 必须成对提供，提供后自动启用 TLS；这类证书的续期由部署者自己负责，Ferry 不管理。

## 数据与 I/O 保证

| 规则 | 机制 | 反例与检查 |
| --- | --- | --- |
| 信任 CA 不等于信任公网 | CA 带 critical name constraints，限定 `localhost` 与私有/loopback/链路本地网段，并带 serverAuth EKU | 用该 CA 私钥为 `example.com`、`8.8.8.8`、`ferry.internal` 签发证书，链验证必须失败 |
| 换 IP 不需要重新信任 | 地址集合变化时只重签叶子证书，CA 保持不变 | 用新 IP 再次启动，CA 指纹不变，叶子序列号改变 |
| CA 不被静默替换 | CA 文件损坏或私钥丢失时启动失败并说明如何处理 | 破坏 `ca.crt` 后启动必须报错，而不是签一张新 CA 让所有设备的信任失效 |
| 叶子证书损坏不打扰用户 | 叶子不承载信任，无法读取时直接重签 | 破坏 `server.crt` 后启动成功，CA 指纹不变 |
| 私钥不被同机其他用户读取 | 写入时 mode `0600`，目录 `0700` | 检查 `ca.key`、`server.key` 权限位不含 group/other |
| 写入中途崩溃不留下半个证书 | 先写临时文件再 `rename` 原子替换 | 崩溃后重启仍能解析现有材料 |
| 证书只声明能连上的地址 | SAN 仅含 bound 地址、`-published-host` 与 loopback | 多网卡机器上 SAN 数量不随网卡数量增长 |

## 已知边界

- **证书只在启动时检查和续期**。服务器连续运行超过叶子证书有效期而不重启，证书会过期。续期窗口设为到期前 30 天，任何在该窗口内重启过的服务器都不会遇到过期。结构性的后续做法是用 `tls.Config.GetCertificate` 做热替换并加一个定期检查，本次不做，因为它引入一个需要单独审查的常驻定时器。
- **运行中换 IP 不会自动重签**。DHCP 在服务器运行期间改变地址时，证书要等下次启动才更新。
- **name constraints 依赖客户端实现**。Chrome、Firefox、Safari、macOS、iOS、Android 都实现了 RFC 5280 name constraints，且扩展标记为 critical，不支持的客户端应当拒绝而不是忽略限制。真机验收要确认 iOS 与 Android 接受该 CA。

## 在设备上信任这个 CA

服务器启动时会打印下载地址和 SHA-256 指纹，先核对指纹再信任：

```
Ferry is running at https://192.168.110.45:8099
Trust this server's CA once per device: https://192.168.110.45:8099/ferry-ca.crt
CA SHA-256 fingerprint: 43:A9:91:8C:...
```

- **macOS**：下载 `ferry-ca.crt`，双击导入「钥匙串访问」的「登录」或「系统」，打开该证书，把「使用此证书时」设为「始终信任」。
- **Windows**：下载后双击，「安装证书」→「本地计算机」→「将所有证书放入下列存储」→「受信任的根证书颁发机构」。
- **iOS**：用 Safari 打开下载地址，允许下载配置描述文件，到「设置 → 通用 → VPN 与设备管理」安装，再到「设置 → 通用 → 关于本机 → 证书信任设置」为该证书打开完全信任。第二步不能省，只安装不启用信任的话 Safari 仍然报错。
- **Android**：下载后到「设置 → 安全 → 加密与凭据 → 安装证书 → CA 证书」安装。

不想安装证书时，浏览器点一次「继续前往」也能得到 secure context（实测见上文证据 4），但每个来源都要点一次，地址栏会一直显示不安全。

## 验收

前置期望：一台开启 `-lan -tls` 的 Ferry Server，一台同网段的电脑。

1. 启动日志打印 `https://` 地址、CA 下载地址与指纹。
2. `curl -sk https://<地址>/ferry-ca.crt` 下载的证书指纹与日志一致。
3. 用下载到的证书作为信任根发起请求，不加 `-k` 也能成功：
   `curl --cacert ferry-ca.crt https://<地址>/healthz` 返回 `{"status":"ok"}`。
4. `openssl x509 -in <data-dir>/tls/server.crt -noout -ext subjectAltName` 只列出 `localhost`、`127.0.0.1`、`::1` 与实际绑定地址。
5. 浏览器打开该地址，页面正常渲染，`window.isSecureContext` 为 `true`，`navigator.clipboard.write` 存在。
6. 不带 `-tls` 启动时，一切行为与本次改动前一致。
