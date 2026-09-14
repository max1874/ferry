# Docker 部署

> [English](docker-deployment.md) | 简体中文

> 准入方式在 2026-08-30 已改变：当前的 Ferry 直接加入，并可选使用在 Web 设置里配置的密码。下文关于配对码的旅程记录是历史部署证据；当前的接入合约以 `docs/password-access.md` 为准。Docker 环境变量只配置网络发布，从不配置产品密码。

## 交付记录

- 状态：在 macmini 上以当前的免密码/可选密码准入模式 `deployed`；跨设备的家庭局域网复验记录在 `docs/release-readiness.md`，不再挂在已退役的配对流程下。
- 对象：当前 main 部署；下面的配对码检查清单只作为原始容器边界的历史证据保留。
- 需求（2026-08-30）：在 `macmini` 上用 Docker 运行 Ferry Server 和 Web，使用高位端口而不是 8080。
- 完成标准：Web 和 iPhone 通过 `http://10.0.0.2:42817` 互发真实消息，容器重启后数据仍在。
- 不做：TLS、域名、反向代理、公网暴露，以及迁移笔记本上的临时测试数据。
- **Decided（Max，2026-09-13，issue #1）**：现在支持运维者自己运行的 TLS 反向代理，通过 `FERRY_TRUSTED_ORIGIN` / `-trusted-origin` 配置。监听规则不变：代理连到发布端口或容器地址，因此不需要绑定通配地址。Ferry 在 `Host` 和 `Origin` 里只放行配置的那个 origin，并忽略 `X-Forwarded-*`。
- 深度：contract，因为部署必须保持 Ferry 的已鉴权局域网监听边界。
- 预算：Dockerfile、Compose、`.dockerignore`、部署文档和直接必要的测试；不改 API 或数据库。

## 证据与决策

- **Observed**：Ferry 的 Go 进程同时内嵌 Web UI 和 API，并把 SQLite 和 blob 持久化在 `-data-dir` 下。
- **Observed**：`macmini` 是 arm64，地址 `10.0.0.2`，运行 OrbStack Docker 29.4.0 / Compose 5.1.2，预检时 42817 端口未被占用。
- **Observed**：前一个项目 `avocado` 的部署使用 Compose、命名数据卷、`restart: unless-stopped` 和 host 网络。
- **Observed**：Ferry 即使在 LAN 模式下也拒绝通配监听；普通的 bridge 容器因此不能绑定 `0.0.0.0`。
- **Recommended，在用户授权执行的前提下选定**：使用 bridge 网络，启动时取出容器唯一的 IPv4 地址，让 Ferry 绑定这个私有地址，并只发布到配置的 Mac 局域网地址。
- **Rejected**：host 网络会让 OrbStack 的转发边界变得隐式，而且发布时不带 host-IP 映射。
- **Rejected**：允许 `0.0.0.0` 会仅仅为了部署方便而削弱一条已测试的安全边界。

```text
iPhone / browser
      |
      | http://10.0.0.2:42817
      v
Mac mini host-IP port mapping
      |
      v
Ferry container private IPv4:42817
      |
      +-- embedded Web + authenticated API
      +-- /data named volume (SQLite + blobs)
```

一句话说明：这一步为现有的 Ferry Server + Web UI 组合加了一个可重复的容器打包方式。容器保留 Ferry「只监听具体私有地址」的规则，同时在 Mac mini 上暴露一个可配置的高位端口。如果地址接线或数据卷配错，设备会连不上，或者重启后数据消失。

## 约束性检查

- `go test ./...` 和 `go vet ./...` 必须保持绿色。
- 镜像构建必须编译和 Docker 之外相同的 `./cmd/ferry` 入口。
- 如果容器地址为空或有歧义，或者 `FERRY_HOST_IP` 不是 loopback/私有/链路本地地址，启动必须失败；Ferry 本身仍是监听地址和发布主机两者数值/私有地址的最终裁决者。
- Compose 默认发布到 loopback；Mac mini 部署通过一个未跟踪的 `.env` 文件选择 `10.0.0.2`。
- `git diff --check` 和一次完整文件的安全评审是发布门槛。

## 配对时代的历史检查清单

1. **REQUESTED** — `docker compose config` 解析出宿主端口 42817 和持久化的 `/data` 数据卷。
2. **DESIGN_NECESSARY** — 运行中的进程绑定一个具体的容器私有 IP，而 Docker 只发布 `10.0.0.2:42817`；既有测试仍然拒绝通配绑定。
3. **REPO_REQUIRED** — `go test ./...`、`go vet ./...`、镜像构建和 `git diff --check` 通过。
4. **REQUESTED 旅程** — 真实浏览器在 `http://10.0.0.2:42817` 领取 bootstrap code，生成第二个 code，真实 iPhone 领取它。
5. **REQUESTED 旅程** — 浏览器发给 iPhone、iPhone 发给浏览器的消息，在两端都以完全相同的正文出现。
6. **REPO_REQUIRED** — `docker compose restart` 之后，已配对身份、消息和存储的文件都还在，并且不会签发新的 bootstrap code。
7. **DESIGN_NECESSARY kill probe** — 发布一个不同的/未配置的端口不会让已接受的 URL 成功；停掉容器会让旅程不可用。
8. **REPO_REQUIRED** — 对抗式自审和全新零上下文合约评审都没有未解决的 P0/P1/P2。

## 配对时代的历史旅程预期

- 浏览器入口：打开 `/` 显示 Ferry 的配对页面，而不是 API 错误或其他服务。
- Bootstrap：第一次成功领取返回一个 Web 设备身份；重放该 code 被拒绝。
- 第二台设备：Web 创建一个新的一次性 code；iPhone 配对后显示时间线，而不是一直停在连接中。
- 互发：浏览器发送 `from web via macmini 42817`，iPhone 显示完全相同的文字。iPhone 发送 `from iphone via macmini 42817`，Web 显示完全相同的文字。
- 重启：同一个 URL 恢复访问，两个 token 都仍有效，两条消息原样都在。

## 构建与评审记录

- `docker compose config` 默认使用 `127.0.0.1:42817` 和命名数据卷 `ferry-data:/data`；macmini 未跟踪的 `.env` 明确发布到 `10.0.0.2:42817`。
- 本地和 macmini 的多阶段构建都通过。运行中的 macmini 容器报告用户 `ferry`、状态 `running`、容器 IP `192.168.148.2`、宿主映射 `10.0.0.2:42817->42817/tcp`。
- 重建后的容器返回 HTTP 200，真实 Web 会话从命名数据卷中保留了已配对身份和消息。Server 日志写明了具体的私有监听地址和可信 HTTP 警告。
- 在改成只允许四位数字的 UI 之前，Web/iPhone 互发产生了完全一致的消息 `from web via macmini 42817` 和 `hiho`；更新后的 iPhone 构建已安装，新的四位数字领取等待用户手动确认。
- 配对合约攻击和 13 项作者评审记录在 `docs/four-digit-pairing.md`；全新零上下文评审仍是 push 前的最后一道门。
- 一次独立评审发现：即使进程绑定的是私有 bridge IP，Docker 发布仍可能指向公网主机。运行时现在会校验 `-published-host`；一个带敌意公网 IP 的容器探针在监听之前就退出，而一个合法的 loopback 冒烟容器以 `ferry` 身份运行并返回 HTTP 200。
- 严格的无空白 PIN 归一化上线后，macmini 重建了镜像 `sha256:209283f…`，用同一个命名数据卷重新创建容器，并在配置的局域网 URL 返回 HTTP 200。
