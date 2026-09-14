# MVP first vertical slice delivery record

> English | [简体中文](mvp-delivery.zh-Hans.md)

## S0 — Confirmed scope

- **REQUESTED**: an open-source project (made public once finished) with iOS, Android, a self-hosted Web app and a Server, sharing the LAN clipboard and files through a chat interface, with a UI modelled on ChatGPT.
- **Done**: a real local Server + browser completes text sending, file upload, a unified timeline, download and persistence across restart.
- **Non-goals**: this slice writes no mobile clients, does not watch the clipboard automatically, and has no multi-user/channels/public release.
- **INFERRED accepted (Max, 2026-08-29, replied `go`)**: Go + SQLite, Web embedded in the Server, a default personal space, deliberate sending, loopback listening by default.
- **REPO_REQUIRED**: product core, real behaviour evidence, minimal simulator load (no simulator this time), adversarial self-review.
- **Depth**: contract; a new cross-runtime HTTP contract and persistent file boundary.
- **Budget**: ≤12 production files, about ≤1,400 production LOC, ≤3 process/design docs; no publishing, no push, no global tool installs; at most 3 ineffective attempts.
- **Boundary plan**: a real local Go service + a real browser close Done; Go HTTP tests close the boundary counterexamples. No external provider.
- **Expansion triggers**: accounts/TLS/device discovery/background clipboard/multiple spaces/an extra long-running service require HALT.

## S1 — Design decision

Candidate A from `docs/architecture.md` was chosen: single Go process + SQLite + embedded Web. Implementation began after the user confirmed the scope card; candidate B's multi-service operations and candidate C's peer-to-peer complexity were rejected.

## S2 — Frozen ship checklist

| ID | Closable property | Machine check |
| --- | --- | --- |
| MVP-01 | Go production and tests compile and are correctly formatted | `go test ./...`, `go vet ./...`, `gofmt` drift check |
| MVP-02 | OpenAPI agrees with the real endpoints/discriminated union | HTTP contract tests cover text/file/list/download/error shape |
| MVP-03 | the text boundary rejects whitespace and 64 KiB + 1 while preserving valid originals | discriminating tests |
| MVP-04 | file names cannot traverse paths, and 64 MiB + 1 is rejected without adding a message | traversal/oversize HTTP tests |
| MVP-05 | messages and files remain readable after reopening the store with the same data dir | persistence integration test |
| MVP-06 | an unauthenticated process may listen only on loopback, with Web and API served by the same process | CLI rejection probe + config/run tests + running process observation |
| MVP-07 | a real browser completes the text/file/download/restart journey | final-subject transcript, read item by item against the expectations in `docs/architecture.md` |
| MVP-08 | the non-trivial diff completes 1–13 self-review and seven boundary attacks | S3/S4 of this document have per-item command or test evidence |
| MVP-09 | full-file review has no open P0/P1; the fresh gate has an explicit verdict | S5/S6 of this document |
| MVP-10 | budget, git whitespace and workspace ownership closed | file/LOC count, `git diff --check`, `git status --short` |

The checklist is frozen. No new feature without a REQUESTED or DESIGN_NECESSARY source may be added.

## S3 — Build and attack record

The final binary under test is `/private/tmp/ferry-final-acceptance/ferry`, SHA-256 `709f1dbeff55419a63a9d49fc76d599bed22bacb65808936fbcdb86f7c0256d8`. It is built from the current 8 production files; after that only tests and this delivery record were added, with no change to production source.

Record of the seven attacks:

1. **Two judges**: HTTP contract tests check both the OpenAPI declaration and the handler's text/file/error shape; checking against the complete code found and corrected a blob rename described in the architecture that does not exist.
2. **Extremes**: `TestTextBoundariesAndPreservation` covers 64 KiB ±1/whitespace/original preservation; `TestFileSizeBoundaries` and a browser sparse-file probe cover 0, 64 MiB and 64 MiB+1.
3. **Equivalent spellings**: JSON tests reject duplicate keys (including escape-equivalent spellings), invalid UTF-8 and lone surrogates; listener tests cover `localhost`/`LOCALHOST`, IPv4/IPv6 loopback and fake hostnames.
4. **Defaults as backdoors**: the default is `127.0.0.1:8080`, but an explicit `-listen 0.0.0.0:18089` also fails before starting, exit code 1, error `listen host must be localhost or a loopback IP address`.
5. **Side doors**: `parseConfig`, a direct `run(config)` and the actual `net.Listener` address all reject non-loopback at three layers; `TestRunRejectsNonLoopbackListenerWithoutCreatingData` proves the direct-call bypass creates no persistent files either.
6. **Policy needs a gate**: loopback, Host, Origin, input size, path and DB discrimination all have corresponding failing tests; sending `0.0.0.0` into the CLI and `run` really turns red, not relying on documentation alone.
7. **No self-certification**: the author closed the journey with a real process/browser; the independent verifier first found 3 problems, then rechecked the original reproductions after the fixes, with results recorded in S6.

Real browser journey on current production (recording directory `<browser-harness recordings>/ferry-current-head`, 19 frames):

- After starting with an empty timeline, sending `hello ferry current head` gave 1 DOM entry with the same body, connection `Local`, no error.
- After uploading `README-upload.md` the timeline had 2 entries and the file card URL was `/api/v1/files/<message-id>`; clicking the card to download matched the uploaded original under `cmp`.
- Uploading a 67,108,865-byte file and waiting 7 seconds (more than several polling cycles), the timeline still had 2 entries, the error kept showing `file exceeds 67108864 bytes`, and the file chip remained.
- Restarting with the same data dir, both history entries and the file card were still there; after successfully sending `recovery probe` the status returned to the normal privacy notice.
- After stopping the service the browser showed `Offline / Failed to fetch / error=true`; restarting again with the same directory changed it automatically to `Local / Your data stays on this Ferry server. / error=false`, with 3 history entries kept.

## S4 — Adversarial self-review

Conclusion: **the author side can ship to the frozen local MVP boundary; this conclusion is not an independent final green.** This round corrected three items after the fresh verifier's first findings, then completed the 1–13 record below:

1. **Coupled state**: all producers of `cursor/loading/activityStatus/connectionError/sendError/rendered` were enumerated; real interleavings verified that "an oversize send error is not cleared by polling" and "a pure connection error clears after recovery", evidenced by the S3 DOM triple.
2. **Failure paths**: the listener is validated before the store opens and has `defer Close`; send/poll success/catch each write their own error source; evidence is `TestRunRejectsNonLoopbackListenerWithoutCreatingData`, the race suite and the disconnection journey.
3. **Unchanged callers**: `rg 'run\(|parseConfig|ListenAndServe|server.Serve'` finds only `main`, config tests and the new direct-run test; no unmigrated `ListenAndServe` caller.
4. **Contract surfaces**: endpoints, OpenAPI, DB schema and message shape did not change because of the fixes; `node --check`, OpenAPI YAML parsing and the full HTTP contract test suite pass.
5. **Original reproduction**: `ferry -listen 0.0.0.0:18089 ...` exits 1; browser Offline→restart changes from `Failed to fetch/error=true` to the privacy text/error=false.
6. **Current journey**: replayed on SHA-256 `709f…256d8` built from the final production source and read the DOM content, 19 frames recorded; the download was separately verified byte for byte with `cmp`.
7. **Mechanism discrimination**: stopping the service made Local/the normal notice disappear and Offline/an error appear, and restarting restored it; an oversize file added no message, proving it is not a UI-fallback false success.
8. **Regression scan**: the candidate regression was valid IPv6/uppercase localhost being wrongly rejected; the focused config test's `[::1]` and `LOCALHOST` both pass, while non-loopback and fake hostnames are rejected.
9. **Scale/edge**: 0-byte, 64 MiB, 64 MiB+1, 64 KiB ±1, empty timeline, repeated requests and corrupted DB metadata all have tests; tenant isolation is N/A — this slice explicitly has only one personal space.
10. **Contract attacks**: the seven attacks are each recorded in S3, with actual results from focused listener tests, HTTP tests, the CLI kill probe and the browser kill probe.
11. **Predicate producers**: `rg 'activityStatus|connectionError|sendError|setStatus|renderStatus|validateLoopbackAddress|IsLoopback'` shows UI state is produced only by the load/send paths, and the listening decision is produced in three places: config, run and the actual socket.
12. **Reversed findings**: the forgeable-Host problem was not masked with a stricter Host but closed at the real listener boundary; the recovery problem keeps persistent send-error semantics and clears only the recovered connection error; the rename wording was corrected to the real O_EXCL write mechanism.
13. **Pass limit**: the author record closes only the pre-filter; the finite Done is MVP-01…10, and the independent verdict is listed separately in S6, with "nobody finds anything else" not a termination condition.

## S5 — Full-code review

After the fixes the author reread the complete changed files, grepped every caller/state producer, and verified consumers with the full test suite, race, vet and a real process. Author verdict: no open P0/P1 now; the 3 problems from the first review were fixed respectively as tests, a kill probe and accurate documentation. Limit: this is an author review and cannot replace S6.

## S6 — Fresh verification

The independent verifier's sealed first-round conclusion was **cannot ship**: P1 a non-loopback listener could be bypassed with a forged Host; P2 the UI kept `Failed to fetch` after the Server recovered; documentation P2 described a blob rename that does not exist.

After the fixes the same verifier ran read-only closure QA, with a final verdict of **PASS; no new P0/P1/P2**:

- `0.0.0.0:18109` startup exits 1 with no listener; the valid `localhost:18110` has an actual listener on `127.0.0.1`; the direct `run(config)` test passes and creates no data content before rejecting.
- A real browser observed in turn Local/normal → Offline/`Failed to fetch` after stopping the service → Local/normal after recovery; the 64 MiB+1 send error persisted through several successful polls and cleared after the next successful send. The independent recording directory is `<browser-harness recordings>/ferry-closure-status` (15 frames).
- The crash-window wording matches the real copy → sync → close → SQLite insert order.
- The verifier independently reran `go test ./...`, race, vet and `git diff --check`, all passing, and stopped every QA Server.

## S7 — Closure

| Checklist | Closure evidence |
| --- | --- |
| MVP-01 | `go test -count=1 ./...`, `go vet ./...`, `test -z "$(gofmt -l cmd internal)"` pass |
| MVP-02 | HTTP contract tests all green; `api/openapi.yaml` read successfully by the Ruby YAML parser |
| MVP-03 | text 64 KiB ±1, whitespace, Unicode/JSON counterexample tests all green |
| MVP-04 | traversal, 0/64 MiB/64 MiB+1 tests and the real browser kill probe pass |
| MVP-05 | store reopen test and two real same-directory Server restarts pass |
| MVP-06 | CLI kill probe, config/direct-run tests and real listener verification pass |
| MVP-07 | the author's 19-frame current-binary journey + the independent verifier's 15-frame closure journey pass |
| MVP-08 | S3's seven attacks and S4's 1–13 all have concrete command/test/runtime evidence |
| MVP-09 | the author's full-code review has no open P0/P1; fresh closure verdict PASS, no new P0/P1/P2 |
| MVP-10 | this slice has 8 production code files, 1,386 LOC; `git diff --check` passes; no commit/push/publish |

The final rerun also included `go test -race -count=1 ./...`, `go mod verify`, `node --check` and a Linux amd64 `CGO_ENABLED=0 go build`. The rebuilt binary and the browser-tested binary have exactly the same SHA-256. `AppIcon.appiconset/` and `Ferry.icns` are brand assets Max added to the repository himself; they do not count toward this slice's production code file/LOC budget. (The `Ferry_source_1024.png` from the same period was the old letter logo, deleted on 2026-09-11 after the brand moved to the paper-boat mark.)

**Closure: MVP-01…10 all closed; the first vertical slice can be delivered within the confirmed local-only boundary.**
