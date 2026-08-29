# Ferry

Ferry is a self-hosted, chat-shaped clipboard and file ferry for your own devices.

The project is in its first walking-skeleton milestone. The current Go process serves both the Web chat and the API, persists message metadata in SQLite, and stores uploaded file bytes in a local blob directory. iOS, Android, pairing, authentication, and LAN exposure come after the local contract is proven.

## Run locally

Requirements: Go 1.26 or newer.

```bash
go run ./cmd/ferry
```

Open <http://127.0.0.1:8080>. By default Ferry writes local state to `./ferry-data`, which is ignored by Git.

Choose another loopback address or data directory explicitly:

```bash
go run ./cmd/ferry -listen 127.0.0.1:18080 -data-dir /path/to/ferry-data
```

## Security status

This milestone has no authentication, so it rejects non-loopback listener addresses. Its HTTP boundary also rejects non-loopback Host values and cross-origin browser writes. Pairing, authentication, and a safe LAN setup are the next protocol boundary.

## Verify

```bash
go test ./...
go vet ./...
```

The product boundary is in `docs/product-core.md`; the first-slice architecture and contract are in `docs/architecture.md` and `api/openapi.yaml`.

Ferry will become public after it is ready. A license has not been selected yet.
