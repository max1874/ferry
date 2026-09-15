# Docker deployment

> English | [简体中文](docker-deployment.zh-Hans.md)

The README is the deployment guide. This document records the container boundary behind it: listen addresses, reverse proxies, the gates that protect them and the decisions that shaped them. The original 2026-08-30 test-server delivery and its pairing-era evidence follow as history.

## Listen address options

| `FERRY_LISTEN_HOST` | Listener inside the container | Who can reach Ferry | Use when |
| --- | --- | --- | --- |
| unset (default) | the container's only IP | through the port published on `FERRY_HOST_IP`, plus other containers on the same Docker network | the container has one network |
| a specific private IP | that IP | as above, restricted to that network | you want to pick one of several networks |
| `0.0.0.0` or `::` | every container interface | through the published port, plus containers on every attached network | the container joins several networks, such as a proxy network |

- With the default, startup fails if the container has zero or several IP addresses; the error names `FERRY_LISTEN_HOST` as the way out.
- In a bridge container, `0.0.0.0` does not widen access from outside the host: Docker still publishes only on `FERRY_HOST_IP`, which Ferry validates as loopback or private.
- With `network_mode: host`, `ports:` is ignored and `0.0.0.0` means every host interface. Reachability then depends entirely on the host firewall. Note that Docker's own published ports bypass host firewalls such as ufw, which is why Compose always names a host IP.
- Ferry logs a warning whenever it listens on every interface.

### Behind a reverse proxy

| Where the proxy runs | Ferry settings | Proxy target |
| --- | --- | --- |
| On the host, or a container with `network_mode: host` | `FERRY_TRUSTED_ORIGIN` | `127.0.0.1:42817` (the loopback-published port) |
| In its own bridge container | `FERRY_TRUSTED_ORIGIN`, `FERRY_LISTEN_HOST=0.0.0.0`, and a Compose override that adds the proxy's external network next to `default` | `ferry:42817` |
| Ferry run from source | `-listen 127.0.0.1:42817 -trusted-origin …` | `127.0.0.1:42817` |

- **Observed (2026-09-14)**: the host-proxy row was exercised on a Linux host with Caddy v2.11.3 on host networking and Ferry built from `382f194`: without the trusted origin the proxied page returned 421; with it, Chrome joined two sessions, exchanged text, an image and a 12 MB video, and foreign `Host` / `Origin` values were rejected.
- The bridge-container row has not been exercised end to end.

## Governing gates

- `go test ./...` and `go vet ./...` must remain green.
- Image build must compile the same `./cmd/ferry` entry point used outside Docker.
- Startup must fail if `FERRY_LISTEN_HOST` is unset and the container address is empty or ambiguous, or if `FERRY_HOST_IP` is not loopback/private/link-local; Ferry itself remains the final numeric/private-address judge for both listener and published host.
- Compose defaults to loopback publication; a deployment opts into a LAN address through its untracked `.env` file.
- CI builds both architectures, starts each through `compose.yaml`, asserts the startup log's browser address, keeps a device, text, file and password across a restart, and moves a source-built deployment to the image without losing them (`scripts/image-smoke.sh`).
- `scripts/ferry-data.sh self-test` builds the current source through `compose.build.yaml`; it must never test a pulled image.
- `git diff --check` and a full-file security review gate shipping.

## Decisions

- **Decided (Max, 2026-09-13, issue #1)**: an operator-run TLS reverse proxy is now supported through `FERRY_TRUSTED_ORIGIN` / `-trusted-origin`. The listener rule is unchanged: the proxy reaches the published port or the container address, so no wildcard bind is needed. Ferry admits only the configured origin in `Host` and `Origin` and ignores `X-Forwarded-*`.
- **Decided (Max, 2026-09-14)**: the listen address is the deployer's choice. `FERRY_LISTEN_HOST` defaults to the container's single IP, as before, and may be set to a specific private IP or to `0.0.0.0` / `::`. Outside Docker, `-lan` now accepts `0.0.0.0` and `[::]`; without `-lan` Ferry still listens on loopback only. This supersedes the 2026-08-30 rejection of `0.0.0.0` below.
- **Decided (Max, 2026-09-14)**: deployment defaults to the published image `ghcr.io/max1874/ferry:<version>` for `linux/amd64` and `linux/arm64`. `compose.yaml` pulls it, `FERRY_IMAGE` selects the version, and `compose.build.yaml` builds the current source for developers and for the data self-test. The service name, volume, data directory and runtime user are unchanged, so a source deployment upgrades in place. The release flow lives in `docs/release-process.md`.
- **Observed**: the image's build stage cross-compiles on the builder's platform (`CGO_ENABLED=0`), so the `arm64` image does not compile Go under emulation.
- **Observed**: Ferry's Go process embeds both the Web UI and API, and persists SQLite plus blobs below `-data-dir`.
- **Observed (2026-08-30; superseded 2026-09-14)**: Ferry rejected wildcard listeners even in LAN mode, so a normal bridge container could not bind `0.0.0.0`.
- **Recommended, selected under the user's execution delegation**: use a bridge network, derive the container's single IPv4 address at startup, bind Ferry to that private address, and publish only the configured host LAN address.
- **Rejected**: host networking makes OrbStack's forwarding boundary implicit and publishes without a host-IP mapping.
- **Rejected (2026-08-30; superseded 2026-09-14)**: allowing `0.0.0.0` would weaken an existing tested security boundary solely for deployment convenience. The rejection was reversed once a real deployment appeared that the single-IP listener cannot serve: a proxy container on a second Docker network (issue #1).

## History: the 2026-08-30 test-server delivery

> Admission was changed on 2026-08-30: current Ferry joins directly and optionally uses the password configured in Web settings. Pairing-code journey notes below are historical deployment evidence; `docs/password-access.md` governs the live access contract. Docker environment values configure only network publication, never the product password.

### Delivery record

- Status: `deployed` on the test server under the current passwordless/optional-password admission model; cross-device home-LAN revalidation is tracked in `docs/release-readiness.md` rather than under the retired pairing flow.
- Subject: the original container deployment; the pairing-code checklist below is retained only as historical evidence for the original container boundary.
- Requested (2026-08-30): run Ferry Server and Web on the test server in Docker on a high port rather than 8080.
- Done: Web and iPhone use `http://192.168.1.20:42817`, exchange real messages, and retain data across a container restart.
- Non-goals at the time: TLS, a domain, reverse proxying, public-Internet exposure, and migration of the temporary laptop test data.
- Depth: contract, because the deployment must preserve Ferry's authenticated LAN-listener boundary.
- Budget: Dockerfile, Compose, `.dockerignore`, deployment documentation, and directly necessary tests; no API or database changes.
- **Observed**: the test server is arm64 at `192.168.1.20`, runs OrbStack Docker 29.4.0 / Compose 5.1.2, and port 42817 was unused at preflight.
- **Observed**: a predecessor project's deployment uses Compose, a named volume, `restart: unless-stopped`, and host networking.

```text
iPhone / browser
      |
      | http://192.168.1.20:42817
      v
The test server host-IP port mapping
      |
      v
Ferry container private IPv4:42817
      |
      +-- embedded Web + authenticated API
      +-- /data named volume (SQLite + blobs)
```

Plain brief: this adds a repeatable container package for the existing combined Ferry Server and Web UI. The container keeps Ferry's specific-private-address listener rule while exposing one configurable high port on the test server. If the address wiring or volume is wrong, devices cannot connect or data disappears after restart.

### Pairing-era ship checklist

1. **REQUESTED** — `docker compose config` resolves host port 42817 and a persistent `/data` volume.
2. **DESIGN_NECESSARY** — the running process binds a specific container-private IP, while Docker publishes only `192.168.1.20:42817`; wildcard binding remains rejected by existing tests.
3. **REPO_REQUIRED** — `go test ./...`, `go vet ./...`, image build, and `git diff --check` pass.
4. **REQUESTED journey** — a real browser claims the bootstrap code at `http://192.168.1.20:42817`, generates a second code, and the real iPhone claims it.
5. **REQUESTED journey** — browser-to-iPhone and iPhone-to-browser messages appear with exact bodies on both surfaces.
6. **REPO_REQUIRED** — after `docker compose restart`, paired identities, messages, and stored files remain available and no new bootstrap code is issued.
7. **DESIGN_NECESSARY kill probe** — publishing a different/unconfigured port does not make the accepted URL succeed; stopping the container makes the journey unavailable.
8. **REPO_REQUIRED** — adversarial self-review and fresh zero-context contract review have no unresolved P0/P1/P2 findings.

### Pairing-era journey expectations

- Browser entry: opening `/` renders Ferry's pairing screen, not an API error or another service.
- Bootstrap: the first successful claim returns a Web device identity; replaying that code is rejected.
- Second device: Web creates a new single-use code; iPhone pairing shows the timeline rather than an indefinite connecting state.
- Exchange: browser sends `from web via test-server 42817`; iPhone renders that exact text. iPhone sends `from iphone via test-server 42817`; Web renders that exact text.
- Restart: the same URL recovers, both tokens remain valid, and both exact messages remain present.

### Build and review records

- `docker compose config` defaults to `127.0.0.1:42817` and a named `ferry-data:/data` volume; the test server's untracked `.env` explicitly publishes `192.168.1.20:42817`.
- Local and test-server multi-stage builds passed. The running test-server container reports user `ferry`, status `running`, container IP `192.168.148.2`, and host mapping `192.168.1.20:42817->42817/tcp`.
- The rebuilt container returned HTTP 200 and the real Web session retained paired identity plus messages from the named volume. Server logs state the precise private listener and trusted-HTTP warning.
- Web/iPhone exchange before the four-digit-only UI change produced exact messages `from web via test-server 42817` and `hiho`; the updated iPhone build is installed, with the new four-digit claim awaiting manual user confirmation.
- Pairing contract attacks and the 13-item author review are recorded in `docs/four-digit-pairing.md`; fresh zero-context review remains the final pre-push gate.
- An independent review found that Docker publication could name a public host even while the process bound a private bridge IP. The runtime now validates `-published-host`; a hostile public-IP container probe exits before listening, while a valid loopback smoke container runs as `ferry` and returns HTTP 200.
- After strict no-whitespace PIN normalization landed, the test server rebuilt image `sha256:209283f…`, recreated the container with the same named volume, and returned HTTP 200 at the configured LAN URL.
