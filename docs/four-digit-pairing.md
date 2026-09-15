# Four-digit pairing (historical)

> English | [简体中文](four-digit-pairing.zh-Hans.md)

> Superseded on 2026-08-30 by `docs/password-access.md`. The live product no longer uses pairing codes.

## Scope and decision

- Status: `implemented`; manual iPhone claim is awaiting user confirmation.
- Requested (2026-08-30): pairing codes should be four digits because the existing code is too complex to enter.
- Done: Server, Web, OpenAPI, and iOS accept and present exactly four ASCII digits, including leading zeroes; codes remain ten-minute and single-use.
- Non-goals: rate limiting, hostile-LAN protection, device-token changes, persistence, or API route changes.
- Depth: contract, because the accepted credential format changes across Server, Web, and iOS.
- Budget: pairing generator/normalizer, the two input surfaces, OpenAPI, directly coupled tests/docs, and deployment refresh; no new persistent mechanism.
- **Decided — user, 2026-08-30**: the LAN is trusted; accept the 10,000-code search space and do not add brute-force defenses.

## Design

- **Observed**: before this change, the code was 16-character base32, random, ten-minute, memory-only, and atomically single-use.
- **Selected**: uniformly map two random bytes into `0000`–`9999` with rejection sampling, preserving leading zeroes.
- **Rejected by user**: four digits plus per-IP rate limiting; it treats the trusted LAN as hostile and adds state the user does not want.
- **Rejected**: six digits; it does not satisfy the explicit four-digit interaction requirement.

Plain brief: Ferry will show a short PIN that is easy to type on a phone. Only the PIN alphabet and length change; expiry, single-use reservation, issuer revocation, and device tokens stay as they are. If the implementation drifts, one surface will reject a code another surface emits.

## Frozen checklist

1. Generated codes match `^[0-9]{4}$`, preserve `0000`, and rejection sampling never introduces modulo bias.
2. Normalization accepts exactly four ASCII digits and rejects whitespace, wrong length, letters, and non-ASCII digits.
3. Existing expiry, collision retry, reservation rollback, concurrent single winner, and issuer revocation tests remain green.
4. OpenAPI, Web constraints, Server, and iOS numeric keyboard agree on four ASCII digits.
5. Full Go tests, race tests, vet, signed iOS tests/build, Docker build, and `git diff --check` pass.
6. Final Docker image on `test-server:42817` emits a four-digit code; real Web and iPhone can claim it once and replay fails.
7. Seven-pattern attack record, 13-item author review, and fresh zero-context contract verdict contain no unresolved P0/P1/P2.

## Contract matrix

| Direction | Declared contract | Implementation |
| --- | --- | --- |
| Loosen | No loosening: exact four digits only | Wrong-length and non-ASCII inputs remain rejected |
| Tighten | OpenAPI changes from 16-char base32 to four ASCII digits | Generator, normalizer, Web, and iOS change together |

## Build and journey record

- `env GOCACHE=/private/tmp/ferry-go-cache go test -race -count=1 ./...` — passed for all Go packages; `go vet ./...` and `git diff --check` also passed.
- Signed Simulator run — all 20 `FerryTests` passed, including invalid PIN paste rejection and proof that a short PIN makes zero client calls. A prior run with `CODE_SIGNING_ALLOWED=NO` failed only the Keychain integration with OSStatus `-34018`; rerunning with normal Simulator signing passed and confirmed the test command, not product code, was at fault.
- Signed device build — `** BUILD SUCCEEDED **`, deep/strict signature verification passed, no `.xctest` bundle was present, and `devicectl` installed/launched `com.max1874.ferry` on the connected iPhone.
- Docker — local image `sha256:7c6d6f…` built; the test server rebuilt `ferry:local`, runs as UID 10001 on container IP `192.168.148.2`, publishes only `192.168.1.20:42817`, and `/` returns HTTP 200.
- Real Web — after the container rebuild, the paired Web identity and old exact messages remained present; Web generated `5045`, proving the deployed Server/Web candidate emits exactly four digits. Leading-zero preservation is covered by the `0000` generator test. Local browser-harness recording: `ferry-four-digit-test-server` (not committed).
- Real iPhone — the old iPhone identity was revoked and the updated App launched, so the next claim exercises the new four-digit field. Claim/single-use replay remains pending user input because physical-device UI automation was denied by iOS before test execution.

## Seven contract attacks

1. **Dual judges** — Server is final judge; machine-readable OpenAPI request/response patterns, Web `[0-9]{4}`, and iOS reject-without-rewrite input validation agree. Evidence: `rg` found no live base32/16-character consumer outside the explicitly historical design record.
2. **Extremes** — `0000` and `9999`, expiry boundary, wrong lengths, and exhausted randomness are covered by named auth tests.
3. **Equivalent spellings** — there are none: Server rejects surrounding whitespace, letters, and full-width digits; only the exact four ASCII bytes are accepted.
4. **Defaults as backdoors** — ten-minute and single-use defaults are unchanged; no rate limit is an explicit user decision, not an accidental unset default.
5. **Side doors** — every claim route reaches `PairingManager.Reserve`; HTML/iOS filtering is convenience only and cannot bypass Server normalization.
6. **Policy needs a gate** — race tests, OpenAPI pattern, HTML pattern, and signed Swift compile are the executable gates.
7. **No self-certification** — deployed Web generated a real four-digit value and retained persisted state across a container recreation; physical claim is explicitly left for the user rather than claimed from a mock.

## Author adversarial review — ship with one pending manual proof

1. Coupled state — code map, `reserved`, expiry, and issuer fields are unchanged; a race test covers concurrent one-winner, while focused sequential tests cover rollback, revoke, and expiry state transitions.
2. Failure paths — 16 rejected samples, collision retry, DB rollback, and revoked issuer are covered by tests; direct entropy-reader failure propagation is unchanged code and covered by `io.ReadFull` return handling rather than a new test.
3. Unchanged consumers — full-repo `rg` checked all `NewCode`/`Reserve` and UI/API consumers; no live 16-character constraint remains.
4. Contract surfaces — OpenAPI, Web, Swift, Server, README, and deployment docs changed together; SQLite/device tokens/routes did not change.
5. Original reproduction — deployed Web shows `5045`, replacing the long entry the user rejected.
6. Current journey — The test server was rebuilt from the candidate and Web/body/persistence were reread after recreation; iPhone manual claim remains open.
7. Mechanism discrimination — `0000`, `9999`, rejection sampling, replay rejection, and invalid alphabet each have distinct tests rather than a shared status-only assertion.
8. Regression scan — candidate regression was modulo bias and smaller collision space; rejection sampling plus collision retry tests clear the first, while the smaller space is the user's accepted tradeoff.
9. Scale/edge — zero/upper boundaries, 32 concurrent claims, collisions, expired values, and 16 unusable samples are covered.
10. Contract attacks — all seven results are recorded above with concrete gates.
11. Predicate producers — `reserved` is produced only by `Reserve` and cleared only by `Rollback`; issuer validity/revocation producers were traced and tested.
12. Reversed findings — Keychain failure was traced to unsigned Simulator test execution (`-34018`); the same full suite passed with normal local signing, and pairing code codepaths do not touch Keychain.
13. Pass limit — this author pass does not close independent verification; fresh zero-context verdict is required before push.

## Independent review corrections

The first zero-context review of candidate `200f890` returned FAIL with two P1 and one P2; all three conclusions were accepted rather than waived:

- Public `FERRY_HOST_IP` could bypass the process listener check through Docker port publication. The container now passes the published host into Ferry's Go config validator; a `203.0.113.10` container probe exits closed with `published-host must be a loopback or private/link-local IP address`.
- iOS previously deleted invalid characters and truncated long pasted values, which could turn `12a34` into another credential, `1234`. It now rejects the whole proposed edit and retains the prior field value; a signed Swift test covers letters, superscript digits, full-width digits, and five digits.
- The OpenAPI claim request described four digits only in prose. Request and response now both use strict machine-readable `^[0-9]{4}$`; Server rejects whitespace too, avoiding incompatible Unicode whitespace tables across validators.
- iOS allowed editing an incomplete PIN but also allowed submitting it. The Pair button and `AppModel.pair()` now independently require exactly four ASCII digits; a model test proves `123` makes zero client claim calls.

Final zero-context verification of code commit `99b278c` returned PASS with no P0/P1/P2 after Go race/vet, signed 20-test Simulator execution, Docker hostile-host probing, syntax checks, and complete contract review. The only open proof is the user's physical iPhone claim.
