<div align="center">
  <img src="AppIcon.appiconset/icon_256x256.png" width="160" alt="Ferry app icon">
  <h1>Ferry</h1>
  <p><strong>Your devices, one timeline, on your own network.</strong></p>
  <p>
    <img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white">
    <img alt="iOS 26+" src="https://img.shields.io/badge/iOS-26%2B-111827?logo=apple">
    <img alt="Android 8+" src="https://img.shields.io/badge/Android-8%2B-3DDC84?logo=android&logoColor=white">
    <img alt="Apache 2.0 License" src="https://img.shields.io/badge/license-Apache%202.0-22c55e">
    <img alt="No cloud account" src="https://img.shields.io/badge/cloud%20account-none-06b6d4">
  </p>
  <p><a href="#quick-start"><strong>Quick start</strong></a> · <a href="#troubleshooting">Troubleshooting</a> · <a href="README.zh-Hans.md">简体中文</a></p>
</div>

Ferry is a self-hosted clipboard and file ferry for the devices on one trusted private network. It looks like a chat: everything you send lands in a single timeline that every joined device can read, so moving a link from your phone to your laptop is a paste and a copy, not an email to yourself.

One Go process serves the Web app and the API, stores messages and device identities in SQLite, and keeps uploaded bytes in a local blob directory. Nothing leaves the network you run it on, and there is no account to create.

<p align="center">
  <img src="docs/screenshot-web.png" width="820" alt="The Ferry web timeline: a shared link, a shell command, a text file and an image, each labelled with the device that sent it">
</p>

## Why not just message yourself?

Because the round trip is the cost. A self-chat in a messaging app sends your clipboard to somebody else's servers, compresses your screenshots, and needs an account on every device. AirDrop needs both devices awake, unlocked and in the same room, and it has no history to scroll back through.

| Area | Ferry behavior |
| --- | --- |
| Where content lives | One process you run, on hardware you own |
| Account | None; a device joins with an optional shared password and gets a revocable token |
| History | A persistent timeline, not a transfer that disappears when it lands |
| Files | Stored and served as uploaded, up to 64 MB, no re-encoding |
| Clipboard | Read and written only by your own paste shortcut and copy control |
| Network | Loopback by default; LAN and all-interface listeners are explicit opt-ins; never a hostname or public address |

## A browser is all a device needs

Phones, tablets and computers join Ferry by opening its address in a browser. There is nothing to install on them. The Web app sends text, links, photos and files, copies a message with one tap, previews images and downloads files.

Native iOS and Android apps exist, but they are optional and are not part of the 1.0.0 release; see [Native apps](#native-apps).

## Quick start

You need one computer that stays on, with Docker and Docker Compose v2, on a 64-bit x86 (`amd64`) or ARM (`arm64`) system. You do not need to clone this repository or install Go.

1. **Download the Compose files** from the [latest release](https://github.com/max1874/ferry/releases/latest) and enter their directory:

   ```bash
   curl -fLO https://github.com/max1874/ferry/releases/download/v1.0.0/ferry-1.0.0-docker-compose.tar.gz
   tar -xzf ferry-1.0.0-docker-compose.tar.gz
   cd ferry
   ```

   The download holds `compose.yaml`, `.env.example` and `scripts/ferry-data.sh` for backups; keep that layout. It is not the program itself: Compose pulls the public image `ghcr.io/max1874/ferry:1.0.0` when Ferry starts.

2. **Create your settings file:**

   ```bash
   cp .env.example .env
   ```

3. **Find this computer's LAN IP address.** It usually starts with `192.168.`, `10.` or `172.16.`–`172.31.`.

   | System | Command |
   | --- | --- |
   | macOS | `ipconfig getifaddr en0` (Wi-Fi) or `ipconfig getifaddr en1` |
   | Linux | `hostname -I` |
   | Windows | `ipconfig`, then read "IPv4 Address" |

4. **Write it into `.env`.** Change `FERRY_HOST_IP=127.0.0.1` to your address, for example:

   ```dotenv
   FERRY_HOST_IP=192.168.1.20
   ```

   Leaving `127.0.0.1` keeps Ferry reachable from this computer only.

5. **Start Ferry:**

   ```bash
   docker compose up -d
   docker compose logs ferry
   ```

   The log names the address to open, for example `open Ferry at http://192.168.1.20:42817 from devices on the same private network`.

6. **Open that address in a browser** on this computer. The first device joins directly.

## Your first transfer between two devices

1. On the computer, choose **Connect another device** on the empty timeline, or open **Devices**. Ferry shows its address and a QR code.
2. On your phone, join the same Wi-Fi or private network, then scan the code with the camera, or type the address into the browser.
3. The phone joins directly. If you have set an access password, it asks for that password first.
4. Paste some text on the phone and send it. It appears on the computer; choose **Copy** on the message.
5. Send a photo or file from the computer. The phone previews images inline and downloads other files.

Any joined browser can turn on the shared password in **Devices → Access password**. It gates only new devices; joined devices stay joined until revoked.

## Upgrade and back up

Back up before every upgrade. From the deployment directory:

```bash
scripts/ferry-data.sh backup ../ferry-backup-$(date +%F).tar.gz
```

Then change only the version at the end of `FERRY_IMAGE` in `.env`, pull and recreate the container:

```bash
docker compose pull
docker compose up -d
docker compose logs --tail=100 ferry
```

To restore, name the archive and a new safety backup of the current data:

```bash
scripts/ferry-data.sh restore ../ferry-backup-2026-09-14.tar.gz ../before-restore.tar.gz
```

The tool briefly stops a running Ferry so SQLite and blobs are archived together, and refuses archives that contain anything other than Ferry's data. Restore writes the safety backup before it replaces the volume, and restarts Ferry only after a successful restore that began with Ferry running. Copy backups off the server; they contain messages, files, hashed device tokens and the password verifier.

### Moving an existing source deployment to the image

If you deployed Ferry from a `git clone` with `docker compose up --build`, upgrade in the same directory so Compose keeps the same volume:

```bash
scripts/ferry-data.sh backup ../ferry-backup-before-1.0.0.tar.gz
git pull --ff-only
docker compose pull
docker compose up -d
docker compose logs --tail=100 ferry
```

If your `.env` sets `COMPOSE_FILE` to include `compose.build.yaml`, remove that file from the line first, keeping any override file, or Compose keeps building the source instead of using the image. Messages, files, device identities and the password carry over. To keep building from source instead, see [Build the image from source](#build-the-image-from-source).

## Other ways to deploy

### Try it on this computer only

Skip steps 3 and 4 of the quick start. Ferry then listens on `http://127.0.0.1:42817`, which only this computer can open, and the Devices page tells you to use a LAN address before it shows a QR code.

### Settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `FERRY_IMAGE` | `ghcr.io/max1874/ferry:1.0.0` | Image and version to run |
| `FERRY_HOST_IP` | `127.0.0.1` | Host address Docker publishes the port on; must be loopback or private |
| `FERRY_PORT` | `42817` | Port on the host and in the container |
| `FERRY_TRUSTED_ORIGIN` | empty | Origin of your own TLS reverse proxy |
| `FERRY_LISTEN_HOST` | the container's own IP | Address Ferry listens on inside the container |

The default listener needs the container to have exactly one IP address, and startup fails otherwise. Set `FERRY_LISTEN_HOST=0.0.0.0` (or `::`) to listen on every interface of the container, for example when it joins a second Docker network. In a normal bridge container, `0.0.0.0` still reaches the outside only through the port published on `FERRY_HOST_IP`. With `network_mode: host` the container's interfaces are the host's, so every host interface the firewall allows can reach Ferry.

### Behind a reverse proxy you already run

To reach Ferry through a private domain with your own TLS proxy (for example on a VPN), set the one browser origin Ferry should accept. The proxy must forward the original `Host` header, which Caddy does by default. Requests whose `Host` or `Origin` names any other domain are still rejected. The proxy-to-Ferry hop is plain HTTP, so keep it on loopback or a private network. Where `reverse_proxy` points depends on where the proxy runs.

**Proxy on the host, or a proxy container with `network_mode: host`.** Set `FERRY_TRUSTED_ORIGIN=https://ferry.example.com` in `.env`, keep `FERRY_HOST_IP=127.0.0.1`, and connect to the published port:

```Caddyfile
ferry.example.com {
  reverse_proxy 127.0.0.1:42817
}
```

**Proxy in its own bridge container.** Inside that container `127.0.0.1` is the proxy itself, so attach Ferry to the proxy's Docker network and use the service name. Ferry then has two IP addresses, so set both `FERRY_TRUSTED_ORIGIN=https://ferry.example.com` and `FERRY_LISTEN_HOST=0.0.0.0` in `.env`, and add a `compose.override.yaml` next to `compose.yaml`. Compose loads that file automatically only when no files are named; with `COMPOSE_FILE` or `-f`, list it last, as in `COMPOSE_FILE=compose.yaml:compose.build.yaml:compose.override.yaml`, or the container loses the proxy network. This setup has not yet been tested end to end; please open an issue if it misbehaves.

```yaml
services:
  ferry:
    networks: [default, proxy]

networks:
  proxy:
    external: true
    name: caddy_default # the proxy's network; see `docker network ls`
```

```Caddyfile
ferry.example.com {
  reverse_proxy ferry:42817
}
```

### Build the image from source

From a clone of this repository, `compose.build.yaml` builds the current checkout instead of pulling the published image. It uses the same service, volume and settings. To use it for every command, including the backup script, add `COMPOSE_FILE=compose.yaml:compose.build.yaml` to `.env` (separate the files with `;` on Windows) and run `docker compose up --build -d`. The one-off equivalent is:

```bash
docker compose -f compose.yaml -f compose.build.yaml up --build -d
```

Naming files turns off automatic loading of `compose.override.yaml`. If you use one, such as the proxy network above, name it too:

```bash
docker compose -f compose.yaml -f compose.build.yaml -f compose.override.yaml up --build -d
```

### Run from source without Docker

Requirements: Go 1.26.3 or newer.

```bash
go run ./cmd/ferry -listen 127.0.0.1:42817
go run ./cmd/ferry -lan -listen 192.168.1.20:42817   # reachable on the LAN
go run ./cmd/ferry -lan -listen 0.0.0.0:42817        # every interface; Ferry logs a warning
go run ./cmd/ferry -listen 127.0.0.1:42817 -trusted-origin https://ferry.example.com
```

Local state goes to the ignored `./ferry-data` directory; use `-data-dir` to choose another.

## Troubleshooting

| Symptom | Likely cause and fix |
| --- | --- |
| The phone cannot open the address | The log says `on this computer only`: set `FERRY_HOST_IP` in `.env` to the server's LAN IP and run `docker compose up -d`. Otherwise check that the phone is on the same network, not a guest Wi-Fi with client isolation or a mobile connection, and that the server's firewall allows the port. |
| The Devices page shows no QR code | The page is open at `localhost` or `127.0.0.1`. Open Ferry through the server's LAN IP, then use the QR code from there. |
| The proxy returns `421 Misdirected Request` with `invalid_host` | `FERRY_TRUSTED_ORIGIN` is missing or does not exactly match the address in the browser, or the proxy rewrites `Host`. |
| The container exits with `expected exactly one container IP address` | The container joined more than one Docker network. Set `FERRY_LISTEN_HOST=0.0.0.0` in `.env`. |
| `published-host must be a loopback or private/link-local IP address` | `FERRY_HOST_IP` is a public address or a hostname. Use the server's private IP. |
| `bind: address already in use` or `port is already allocated` | Another program uses port 42817. Set another `FERRY_PORT` in `.env` and open the new address. |
| `docker compose pull` fails | `denied` or `unauthorized`: run `docker logout ghcr.io` to drop stale credentials; the image is public. `no matching manifest`: the server is not `amd64` or `arm64`, such as a 32-bit Raspberry Pi OS; build from source there. A timeout: check the server's Internet access or proxy. |

## Native apps

The Web app is the supported way to use Ferry on every device. The native apps talk to the same Server and are not published by this release.

| App | Current state |
| --- | --- |
| iOS | Native SwiftUI MVP for iOS 26; builds go only to internal TestFlight testers under the name FerryDrop, with no public download |
| Android | Native Compose MVP for Android 8+; real-device install and launch confirmed; build it yourself, no published APK |

**iOS.** Open `ios/Ferry/Ferry.xcodeproj` in Xcode 26.6 or newer and run the `Ferry` scheme. Simulator builds need no Team; a physical device requires your Apple Development Team. Enter the Server URL, a device name and the optional password. For an archive, set the Team in Xcode and keep certificates and profiles outside Git:

```bash
xcodebuild archive -project ios/Ferry/Ferry.xcodeproj -scheme Ferry \
  -destination 'generic/platform=iOS' -archivePath /tmp/Ferry.xcarchive \
  DEVELOPMENT_TEAM=YOUR_TEAM_ID
```

Store export is a maintainer-authorized step. When it is authorized, copy `ios/ExportOptions.plist.example` to the ignored `ios/ExportOptions.plist`, replace `YOUR_TEAM_ID`, and pass that file to `xcodebuild -exportArchive`.

**Android.** Open `android/` in Android Studio, or build the debug APK with JDK 17 and Android SDK 35:

```bash
cd android
./gradlew :app:assembleDebug
```

The APK is produced below `android/app/build/outputs/apk/debug/`. For a signed release, copy `android/signing.properties.example` to the ignored `android/signing.properties`, restrict it to the current user, create the referenced keystore locally, and run `./gradlew :app:bundleRelease`. Any Gradle task graph that packages a release fails if signing configuration is absent or incomplete.

## Development

| Component | Current state |
| --- | --- |
| Server + Web | Go/SQLite; published as a two-architecture container image from 1.0.0 |
| iOS App | Native SwiftUI MVP; internal TestFlight only |
| Android App | Native Compose MVP; built from source |

Automatic discovery, clipboard synchronisation, background transfer, TLS/public-Internet exposure and App Store publication are not part of the current milestone.

Requirements for contributors: Go 1.26.3, Docker with Compose v2, and optionally Xcode 26.6 or Android Studio with JDK 17 and SDK 35. The gates GitHub Actions runs are:

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

CI also builds the `linux/amd64` and `linux/arm64` image, starts each architecture, checks that data survives a restart and a move from a source build, and keeps the result as the release candidate. [docs/release-process.md](docs/release-process.md) describes how a candidate becomes a release.

See [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a change. Product boundaries live in [docs/product-core.md](docs/product-core.md), and [api/openapi.yaml](api/openapi.yaml) is the HTTP contract.

## Security

Ferry uses unencrypted HTTP on a trusted private network. Keep it off the public Internet. Every content, settings and device endpoint requires a revocable device Bearer token; the optional shared password gates only new devices. Read [SECURITY.md](SECURITY.md) before deployment or vulnerability reporting.

## License

Ferry is licensed under the [Apache License 2.0](LICENSE). The Web app bundles [Tabler Icons](internal/webui/assets/tabler-icons-LICENSE.txt) and the [QR Code generator library](internal/webui/assets/qrcodegen-LICENSE.txt), both under the MIT License.
