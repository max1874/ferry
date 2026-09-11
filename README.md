# Ferry

Ferry is a self-hosted, chat-shaped clipboard and file ferry for devices on the same trusted local network.

One Go process serves the Web app and API, stores messages and device identities in SQLite, and keeps uploaded bytes in a local blob directory. Native iOS and Android apps use the same timeline as the browser.

![The Ferry web timeline: a shared link, a command, a text file and an image, each labelled with the device that sent it](docs/screenshot-web.png)

## Project status

Ferry is under active development and has no tagged release.

| Component | Current state |
| --- | --- |
| Server + Web | Go/SQLite MVP, deployed together with Docker Compose |
| iOS App | Native SwiftUI MVP for iOS 26 |
| Android App | Native Compose MVP for Android 8+; real-device install and launch confirmed |

Ferry never reads or writes a device's clipboard on its own. You put content in with the system's own paste shortcut and take it out with a copy control on the message; nothing is synchronised in the background.

Automatic discovery, clipboard synchronisation, background transfer, TLS/public-Internet exposure and store publication are not part of the current milestone.

## Docker quick start

Requirements: Docker with Compose v2.

Publish Ferry on one specific trusted-LAN address and a high port:

```bash
FERRY_HOST_IP=192.168.1.20 FERRY_PORT=42817 docker compose up --build -d
docker compose logs ferry
```

Open `http://192.168.1.20:42817`. Compose defaults to `127.0.0.1:42817` unless `FERRY_HOST_IP` is supplied. Ferry rejects wildcard, hostname and public-IP publication; keep it on a trusted private network and do not expose it to the public Internet.

The first browser or App joins directly when no access password is configured. Any connected Web device can enable, change or disable the shared password in **Devices → Access password**. The password setting lives in Ferry's SQLite database, not deployment configuration.

### Back up and restore

Build the image once, then use the data tool from the repository root:

```bash
scripts/ferry-data.sh backup ./ferry-backup-2026-08-31.tar.gz
scripts/ferry-data.sh restore ./ferry-backup-2026-08-31.tar.gz ./before-restore.tar.gz
```

The tool briefly stops a running Ferry service so SQLite and blobs are archived together. It refuses to call a snapshot successful if the volume contains unsupported entries. Restore validates a private copy of the archive, creates and validates the requested safety backup, replaces the named volume, and restarts Ferry only after a successful restore and only if it was running before the operation. A failed restore leaves the service stopped so partial data is not served. Copy backups away from the Server host; they contain messages, files, hashed device tokens and the password verifier.

Upgrade after taking a backup:

```bash
git pull --ff-only
docker compose build --pull ferry
docker compose up -d ferry
docker compose logs --tail=100 ferry
```

## Run from source

Requirements: Go 1.26.3 or newer.

```bash
go run ./cmd/ferry -listen 127.0.0.1:42817
```

For LAN access, use an explicit private address:

```bash
go run ./cmd/ferry -lan -listen 192.168.1.20:42817
```

By default local state is written to the ignored `./ferry-data` directory. Use `-data-dir` to choose another location.

## Native apps

### iOS

Open `ios/Ferry/Ferry.xcodeproj` in Xcode 26.6 or newer and run the `Ferry` scheme. Simulator builds need no Team; a physical device requires your Apple Development Team. Enter the Docker URL, a device name and the optional Web-configured password.

For an eventual archive, set the Team in Xcode and keep certificates/profiles outside Git:

```bash
xcodebuild archive -project ios/Ferry/Ferry.xcodeproj -scheme Ferry \
  -destination 'generic/platform=iOS' -archivePath /tmp/Ferry.xcarchive \
  DEVELOPMENT_TEAM=YOUR_TEAM_ID
```

Store export remains a maintainer-authorized step; this repository does not contain Apple credentials or an App Store export profile. When publication is explicitly authorized, copy `ios/ExportOptions.plist.example` to the ignored `ios/ExportOptions.plist`, replace `YOUR_TEAM_ID`, and pass that local file to `xcodebuild -exportArchive`.

### Android

Open `android/` in Android Studio, or build the debug APK with JDK 17 and Android SDK 35:

```bash
cd android
./gradlew :app:assembleDebug
```

The APK is produced below `android/app/build/outputs/apk/debug/`. To configure a signed release, copy `android/signing.properties.example` to the ignored `android/signing.properties`, restrict it to the current user, create the referenced keystore locally, and run `./gradlew :app:bundleRelease`. Any Gradle task graph that packages a release fails if signing configuration is absent or incomplete.

## Development

Run the local gates that match your change:

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

GitHub Actions runs these Server/Web, Android and iOS gates. See [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a change. Product boundaries live in [docs/product-core.md](docs/product-core.md), while [api/openapi.yaml](api/openapi.yaml) is the HTTP contract.

## Security

Ferry uses unencrypted HTTP on a trusted LAN. Every content/settings/device endpoint requires a revocable device Bearer token; the optional shared password gates only new devices. Read [SECURITY.md](SECURITY.md) before deployment or vulnerability reporting.

## License

Ferry is licensed under the [Apache License 2.0](LICENSE).
