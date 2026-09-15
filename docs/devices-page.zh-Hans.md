# Devices 主页面导航

> [English](devices-page.md) | 简体中文

## 范围与决策

- 需求（Max，2026-09-01）：点击左侧的 Devices 不能在最右边打开一个脱节的面板；视觉交互必须在交付前实际走过。
- 完成标准：Timeline 和 Devices 是由侧边栏控制的两个平级主视图。Devices 替换时间线内容，没有模态框、遮罩或关闭按钮；点回 Timeline 时 composer 保持原样。
- 不做：设备/密码 API 行为、撤销确认、URL 路由、原生客户端，以及更大范围的侧边栏重设计。
- 根因：`Devices` 的样式是导航，但调用的是 `dialog.showModal()`，打开一个靠右的 430 px 对话框。触发器在空间和语义上的承诺，与结果相互矛盾。
- 设计：两个侧边栏按钮共同拥有一个 `activeView`；`selectView` 由它派生主视图可见性和 `aria-current`。Devices 页面在桌面端使用同一个内容列，在手机端使用同一个 56 px 窄栏。

## 冻结检查清单与本地旅程

1. 登录后的初始视图是 Timeline，composer 可见，`aria-current=page`。
2. 点击 Devices 会隐藏 Timeline/composer，在侧边栏旁边立即显示 Devices 主页面，并移动激活样式和 ARIA，不出现对话框。
3. 设备列表和密码状态加载出来；先开启再关闭一个本地测试密码，都给出真实的终态提示。
4. 点击 Timeline 恢复内容和 composer，并让键盘焦点留在选中的导航项上。
5. 桌面 1383×997 和手机 390×844 都没有横向溢出；各行、撤销控件和密码卡片都在内容边界内。
6. 完整 Go tests/vet、JS 语法解析、仓库策略、diff check、push、保留数据卷的测试服务器构建，以及部署后的点击走查都通过。

本地最终工作树证据，基线 `e13b71a`：桌面端侧边栏结束于 x=257，Devices 页面从 x=257 开始；内容宽 760 px，居中于 x=440。点击 Devices 得到 `{timelineHidden:true,devicesHidden:false,composerHidden:true,devicesActive:page}`，有两行设备和 `No password is required.`。开启和关闭密码都得到真实的成功文案。点击 Timeline 让每个视图断言都反转回来。在 390 px 下，页面 x=56…390，内容/卡片/行 x=72…374，溢出 0。录制：`ferry-devices-page-local`（18 帧）。

## 作者对抗式评审

1. 耦合状态 — `activeView` 只有一个所有者，`selectView` 派生可见性/激活态/ARIA；所有 producer 都 grep 过。
2. 失败路径 — 设备/设置失败在 Devices 状态里保持可见；401 仍通过既有路径回到接入页。
3. 未改动的消费者 — 重读了完整 HTML、相关 CSS/JS 和 Web 测试；时间线消息、轮询、上传和设置 API 都没变。
4. 合约面 — 网络/schema/存储合约都没变；DOM 有意去掉了对话框语义，加上了主视图导航语义。
5. 原始复现 — 部署版的对话框位于 x=935，而触发器在侧边栏 x=0…257；替换后的页面从侧边栏边缘开始。
6. 当前 HEAD 旅程 — 桌面/手机截图和点击都在最后一次桌面按钮紧凑化修复之后进行。
7. 机制区分 — 只改变点击的导航按钮时，内容可见性和 `aria-current` 都会反转。
8. 回归扫描 — 主要候选风险是 composer 意外保持可见；它在 Devices 上被隐藏，在 Timeline 上恢复。
9. 规模/边界 — 部署环境里的八台设备列表、手机端各行、撤销控件和密码卡片都放得下，没有横向溢出。
10. 合约攻击 — 不适用；没有改动外部协议、鉴权策略、持久化或普遍性边界。
11. 断言 producer — 列出了所有 `conversationElement.hidden`、`devicesPage.hidden`、`composerShell.hidden`、激活 class 和 `aria-current` 的写入方。
12. 被推翻的发现 — 否定了原对话框的位置；在主页面上重新检查了设备加载、错误和密码问题。
13. 轮次上限 — 只有作者完整一轮；按 Max 的长期决定不用 subagent，以真实浏览器点击走查和机器检查作为收口证据。

## 发布证据

- 源码 commit：`6dd8416`（`fix: make Devices a main navigation view`）。
- 测试服务器镜像：`sha256:fa26f3aa4cf616adcdbd4abe1fe3f5d869f282c1552c703ae2d361d11a74b186`；容器 `ferry` 在运行，`/healthz` 返回 `{"status":"ok"}`。
- 桌面 1383×997：Devices 页面紧接侧边栏从 x=257 开始，760 px 内容列居中，保留下来的八台设备全部显示，只有一个 `This device`，不存在对话框，横向溢出为 0。
- 手机 390×844：页面 x=56…390，内容和每一行都在 x=72…374，密码卡片跟在列表后面，横向溢出为 0。回到 Timeline 会恢复消息列表和 composer，焦点落在 Timeline 上，滚动位置回到顶部。
- 部署后交互录制：`ferry-devices-page-test-server`（10 帧）。最终截图：`/private/tmp/ferry-devices-page-test-server-desktop.png`、`/private/tmp/ferry-devices-page-test-server-mobile.png` 和 `/private/tmp/ferry-devices-page-test-server-mobile-return-timeline.png`。

状态：已发布，并在部署的测试服务器实例上做过视觉走查。
