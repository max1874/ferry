# 发布准备

> [English](release-readiness.md) | 简体中文

这是仓库准备里程碑的权威记录。它不证明已推迟的真机或家庭局域网旅程。

## S0 — 确认的范围

- **REQUESTED（Max，2026-08-31）**：完成现有 TODO 中除验证外的工作；License 由 Codex 选择最合适的类型。
- **完成标准**：仓库有准确的对外文档、一个覆盖 Go/Web/iOS/Android 的 CI 门禁、可恢复的 Docker 数据流程、release 签名模板，以及开源的贡献/安全文件。
- **不做**：真机互传、Mac mini 恢复演练、商店提审、公网部署、TLS、API/schema/数据库/产品行为改动，以及提交任何秘密。
- **在用户授权下推荐并选定**：Apache License 2.0，因为它的宽松条款包含明确的专利授权和专利终止保护。
- **深度**：full；涉及四个交付面，但不改变对外的产品合约。
- **预算**：最多新增/修改 14 个文件、约 900 行编写内容；不加运行时依赖或持久化机制。
- **边界计划**：本地构建/测试和一个隔离的临时 Compose 数据卷可以关闭仓库工具相关的结论；真机/局域网验收仍是 `BLOCKED (external)`，不能被这些检查替代。
- **扩张触发**：运行时/API/schema 改动、新依赖、秘密、发布、部署，或超过 14 个文件，都会停止追加工作。

状态：`shipped`；对象：`060bd2f`；远端门禁：GitHub Actions run `33354550978`；评审轮次：1 次作者全新一轮；失效：0。

## S1 — 设计决策

### 证据

- **Observed**：Go 内嵌 Web UI；因此 `go test ./...` 覆盖 Server/Web 包的边界，而 Web 资源测试覆盖内嵌资源。
- **Observed**：Android 已经有官方 Gradle wrapper，以及一条能通过的离线 test/lint/assemble 命令。
- **Observed**：iOS 工程有共享 target，但没有提交 scheme 文件；`xcodebuild -scheme Ferry` 目前解析的是自动生成的 scheme。
- **Observed**：Compose 把完整的持久状态存在一个命名的 `/data` 数据卷里；SQLite 使用 WAL，所以停掉容器再归档是保证一致性备份的最窄机制。
- **Observed**：GitHub 的 `macos-26` 镜像目前提供 Xcode 26.6、iOS 26 模拟器和 iPhone 17 Pro 设备。

### 候选方案

1. **Selected — 一个 workflow 加仓库脚本**：CI 调用维护者本地也能运行的同一套 Gradle、Go、Xcode 和数据管理入口。这样策略可执行，又不引入任务运行器依赖。
2. **Rejected — 只写文档**：更小，但备份、秘密卫生和跨平台构建的承诺都没有门禁。
3. **Rejected — 发布框架/任务运行器**：Fastlane/Gradle 发布插件或一层 Makefile，会在商店发布还不存在时就加上依赖和另一套命令词汇。

一句话说明：这项工作让 Ferry 仓库为协作者做好准备，而不改变产品做什么。它让构建检查和 Docker 数据恢复可重复，同时把签名秘密和发布留在维护者手里。如果做错了，贡献者会拿到误导性的设置说明，或者太晚才发现备份恢复不了。

## S2 — 冻结的交付检查清单

1. **REQUESTED** — Apache-2.0 许可证、贡献指南、安全策略和 README 描述当前的四组件产品及其未发布状态；仓库策略扫描。
2. **REQUESTED** — 一个 GitHub Actions workflow 在匹配的工具链上执行 Go test/vet、Docker build/config、Android unit/lint/assemble 和 iOS 单元测试；YAML 检查加本地等价命令。
3. **REQUESTED** — Android release 构建只接受一个未跟踪的签名配置文件，其中声明的文件/值缺失时明确失败；Gradle 配置探针和被忽略秘密的扫描。
4. **REQUESTED** — iOS 导出配置是一个不含秘密的模板，发布说明要求在发布时明确提供 Team ID/签名身份；plist 校验和被忽略秘密的扫描。
5. **REQUESTED** — Docker 备份先停止写入、归档命名数据卷，只恢复之前在运行的服务；恢复先创建安全备份，再从白名单归档替换数据；隔离数据卷的往返测试和敌意归档拒绝。
6. **REPO_REQUIRED** — `CLAUDE.md` 仍是 `AGENTS.md` 的软链接；过时的产品/配对说法被删除或标为历史；仓库检查和 `git diff --check`。
7. **REPO_REQUIRED** — 记录作者攻击记录、有序的自审和完整文件评审；push 前修复每个已确认的缺陷。

冻结的工具旅程：创建一个隔离的 Compose 项目，写入 `/data/ferry.db` 和一个服务端形状的 blob → 备份 → 替换数据卷内容 → 恢复 → 观察数据库/blob 逐字节一致 → 给恢复一个包含 Ferry 数据白名单之外路径的归档 → 观察到在改动数据卷之前就被拒绝。真机 App 旅程明确不在本里程碑之内。

## S3 — 构建者攻击记录

- **双裁判**：对比了 README 命令和 CI 命令在 Go、Web、Android 和 iOS 上的一致性；仓库检查器防止风险最高的文档标记悄悄漂移。
- **极值**：备份/恢复演练了空服务状态、精确的 Ferry 数据库/blob 名称、被改动的数据卷和意外的归档成员。已有归档和安全备份路径会失败，而不是被覆盖。
- **等价拼写**：归档条目只在去掉恰好一个前导 `./` 之后才接受备份工具的 `./name` 形式；绝对路径、嵌套穿越和任意名称仍在白名单之外。
- **默认值**：Compose 保持 loopback/高位端口的默认值；密码仍是 Web 设置；缺少 release 签名会让任何打包 release 的任务图失败，但不会破坏 debug CI。
- **旁路**：恢复先把源文件复制成私有快照，然后校验并解压这同一份字节，之后才改动服务数据卷；符号链接条目和非 Ferry 文件被拒绝。
- **失败边界**：安全归档落盘之后，恢复在替换数据之前清掉自动恢复运行的标记；解压失败会让 Ferry 保持停止，而不是对外提供不完整的数据卷。
- **信号边界**：HUP/INT/TERM 在通用的 EXIT 清理运行之前映射到固定的非零退出码，所以被中断的操作不能在清理后报告成功。
- **策略门禁**：`scripts/check-repo.sh` 强制检查说明文件软链接、被忽略的秘密/数据类别、历史配对标记、shell 语法和许可证声明。
- **供应链旁路**：所有第三方/官方 workflow action 都固定到解析出的 commit SHA；版本注释保留更新上下文，执行时不信任可变的主版本 tag。
- **不自我认证**：Docker 自测使用了一个隔离的真实 Compose 数据卷，并重新读取了恢复出来的字节。真机/局域网结论仍被排除并标为阻塞。

已确认的反例：第一版 iOS CI 命令跑了整个共享 scheme，所以即使 20 个单元测试全部通过，两个依赖环境的 UI 旅程也在干净机器上失败。CI 和 README 现在明确选择 `FerryTests`；UI 旅程保留它们单独的真实 Server 门禁。

## S4 — 作者对抗式自审

作者预筛结论：`ship candidate`；这不是独立评审。

1. **耦合状态** — `rg WAS_RUNNING|PARTIAL_ARCHIVE|RESTORE_COPY|TEMP_*|SELF_TEST_* scripts/ferry-data.sh` 列出了所有读写方；由一个进程拥有它们，清理只清除确切的路径，恢复在破坏性解压之前清掉 `WAS_RUNNING`，所以失败时不会自动启动不完整的数据。
2. **失败路径** — 在探测了不支持的数据卷数据、已存在的目标、被改动的数据卷、意外成员、允许名称的符号链接和损坏的 tar 之后，最终的 `scripts/ferry-data.sh self-test` 通过；被拒绝的路径不会留下发布出去的备份，恢复出的数据库仍是 `original-db`。
3. **未改动的代码** — `rg ferry-data.sh|check-repo.sh|signing.properties|only-testing:FerryTests` 只在 README/CI/新脚本/Android 构建配置中找到；Server/Web/iOS 生产调用方和 API 代码都没动。
4. **合约面** — `git diff -- api/openapi.yaml cmd internal ios/Ferry/Ferry android/app/src` 为空；没有 HTTP、SQLite、UI 或运行时环境合约的改动。
5. **原始复现** — 宽泛的 `xcodebuild test -scheme Ferry` 复现了两个干净机器上的 UI 测试失败，而 20 个单元测试通过；同一命令加上 `-only-testing:FerryTests` 以 `TEST SUCCEEDED` 结束，20/20。
6. **当前旅程** — 最终工作树的 Docker 自测在恢复后重新读取了精确的数据库/blob 字节；之后的改动只涉及 CI/文档/检查策略，所以这份记录在运行时层面仍然有效。真机 Web/iOS/Android 局域网旅程仍是 `BLOCKED (external)`。
7. **机制区分** — Mac mini 在没有签名时运行 `./gradlew build` 给出了精确的配置拒绝；一个一天有效的测试 keystore 随后产出了 `bundleRelease`，`jarsigner -verify` 成功。Docker 恢复成功和三类无效归档都在同一个隔离项目中观察到。
8. **回归扫描** — 候选风险是悄悄丢掉 UI 覆盖；完整文件阅读确认两个 UI 旅程仍在 `FerryUITests.swift` 里，CI 只明确列出单元测试，因为那些旅程需要真实的外部 Server，仍单独设门禁。
9. **规模/边界** — 签名为零/缺失、签名完整、归档路径已占用、不支持的源数据卷、损坏的归档和链接条目都有覆盖；备份通过 tar 流式处理，不会在 shell 内存里缓冲 64 MiB 的产品文件上限。
10. **合约攻击** — S3 记录了双裁判、归档拼写/默认值/旁路、策略执行和不自我认证；最终探针还把 shell 里的 tar 失败改成显式处理，而不是依赖对上下文敏感的 `set -e`。
11. **断言 producer** — `WAS_RUNNING` 和清理路径的所有 producer 都由第 1 项的 grep 列出；Gradle 的 release 断言读取的是展开后的任务图，并由聚合的 `build` 证明，而不只是直接指定的 release 任务。
12. **被推翻的发现** — 两次 Android 失败都是环境问题（先是 JDK 形态不对，再是缓存不对）；验证过的普通 JDK 路径加上已有的 `.gradle-home` 离线完成了全部 51 个任务，没有加入任何下载/安装或产品层面的绕过。
13. **轮次上限** — 这一轮有序评审只关闭作者已知的风险；S5 必须重读完整文件，并明确标为作者全新一轮的降级方案，因为本任务没有授权委派。

## S5 — 完整文件评审

评审者来源：**作者全新一轮**，不是独立上下文；本任务没有授权委派。

修复后的结论：P0 0、P1 0、P2 0。评审者完整阅读了每个改动文件，把 README 和 Gradle 与它们在 `b50c406` 的版本对比，追踪了每一个新脚本/CI/签名引用，重读了两个外部 iOS UI 旅程，并把 S2 的七项全部与实现对照。

- **P1 设计偏差，已修复**：S2 要求一个不含秘密的 iOS 导出模板，但第一版候选只有文字说明。为了保持在 14 个文件内，删掉了一处对已被取代的历史文档的不必要编辑；`ios/ExportOptions.plist.example`、它的忽略规则、plist 门禁和 README 流程现在关闭了原始条目。
- **P2 中断结果，已修复**：把同一个函数直接用于 EXIT 和信号 trap，可以正确清理，但在某些 shell 上会让信号返回成功。信号现在映射到 129/130/143，然后进入通用的 EXIT 清理。
- **P1 Linux 所有权边界，第一次远端运行后修复**：容器 root 创建的 `0600` 备份在 Docker Desktop 上可读，但 Linux runner 的宿主用户读不到，所以恢复无法对它做快照。备份和安全 tar 流现在通过 stdout 输出到宿主创建的 `0600` 文件，完全避开 bind mount 的所有权和 rootless/user-namespace 映射问题。
- **检查了被替换的语义**：旧 README 缺少 Android/恢复/发布路径，还说没有选定许可证；旧 Android 构建没有 release 签名配置。产品/API/SQLite 语义没有被替换。
- **修复后的机器证据**：最终 Docker 自测通过；仓库/shell/YAML/plist/规范许可证/diff 检查通过。之前运行时等价的证据对 Go race/vet/JS、Android debug + 签名/未签名 release 门禁，以及 iOS 20/20 单元测试仍然有效，因为后续改动不涉及这些调用路径。
- **剩余设计偏差**：无。推迟的真机/家庭局域网旅程是本里程碑明确的非目标，不是被替代的通过项。

## S7 — 收口

1. 许可证/文档 — 本地策略通过；`LICENSE` 与 apache.org 的规范 Apache-2.0 文本逐字节一致。
2. CI — YAML 解析通过；每个 action 都固定了 SHA；本地等价的 Go/Web/Docker/Android/iOS 命令通过。第一次远端运行出现了 checkout-v4 的 Node 20 弃用警告，所以候选版本现在固定到 checkout v5。[Run 33354550978](https://github.com/max1874/ferry/actions/runs/33354550978) 针对 `060bd2f` 通过了 repository、Server/Web/Docker、Android 和 iOS 四个 job。
3. Android release — 聚合的 `build` 拒绝了缺失的签名；一份完整的临时配置产出了验证过签名的 AAB；测试文件已删除。
4. iOS release — 三个 plist 都通过校验；模板只包含策略和 Team 占位符，真实的导出文件被忽略。
5. Docker 数据 — 最终的隔离 Compose 旅程恢复出精确的数据库/blob 字节，并拒绝了不支持的数据卷数据、覆盖、意外成员、符号链接和损坏的归档。
6. 仓库策略 — 说明文件软链接、过时的线上产品文字、被忽略的本地数据/签名材料、action 固定、shell 语法和 `git diff --check` 通过。
7. 评审 — 七类攻击、有序的 13 项作者自审和作者全新完整文件评审都已记录；所有仓库本地门禁都已关闭。只剩明确推迟的真机/家庭局域网旅程在外部。

旅程脚本：已提交为 `scripts/ferry-data.sh self-test` — 它长期区分可恢复的数据库/blob 恢复，与覆盖、不支持的数据卷、损坏归档和链接条目这几类失败。
