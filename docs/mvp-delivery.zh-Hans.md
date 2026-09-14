# MVP 第一纵切交付记录

> [English](mvp-delivery.md) | 简体中文

## S0 — Confirmed scope

- **REQUESTED**：开源项目（完成后再 public），包含 iOS、Android、self-host Web 与 Server，通过聊天界面实现局域网剪贴板和文件共享，UI 学 ChatGPT。
- **Done**：真实本地 Server + 浏览器完成文本发送、文件上传、统一时间线、下载与重启持久化。
- **Non-goals**：本纵切不写移动端、不自动监听剪贴板、不做多用户/频道/公网发布。
- **INFERRED accepted（Max，2026-08-29，回复 `go`）**：Go + SQLite、Server 内嵌 Web、默认个人空间、主动发送、loopback 默认监听。
- **REPO_REQUIRED**：产品核心、真实行为证据、最小模拟器负载（本次无模拟器）、对抗式自审。
- **Depth**：contract；新建跨运行时 HTTP 合约与持久化文件边界。
- **Budget**：≤12 production files、约 ≤1,400 production LOC、≤3 process/design docs；不发布、不 push、不装全局工具；最多 3 次无效尝试。
- **Boundary plan**：真实本地 Go service + 真实浏览器关闭 Done；Go HTTP tests 关闭边界反例。无外部 provider。
- **Expansion triggers**：账号/TLS/设备发现/后台剪贴板/多空间/额外常驻服务需要 HALT。

## S1 — Design decision

选择 `docs/architecture.md` 候选 A：Go 单进程 + SQLite + 内嵌 Web。用户确认 scope card 后进入实现；候选 B 的多服务运维和候选 C 的点对点复杂度被拒绝。

## S2 — Frozen ship checklist

| ID | 可关闭属性 | 机器检查 |
| --- | --- | --- |
| MVP-01 | Go production 与 tests 可编译、格式正确 | `go test ./...`、`go vet ./...`、`gofmt` drift check |
| MVP-02 | OpenAPI 与真实 endpoint/判别联合一致 | HTTP contract tests 覆盖 text/file/list/download/error shape |
| MVP-03 | 文本边界拒绝空白和 64 KiB + 1，保留合法原文 | discriminating tests |
| MVP-04 | 文件名不能穿越路径，64 MiB + 1 被拒绝且不新增消息 | traversal/oversize HTTP tests |
| MVP-05 | 同 data dir 重开 store 后消息与文件仍可读 | persistence integration test |
| MVP-06 | 未鉴权进程只允许监听 loopback，Web 与 API 由同一进程提供 | CLI 拒绝探针 + config/run tests + running process observation |
| MVP-07 | 真实浏览器完成 text/file/download/restart journey | final-subject transcript，按 `docs/architecture.md` 预期逐项读取 |
| MVP-08 | 非平凡 diff 完成 1–13 自审与七类边界攻击 | 本文 S3/S4 有逐项命令或测试证据 |
| MVP-09 | 完整文件审查没有未关闭 P0/P1；fresh gate 有明确 verdict | 本文 S5/S6 |
| MVP-10 | 预算、git whitespace 与工作区归属关闭 | file/LOC count、`git diff --check`、`git status --short` |

Checklist 已冻结。没有 REQUESTED 或 DESIGN_NECESSARY 来源的新功能不得加入。

## S3 — Build and attack record

最终受测二进制为 `/private/tmp/ferry-final-acceptance/ferry`，SHA-256 `709f1dbeff55419a63a9d49fc76d599bed22bacb65808936fbcdb86f7c0256d8`。它由当前 8 个 production files 构建；此后只新增测试与本交付记录，没有修改 production source。

七类攻击记录：

1. **双裁判**：用 HTTP contract tests 同时检查 OpenAPI 声明与 handler 的 text/file/error shape；用完整代码核对发现并修正架构中不存在的 blob rename 描述。
2. **极值**：`TestTextBoundariesAndPreservation` 覆盖 64 KiB ±1/空白/原文保留；`TestFileSizeBoundaries` 与浏览器稀疏文件探针覆盖 0、64 MiB、64 MiB+1。
3. **等价拼写**：JSON 测试拒绝重复 key（含 escape 等价拼写）、非法 UTF-8 与 lone surrogate；listener 测试覆盖 `localhost`/`LOCALHOST`、IPv4/IPv6 loopback 与伪 hostname。
4. **默认后门**：默认值是 `127.0.0.1:8080`，但显式 `-listen 0.0.0.0:18089` 同样在启动前失败，退出码 1，错误为 `listen host must be localhost or a loopback IP address`。
5. **旁路**：`parseConfig`、直接 `run(config)`、实际 `net.Listener` 地址三层都拒绝非 loopback；`TestRunRejectsNonLoopbackListenerWithoutCreatingData` 证明直接调用旁路也不会创建持久化文件。
6. **策略有门**：loopback、Host、Origin、输入大小、路径与 DB 判别均有对应失败测试；把 `0.0.0.0` 送入 CLI 与 `run` 会实际变红，不只依赖文档。
7. **不能自证**：作者以真实进程/浏览器关闭旅程；独立 verifier 先找到 3 个问题，修复后再针对原复现复核，结果记录在 S6。

当前 production 的真实浏览器旅程（录制目录 `<browser-harness recordings>/ferry-current-head`，19 frames）：

- 空时间线启动后发送 `hello ferry current head`，DOM 得到 1 条相同正文，连接 `Local`，无错误。
- 上传 `README-upload.md` 后时间线为 2 条，文件卡 URL 为 `/api/v1/files/<message-id>`；点击卡片下载后 `cmp` 与上传原件一致。
- 上传 67,108,865-byte 文件并等待 7 秒（超过多个 polling 周期），时间线仍为 2 条，错误持续显示 `file exceeds 67108864 bytes`，文件 chip 保留。
- 同 data dir 重启，2 条历史与文件卡仍在；成功发送 `recovery probe` 后状态恢复为正常隐私提示。
- 停服后浏览器显示 `Offline / Failed to fetch / error=true`；再次同目录重启后自动变为 `Local / Your data stays on this Ferry server. / error=false`，3 条历史保留。

## S4 — Adversarial self-review

结论：**作者侧可以 ship 到已冻结的本地 MVP 边界；该结论不是独立最终绿灯。** 本轮在 fresh verifier 首轮发现后修正三项，随后完成下列 1–13 记录：

1. **Coupled state**：枚举 `cursor/loading/activityStatus/connectionError/sendError/rendered` 的全部 producer；真实交错验证“超限 send error 不被 polling 清除”和“纯 connection error 在恢复后清除”，证据为 S3 DOM 三元组。
2. **Failure paths**：listener 在 store 打开前验证且有 `defer Close`；send/poll 的 success/catch 分别写自己的错误源；证据为 `TestRunRejectsNonLoopbackListenerWithoutCreatingData`、race suite 与断线旅程。
3. **Unchanged callers**：`rg 'run\(|parseConfig|ListenAndServe|server.Serve'` 仅找到 `main`、config tests 与新 direct-run test；没有未迁移的 `ListenAndServe` caller。
4. **Contract surfaces**：endpoint、OpenAPI、DB schema 和消息 shape 未因修复改变；`node --check`、OpenAPI YAML parse 与全套 HTTP contract tests 通过。
5. **Original reproduction**：`ferry -listen 0.0.0.0:18089 ...` 退出 1；浏览器 Offline→restart 后从 `Failed to fetch/error=true` 变为 privacy text/error=false。
6. **Current journey**：在最终 production source 构建的 SHA-256 `709f…256d8` 上重放并读取 DOM 内容，录制 19 frames；下载另以 `cmp` 验证字节。
7. **Mechanism discrimination**：停服使 Local/正常提示消失并出现 Offline/错误，重启使其恢复；超限文件没有新增消息，证明不是 UI fallback 假成功。
8. **Regression scan**：候选回归是合法 IPv6/大小写 localhost 被误拒；focused config test 的 `[::1]`、`LOCALHOST` 均通过，非 loopback 与伪 hostname 均拒绝。
9. **Scale/edge**：0-byte、64 MiB、64 MiB+1、64 KiB ±1、空时间线、重复请求与损坏 DB metadata 均有测试；tenant isolation 为 N/A — 本纵切明确只有一个个人空间。
10. **Contract attacks**：七类攻击逐项落在 S3，并由 focused listener tests、HTTP tests、CLI kill probe 与浏览器 kill probe 给出实际结果。
11. **Predicate producers**：`rg 'activityStatus|connectionError|sendError|setStatus|renderStatus|validateLoopbackAddress|IsLoopback'` 显示 UI 状态只由 load/send 两路径生产，监听判定由 config、run 与实际 socket 三处生产。
12. **Reversed findings**：Host 可伪造的问题没有靠更严 Host 掩盖，而是在真正 listener 边界关闭；恢复问题保留 send-error 持久语义，只清除已恢复的 connection-error；rename 文案按真实 O_EXCL 写入机制修正。
13. **Pass limit**：作者记录只关闭预过滤；有限 Done 是 MVP-01…10，独立 verdict 单列 S6，不以“没人再发现问题”为终止条件。

## S5 — Full-code review

作者在修复后重读完整 changed files、grep 全部 caller/state producer，并以全套 tests、race、vet、真实进程验证消费者。作者 verdict：当前没有未关闭 P0/P1；首轮审查的 3 个问题已分别固化为 tests、kill probe 与准确文档。限制：这是作者审查，不能替代 S6。

## S6 — Fresh verification

独立 verifier 的 sealed 首轮结论为 **不能 ship**：P1 非 loopback 监听可用伪造 Host 绕过；P2 Server 恢复后 UI 保留 `Failed to fetch`；文档 P2 描述了不存在的 blob rename。

修复后由同一位 verifier 做只读 closure QA，最终 verdict 为 **PASS；没有新的 P0/P1/P2**：

- `0.0.0.0:18109` 启动退出 1 且没有 listener；合法 `localhost:18110` 的实际 listener 为 `127.0.0.1`；direct `run(config)` 测试通过且拒绝前没有创建 data 内容。
- 真实浏览器依次观察 Local/normal → 停服后的 Offline/`Failed to fetch` → 恢复后的 Local/normal；64 MiB+1 send error 经多个成功 poll 仍保留，下一次成功 send 后清除。独立录制目录为 `<browser-harness recordings>/ferry-closure-status`（15 frames）。
- crash-window 文案与真实 copy → sync → close → SQLite insert 顺序一致。
- verifier 独立重跑 `go test ./...`、race、vet、`git diff --check` 全部通过，并停止全部 QA Server。

## S7 — Closure

| Checklist | Closure evidence |
| --- | --- |
| MVP-01 | `go test -count=1 ./...`、`go vet ./...`、`test -z "$(gofmt -l cmd internal)"` 通过 |
| MVP-02 | HTTP contract tests 全绿；`api/openapi.yaml` 由 Ruby YAML parser 成功读取 |
| MVP-03 | text 64 KiB ±1、空白、Unicode/JSON 反例 tests 全绿 |
| MVP-04 | traversal、0/64 MiB/64 MiB+1 tests 与真实浏览器 kill probe 通过 |
| MVP-05 | store reopen test 与两次真实同目录 Server restart 通过 |
| MVP-06 | CLI kill probe、config/direct-run tests、真实 listener 验证通过 |
| MVP-07 | 作者 19-frame 当前二进制旅程 + 独立 verifier 15-frame closure 旅程通过 |
| MVP-08 | S3 七类攻击、S4 1–13 均有具体命令/test/运行中证据 |
| MVP-09 | 作者完整代码审查无未关闭 P0/P1；fresh closure verdict PASS，无新 P0/P1/P2 |
| MVP-10 | 本纵切 8 个 production code files、1,386 LOC；`git diff --check` 通过；没有 commit/push/publish |

最终复跑还包括 `go test -race -count=1 ./...`、`go mod verify`、`node --check`、Linux amd64 `CGO_ENABLED=0 go build`。重建二进制与浏览器受测二进制 SHA-256 完全相同。`AppIcon.appiconset/`、`Ferry.icns` 是 Max 主动加入仓库的品牌资产；它们不计入本纵切的 production code file/LOC 预算。（同期的 `Ferry_source_1024.png` 是旧字母 logo，已于 2026-09-11 随品牌换成纸船 mark 后删除。）

**Closure：MVP-01…10 全部关闭；第一纵切可以在已确认的 local-only 边界交付。**
