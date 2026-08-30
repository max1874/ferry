# Optional shared-password access

## Scope card

- **REQUESTED — Max, 2026-08-30**: “全设备免配对；支持一个密码功能；部署 Server 的人决定访问这个地址要不要密码。” Follow-up: password configuration must live in Web settings, not environment variables.
- **Done**: a new Web browser and iPhone can join Ferry without a pairing code; with no configured password they enter directly, and with a configured password they enter after one shared-password prompt.
- **Non-goals**: user accounts/roles, per-device passwords, Internet exposure, TLS, password recovery, forced logout of already issued device tokens, or Android implementation in this phase.
- **REPO_REQUIRED**: keep device tokens for remembered sessions/revocation, strict request decoding, private-listener boundary, no password logging/persistence, full Go/iOS tests, real Web/iPhone journey, adversarial and zero-context review.
- **Depth**: contract — replaces the public device-admission API and authentication journey across Server, Web, iOS, OpenAPI, and Docker.
- **Budget**: existing Server/auth/client surfaces, one singleton SQLite access-setting table, and one access document; no account/role schema or second service.
- **Boundary plan**: real Docker Server on macmini, real browser, signed iPhone build; unit tests close only error/edge contracts.
- **Expansion triggers**: HALT if implementation requires accounts/roles, TLS, a second service, or alteration of existing message/device rows.

## Decision

- **Observed**: pairing requires an already trusted device or out-of-band bootstrap code, which made the first-use path circular and visibly failed the user.
- **Rejected**: Web-only exemption; users would still need to understand why one device is privileged.
- **Rejected**: retain pairing and make codes refresh automatically; it improves expiry but preserves the confusing issuer model.
- **Decided — Max**: every device uses the same join flow; Server password is optional and chosen at deployment.
- **Selected mechanism**: `POST /api/v1/access/join` accepts `device_name` and optional `password`, verifies the configured password in constant time, creates a normal revocable device token, and returns it once. Web settings update one global salted password verifier in SQLite; an absent verifier means passwordless join.
- **Decided — Max**: password is configured through authenticated Web settings, not deployment environment variables.
- **Decided — Max's trusted-LAN boundary**: every currently admitted device may change this global setting, matching the existing peer permission to list/revoke other devices. Ferry does not defend against a hostile device racing to become the first LAN client; adding an administrator bootstrap would reintroduce the account/pairing complexity Max rejected. Enabling or changing the password gates future joins; it does not revoke current tokens.

```text
Web / iPhone
     |
     | join(device name, optional shared password)
     v
Ferry admission boundary
     |-- password disabled -> admit
     |-- password enabled + exact match -> admit
     `-- otherwise -> reject without a token
     v
remembered revocable device token -> chat/files
```

Plain brief: Ferry no longer asks users to understand pairing codes. The deployer may set one shared password; otherwise devices join immediately. If this boundary is wrong, a valid password cannot enter, an invalid password receives a token, or the password leaks.

## Frozen ship checklist

1. No-password Server: real new Web session joins automatically and reaches the timeline; signed iPhone joins without a pairing code.
2. Password Server: empty/wrong password returns a stable rejection and no device; exact password returns one token and reaches the timeline on Web and iPhone.
3. Raw password is compared in constant time and never returned/logged/stored; SQLite contains only a random salt and slow derived verifier. Request decoding rejects unknown/duplicate/malformed fields.
4. Existing valid device tokens continue to work and remain revocable; revoked tokens cannot re-enter without satisfying current admission configuration.
5. Authenticated Web settings can enable/change/disable the password and survive restart; enabling it does not revoke existing tokens.
6. Pairing-code UI/API/current docs are removed or explicitly historical; OpenAPI, Web, iOS, and Server share one admission contract; Docker has no password environment setting.
7. Go race/vet, signed iOS tests/build, Docker hostile-host probe, browser journeys, `git diff --check`, seven attacks, author review, and fresh verifier pass.

## Records

- Server/Web/iOS/OpenAPI implemented with no password environment setting. SQLite stores a singleton 16-byte salt, 32-byte PBKDF2-SHA256 verifier, and iteration count; device and message rows are unchanged.
- `go test -race -count=1 ./...`, `go vet ./...`, Web JavaScript syntax, and `git diff --check` pass.
- Signed iPhone 17 Pro Simulator unit suite passes 20/20, including proof that changing the Server clears its password and invalidates an in-flight response from the previous origin. A sandboxed generic-device build was blocked by CoreSimulator asset tooling; the subsequent permitted Simulator run compiled and exercised the production target successfully.
- Real Chromium journey against the production handler proved: first passwordless auto-join, Web setting enable, wrong-password rejection, correct-password join, old-token survival, Web setting disable, and passwordless auto-join again.
- A current-HEAD browser race probe enabled the setting while a credential-less form was open; the join returned `invalid_password`, displayed `password is incorrect`, and revealed the password field. Its kill probe injected network loss and observed `Offline` with the password field still hidden, proving the UI distinguishes protocol state from transport failure.
- SQLite close/reopen test proves the verifier survives restart, disables cleanly, and the raw test password is absent from the database, WAL, and shared-memory files.
- Final local gates pass: Go race/vet, JavaScript syntax, shell syntax, Compose config, Docker image build (`sha256:ceb764…`), `git diff --check`, four author passes, and fresh verifier SHIP with P0/P1/P2 all zero.
- Pending deployment gates: macmini Docker rebuild/restart, physical iPhone install, commit and push.

## Adversarial author review

Verdict after four passes: **ship candidate; no unresolved author-known P0/P1/P2**. Pass 1 fixed a stale `FERRY_UI_CODE` test-bundle key and live Web CSS names that still said pairing. Pass 2 incorporated fresh-review findings for the Web setting race and cross-origin iOS password carryover. Pass 3 found and fixed the sibling manual-form recovery path plus the in-flight iOS address-edit race. Pass 4 replaced presentation-string classification with the stable Server error code and produced no further finding.

1. **Coupled state** — `accessMu`, SQLite setting, Web `deviceToken/authGeneration/authController`, iOS `generation/token/phase`, and device revocation were traced; `TestHTTPPasswordlessPasswordAndRevocationJourney`, `TestStaleDeviceCannotChangeAccessSettings`, and `testChangingServerDuringJoinInvalidatesOldResponse` cover change/revoke/request interleavings.
2. **Failure paths** — entropy failure, malformed/oversized JSON, wrong/empty password, canceled/stale identity, store failure, browser storage failure, and network loss are explicit; evidence: strict Go tests, 20/20 iOS tests, and the real Chromium `invalid_password` versus injected-network-loss probe.
3. **Unchanged consumers** — full-repo live-surface grep found no `PairingManager`, pairing route, `PairingClaim`, `FERRY_UI_CODE`, or iOS `pair()` consumer after the test-plist fix.
4. **Contract surfaces** — OpenAPI 0.3, handler routes, Web JSON, iOS JSON, SQLite schema, README, and historical markers agree on access join/settings; `rg 'access/join|settings/access|password_required'` enumerated all producers/consumers.
5. **Original reproduction** — current-HEAD real Chromium opens without a code, rejects `wrong`, accepts the exact password, and keeps the earlier token valid; output: `{passwordless:true, wrongRejected:"password is incorrect", rightAccepted:true, oldTokenValid:true}`.
6. **Journey replay** — the final current-HEAD journey was rerun after error-code classification changed; task space 7 returned the expected race/offline bodies and was closed successfully.
7. **Mechanism discrimination** — same-run setting activation produced `invalid_password`, exposed the hidden field, and showed `Password required`; force-failing `fetch` instead showed `simulated network loss`/`Offline` with the field hidden, excluding the fallback.
8. **Regression scan** — candidate regression was existing data/session loss; schema uses `CREATE TABLE IF NOT EXISTS`, existing tables are untouched, store reopen retains history in prior migration tests, and old-token survival is covered by HTTP plus real browser.
9. **Scale/edge** — empty, 256±1 bytes, invalid UTF-8, Unicode, leading/trailing spaces, duplicate/escaped keys, and 512 KiB request cap are closed by auth/HTTP tests; PBKDF work is bounded by the 256-byte password limit.
10. **Seven attacks** — dual judges (OpenAPI/strict Go), extremes, equivalent spellings, default passwordless mode, side routes, authenticated settings gate, and non-author browser evidence were each exercised; no alternate product password environment producer exists in Docker files.
11. **Predicate producers** — every `password_required`, 401 mapping, setting write, token persistence write, and phase transition was grepped; public join 401 remains a Server rejection while Bearer 401 becomes revoked-device state on iOS/Web.
12. **Reversed findings** — removing pairing re-asked its recovery, revocation, stale-request, and multi-tab questions: the explicit trusted-LAN boundary accepts first-client control rather than inventing an owner bootstrap, settings writes recheck requester existence transactionally, tokens remain independently revocable, and only successful join writes shared Web storage.
13. **Pass limit** — author review did not self-certify: the final fresh verifier returned SHIP with P0/P1/P2 all zero; macmini and physical-device evidence remain separate deployment gates.

The first fresh pass returned NO-SHIP with two trusted-LAN threat-model objections and two concrete P2s. The follow-up found two sibling P2s, which were also fossilized: Web classifies only Server-issued `invalid_password` as a password gate, and iOS invalidates the old generation when its address changes mid-join. The two LAN-adversary findings are outside the user-decided threat model rather than silently accepted implementation bugs: Ferry currently trusts admitted LAN devices and intentionally has no administrator bootstrap or hostile-client throttling. The final fresh pass returned SHIP with no in-scope P0/P1/P2.
