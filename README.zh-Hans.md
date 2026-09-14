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
  <p><a href="#快速开始"><strong>快速开始</strong></a> · <a href="#排错">排错</a> · <a href="README.md">English</a></p>
</div>

Ferry 是一个自托管的剪贴板与文件摆渡工具，服务于同一个可信私有网络里的设备。它长得像聊天：你发出的所有内容都落在一条时间线上，每台已接入的设备都能看到，所以把一个链接从手机挪到电脑，就是一次粘贴加一次复制，不用再给自己发邮件。

一个 Go 进程同时提供 Web 应用和 API，把消息和设备身份存进 SQLite，把上传的文件字节放在本地 blob 目录。任何内容都不会离开你运行它的网络，也不需要注册账号。

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

## 设备只需要一个浏览器

手机、平板和电脑在浏览器里打开 Ferry 的地址就能加入，不用在上面安装任何东西。Web 应用可以发送文字、链接、照片和文件，一点就能复制消息，能预览图片、下载文件。

iOS 和 Android 原生 App 已经有了，但它们是可选的，也不属于 1.0.0 版本的发布内容；见[原生 App](#原生-app)。

## 快速开始

你需要一台常开的电脑，装有 Docker 和 Docker Compose v2，系统是 64 位 x86（`amd64`）或 ARM（`arm64`）。不需要克隆本仓库，也不需要安装 Go。

1. **下载部署包**并进入目录：

   ```bash
   curl -fLO https://github.com/max1874/ferry/releases/download/v1.0.0/ferry-1.0.0-deploy.tar.gz
   tar -xzf ferry-1.0.0-deploy.tar.gz
   cd ferry
   ```

   部署包里有 `compose.yaml`、`.env.example` 和用于备份的 `scripts/ferry-data.sh`。请保持这个目录结构。

2. **创建配置文件：**

   ```bash
   cp .env.example .env
   ```

3. **查到这台电脑的局域网 IP。** 它通常以 `192.168.`、`10.` 或 `172.16.`–`172.31.` 开头。

   | 系统 | 命令 |
   | --- | --- |
   | macOS | `ipconfig getifaddr en0`（Wi-Fi）或 `ipconfig getifaddr en1` |
   | Linux | `hostname -I` |
   | Windows | `ipconfig`，看“IPv4 地址” |

4. **把它写进 `.env`。** 把 `FERRY_HOST_IP=127.0.0.1` 改成你的地址，例如：

   ```dotenv
   FERRY_HOST_IP=192.168.1.20
   ```

   保留 `127.0.0.1` 的话，只有这台电脑自己能访问 Ferry。

5. **启动 Ferry：**

   ```bash
   docker compose up -d
   docker compose logs ferry
   ```

   日志会写出要打开的地址，例如 `open Ferry at http://192.168.1.20:42817 from devices on the same private network`。

6. 在这台电脑的**浏览器里打开这个地址**。第一台设备会直接加入。

## 第一次在两台设备间传送

1. 在电脑上，点空时间线上的 **Connect another device**，或者打开 **Devices**。Ferry 会显示它的地址和一个二维码。
2. 在手机上，连到同一个 Wi-Fi 或私有网络，然后用相机扫码，或者在浏览器里输入这个地址。
3. 手机会直接加入。如果你设置过访问密码，它会先要求输入密码。
4. 在手机上粘贴一段文字并发送。它会出现在电脑上；点消息上的 **Copy**。
5. 在电脑上发送一张照片或一个文件。手机上图片直接预览，其他文件可以下载。

任何已加入的浏览器都可以在 **Devices → Access password** 里开启共享密码。它只拦截新设备；已加入的设备在被撤销前一直保持加入。

## 升级与备份

每次升级前先备份。在部署目录里运行：

```bash
scripts/ferry-data.sh backup ../ferry-backup-$(date +%F).tar.gz
```

然后只修改 `.env` 里 `FERRY_IMAGE` 末尾的版本号，拉取镜像并重建容器：

```bash
docker compose pull
docker compose up -d
docker compose logs --tail=100 ferry
```

恢复时，指定要恢复的归档，以及一个新的、保存当前数据的安全备份：

```bash
scripts/ferry-data.sh restore ../ferry-backup-2026-09-14.tar.gz ../before-restore.tar.gz
```

这个工具会短暂停止正在运行的 Ferry，让 SQLite 和 blob 一起归档；归档里如果有 Ferry 数据之外的东西，它会拒绝。恢复时先写好安全备份再替换数据卷，并且只有在恢复成功、且操作前 Ferry 本来在运行时才重启它。请把备份拷到服务器以外的地方；备份里有消息、文件、设备 token 的 hash 和密码校验值。

### 把现有的源码部署切换到镜像

如果你之前用 `git clone` 和 `docker compose up --build` 部署过更早的提交，请在同一个目录里升级，这样 Compose 会继续使用同一个数据卷：

```bash
scripts/ferry-data.sh backup ../ferry-backup-before-1.0.0.tar.gz
git pull --ff-only
docker compose pull
docker compose up -d
docker compose logs --tail=100 ferry
```

原有的 `.env`、消息、文件、设备身份和密码都会保留。`compose.yaml` 默认使用 `ghcr.io/max1874/ferry:1.0.0`；想固定版本，就在 `.env` 里加上 `FERRY_IMAGE`。如果想继续从源码构建，见[从源码构建镜像](#从源码构建镜像)。

## 其他部署方式

### 只在这台电脑上试用

跳过快速开始的第 3、4 步。这时 Ferry 监听 `http://127.0.0.1:42817`，只有这台电脑能打开；Devices 页面会提示你先改用局域网地址，才会显示二维码。

### 配置项

| 配置 | 默认值 | 含义 |
| --- | --- | --- |
| `FERRY_IMAGE` | `ghcr.io/max1874/ferry:1.0.0` | 运行的镜像和版本 |
| `FERRY_HOST_IP` | `127.0.0.1` | Docker 把端口发布到宿主机的哪个地址；必须是 loopback 或私有地址 |
| `FERRY_PORT` | `42817` | 宿主机和容器里使用的端口 |
| `FERRY_TRUSTED_ORIGIN` | 空 | 你自己的 TLS 反向代理的 origin |
| `FERRY_LISTEN_HOST` | 容器自己的 IP | Ferry 在容器里监听的地址 |

默认的监听方式要求容器恰好只有一个 IP 地址，否则启动失败。设置 `FERRY_LISTEN_HOST=0.0.0.0`（或 `::`）可以监听容器的所有网卡，比如容器加入了第二个 Docker 网络时。在普通的 bridge 容器里，`0.0.0.0` 仍然只能通过发布在 `FERRY_HOST_IP` 上的端口被外部访问。使用 `network_mode: host` 时，容器的网卡就是宿主机的网卡，防火墙放行的所有宿主机网卡都能访问 Ferry。

### 放在你已有的反向代理后面

如果要通过私有域名、用你自己的 TLS 代理访问 Ferry（比如在 VPN 里），要设置 Ferry 应该接受的那一个浏览器 origin。代理必须转发原始的 `Host` 头，Caddy 默认就会这样做。`Host` 或 `Origin` 指向其他域名的请求仍会被拒绝。代理到 Ferry 这一跳是明文 HTTP，所以要让它留在 loopback 或私有网络里。`reverse_proxy` 后面填什么，取决于代理跑在哪里。

**代理装在宿主机上，或者代理容器使用 `network_mode: host`。** 在 `.env` 里设置 `FERRY_TRUSTED_ORIGIN=https://ferry.example.com`，保留 `FERRY_HOST_IP=127.0.0.1`，连到发布的端口：

```Caddyfile
ferry.example.com {
  reverse_proxy 127.0.0.1:42817
}
```

**代理跑在自己的 bridge 容器里。** 这时代理容器里的 `127.0.0.1` 指向代理自己，所以要把 Ferry 接入代理所在的 Docker 网络，并用服务名访问。Ferry 这时有两个 IP 地址，所以要在 `.env` 里同时设置 `FERRY_TRUSTED_ORIGIN=https://ferry.example.com` 和 `FERRY_LISTEN_HOST=0.0.0.0`，并在 `compose.yaml` 旁边新建 `compose.override.yaml`：

```yaml
services:
  ferry:
    networks: [default, proxy]

networks:
  proxy:
    external: true
    name: caddy_default # 代理所在的网络名，用 `docker network ls` 查看
```

```Caddyfile
ferry.example.com {
  reverse_proxy ferry:42817
}
```

### 从源码构建镜像

在本仓库的克隆里，`compose.build.yaml` 会构建当前检出的源码，而不是拉取已发布的镜像。服务、数据卷和配置都保持不变：

```bash
docker compose -f compose.yaml -f compose.build.yaml up --build -d
```

### 不用 Docker，直接从源码运行

要求：Go 1.26.3 或更新版本。

```bash
go run ./cmd/ferry -listen 127.0.0.1:42817
go run ./cmd/ferry -lan -listen 192.168.1.20:42817   # 局域网可访问
go run ./cmd/ferry -lan -listen 0.0.0.0:42817        # 所有网卡；Ferry 会在日志里警告
go run ./cmd/ferry -listen 127.0.0.1:42817 -trusted-origin https://ferry.example.com
```

本地状态写到被 git 忽略的 `./ferry-data` 目录；用 `-data-dir` 可以换一个位置。

## 排错

| 症状 | 可能原因与解决办法 |
| --- | --- |
| 手机打不开地址 | 日志里写着 `on this computer only`：把 `.env` 里的 `FERRY_HOST_IP` 改成服务器的局域网 IP，再运行 `docker compose up -d`。否则检查手机是否在同一个网络里，而不是开了客户端隔离的访客 Wi-Fi 或移动网络，以及服务器防火墙是否放行了这个端口。 |
| Devices 页面不显示二维码 | 页面是通过 `localhost` 或 `127.0.0.1` 打开的。改用服务器的局域网 IP 打开 Ferry，再从那里扫码。 |
| 代理返回 `421 Misdirected Request`，错误码 `invalid_host` | 没有设置 `FERRY_TRUSTED_ORIGIN`，或者它和浏览器里的地址不完全一致，或者代理改写了 `Host`。 |
| 容器退出并提示 `expected exactly one container IP address` | 容器接入了不止一个 Docker 网络。在 `.env` 里设置 `FERRY_LISTEN_HOST=0.0.0.0`。 |
| `published-host must be a loopback or private/link-local IP address` | `FERRY_HOST_IP` 是公网地址或主机名。改用服务器的私有 IP。 |
| `bind: address already in use` 或 `port is already allocated` | 端口 42817 被其他程序占用。在 `.env` 里换一个 `FERRY_PORT`，并打开新的地址。 |
| `docker compose pull` 失败 | `denied` 或 `unauthorized`：运行 `docker logout ghcr.io` 清掉过期凭据，镜像是公开的。`no matching manifest`：服务器不是 `amd64` 或 `arm64`，比如 32 位的 Raspberry Pi OS，请在那里从源码构建。超时：检查服务器能否访问互联网，或代理设置。 |

## 原生 App

Web 应用是在所有设备上使用 Ferry 的受支持方式。原生 App 连接同一个 Server，但不随本次版本发布。

| App | 当前状态 |
| --- | --- |
| iOS | 面向 iOS 26 的原生 SwiftUI MVP；构建只以 FerryDrop 之名分发给内部 TestFlight 测试者，没有公开下载 |
| Android | 面向 Android 8+ 的原生 Compose MVP；已确认真机安装和启动；需要自行构建，没有发布 APK |

**iOS。** 用 Xcode 26.6 或更新版本打开 `ios/Ferry/Ferry.xcodeproj`，运行 `Ferry` scheme。模拟器构建不需要 Team；真机需要你的 Apple Development Team。输入 Server 地址、设备名，以及可选的密码。要打 archive 时，在 Xcode 里设置 Team，并把证书和描述文件留在 Git 之外：

```bash
xcodebuild archive -project ios/Ferry/Ferry.xcodeproj -scheme Ferry \
  -destination 'generic/platform=iOS' -archivePath /tmp/Ferry.xcarchive \
  DEVELOPMENT_TEAM=YOUR_TEAM_ID
```

商店导出是需要维护者授权的一步。获得授权后，把 `ios/ExportOptions.plist.example` 复制成被忽略的 `ios/ExportOptions.plist`，替换 `YOUR_TEAM_ID`，再把这个文件传给 `xcodebuild -exportArchive`。

**Android。** 用 Android Studio 打开 `android/`，或者用 JDK 17 和 Android SDK 35 构建 debug APK：

```bash
cd android
./gradlew :app:assembleDebug
```

APK 生成在 `android/app/build/outputs/apk/debug/` 下。要做签名 release，把 `android/signing.properties.example` 复制成被忽略的 `android/signing.properties`，把权限限制到当前用户，在本地创建它引用的 keystore，然后运行 `./gradlew :app:bundleRelease`。任何会打包 release 的 Gradle 任务图，在签名配置缺失或不完整时都会失败。

## 开发

| 组件 | 当前状态 |
| --- | --- |
| Server + Web | Go/SQLite；从 1.0.0 起以双架构容器镜像发布 |
| iOS App | 原生 SwiftUI MVP；仅限内部 TestFlight |
| Android App | 原生 Compose MVP；从源码构建 |

自动发现、剪贴板同步、后台传输、TLS/公网暴露和 App Store 上架都不在当前里程碑内。

贡献者需要：Go 1.26.3、Docker 和 Compose v2，按需安装 Xcode 26.6，或 Android Studio、JDK 17 和 SDK 35。GitHub Actions 运行的检查是：

```bash
scripts/check-repo.sh
go test -race -count=1 ./...
go vet ./...
node --check internal/webui/assets/app.js
docker compose config
scripts/ferry-data.sh self-test
(cd android && ./gradlew :app:testDebugUnitTest :app:lintDebug :app:assembleDebug)
xcodebuild test -project ios/Ferry/Ferry.xcodeproj -scheme Ferry \
  -destination 'platform=iOS Simulator,name=iPhone 17 Pro,OS=latest' \
  -only-testing:FerryTests \
  -derivedDataPath /tmp/FerryDerivedData
```

CI 还会构建 `linux/amd64` 和 `linux/arm64` 镜像，分别启动两个架构，检查数据能挺过重启、也能从源码构建切换过来，并把结果保留为发布候选。候选如何变成正式版本，见 [docs/release-process.zh-Hans.md](docs/release-process.zh-Hans.md)。

提交改动前请先读 [CONTRIBUTING.zh-Hans.md](CONTRIBUTING.zh-Hans.md)。产品边界在 [docs/product-core.zh-Hans.md](docs/product-core.zh-Hans.md)，HTTP 合约是 [api/openapi.yaml](api/openapi.yaml)。

## 安全

Ferry 在可信私有网络里使用未加密的 HTTP，请不要把它暴露到公网。所有内容、设置和设备接口都需要可撤销的设备 Bearer token；可选的共享密码只拦截新设备。部署前或报告漏洞前，请先读 [SECURITY.zh-Hans.md](SECURITY.zh-Hans.md)。

## 许可证

Ferry 以 [Apache License 2.0](LICENSE) 授权。Web 应用内置了 [Tabler Icons](internal/webui/assets/tabler-icons-LICENSE.txt) 和 [QR Code generator library](internal/webui/assets/qrcodegen-LICENSE.txt)，两者均为 MIT 许可证。
