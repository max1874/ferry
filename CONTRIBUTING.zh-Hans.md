# 为 Ferry 做贡献

> [English](CONTRIBUTING.md) | 简体中文

Ferry 是一个公开仓库；它的 Server 和 Web 应用以带版本号的 [Release](https://github.com/max1874/ferry/releases) 发布。请把每一处改动都当作公开可见，还没验收的部分见 `README.md`。

## 改代码之前

1. 先读 `AGENTS.md` 和 `docs/product-core.md`。
2. 让改动紧扣 Ferry 的核心：局域网剪贴板与文件旅程。
3. 没有明确的产品决定，不要加入 TLS、互联网暴露、自动剪贴板捕获、后台传输、账号或商店发布行为。
4. 永远不要提交设备 token、密码、签名文件、本地 Server 数据或构建产物。

## 开发要求

- 优先使用平台原生代码和仓库里已有的机制。
- 线上行为改变时，保持 Server/OpenAPI/Web/iOS/Android 各处合约一致。
- 每个确认的缺陷都要加回归测试。
- 失败要可见地报告出来；不要藏在重试或加载状态后面。
- 保持 `CLAUDE.md` 是指向 `AGENTS.md` 的软链接。
- 文档同时提供英文和简体中文：英文在原路径，中文在同名的 `*.zh-Hans.md`，改一种语言时在同一个 commit 里同步另一种。

运行与改动相关的检查；完整命令列表在 `README.md`。至少要运行 `scripts/check-repo.sh`、`git diff --check`，以及每个被改动组件的测试和构建。关于真机的结论必须有真机证据，拿不到证据时必须明确标注为未验证。

## 改动与评审

commit message 和 pull request 用英文写。commit message 要聚焦，说清可观察到的变化。pull request 里要写明：

- 解决的是哪个用户问题；
- 改动了哪些组件和合约；
- 确切的验证命令和结果；
- 只有在视觉行为发生实质变化时才附截图；
- 还剩哪些外部或只能在设备上完成的验证。

提交贡献即表示你同意你的贡献以 Apache License 2.0 授权。
