# Device icons

## S0 — Scope card

- **REQUESTED — Max, 2026-08-30**: “icon 也按之前的讨论补上”；此前明确示例是 iPhone 显示 iPhone、Mac 显示 Mac。
- **Done**: Web 时间线和 Devices 面板按 `iphone / ipad / mac / android / windows / browser` 显示对应设备图标；已有 macmini 历史也不显示名字首字母。
- **Non-goals**: 不识别具体机型，不采集硬件序列号/指纹，不引入完整图标包，不改变账户或权限模型，不改 iOS 时间线视觉。
- **INFERRED (excluded)**: 设备类型不是安全身份，不能用于鉴权或设备所有权判断。
- **REPO_REQUIRED**: 旧数据库原地升级；新客户端可连接旧 Server；旧客户端可使用新 Server；API/OpenAPI/SQLite/Web/iOS join 一致；Tabler MIT notice 随发布物分发。
- **Depth**: contract — 扩展公开 JSON 与 SQLite，但不改变现有字段语义。
- **Budget**: 最多 9 个 production files，1 个 SQLite 迁移机制，0 个依赖；不创建新 endpoint/table/service。
- **Artifact budget**: 本文件作为 S0–S7 唯一状态记录。
- **Execution budget**: 本轮完成；最多 2 次无效浏览器验收尝试。
- **Boundary plan**: Go 契约/迁移测试证明兼容；真实 Web 证明用户可见图标；macmini 重建证明旧数据保留。
- **Expansion triggers**: 若需要设备指纹、具体机型库、破坏性迁移或 API v2，立即 HALT。
- **Confirmation**: Max 在看到此前“显式 device kind + 历史名字 fallback”建议后要求“按之前的讨论补上”，并再次用本 scope card 明确执行边界。

## S1 — Design decision

### Problems and evidence

- **Observed**: Web `renderMessage` 和 iOS `MessageRow` 都取发送者名字首字母作为头像；`Device` / `Message` / SQLite 当前没有设备类型。
- **Observed**: join JSON 由 `decodeStrictObject` 要求字段集合完全相等；直接增加客户端 JSON 字段会被旧 Server 拒绝。
- **Observed**: Tabler Icons 官方仓库允许 inline SVG，并以 MIT 许可发布。
- **Inferred**: 仅凭名字无法可靠覆盖用户自定义名，但足以兼容已有 `iPhone 17 Pro`、`Mac Web` 等历史记录。

### Candidates

1. **Front-end name inference only** — 文件少、无迁移；自定义名会永久显示错，Devices 与消息可能各自漂移。
2. **Required join JSON field** — 类型明确；新客户端无法连接旧 self-host Server，拒绝。
3. **Recommended / confirmed: optional header + persisted kind + history fallback** — 新旧版本双向兼容；需要两个可空 SQLite 列和幂等迁移。

### Selected mechanism

- Clients send optional `X-Ferry-Device-Kind`; an old Server ignores it.
- New Server accepts only the six enum values; missing header infers from the normalized device name.
- `devices.kind` owns current device type; message creation copies it to `messages.sender_kind`, so revocation cannot rewrite history.
- Existing nullable rows are projected through the same name inference; API responses always emit a known value.
- Web SVGs are a small vendored Tabler path subset built with trusted SVG DOM APIs; license text ships inside embedded assets.

Plain brief: Ferry learns only a broad UI category such as iPhone or Mac, not a unique hardware identity. It stores that category with the device and each new message, while old rows are inferred from their existing names. If this is wrong, the visible icon is wrong; authentication and message ownership remain unchanged.

### Rules and counterexamples

| Rule | Mechanism | Counterexample that must fail/fallback |
| --- | --- | --- |
| Only known kinds enter storage | strict header parser + DB CHECK | `X-Ferry-Device-Kind: car` → 400, no device |
| New client works with old Server | optional HTTP header, unchanged JSON | old strict two-field join still succeeds |
| Old client works with new Server | missing header → name inference | `{device_name:"iPhone",password:""}` → iphone |
| History survives migration | nullable columns + read-time inference | pre-migration `Mac Web` row → mac |
| Unknown names are deterministic | one server fallback | `Kitchen Display` → browser |

## S2 — Frozen ship checklist

1. **REQUESTED** — real Web renders distinct iPhone and Mac SVG avatars and no initial letters for those rows.
2. **REQUESTED** — real Devices panel renders the corresponding device icons.
3. **DESIGN_NECESSARY** — old DB migrates without data loss; old/null iPhone and Mac rows infer correctly; unknown falls back to browser.
4. **DESIGN_NECESSARY** — missing/valid header joins succeed, invalid/duplicate header fails without creating a device; unchanged JSON remains old-Server compatible.
5. **REPO_REQUIRED** — OpenAPI, Go/iOS/Web tests, full build/race/vet, Docker build, license notice, adversarial review, independent sealed contract review and final macmini journey pass.

Status: `shipped`; source: `cf9f3af`; deployed image: `sha256:c67deffa1aad731f5783ddab0962e918cd31857e335c5ab14f7deb454d23f06f`; review round: 2; invalidations: 1.

## S3 — Build and boundary evidence

Production budget: 8/9 files, one idempotent SQLite migration, no new dependency/endpoint/table/service.

- `go test ./...`, `go test -race ./...`, and `go vet ./...` passed.
- iOS `FerryTests` passed on one iPhone 17 Pro Simulator: 20 tests, `** TEST SUCCEEDED **`; the simulator was shut down afterwards.
- Final `docker compose build` passed and produced local image `sha256:0fbf4f4d2da813258a34d30b5fd4f6ab8fca9b9517d776fce25e3002dc3178c6`.
- OpenAPI YAML parsed successfully and its `DeviceKind` enum matched all six UI kinds.
- A real checkout of old Server commit `65adc74` accepted a new-client join carrying `X-Ferry-Device-Kind: iphone` with unchanged JSON and returned `201 Created`.
- A pre-migration SQLite fixture reopened twice without data loss; null historical rows remained nullable on disk and projected as Mac/iPhone through read-time inference.
- Real local API joins produced an iPhone message with `sender_kind: iphone` and an arbitrary-name Studio message with `sender_kind: mac`.
- Real browser journey at 390×844 rendered two empty-text SVG avatars with different mobile/desktop paths; Devices rendered three device SVGs and no initials. Evidence: `/private/tmp/ferry-device-icons-timeline.png`, `/private/tmp/ferry-device-icons-devices.png`.

### Contract matrix

| Change | Declaration | Implementation evidence |
| --- | --- | --- |
| Loosen: kind may be absent | optional OpenAPI join header | missing-header tests infer from name |
| Loosen: old storage may lack columns/values | nullable migration contract | legacy fixture migrates and second open is idempotent |
| Tighten: present kind is a six-value enum | OpenAPI `DeviceKind` | invalid/empty/whitespace/comma/duplicate header table returns 400 |
| Tighten: stored non-null kind must be valid | SQLite `CHECK` + Go validation | corrupt stored kind fails closed in store test |

### Builder attack record

1. **Boundary inversion**: treated the kind as untrusted presentation metadata; it does not participate in auth, ownership, or token issuance.
2. **Temporal**: reopened the migrated legacy database twice and verified historical null rows still project deterministically.
3. **Input equivalence**: tested missing, empty, whitespace, uppercase, unknown, comma-joined, and duplicate header forms. This caught and fixed the original empty-header/absent-header ambiguity.
4. **Partial failure**: invalid headers are rejected before device creation; tests assert the device count remains unchanged.
5. **Compatibility**: executed a new-header join against the actual old Server commit; JSON body remained the old strict two-field shape.
6. **Presentation fallback**: unknown or absent API kinds use the browser icon; historical known names are inferred by the Server rather than duplicated UI heuristics.
7. **Supply-chain/license**: vendored only six path definitions from official Tabler SVGs and embedded the complete upstream MIT notice in the Web assets.

## S4 — Adversarial self-review, round 1

Round-1 conclusion was invalidated by the independent sealed review. It found one P2: Store message creation accepted a caller-supplied legal kind when only `id + name` matched the persisted device. The fix makes the same atomic `INSERT … SELECT` require the stored kind to match; legacy NULL rows accept only the server inference for their persisted name. `TestMessageKindMustMatchPersistedDeviceKind` fossilizes explicit-text, explicit-file, legacy-success, and forged-legacy failures, including blob cleanup.

Known compatibility boundary: API 0.4 adds required response properties, so an external client that rejects unknown JSON properties must update; Ferry's previous iOS/Web consumers are tolerant, and the actual old Server accepted the new optional header.

1. **Coupled state** — The diff adds no timer/counter/cache; persistent producers are `devices.kind` and `messages.sender_kind`, while Web adds immutable `localDeviceKind` and icon paths. `rg CreateDevice/CreateText/CreateFile/scanDevice/scanMessage` found every reader/writer; race suite passed.
2. **Failure paths** — Missing/null values infer, corrupt non-null values fail closed, and invalid headers return before token/device creation. `TestLegacyDatabaseMigratesDeviceKindsAndInfersHistory` and `TestJoinDeviceKindHeaderIsOptionalStrictAndCopiedToMessages` passed.
3. **Unchanged consumers** — Full caller grep found Go store/server/tests and Swift decoding. Swift `Device`/`MessagePayload` omit the new keys and `JSONDecoder` ignores unknown keys; real old commit accepted the unchanged join JSON plus header with 201.
4. **Contract surfaces** — OpenAPI 0.4, two nullable checked SQLite columns, optional header, and MIT asset were enumerated. `ruby YAML.safe_load` parsed the schema; the 2×2 contract matrix above has declaration and implementation witnesses.
5. **Original behavior** — Local browser at 390×844 showed distinct phone/desktop paths in the original avatar locations, empty avatar text, and three icon-bearing device rows; screenshots recorded under `/private/tmp`.
6. **Journey freshness** — No production code changed after that journey; only this evidence document changed. Final macmini replay remains an S7 gate.
7. **Mechanism discrimination** — Same-run API responses contained `sender_kind: iphone/mac`; same-run DOM paths differed and avatar text was empty. The unknown-kind DOM fallback is separately pinned by `DEVICE_ICON_PATHS[kind] || ...browser` and Web asset tests.
8. **Regression scan** — Candidate regression was old-client/new-server decoding and new-client/old-server strict join. Swift ignores new response keys; old commit 65adc74 returned 201 to the new header; complete Go/race/iOS suites passed.
9. **Scale/edge** — Device kind is constant-size and validated before storage; absent/empty/duplicate/extreme-equivalent inputs are covered. Empty timelines/device arrays remain handled by existing render loops. Tenant isolation is N/A for one self-host instance.
10. **Contract attacks** — Dual schema/runtime judges, old/new extremes, equivalent header spellings, missing-default gate, direct store side doors, auth non-use, and independent-verifier requirement are all recorded in S3; empty-vs-absent was found and fossilized before this pass.
11. **Predicate producers** — `rg sender_kind|DeviceKind|localDeviceKind` found only validated header/name inference, checked SQLite scanners, typed message creation, and the finite browser detector; no repair/stub producer can assert a privileged state because kind is presentation-only.
12. **Reversed findings** — N/A: no external finding was reversed in this round; the builder's earlier empty-header finding retained and answered its producer/sibling/default questions in the invalid-input table.
13. **Pass limit** — This is only the author pre-filter. S5 plan comparison and S6 fresh zero-context contract verification remain mandatory before commit/push.

## S4 — Adversarial self-review, round 2

Conclusion: `ship candidate`; the round-1 P2 is closed with a same-statement ownership gate and a direct side-door regression test. A fresh S6 verifier and deployed S7 journey remain mandatory.

1. **Coupled state** — No timer/cache/counter changed. The repaired predicates are immutable device `id/name/kind` and the message snapshot; `rg CreateTextForDevice|CreateFileForDevice|sender_kind` enumerated all producers, and targeted race tests passed.
2. **Failure paths** — Mismatched explicit and legacy kinds return `ErrUnauthorized`; file failure removes its already-created blob via the existing defer. `TestMessageKindMustMatchPersistedDeviceKind` asserts both behavior and an empty blob directory.
3. **Unchanged consumers** — HTTP calls still pass the authenticated DB-scanned Device; direct anonymous Store helpers still infer from sender name. Full `go test ./...` passed after the query change.
4. **Contract surfaces** — No contract shape changed in the repair; only Store enforcement tightened to match the already documented owner invariant. OpenAPI and SQLite enum remain unchanged.
5. **Original reproduction** — The exact same-ID/same-name/different-legal-kind text and file calls now return `ErrUnauthorized`; the test also proves the correct inferred legacy path still writes one iPhone message.
6. **Journey freshness** — Current production working tree was restarted and replayed at 390×844: two messages, distinct iPhone/Mac paths, empty avatar text; Devices opened with three icons and empty icon text. Recording: `<browser-harness recordings>/ferry-device-icons-final-local`.
7. **Mechanism discrimination** — The positive explicit/inferred paths and negative forged-kind paths execute the same atomic SQL branch; removing the kind predicate would make the two forged assertions fail by creating extra messages/blobs.
8. **Regression scan** — Candidate regression was rejecting legacy NULL devices. The test explicitly NULLs a real device kind, re-authenticates to inferred iPhone, and successfully creates exactly one message before rejecting a forged Mac kind.
9. **Scale/edge** — The repair adds constant comparisons only; NULL and non-NULL extremes are both covered. Empty blob cleanup is asserted. Single self-host tenant remains unchanged.
10. **Contract attacks** — Dual DB/API judges remain aligned; legal-but-wrong enum is now covered as the equivalent side-door spelling missed in round 1; NULL is no longer a defaults backdoor; kind remains presentation-only.
11. **Predicate producers** — The SQL accepts either exact persisted non-NULL kind or exact server inference for a persisted-name-matching NULL row. Parameters are caller kind plus server `inferDeviceKind(senderName)`; `id + name` must match the same persisted row.
12. **Reversed finding questions** — Both sibling writes (text/file), both producer classes (explicit/non-NULL and legacy/NULL), failure cleanup, and the normal legacy success path are answered by one regression test.
13. **Pass limit** — Author review cannot close S6. A new zero-context reviewer must read the repaired complete Store and re-run the side-door test before ship.

## S5/S6 — Independent acceptance, round 2

Fresh zero-context verifier verdict: code-level **SHIP**, P0/P1/P2 = 0; overall remains gated only by S7 deployed journey.

- Read every changed file in full, all Device/Message kind producers and consumers, and the replaced base `65adc74` implementation before reading this document.
- Reproduced the round-1 same-ID/name/different-kind side door and confirmed the repaired text/file statements reject it atomically.
- Confirmed explicit and legacy NULL owners, positive and forged paths, blob cleanup, in-flight revoke, and cross-revoke behavior; seven targeted tests passed for 20 consecutive runs.
- Independently ran Go, race, vet, Node syntax, OpenAPI enum parse, and diff-check successfully.
- Confirmed S0 scope/budget, S1 mechanism, S2 items 1–4, and S4 repair parity. No code or design deviation remains.
- The verifier intentionally did not run Xcode or deploy; the recorded 20-test iOS pass remains the iOS evidence, and the macmini journey remains the release gate.

## S7 — Mac mini deployment journey

- Pushed source commit `cf9f3af` to `origin/main`, synchronized the reviewed runtime files, and rebuilt the existing Docker Compose service in place.
- Container `ferry` is up on `10.0.0.2:42817` with image `sha256:c67deffa1aad731f5783ddab0962e918cd31857e335c5ab14f7deb454d23f06f`; startup completed without migration error.
- Live `/healthz` returned 200 with `{"status":"ok"}`; the embedded Tabler license endpoint returned the upstream title, URL, MIT notice, and copyright.
- Existing five-message history remained present after migration. A live authenticated API read returned sender kinds in order: `mac, iphone, mac, android, android`, including the existing file message.
- Real deployed Web at 390×844 rendered five SVG avatars with empty text and distinct desktop, phone, and Android paths. The Devices panel rendered seven SVG icons with empty text, including existing Mac, iPhone, Android, and unknown-name browser fallback devices.
- Evidence screenshots: `/private/tmp/ferry-device-icons-macmini-cf9f3af-timeline.png`, `/private/tmp/ferry-device-icons-macmini-cf9f3af-devices.png`.
- Recording: `<browser-harness recordings>/ferry-device-icons-macmini-cf9f3af`.

Final verdict: **SHIP**. All five frozen checklist items passed; P0/P1/P2 = 0 after the repaired side-door re-review.
