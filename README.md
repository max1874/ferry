# Ferry

Ferry is a self-hosted, chat-shaped clipboard and file ferry for your own devices.

The Go process serves both the Web chat and API, persists messages and paired devices in SQLite, and stores uploaded file bytes in a local blob directory. A native iOS 26 App now covers pairing plus text and file sending; Android, automatic discovery, and TLS come later.

## Run locally

Requirements: Go 1.26 or newer.

```bash
go run ./cmd/ferry
```

Open <http://127.0.0.1:8080>. On an empty data directory, Ferry prints a ten-minute bootstrap code in the terminal; enter it in the Web pairing screen. If every browser loses its paired token, restart with `-pair` to issue a recovery code. By default Ferry writes local state to `./ferry-data`, which is ignored by Git.

Choose another loopback address or data directory explicitly:

```bash
go run ./cmd/ferry -listen 127.0.0.1:18080 -data-dir /path/to/ferry-data
```

## Security status

Every message, file, and device endpoint requires a paired-device Bearer token. The Web client stores it in origin-scoped browser storage, so another HTTP service on a different port does not receive it as a cookie. Each device has an independently revocable token whose hash—not its original value—is stored by the Server.

To listen on private LAN addresses, opt in explicitly:

```bash
go run ./cmd/ferry -lan -listen 192.168.1.20:8080
```

Replace the example with a specific private or link-local Server address, then open it from each device. Wildcard listeners, hostnames, and public IPs are rejected. Ferry currently uses unencrypted HTTP, so LAN mode protects device identity but not content from someone who can capture trusted-network traffic. Do not expose this milestone to the public internet; TLS is a separate future layer.

## Run the iOS App

Open `ios/Ferry/Ferry.xcodeproj` in Xcode 26.6 or newer and run the `Ferry` scheme on iOS 26. The project intentionally has no Development Team configured; Simulator builds work as-is, while a real device requires your own signing team.

For Simulator development, the default Server address is `http://127.0.0.1:8080`. For a real iPhone, start Ferry with an explicit private LAN address as shown above, enter that origin in the App, and claim a pairing code issued by the Server or an already paired Web client.

## Verify

```bash
go test ./...
go vet ./...
```

The product boundary is in `docs/product-core.md`; the first-slice architecture and contract are in `docs/architecture.md` and `api/openapi.yaml`.

Ferry will become public after it is ready. A license has not been selected yet.
