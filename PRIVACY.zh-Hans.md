# 隐私政策

> [English](PRIVACY.md) | 简体中文

生效日期：2026-09-27

本政策适用于 Ferry Server、它提供的 Web 应用、Ferry iOS App（上架名 FerryDrop）和 Ferry Android App。

## 简要说明

Ferry 项目不运营服务器，没有账号系统，也不收集任何数据。你通过 Ferry 发送的所有内容，只会发到你自己（或你信任的人）在自己网络里运行的 Ferry Server。

## App 在你的设备上保存什么

- 你填写的**服务器地址**，用于重新连接。Android 还会保存你填写的设备名。iOS 不保存设备名：它用你在 iOS 设置里给这台 iPhone 起的名字，在设备加入时发给你的 Server。
- 设备加入时由你的 Ferry Server 签发的**设备 token**。iOS 把它存在钥匙串里；Android 用 Android Keystore 中的密钥加密保存。Android 的应用数据不参与系统备份。
- 你主动保存的文件会离开 Ferry，存到你选择的位置：iOS 上是你在「文件」里选的位置，Android 上是下载目录等位置。

App 只连接你填写的服务器地址。它们不在后台读取剪贴板，不包含统计、广告或崩溃上报 SDK，也不连接任何第三方服务。

## Ferry Server 保存什么

Server 只保存显示共享时间线所需的数据：

- 按发送原样保存的消息和上传的文件，包括每个文件的文件名、类型和大小，以及每条消息的发送时间和发送设备的名称、类型（如 iPhone、Android）和 ID；
- 每台已加入设备的名称、类型、加入时间，以及它的 token 的 hash；
- 可选共享访问密码的校验值。

这些数据存放在运行 Server 的机器上的数据目录里。Server 日志只记录启动配置和错误，不记录消息内容。数据由运行 Server 的人掌控，包括备份和删除。在 Web 应用里撤销一台设备，它的 token 就会失效。

## 网络安全

Ferry 为可信的私有网络设计，除非部署者自己在前面加一层 TLS 反向代理，否则使用未加密的 HTTP。同一网络里的其他人可能观察到流量。详见 [SECURITY.zh-Hans.md](SECURITY.zh-Hans.md)。

## 儿童

Ferry 是通用工具，不面向儿童。

## 变更与联系

本政策的变更会发布在 Ferry 仓库的这个文件里，并更新生效日期。有问题可以在 [GitHub issues](https://github.com/max1874/ferry/issues) 里提出。
