<div align="center">
  <img src="AppIcon.appiconset/icon_256x256.png" width="160" alt="Ferry 应用图标">
  <h1>Ferry</h1>
  <p><strong>你的设备，一条时间线，就在你自己的网络里。</strong></p>
  <p>
    <img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white">
    <img alt="iOS 26+" src="https://img.shields.io/badge/iOS-26%2B-111827?logo=apple">
    <img alt="Android 8+" src="https://img.shields.io/badge/Android-8%2B-3DDC84?logo=android&logoColor=white">
    <img alt="Apache 2.0 License" src="https://img.shields.io/badge/license-Apache%202.0-22c55e">
    <img alt="No cloud account" src="https://img.shields.io/badge/cloud%20account-none-06b6d4">
  </p>
  <p><a href="#如何运行-ferry"><strong>两条命令跑起来</strong></a> · <a href="README.md">English</a></p>
</div>

Ferry 是一个自托管的剪贴板与文件摆渡工具，服务于同一个可信局域网里的设备。它长得像聊天：你发出的所有内容都落在一条时间线上，每台已接入的设备都能看到，所以把一个链接从手机挪到电脑，就是一次粘贴加一次复制，不用再给自己发邮件。

一个 Go 进程同时提供 Web 应用和 API，把消息和设备身份存进 SQLite，把上传的文件字节放在本地 blob 目录。iOS 和 Android 原生 App 读的是和浏览器同一条时间线。任何内容都不会离开你运行它的网络，也不需要注册账号。

<p align="center">
  <img src="docs/screenshot-web.png" width="820" alt="Ferry Web 时间线：一个分享的链接、一条 shell 命令、一个文本文件和一张图片，每条都标注了发送设备">
</p>

## 为什么不直接给自己发消息？

因为来回一趟就是成本。在聊天软件里给自己发消息，会把你的剪贴板内容发到别人的服务器上，会压缩你的截图，还要求每台设备都登录账号。AirDrop 要求两台设备都醒着、解锁、在同一个房间，而且没有可以往回翻的历史。

| 方面 | Ferry 的做法 |
| --- | --- |
| 内容存在哪 | 你自己运行的一个进程，在你自己的硬件上 |
| 账号 | 没有；设备凭可选的共享密码加入，拿到一个可撤销的 token |
| 历史 | 一条持久的时间线，而不是传完就消失的一次传输 |
| 文件 | 按上传原样存储和提供，最大 64 MB，不重新编码 |
| 剪贴板 | 只由你自己的粘贴快捷键和复制控件读写 |
| 网络 | 默认只监听 loopback；局域网地址和监听所有网卡都要显式开启；从不绑定主机名或公网地址 |

## 功能

- **是时间线，不是一次传输。** 文字、链接和文件按顺序排列，标注发送设备，明天还在。
- **图片内联显示。** 照片和截图直接显示在消息里，点开全屏，并能保存到你手上这台设备。
- **剪贴板归你自己。** Ferry 从不在后台读写剪贴板。用系统快捷键粘贴进来，用消息上的复制控件拿出去。
- **四个入口，一个服务器。** Web、iOS 和 Android 都通过同一份有文档的 HTTP API 连接同一个 Go 进程。
- **设备可撤销。** 所有内容、设置和设备接口都需要设备 token，任何已接入的浏览器都能撤销它。
- **从构造上保证私密。** 没有账号、没有遥测、没有第三方服务，也没有离开局域网的路径。

## 环境要求

- 托管 Ferry 的机器上装有 Docker 和 Compose v2，或者用 Go 1.26.3 从源码运行
- 一个可信的私有网络；Ferry 拒绝绑定主机名或公网地址
- 可选：iOS App 需要 Xcode 26.6，Android App 需要 Android Studio、JDK 17 和 SDK 35

## 如何运行 Ferry

1. 把 Ferry 发布到一个具体的私有地址和一个高位端口：

   ```bash
   FERRY_HOST_IP=192.168.1.20 FERRY_PORT=42817 docker compose up --build -d
   docker compose logs ferry
   ```

2. 在该网络里的任意设备上打开 `http://192.168.1.20:42817`。不提供 `FERRY_HOST_IP` 时，Compose 默认发布到 `127.0.0.1:42817`。

3. 没有配置访问密码时，第一个浏览器或 App 直接加入。任何已连接的 Web 设备都可以在 **Devices → Access password** 里开启、修改或关闭共享密码。密码设置存在 Ferry 的 SQLite 数据库里，不在部署配置里。

请把 Ferry 留在可信的私有网络里，不要暴露到公网；见[安全](#安全)。

### 容器监听地址

谁能访问容器，由两个设置决定：

| 设置 | 默认值 | 含义 |
| --- | --- | --- |
| `FERRY_HOST_IP` | `127.0.0.1` | Docker 把端口发布到宿主机的哪个地址；必须是 loopback 或私有地址 |
| `FERRY_LISTEN_HOST` | 容器自己的 IP | Ferry 在容器里监听的地址 |

默认的监听方式要求容器恰好只有一个 IP 地址，否则启动失败。设置 `FERRY_LISTEN_HOST=0.0.0.0`（或 `::`）可以监听容器的所有网卡，比如容器加入了第二个 Docker 网络时：

```bash
FERRY_LISTEN_HOST=0.0.0.0 docker compose up --build -d
```

在普通的 bridge 容器里，`0.0.0.0` 仍然只能通过发布在 `FERRY_HOST_IP` 上的端口被外部访问。使用 `network_mode: host` 时，容器的网卡就是宿主机的网卡，防火墙放行的所有宿主机网卡都能访问 Ferry。

不用 Docker 时，对应的写法是 `-lan -listen 0.0.0.0:42817`；Ferry 监听所有网卡时会在日志里给出警告。

### 放在反向代理后面

如果要通过私有域名、用你自己的 TLS 代理访问 Ferry（比如在 VPN 里），要告诉 Ferry 它应该接受的那一个浏览器 origin。代理必须转发原始的 `Host` 头，Caddy 默认就会这样做。`Host` 或 `Origin` 指向其他域名的请求仍会被拒绝。代理到 Ferry 这一跳是明文 HTTP，所以要让它留在 loopback 或私有网络里。

`reverse_proxy` 后面填什么，取决于代理跑在哪里。

**代理装在宿主机上，或者代理容器使用 `network_mode: host`。** 连到 Compose 发布在 loopback 上的端口：

```bash
FERRY_TRUSTED_ORIGIN=https://ferry.example.com docker compose up --build -d
```

```Caddyfile
ferry.example.com {
  reverse_proxy 127.0.0.1:42817
}
```

**代理跑在自己的 bridge 容器里。** 这时代理容器里的 `127.0.0.1` 指向代理自己，所以要把 Ferry 接入代理所在的 Docker 网络，并用服务名访问。Ferry 这时有两个 IP 地址，必须监听容器的所有网卡。在 `compose.yaml` 旁边新建 `compose.override.yaml`：

```yaml
services:
  ferry:
    networks: [default, proxy]

networks:
  proxy:
    external: true
    name: caddy_default # 代理所在的网络名，用 `docker network ls` 查看
```

```bash
FERRY_LISTEN_HOST=0.0.0.0 FERRY_TRUSTED_ORIGIN=https://ferry.example.com docker compose up --build -d
```

```Caddyfile
ferry.example.com {
  reverse_proxy ferry:42817
}
```

**从源码运行。** 用参数传入 origin，代理指向 loopback 监听地址：

```bash
go run ./cmd/ferry -listen 127.0.0.1:42817 -trusted-origin https://ferry.example.com
```

### 备份与恢复

先构建一次镜像，然后在仓库根目录使用数据工具：

```bash
scripts/ferry-data.sh backup ./ferry-backup-2026-08-31.tar.gz
scripts/ferry-data.sh restore ./ferry-backup-2026-08-31.tar.gz ./before-restore.tar.gz
```

这个工具会短暂停止正在运行的 Ferry 服务，让 SQLite 和 blob 一起归档。如果数据卷里有不支持的条目，它不会宣称快照成功。恢复时，它会先校验归档的私有副本，创建并校验你指定的安全备份，替换命名数据卷，并且只在恢复成功、且操作前服务本来在运行时才重启 Ferry。恢复失败时服务保持停止，避免对外提供不完整的数据。请把备份拷到 Server 主机以外的地方；备份里有消息、文件、设备 token 的 hash 和密码校验值。

备份之后再升级：

```bash
git pull --ff-only
docker compose build --pull ferry
docker compose up -d ferry
docker compose logs --tail=100 ferry
```

## 从源码运行

要求：Go 1.26.3 或更新版本。

```bash
go run ./cmd/ferry -listen 127.0.0.1:42817
```

要在局域网访问，使用一个明确的私有地址：

```bash
go run ./cmd/ferry -lan -listen 192.168.1.20:42817
```

本地状态默认写到被 git 忽略的 `./ferry-data` 目录。用 `-data-dir` 可以换一个位置。

## 原生 App

### iOS

用 Xcode 26.6 或更新版本打开 `ios/Ferry/Ferry.xcodeproj`，运行 `Ferry` scheme。模拟器构建不需要 Team；真机需要你的 Apple Development Team。输入 Docker 的地址、设备名，以及在 Web 里配置的可选密码。

要打 archive 时，在 Xcode 里设置 Team，并把证书和描述文件留在 Git 之外：

```bash
xcodebuild archive -project ios/Ferry/Ferry.xcodeproj -scheme Ferry \
  -destination 'generic/platform=iOS' -archivePath /tmp/Ferry.xcarchive \
  DEVELOPMENT_TEAM=YOUR_TEAM_ID
```

商店导出仍然是需要维护者授权的一步；本仓库不包含 Apple 凭据，也没有 App Store 导出配置。在明确获得发布授权后，把 `ios/ExportOptions.plist.example` 复制成被忽略的 `ios/ExportOptions.plist`，替换 `YOUR_TEAM_ID`，再把这个本地文件传给 `xcodebuild -exportArchive`。

### Android

用 Android Studio 打开 `android/`，或者用 JDK 17 和 Android SDK 35 构建 debug APK：

```bash
cd android
./gradlew :app:assembleDebug
```

APK 生成在 `android/app/build/outputs/apk/debug/` 下。要配置签名 release，把 `android/signing.properties.example` 复制成被忽略的 `android/signing.properties`，把权限限制到当前用户，在本地创建它引用的 keystore，然后运行 `./gradlew :app:bundleRelease`。任何会打包 release 的 Gradle 任务图，在签名配置缺失或不完整时都会失败。

## 项目状态

Ferry 正在积极开发中，还没有打过版本 tag。

| 组件 | 当前状态 |
| --- | --- |
| Server + Web | Go/SQLite MVP，与 Docker Compose 一起部署 |
| iOS App | 面向 iOS 26 的原生 SwiftUI MVP；构建以 FerryDrop 之名分发给内部 TestFlight 测试者 |
| Android App | 面向 Android 8+ 的原生 Compose MVP；已确认真机安装和启动 |

自动发现、剪贴板同步、后台传输、TLS/公网暴露和 App Store 上架都不在当前里程碑内。TestFlight 是 iOS 唯一的分发渠道。

## 开发

运行与你的改动相匹配的本地检查：

```bash
scripts/check-repo.sh
go test -race -count=1 ./...
go vet ./...
node --check internal/webui/assets/app.js
docker compose config
(cd android && ./gradlew :app:testDebugUnitTest :app:lintDebug :app:assembleDebug)
xcodebuild test -project ios/Ferry/Ferry.xcodeproj -scheme Ferry \
  -destination 'platform=iOS Simulator,name=iPhone 17 Pro,OS=latest' \
  -only-testing:FerryTests \
  -derivedDataPath /tmp/FerryDerivedData
```

GitHub Actions 会运行这些 Server/Web、Android 和 iOS 检查。提交改动前请先读 [CONTRIBUTING.zh-Hans.md](CONTRIBUTING.zh-Hans.md)。产品边界在 [docs/product-core.zh-Hans.md](docs/product-core.zh-Hans.md)，HTTP 合约是 [api/openapi.yaml](api/openapi.yaml)。

## 安全

Ferry 在可信局域网里使用未加密的 HTTP。所有内容、设置和设备接口都需要可撤销的设备 Bearer token；可选的共享密码只拦截新设备。部署前或报告漏洞前，请先读 [SECURITY.zh-Hans.md](SECURITY.zh-Hans.md)。

## 许可证

Ferry 以 [Apache License 2.0](LICENSE) 授权。
