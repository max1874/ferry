# 设备图标

> [English](device-icons.md) | 简体中文

## S0 — 范围卡

- **REQUESTED — Max，2026-08-30**：“icon 也按之前的讨论补上”；此前明确示例是 iPhone 显示 iPhone、Mac 显示 Mac。
- **完成标准**：Web 时间线和 Devices 面板按 `iphone / ipad / mac / android / windows / browser` 显示对应设备图标；已有测试服务器历史也不显示名字首字母。
- **不做**：不识别具体机型，不采集硬件序列号/指纹，不引入完整图标包，不改变账户或权限模型，不改 iOS 时间线视觉。
- **INFERRED（不纳入）**：设备类型不是安全身份，不能用于鉴权或设备所有权判断。
- **REPO_REQUIRED**：旧数据库原地升级；新客户端可连接旧 Server；旧客户端可使用新 Server；API/OpenAPI/SQLite/Web/iOS join 一致；Tabler MIT notice 随发布物分发。
- **深度**：contract — 扩展公开 JSON 与 SQLite，但不改变现有字段语义。
- **预算**：最多 9 个生产文件，1 个 SQLite 迁移机制，0 个依赖；不创建新 endpoint/table/service。
- **产物预算**：本文件作为 S0–S7 唯一状态记录。
- **执行预算**：本轮完成；最多 2 次无效浏览器验收尝试。
- **边界计划**：Go 契约/迁移测试证明兼容；真实 Web 证明用户可见图标；测试服务器重建证明旧数据保留。
- **扩张触发**：若需要设备指纹、具体机型库、破坏性迁移或 API v2，立即 HALT。
- **确认**：Max 在看到此前“显式 device kind + 历史名字 fallback”建议后要求“按之前的讨论补上”，并再次用本范围卡明确执行边界。

## S1 — 设计决策

### 问题与证据

- **Observed**：Web `renderMessage` 和 iOS `MessageRow` 都取发送者名字首字母作为头像；`Device` / `Message` / SQLite 当前没有设备类型。
- **Observed**：join JSON 由 `decodeStrictObject` 要求字段集合完全相等；直接增加客户端 JSON 字段会被旧 Server 拒绝。
- **Observed**：Tabler Icons 官方仓库允许 inline SVG，并以 MIT 许可发布。
- **Inferred**：仅凭名字无法可靠覆盖用户自定义名，但足以兼容已有 `iPhone 17 Pro`、`Mac Web` 等历史记录。

### 候选方案

1. **仅前端按名字推断** — 文件少、无迁移；自定义名会永久显示错，Devices 与消息可能各自漂移。
2. **必填的 join JSON 字段** — 类型明确；新客户端无法连接旧的自托管 Server，拒绝。
3. **推荐 / 已确认：可选 header + 持久化 kind + 历史 fallback** — 新旧版本双向兼容；需要两个可空 SQLite 列和幂等迁移。

### 选定机制

- 客户端发送可选的 `X-Ferry-Device-Kind`；旧 Server 忽略它。
- 新 Server 只接受六个枚举值；缺少 header 时从归一化后的设备名推断。
- `devices.kind` 拥有设备当前类型；创建消息时把它复制到 `messages.sender_kind`，所以撤销设备不会改写历史。
- 已有的可空行通过同一套名字推断投影出来；API 响应总是给出一个已知值。
- Web SVG 是 vendored 的一小组 Tabler path，用可信的 SVG DOM API 构建；许可证文字随内嵌资源一起分发。

一句话说明：Ferry 只知道一个粗略的 UI 类别，比如 iPhone 或 Mac，而不是唯一的硬件身份。它把这个类别存到设备和每条新消息上，旧数据则从已有的名字推断。如果这里做错了，错的只是显示的图标；鉴权和消息归属不受影响。

### 规则与反例

| 规则 | 机制 | 必须失败/回退的反例 |
| --- | --- | --- |
| 只有已知类型能进存储 | 严格的 header 解析 + DB CHECK | `X-Ferry-Device-Kind: car` → 400，不创建设备 |
| 新客户端能用旧 Server | 可选 HTTP header，JSON 不变 | 旧的严格两字段 join 仍然成功 |
| 旧客户端能用新 Server | 缺少 header → 名字推断 | `{device_name:"iPhone",password:""}` → iphone |
| 历史在迁移后保留 | 可空列 + 读取时推断 | 迁移前的 `Mac Web` 行 → mac |
| 未知名字结果确定 | 唯一的 Server 回退 | `Kitchen Display` → browser |

## S2 — 冻结的交付检查清单

1. **REQUESTED** — 真实 Web 为这些行渲染出不同的 iPhone 和 Mac SVG 头像，没有首字母。
2. **REQUESTED** — 真实 Devices 面板渲染出对应的设备图标。
3. **DESIGN_NECESSARY** — 旧 DB 迁移不丢数据；旧的/空的 iPhone 和 Mac 行推断正确；未知值回退为 browser。
4. **DESIGN_NECESSARY** — 缺少/合法 header 的 join 成功，非法/重复 header 失败且不创建设备；不变的 JSON 保持与旧 Server 兼容。
5. **REPO_REQUIRED** — OpenAPI、Go/iOS/Web 测试、完整 build/race/vet、Docker build、许可证声明、对抗式评审、独立封闭合约评审和最终测试服务器旅程都通过。

状态：`shipped`；源码：`cf9f3af`；部署镜像：`sha256:c67deffa1aad731f5783ddab0962e918cd31857e335c5ab14f7deb454d23f06f`；评审轮次：2；失效：1。

## S3 — 构建与边界证据

生产预算：8/9 个文件，一个幂等 SQLite 迁移，没有新依赖/endpoint/table/service。

- `go test ./...`、`go test -race ./...` 和 `go vet ./...` 通过。
- iOS `FerryTests` 在一台 iPhone 17 Pro 模拟器上通过：20 个测试，`** TEST SUCCEEDED **`；之后关闭了模拟器。
- 最终 `docker compose build` 通过，生成本地镜像 `sha256:0fbf4f4d2da813258a34d30b5fd4f6ab8fca9b9517d776fce25e3002dc3178c6`。
- OpenAPI YAML 解析成功，其 `DeviceKind` 枚举与 UI 的六种类型一致。
- 真实检出的旧 Server commit `65adc74` 接受了一个带 `X-Ferry-Device-Kind: iphone`、JSON 不变的新客户端 join，返回 `201 Created`。
- 一个迁移前的 SQLite 夹具重新打开两次都没有丢数据；历史的空行在磁盘上仍为空，并通过读取时推断投影为 Mac/iPhone。
- 真实的本地 API join 产生了一条 `sender_kind: iphone` 的 iPhone 消息，以及一条任意名字 Studio、`sender_kind: mac` 的消息。
- 390×844 的真实浏览器旅程渲染出两个文字为空、手机/桌面 path 不同的 SVG 头像；Devices 渲染出三个设备 SVG，没有首字母。证据：`/private/tmp/ferry-device-icons-timeline.png`、`/private/tmp/ferry-device-icons-devices.png`。

### 合约矩阵

| 改动 | 声明 | 实现证据 |
| --- | --- | --- |
| 放宽：kind 可以缺省 | OpenAPI 可选的 join header | 缺少 header 的测试从名字推断 |
| 放宽：旧存储可能缺列/缺值 | 可空迁移合约 | 旧夹具迁移成功，第二次打开是幂等的 |
| 收紧：提供的 kind 是六值枚举 | OpenAPI `DeviceKind` | 非法/空/空白/逗号/重复 header 表返回 400 |
| 收紧：存储的非空 kind 必须合法 | SQLite `CHECK` + Go 校验 | store 测试中损坏的存储 kind 失败关闭 |

### 构建者攻击记录

1. **边界反转**：把 kind 当作不可信的展示元数据；它不参与鉴权、归属或 token 签发。
2. **时间维度**：把迁移后的旧数据库重新打开两次，确认历史空行仍能确定地投影。
3. **输入等价**：测试了缺失、空、空白、大写、未知、逗号拼接和重复的 header 形式。这发现并修复了最初空 header 与缺失 header 的歧义。
4. **部分失败**：非法 header 在创建设备之前被拒绝；测试断言设备数量不变。
5. **兼容性**：对真实的旧 Server commit 执行了带新 header 的 join；JSON 正文保持旧的严格两字段形状。
6. **展示回退**：未知或缺失的 API kind 使用 browser 图标；已知的历史名字由 Server 推断，而不是在 UI 里重复一套启发式。
7. **供应链/许可证**：只从官方 Tabler SVG vendor 了六个 path 定义，并在 Web 资源里内嵌了完整的上游 MIT 声明。

## S4 — 对抗式自审，第 1 轮

第 1 轮的结论被独立封闭评审推翻。它发现了一个 P2：Store 创建消息时，只要 `id + name` 与持久化的设备匹配，就会接受调用方提供的任意合法 kind。修复让同一条原子的 `INSERT … SELECT` 要求存储的 kind 也匹配；旧的 NULL 行只接受 Server 按其持久化名字推断出的值。`TestMessageKindMustMatchPersistedDeviceKind` 固化了显式文字、显式文件、旧数据成功和伪造旧数据失败几种情况，包括 blob 清理。

已知兼容边界：API 0.4 增加了必需的响应属性，所以拒绝未知 JSON 属性的外部客户端需要更新；Ferry 之前的 iOS/Web 消费者是宽容的，真实的旧 Server 也接受了新的可选 header。

1. **耦合状态** — diff 没有新增定时器/计数器/缓存；持久化 producer 是 `devices.kind` 和 `messages.sender_kind`，Web 新增了不可变的 `localDeviceKind` 和图标 path。`rg CreateDevice/CreateText/CreateFile/scanDevice/scanMessage` 找到了所有读写方；race 测试通过。
2. **失败路径** — 缺失/空值走推断，损坏的非空值失败关闭，非法 header 在创建 token/设备之前就返回。`TestLegacyDatabaseMigratesDeviceKindsAndInfersHistory` 和 `TestJoinDeviceKindHeaderIsOptionalStrictAndCopiedToMessages` 通过。
3. **未改动的消费者** — 完整的调用方 grep 找到了 Go store/server/tests 和 Swift 解码。Swift 的 `Device`/`MessagePayload` 不含新 key，`JSONDecoder` 忽略未知 key；真实旧 commit 以 201 接受了不变的 join JSON 加 header。
4. **合约面** — 列出了 OpenAPI 0.4、两个带检查的可空 SQLite 列、可选 header 和 MIT 资源。`ruby YAML.safe_load` 解析了 schema；上面的 2×2 合约矩阵有声明和实现两方面的见证。
5. **原始行为** — 390×844 的本地浏览器在原来头像的位置显示了不同的手机/桌面 path、空的头像文字，以及三行带图标的设备；截图记录在 `/private/tmp` 下。
6. **旅程新鲜度** — 那次旅程之后没有改动生产代码；只改了这份证据文档。最终的测试服务器重放仍是 S7 门禁。
7. **机制区分** — 同一次运行的 API 响应包含 `sender_kind: iphone/mac`；同一次运行的 DOM path 不同，头像文字为空。未知 kind 的 DOM 回退由 `DEVICE_ICON_PATHS[kind] || ...browser` 和 Web 资源测试单独固定。
8. **回归扫描** — 候选回归是旧客户端/新 Server 的解码，以及新客户端/旧 Server 的严格 join。Swift 忽略新的响应 key；旧 commit 65adc74 对新 header 返回 201；完整的 Go/race/iOS 测试通过。
9. **规模/边界** — 设备类型大小固定，存储前校验；缺失/空/重复/极端等价的输入都有覆盖。空时间线/空设备数组仍由既有渲染循环处理。租户隔离对单个自托管实例不适用。
10. **合约攻击** — 双 schema/运行时裁判、新旧版本两个极端、等价的 header 拼写、缺省值门禁、直接的 store 旁路、不参与鉴权和独立验证者要求都记录在 S3；空与缺失的区分在本轮之前就发现并固化了。
11. **断言 producer** — `rg sender_kind|DeviceKind|localDeviceKind` 只找到经过校验的 header/名字推断、带检查的 SQLite 扫描、有类型的消息创建和有限的浏览器检测；由于 kind 只用于展示，没有任何修复/桩 producer 能断言特权状态。
12. **被推翻的发现** — 不适用：本轮没有推翻外部发现；构建者早先的空 header 发现保留下来，并在非法输入表里回答了它的 producer/同类/默认值问题。
13. **轮次上限** — 这只是作者预筛。S5 计划对照和 S6 全新零上下文合约验证在 commit/push 前仍是必需的。

## S4 — 对抗式自审，第 2 轮

结论：`ship candidate`；第 1 轮的 P2 通过同一条语句内的归属门禁和一个直接的旁路回归测试关闭。全新的 S6 验证者和部署后的 S7 旅程仍是必需的。

1. **耦合状态** — 没有改动定时器/缓存/计数器。修复后的断言是不可变的设备 `id/name/kind` 和消息快照；`rg CreateTextForDevice|CreateFileForDevice|sender_kind` 列出了所有 producer，定向的 race 测试通过。
2. **失败路径** — 不匹配的显式和旧数据 kind 都返回 `ErrUnauthorized`；文件失败时通过既有的 defer 删除已创建的 blob。`TestMessageKindMustMatchPersistedDeviceKind` 同时断言了行为和空的 blob 目录。
3. **未改动的消费者** — HTTP 调用仍然传入经过鉴权、从 DB 扫描出来的 Device；直接的匿名 Store helper 仍从发送者名字推断。改完查询后完整的 `go test ./...` 通过。
4. **合约面** — 修复没有改变合约形状；只是收紧了 Store 的执行，使之符合已记录的归属不变式。OpenAPI 和 SQLite 枚举不变。
5. **原始复现** — 完全相同的「同 ID/同名/不同合法 kind」的文字和文件调用现在返回 `ErrUnauthorized`；测试也证明正确推断的旧数据路径仍会写入一条 iPhone 消息。
6. **旅程新鲜度** — 当前生产工作树重启后在 390×844 下重放：两条消息，不同的 iPhone/Mac path，头像文字为空；打开 Devices 有三个图标，图标文字为空。录制：`<browser-harness recordings>/ferry-device-icons-final-local`。
7. **机制区分** — 正向的显式/推断路径和负向的伪造 kind 路径执行的是同一个原子 SQL 分支；去掉 kind 断言会让两个伪造断言因为多出消息/blob 而失败。
8. **回归扫描** — 候选回归是拒绝旧的 NULL 设备。测试把一个真实设备的 kind 显式置为 NULL，重新鉴权得到推断的 iPhone，成功创建恰好一条消息，然后拒绝伪造的 Mac kind。
9. **规模/边界** — 修复只增加常数次比较；NULL 和非 NULL 两个极端都有覆盖。断言了空 blob 的清理。单一的自托管租户不变。
10. **合约攻击** — DB/API 双裁判保持一致；「合法但错误的枚举值」作为第 1 轮漏掉的等价旁路拼写现在已覆盖；NULL 不再是默认值后门；kind 仍只用于展示。
11. **断言 producer** — SQL 只接受与持久化非 NULL kind 完全相同的值，或者对名字匹配的 NULL 行接受 Server 推断出的完全相同的值。参数是调用方 kind 加上 Server 的 `inferDeviceKind(senderName)`；`id + name` 必须匹配同一条持久化行。
12. **被推翻发现的追问** — 两个同类写入（文字/文件）、两类 producer（显式/非 NULL 和旧数据/NULL）、失败清理和正常的旧数据成功路径，都由一个回归测试回答。
13. **轮次上限** — 作者评审不能关闭 S6。上线前，新的零上下文评审者必须读完修复后的完整 Store 并重跑旁路测试。

## S5/S6 — 独立验收，第 2 轮

全新零上下文验证者结论：代码层面 **SHIP**，P0/P1/P2 = 0；整体只剩 S7 部署旅程这一道门禁。

- 在读本文档之前，完整读了每个改动文件、所有 Device/Message kind 的 producer 和消费者，以及被替换的基线 `65adc74` 实现。
- 复现了第 1 轮「同 ID/同名/不同 kind」的旁路，并确认修复后的文字/文件语句会原子地拒绝它。
- 确认了显式和旧 NULL 两种归属、正向和伪造路径、blob 清理、进行中撤销和交叉撤销的行为；七个定向测试连续运行 20 次都通过。
- 独立运行 Go、race、vet、Node 语法、OpenAPI 枚举解析和 diff-check，都成功。
- 确认了 S0 范围/预算、S1 机制、S2 第 1–4 项以及 S4 修复的一致性。没有剩余的代码或设计偏差。
- 验证者有意没有运行 Xcode 或部署；记录的 20 项 iOS 测试通过仍是 iOS 的证据，测试服务器旅程仍是发布门禁。

## S7 — 测试服务器部署旅程

- 源码 commit `cf9f3af` 已推送到 `origin/main`，同步了评审过的运行时文件，并原地重建了既有的 Docker Compose 服务。
- 容器 `ferry` 在 `192.168.1.20:42817` 上运行，镜像 `sha256:c67deffa1aad731f5783ddab0962e918cd31857e335c5ab14f7deb454d23f06f`；启动完成，没有迁移错误。
- 线上 `/healthz` 返回 200 和 `{"status":"ok"}`；内嵌的 Tabler 许可证接口返回了上游标题、URL、MIT 声明和版权信息。
- 迁移后原有的五条消息历史仍在。线上鉴权 API 读取按顺序返回发送者类型：`mac, iphone, mac, android, android`，包括原有的文件消息。
- 390×844 的真实部署 Web 渲染出五个文字为空的 SVG 头像，桌面、手机和 Android 的 path 各不相同。Devices 面板渲染出七个文字为空的 SVG 图标，包括原有的 Mac、iPhone、Android，以及名字未知而回退为 browser 的设备。
- 证据截图：`/private/tmp/ferry-device-icons-test-server-cf9f3af-timeline.png`、`/private/tmp/ferry-device-icons-test-server-cf9f3af-devices.png`。
- 录制：`<browser-harness recordings>/ferry-device-icons-test-server-cf9f3af`。

最终结论：**SHIP**。五项冻结检查全部通过；修复旁路并重新评审后 P0/P1/P2 = 0。
