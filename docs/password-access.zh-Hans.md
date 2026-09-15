# 可选的共享密码接入

> [English](password-access.md) | 简体中文

## 范围卡

- **REQUESTED — Max，2026-08-30**：“全设备免配对；支持一个密码功能；部署 Server 的人决定访问这个地址要不要密码。”追问：密码配置必须放在 Web 设置里，不能用环境变量。
- **完成标准**：新的 Web 浏览器和 iPhone 不用配对码就能加入 Ferry；没有配置密码时直接进入，配置了密码时输入一次共享密码后进入。
- **不做**：用户账号/角色、每台设备单独的密码、互联网暴露、TLS、密码找回、强制登出已签发的设备 token，以及本阶段的 Android 实现。
- **REPO_REQUIRED**：保留设备 token 以支持记住会话和撤销、严格的请求解码、私有监听边界、不记录/不持久化明文密码、完整的 Go/iOS 测试、真实 Web/iPhone 旅程、对抗式评审和零上下文评审。
- **深度**：contract — 在 Server、Web、iOS、OpenAPI 和 Docker 之间替换公开的设备准入 API 和鉴权旅程。
- **预算**：已有的 Server/鉴权/客户端界面、一张单例 SQLite 接入设置表、一份接入文档；不加账号/角色 schema，不加第二个服务。
- **边界计划**：测试服务器上真实的 Docker Server、真实浏览器、签名的 iPhone 构建；单元测试只关闭错误/边界合约。
- **扩张触发**：如果实现需要账号/角色、TLS、第二个服务，或改动已有的消息/设备行，就 HALT。

## 决策

- **Observed**：配对需要一台已受信的设备或带外的 bootstrap code，这让首次使用的路径成了循环依赖，并且明显让用户卡住了。
- **Rejected**：只对 Web 豁免；用户仍然需要理解为什么某一台设备有特权。
- **Rejected**：保留配对并让 code 自动刷新；它改善了过期问题，但保留了令人困惑的签发者模型。
- **Decided — Max**：所有设备使用同一个加入流程；Server 密码可选，由部署时决定。
- **选定机制**：`POST /api/v1/access/join` 接受 `device_name` 和可选的 `password`，以常数时间校验已配置的密码，创建一个普通的可撤销设备 token，并只返回一次。Web 设置更新 SQLite 里唯一一份全局加盐密码校验值；没有校验值就表示免密码加入。
- **Decided — Max**：密码通过已鉴权的 Web 设置配置，不通过部署环境变量。
- **Decided — Max 的可信局域网边界**：当前所有已准入的设备都可以修改这个全局设置，这与已有的「可以列出/撤销其他设备」的对等权限一致。Ferry 不防范敌意设备抢先成为局域网里的第一个客户端；加一个管理员 bootstrap 会把 Max 否决的账号/配对复杂度带回来。开启或修改密码只拦截之后的加入，不撤销当前的 token。

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

一句话说明：Ferry 不再要求用户理解配对码。部署者可以设置一个共享密码；否则设备立即加入。如果这条边界做错了，正确的密码进不去，错误的密码拿到 token，或者密码泄露。

## 冻结的交付检查清单

1. 无密码 Server：真实的新 Web 会话自动加入并进入时间线；签名的 iPhone 不用配对码加入。
2. 有密码 Server：空密码/错误密码返回稳定的拒绝且不创建设备；正确密码返回一个 token，并在 Web 和 iPhone 上进入时间线。
3. 明文密码以常数时间比较，永不返回/记录/存储；SQLite 只保存随机盐和慢速派生的校验值。请求解码拒绝未知/重复/格式错误的字段。
4. 已有的有效设备 token 继续可用且仍可撤销；被撤销的 token 必须满足当前准入配置才能重新进入。
5. 已鉴权的 Web 设置可以开启/修改/关闭密码，并且重启后仍生效；开启密码不会撤销已有 token。
6. 配对码的 UI/API/当前文档都已删除或明确标为历史；OpenAPI、Web、iOS 和 Server 共用一份准入合约；Docker 没有密码环境变量。
7. Go race/vet、签名的 iOS 测试/构建、Docker 敌意主机探针、浏览器旅程、`git diff --check`、七类攻击、作者评审和全新验证者都通过。

## 记录

- Server/Web/iOS/OpenAPI 已实现，没有密码环境变量。SQLite 保存一份单例的 16 字节盐、32 字节 PBKDF2-SHA256 校验值和迭代次数；设备和消息行不变。
- `go test -race -count=1 ./...`、`go vet ./...`、Web JavaScript 语法和 `git diff --check` 通过。
- 签名的 iPhone 17 Pro 模拟器单元测试 20/20 通过，其中证明了切换 Server 会清除它的密码，并让前一个 origin 在途的响应失效。一次沙箱里的 generic-device 构建被 CoreSimulator 资源工具拦住；随后获准的模拟器运行成功编译并执行了生产 target。
- 针对生产 handler 的真实 Chromium 旅程证明了：首次免密码自动加入、Web 设置开启、错误密码被拒、正确密码加入、旧 token 保持有效、Web 设置关闭，以及再次免密码自动加入。
- 一次当前 HEAD 的浏览器竞态探针在没有凭据的表单打开时开启了设置；加入返回 `invalid_password`，显示 `password is incorrect`，并露出密码输入框。它的 kill probe 注入网络中断，观察到 `Offline` 且密码框仍隐藏，证明 UI 能区分协议状态和传输失败。
- SQLite 关闭/重新打开的测试证明校验值在重启后保留、可以干净地关闭，且原始测试密码不出现在数据库、WAL 和共享内存文件中。
- 最终本地检查通过：Go race/vet、JavaScript 语法、shell 语法、Compose config、Docker 镜像构建（`sha256:ceb764…`）、`git diff --check`、四轮作者评审，以及全新验证者 SHIP 且 P0/P1/P2 全为零。
- 测试服务器部署在 `http://192.168.1.20:42817` 通过：镜像重建为 `sha256:773904…`；对运行中的容器断言了 health、免密码加入、`invalid_password`、正确密码加入、旧 token 保持有效、重启持久化，以及最终关闭密码。探针结束后撤销了密码路径产生的额外设备。
- 部署后的 Chromium 进入时间线显示 `Local`，打开 Devices，观察到 Access password 可见且处于关闭状态。签名的真机构建以 `com.max1874.ferry` 安装到 iPhone `9B234E1E…`；自动启动被拒绝，只是因为手机锁着。
- 源码 commit `54570ef` 已推送到 `origin/main`；部署记录在线上验证后单独提交。

## 作者对抗式评审

四轮之后的结论：**ship candidate；作者已知范围内没有未解决的 P0/P1/P2**。第 1 轮修复了一个过时的 `FERRY_UI_CODE` 测试 bundle key，以及仍然叫 pairing 的线上 Web CSS 名称。第 2 轮吸收了全新评审关于 Web 设置竞态和 iOS 跨 origin 密码残留的发现。第 3 轮发现并修复了手动表单恢复这条同类路径，以及 iOS 在途地址编辑的竞态。第 4 轮把基于展示文案的分类换成稳定的 Server 错误码，没有新发现。

1. **耦合状态** — 追踪了 `accessMu`、SQLite 设置、Web 的 `deviceToken/authGeneration/authController`、iOS 的 `generation/token/phase` 和设备撤销；`TestHTTPPasswordlessPasswordAndRevocationJourney`、`TestStaleDeviceCannotChangeAccessSettings` 和 `testChangingServerDuringJoinInvalidatesOldResponse` 覆盖了修改/撤销/请求的交错。
2. **失败路径** — 熵失败、格式错误/超大的 JSON、错误/空密码、取消/过期的身份、存储失败、浏览器存储失败和网络中断都是显式的；证据：严格的 Go 测试、20/20 iOS 测试，以及真实 Chromium 的 `invalid_password` 对比注入网络中断的探针。
3. **未改动的消费者** — 修复测试 plist 后，全仓库对线上界面的 grep 没有找到任何 `PairingManager`、配对路由、`PairingClaim`、`FERRY_UI_CODE` 或 iOS `pair()` 消费者。
4. **合约面** — OpenAPI 0.3、handler 路由、Web JSON、iOS JSON、SQLite schema、README 和历史标记对 access join/settings 一致；`rg 'access/join|settings/access|password_required'` 列出了所有 producer/consumer。
5. **原始复现** — 当前 HEAD 的真实 Chromium 不用 code 就能打开，拒绝 `wrong`，接受正确密码，并让之前的 token 保持有效；输出：`{passwordless:true, wrongRejected:"password is incorrect", rightAccepted:true, oldTokenValid:true}`。
6. **旅程重放** — 错误码分类改动之后，重新跑了最终的当前 HEAD 旅程；task space 7 返回了预期的竞态/离线正文，并成功关闭。
7. **机制区分** — 同一次运行里开启设置产生 `invalid_password`，露出隐藏的输入框并显示 `Password required`；强制让 `fetch` 失败则显示 `simulated network loss`/`Offline` 且输入框隐藏，排除了回退路径。
8. **回归扫描** — 候选回归是已有数据/会话丢失；schema 使用 `CREATE TABLE IF NOT EXISTS`，已有表不动，之前的迁移测试证明 store 重新打开后历史保留，旧 token 保持有效由 HTTP 和真实浏览器共同覆盖。
9. **规模/边界** — 空值、256±1 字节、无效 UTF-8、Unicode、首尾空格、重复/转义的 key 和 512 KiB 请求上限都由鉴权/HTTP 测试关闭；PBKDF 的计算量受 256 字节密码上限约束。
10. **七类攻击** — 双裁判（OpenAPI/严格 Go）、极值、等价拼写、默认免密码模式、旁路路由、已鉴权设置门禁和非作者的浏览器证据都逐一演练过；Docker 文件里不存在其他产品密码环境变量的 producer。
11. **断言 producer** — grep 了每一个 `password_required`、401 映射、设置写入、token 持久化写入和 phase 转换；公开的 join 401 仍是 Server 拒绝，而 Bearer 401 在 iOS/Web 上变成被撤销设备的状态。
12. **被推翻的发现** — 去掉配对后，重新追问了它的恢复、撤销、过期请求和多标签页问题：明确的可信局域网边界接受首个客户端拥有控制权，而不是发明一个所有者 bootstrap；设置写入在事务内复查请求者是否存在；token 仍可单独撤销；只有成功的加入才写共享的 Web 存储。
13. **轮次上限** — 作者评审没有自我认证：最终全新验证者返回 SHIP，P0/P1/P2 全为零；测试服务器和真机证据仍是单独的部署门槛。

第一轮全新评审返回 NO-SHIP，有两条可信局域网威胁模型上的反对意见和两个具体的 P2。后续又发现两个同类 P2，也都固化为测试：Web 只把 Server 发出的 `invalid_password` 归类为密码门槛，iOS 在加入过程中地址变化时让旧 generation 失效。两条局域网敌手的发现在用户决定的威胁模型之外，而不是被悄悄接受的实现缺陷：Ferry 目前信任已准入的局域网设备，并有意不设管理员 bootstrap 和针对敌意客户端的限流。最终全新评审返回 SHIP，范围内没有 P0/P1/P2。
