# 发布流程

> [English](release-process.md) | 简体中文

本文负责 Ferry 的发布身份、CI 候选如何成为正式版本，以及 1.0.0 的验收记录。账号密钥永远不写在这里。

## 发布身份

- **Decided（Max，2026-09-14）**：首个版本是 `1.0.0`，标签 `v1.0.0`。版本号固定、手动发布；不发布滚动的 `edge` 或 `latest` 标签。
- **Decided（Max，2026-09-14）**：一次发布包含 Server + Web 镜像和部署包，不发布原生 App；iOS 仍只走内部 TestFlight，Android 从源码构建。
- 镜像：`ghcr.io/max1874/ferry:<version>`，支持 `linux/amd64` 和 `linux/arm64`，是同一个多架构索引。镜像通过 `org.opencontainers.image.revision` 记录源码提交。
- Compose 下载包：`ferry-<version>-docker-compose.tar.gz`，是唯一上传的附件，内含 `ferry/` 目录，里面是 `compose.yaml`、`.env.example` 和 `scripts/ferry-data.sh`。它是配置文件，不是程序本身；可以运行的程序是镜像。GitHub 会显示每个附件的 SHA-256，所以不再单独提供校验值文件。
- **Decided（Max，2026-09-14）**：GitHub Release 正文使用英文：`docs/releases/<version>.md` 去掉标题和语言切换行，后面附上中文说明的链接，以及一行写有镜像摘要、提交和 CI run 的小字。说明开头先讲清楚该下载哪个文件。
- 认证：只使用仓库的 `GITHUB_TOKEN`。所有 action 都固定到提交 SHA。

## 从候选到正式发布

1. **CI 构建候选。** 每次推送到 `main` 都会运行 `image` job：一次构建两个架构，检查本地 registry 和 OCI 归档里是同一个索引摘要，通过 `compose.yaml` 分别启动两个架构，让设备、文字、文件和密码挺过一次重启，并把源码构建的部署切换到镜像而不丢数据。它上传 `ferry-candidate`：OCI 归档、部署包、`candidate.json`（提交、ref、run、摘要、平台）和 `SHA256SUMS`。
2. **版本文件已经写好版本号。** `compose.yaml` 和 `.env.example` 默认使用 `ghcr.io/max1874/ferry:<version>`，`scripts/check-repo.sh` 保证两者一致。选定候选之前，先用一个提交更新两者并加入发布说明。
3. **在下方清单里记录验收证据**，并取得 Max 的明确发布授权。
4. **运行 workflow**，传入版本号和 `main` 上成功的 CI run：

   ```bash
   gh workflow run release.yml -f version=1.0.0 -f ci_run_id=<run id>
   ```

   以下条件任一不满足，workflow 就会拒绝继续：该 run 是 `main` 上成功的 `CI` push run；它的提交仍在 `main` 上；该版本还没有 Release 或标签；产物校验值和摘要与记录一致；部署包指向所请求的镜像。它不重新构建，直接推送归档里的镜像并保留摘要，然后重新读取 registry 里的摘要。该版本已经以不同摘要存在时拒绝，不会覆盖；摘要相同则允许重跑继续。
5. **仅首次发布：把包设为公开。** GitHub 新建的容器包默认是私有的。这时 workflow 会在匿名拉取一步失败，并给出包设置页的链接。把包设为 Public 后重跑 workflow。
6. **匿名拉取。** workflow 使用空的 Docker 客户端配置，按标签拉取两个架构，确认都解析到发布的摘要，并在每个架构上跑一遍容器旅程。
7. **Release 草稿。** workflow 在候选提交上创建 `v<version>` 的 Release 草稿，附带 Compose 下载包、链接到中文说明的英文正文、镜像摘要和 CI run。发布草稿由 Max 完成，同时创建标签。
8. **切换 README 入口。** 在镜像能匿名拉取、部署包地址能访问之前，README 快速开始从源码构建。两者都确认之后，把中英文 README 切换为下载部署包、`docker compose up -d` 和只改版本号的升级方式。
9. **发布之后**，更新 `SECURITY.md`、`CONTRIBUTING.md` 及其中文版里关于受支持版本的表述。

## 1.0.0 交付清单

2026-09-14 冻结。状态取值：`done` 并附证据，`pending` 并注明负责人，或 `blocked`。

| # | 项目 | 状态 |
| --- | --- | --- |
| 1 | Web：空时间线引导和 **Connect another device**；Devices 页的连接区域，包括地址、复制、本地二维码和私有网络说明；仅本机地址显示提示而不是二维码 | 已实现；加入和传送旅程已在真实浏览器里通过（第 9 项）；第 10 项的边界情况已跳过 |
| 2 | 启动日志分别写出监听地址和浏览器访问地址 | 已实现；CI 容器旅程会断言 loopback 提示 |
| 3 | `compose.yaml` 拉取镜像；`compose.build.yaml` 构建源码；`.env.example` 保存镜像和网络配置 | 已实现 |
| 4 | CI 构建、启动并检查两个架构，针对当前源码检查源码到镜像的升级和备份恢复自检，并保留候选 | done（2026-09-14）：发布 run 34818862090 接受了 `84c355b` 上成功的 CI push run 作为候选 |
| 5 | 发布 workflow：核对 run 和产物、不重新构建、不覆盖、匿名拉取、Release 草稿 | done（2026-09-14）：发布 run 34818862090 推送了 `84c355b` 候选，并创建了 Release 草稿 |
| 6 | README 和本文提供中英文；产品核心记录首次使用旅程 | done |
| 7 | 隔离部署：按 README 从空目录安装，重启保留数据，现有源码部署切换到镜像，备份与恢复 | done（2026-09-14），有限定：在一台已装好 Docker、没有任何 Ferry 容器、数据卷或镜像的 arm64 macOS 主机上，从已发布的 Compose 下载包开始，在空目录里按 README 快速开始操作。第 1–5 步约 13 秒，拉取了已发布的 `linux/arm64` 镜像，日志给出要打开的局域网地址。另一台电脑上的浏览器直接加入，Devices 页显示了二维码；另一个隔离的浏览器会话也加入，两者之间互传了文字、一张图片和一个文件，下载的文件逐字节一致。重启之后，再执行 README 的备份和恢复命令，两个会话都保持加入，消息都在。走查的人本来就熟悉 Ferry，所以看不出新用户会卡在哪里（见「1.0.0 之后」）。未覆盖：手机（见第 9 项）、复制控件、访问密码，以及源码部署的迁移（由 CI 覆盖） |
| 8 | 取自最终候选的真实桌面和手机截图，以及约 15 秒的传送动图，单个文件小于 3 MiB | 暂缓（Max，2026-09-14）；README 保留现有截图 |
| 9 | 真实电脑和手机浏览器：扫码加入、发送并复制文字、图片预览、文件下载，覆盖无密码和有密码；逐一记录设备和浏览器 | done（Max，2026-09-14）：在 macmini 上以 host 网络部署的 `84c355b` 上，iPhone 相机扫码后打开 Safari 并加入；以 Mac 浏览器作为另一台设备，发送并复制文字、图片预览和文件下载在无密码和有密码两种状态下都通过。具体的 iPhone 型号、iOS 版本和桌面浏览器没有记录。原生 App 没有扫码入口，扫码总是通过 Safari 加入 |
| 10 | 仅本机提示、IPv4、IPv6、私有代理域名、复制失败、二维码失败、离线与重连 | 按 Max 的决定跳过（2026-09-14）；未验证 |
| 11 | 包设为 Public，匿名拉取两个架构 | done（2026-09-14）：包本来就是公开的。发布 run [34818862090](https://github.com/max1874/ferry/actions/runs/34818862090) 不带凭据拉取了两个架构，并在每个架构上跑了容器旅程；在 CI 之外匿名请求清单，也返回了 `linux/amd64` 和 `linux/arm64`，摘要为 `sha256:3eba51a1d91902543afd91c8f2c3eed85deb450d291a20d6b5599402e41a5bce`，与 CI 候选的摘要一致 |
| 12 | Max 授权并发布 Release 草稿 | done（Max，2026-09-14T11:27Z）：`v1.0.0` 指向 `84c355b`。发布之后，附件改名为 `ferry-1.0.0-docker-compose.tar.gz`，删除了校验值文件，正文改为英文；新地址匿名可访问，旧地址已不存在 |
| 13 | 镜像能匿名拉取、部署包地址能访问之后，README 快速开始从源码构建切换为部署包；在此之前 README 不得指向未发布的下载 | done（2026-09-14） |

浏览器验收不是原生 App 验收，CI 里的容器运行也不是真机证据。

- **Observed（2026-09-14）**：1.0.0 构建自 `84c355b`，早于 `main` 上后来的提交。它的 Web 应用仍是 `360c300` 重画之前的图标；它 Compose 下载包里的 `compose.yaml` 仍设置了 `TZ: Asia/Shanghai`——这是维护者本地的默认值，`main` 上已经删除，而且 Alpine 镜像里没有时区数据，这项设置本来就没有可靠效果。这两处改动要到下一个版本才会到达用户。

## 1.0.0 之后

三位新用户会各自按 README 独立部署 Ferry。记录每次部署的耗时和每个人卡住的具体位置。Ferry 不为此收集任何遥测。
