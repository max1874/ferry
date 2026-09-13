# Docker deployment

> Admission was changed on 2026-08-30: current Ferry joins directly and optionally uses the password configured in Web settings. Pairing-code journey notes below are historical deployment evidence; `docs/password-access.md` governs the live access contract. Docker environment values configure only network publication, never the product password.

## Delivery record

- Status: `deployed` on macmini under the current passwordless/optional-password admission model; cross-device home-LAN revalidation is tracked in `docs/release-readiness.md` rather than under the retired pairing flow.
- Subject: the current main deployment; the pairing-code checklist below is retained only as historical evidence for the original container boundary.
- Requested (2026-08-30): run Ferry Server and Web on `macmini` in Docker on a high port rather than 8080.
- Done: Web and iPhone use `http://10.0.0.2:42817`, exchange real messages, and retain data across a container restart.
- Non-goals: TLS, a domain, reverse proxying, public-Internet exposure, and migration of the temporary laptop test data.
- **Decided (Max, 2026-09-13, issue #1)**: an operator-run TLS reverse proxy is now supported through `FERRY_TRUSTED_ORIGIN` / `-trusted-origin`. The listener rule is unchanged: the proxy reaches the published port or the container address, so no wildcard bind is needed. Ferry admits only the configured origin in `Host` and `Origin` and ignores `X-Forwarded-*`.
- Depth: contract, because the deployment must preserve Ferry's authenticated LAN-listener boundary.
- Budget: Dockerfile, Compose, `.dockerignore`, deployment documentation, and directly necessary tests; no API or database changes.

## Evidence and decision

- **Observed**: Ferry's Go process embeds both the Web UI and API, and persists SQLite plus blobs below `-data-dir`.
- **Observed**: `macmini` is arm64 at `10.0.0.2`, runs OrbStack Docker 29.4.0 / Compose 5.1.2, and port 42817 was unused at preflight.
- **Observed**: the predecessor `avocado` deployment uses Compose, a named volume, `restart: unless-stopped`, and host networking.
- **Observed**: Ferry rejects wildcard listeners even in LAN mode; a normal bridge container therefore cannot bind `0.0.0.0`.
- **Recommended, selected under the user's execution delegation**: use a bridge network, derive the container's single IPv4 address at startup, bind Ferry to that private address, and publish only the configured Mac LAN address.
- **Rejected**: host networking makes OrbStack's forwarding boundary implicit and publishes without a host-IP mapping.
- **Rejected**: allowing `0.0.0.0` would weaken an existing tested security boundary solely for deployment convenience.

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

Plain brief: this adds a repeatable container package for the existing combined Ferry Server and Web UI. The container keeps Ferry's specific-private-address listener rule while exposing one configurable high port on the Mac mini. If the address wiring or volume is wrong, devices cannot connect or data disappears after restart.

## Governing gates

- `go test ./...` and `go vet ./...` must remain green.
- Image build must compile the same `./cmd/ferry` entry point used outside Docker.
- Startup must fail if the container address is empty or ambiguous, or if `FERRY_HOST_IP` is not loopback/private/link-local; Ferry itself remains the final numeric/private-address judge for both listener and published host.
- Compose defaults to loopback publication; the Mac mini deployment opts into `10.0.0.2` through an untracked `.env` file.
- `git diff --check` and a full-file security review gate shipping.

## Historical pairing-era ship checklist

1. **REQUESTED** — `docker compose config` resolves host port 42817 and a persistent `/data` volume.
2. **DESIGN_NECESSARY** — the running process binds a specific container-private IP, while Docker publishes only `10.0.0.2:42817`; wildcard binding remains rejected by existing tests.
3. **REPO_REQUIRED** — `go test ./...`, `go vet ./...`, image build, and `git diff --check` pass.
4. **REQUESTED journey** — a real browser claims the bootstrap code at `http://10.0.0.2:42817`, generates a second code, and the real iPhone claims it.
5. **REQUESTED journey** — browser-to-iPhone and iPhone-to-browser messages appear with exact bodies on both surfaces.
6. **REPO_REQUIRED** — after `docker compose restart`, paired identities, messages, and stored files remain available and no new bootstrap code is issued.
7. **DESIGN_NECESSARY kill probe** — publishing a different/unconfigured port does not make the accepted URL succeed; stopping the container makes the journey unavailable.
8. **REPO_REQUIRED** — adversarial self-review and fresh zero-context contract review have no unresolved P0/P1/P2 findings.

## Historical pairing-era journey expectations

- Browser entry: opening `/` renders Ferry's pairing screen, not an API error or another service.
- Bootstrap: the first successful claim returns a Web device identity; replaying that code is rejected.
- Second device: Web creates a new single-use code; iPhone pairing shows the timeline rather than an indefinite connecting state.
- Exchange: browser sends `from web via macmini 42817`; iPhone renders that exact text. iPhone sends `from iphone via macmini 42817`; Web renders that exact text.
- Restart: the same URL recovers, both tokens remain valid, and both exact messages remain present.

## Build and review records

- `docker compose config` defaults to `127.0.0.1:42817` and a named `ferry-data:/data` volume; macmini's untracked `.env` explicitly publishes `10.0.0.2:42817`.
- Local and macmini multi-stage builds passed. The running macmini container reports user `ferry`, status `running`, container IP `192.168.148.2`, and host mapping `10.0.0.2:42817->42817/tcp`.
- The rebuilt container returned HTTP 200 and the real Web session retained paired identity plus messages from the named volume. Server logs state the precise private listener and trusted-HTTP warning.
- Web/iPhone exchange before the four-digit-only UI change produced exact messages `from web via macmini 42817` and `hiho`; the updated iPhone build is installed, with the new four-digit claim awaiting manual user confirmation.
- Pairing contract attacks and the 13-item author review are recorded in `docs/four-digit-pairing.md`; fresh zero-context review remains the final pre-push gate.
- An independent review found that Docker publication could name a public host even while the process bound a private bridge IP. The runtime now validates `-published-host`; a hostile public-IP container probe exits before listening, while a valid loopback smoke container runs as `ferry` and returns HTTP 200.
- After strict no-whitespace PIN normalization landed, macmini rebuilt image `sha256:209283f…`, recreated the container with the same named volume, and returned HTTP 200 at the configured LAN URL.
