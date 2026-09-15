# 安全策略

> [English](SECURITY.md) | 简体中文

## 支持的版本

安全修复针对 `main` 上的最新 commit 和最新的 [GitHub Release](https://github.com/max1874/ferry/releases/latest)。旧版本、旧 commit、debug APK 和私人测试部署都不再维护。

## 报告漏洞

如果本仓库开启了 GitHub 的私密漏洞报告，请使用它。如果私密报告不可用，可以开一个 issue 请维护者提供私下联系方式，但不要在公开 issue 里写入利用细节、设备 token、密码、消息内容或上传的文件。

请写明受影响的 commit、组件、部署形态、复现条件和影响。公开披露前，请给维护者留出复现和准备协同修复的时间。

## 部署边界

Ferry 当前里程碑是为可信的私有网络或链路本地网络设计的：

- 传输使用 HTTP，不是 HTTPS；
- 能观察流量的局域网用户可能读到内容或凭据；
- 可选的共享密码控制准入，但不是管理员账号；
- 已准入的设备可以管理共享密码，也可以撤销其他设备；
- 备份里有私密消息、文件、设备 token 的 hash 和密码校验值。

通过 `-trusted-origin`（Docker 里是 `FERRY_TRUSTED_ORIGIN`），支持把 Ferry 放在私有网络或 VPN 里的 TLS 反向代理后面。它只加密浏览器到代理这一跳；代理到 Ferry 这一跳仍是 HTTP，应当留在 loopback 或私有网络里。有了代理也不代表支持公网暴露。

监听所有网卡（`-lan -listen 0.0.0.0:…`，Docker 里是 `FERRY_LISTEN_HOST=0.0.0.0`）需要显式开启。在 bridge 容器里，它仍然受发布端口的限制；直接跑在主机上或使用 host 网络时，防火墙放行的所有网卡都能访问 Ferry。

不要把 Ferry 直接发布到互联网。请使用主机防火墙、具体的私有监听地址、高位端口，以及与网络环境相称的访问控制。TLS、针对敌意网络的加固和稳定的安全支持策略都是后续里程碑。
