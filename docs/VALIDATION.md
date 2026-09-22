# 验收记录（0.1.2）

代码和发布链路已经实现；**真实麦克风/扬声器双端验收尚未完成**，不能将下面的
模拟音频结果解释为已验证真实设备的声音质量、系统麦克风权限或声学回声表现。

## 原生构建、协议与服务

以下 GitHub Actions job 均完成竞态测试、静态检查、完整进程生命周期、原生用户
服务注册/启动/停止，以及发行压缩包安装和离线自更新测试。音频设备部分使用
只在 `talk_test_audio` 构建标签下启用的 miniaudio Null 驱动；发行包不启用该标签，
也禁止在缺少真实音频设备时自动回退到 Null 驱动。

| 平台 | 架构 | 证据 |
|---|---|---|
| macOS | arm64 | [通过](https://github.com/k0ngk0ng/wire-talk/actions/runs/35682072296/job/106600948672) |
| macOS | amd64 | [通过](https://github.com/k0ngk0ng/wire-talk/actions/runs/35682072296/job/106600948641) |
| Linux | amd64 | [通过](https://github.com/k0ngk0ng/wire-talk/actions/runs/35682072296/job/106600948691) |
| Linux | arm64 | [通过](https://github.com/k0ngk0ng/wire-talk/actions/runs/35682072296/job/106600948772) |
| Windows | amd64 | [通过](https://github.com/k0ngk0ng/wire-talk/actions/runs/35682072296/job/106600948626) |

测试覆盖：

- 三个真实 UDP socket 的成员发现和直接音频帧传输；入口节点关闭后其余节点继续互通。
- 篡改、错误密钥、重放、过期/未来时间戳和短包拒绝。
- 多路混音限幅、乱序旧帧丢弃、队列上限和欠载静音。
- Native C/Go 双工回调；关闭设备后不再采集。
- 前台 SIGINT（Unix）、控制停止（所有平台）、后台启动/停止/重启、静音、退出 watch 不影响会话。
- 暂时占用 UDP 端口后释放，持久服务自动恢复在线。
- systemd user、LaunchAgent 和 Windows 计划任务的真实注册、启动、状态查询、停止及清理。
- Windows 私有配置 ACL、Scoop junction 路径、校验失败不改写程序、包管理器安装保护。

## 安装与更新

[0.1.2 发行包](https://github.com/k0ngk0ng/wire-talk/releases/tag/v0.1.2) 已生成五个平台
的压缩包、SHA256SUMS、构建来源证明、Homebrew Formula 和 Scoop Manifest。

[发行包安装复验](https://github.com/k0ngk0ng/wire-talk/actions/runs/35682672389) 已在
四个 Homebrew 平台及 Windows Scoop 全部通过：实际下载、安装、wirectl 插件分发、
初始化配置，以及确认包管理器安装拒绝内置自更新。最初发布任务中的 Scoop 检查曾
把预期的非零退出码误传为任务失败；复验已修正脚本，测试的是同一份 0.1.2 发行包。

本机另外实测手动安装的 `wirectl-talk update` 从 0.1.0 在线升级至 0.1.2，随后执行
`version` 返回 0.1.2。所有下载/安装测试文件都放在工作目录 `.cache` 内。

- Homebrew 自动同步与发布：[通过](https://github.com/k0ngk0ng/homebrew-tap/actions/runs/35682519922)。
- Scoop 自动同步与发布：[通过](https://github.com/k0ngk0ng/scoop-bucket/actions/runs/35682800814)。

## 实机验收仍需完成

当前开发机只枚举到 DELL U2717D 和 Mac mini Speakers 两个输出，没有音频输入设备。
需要提供两台有麦克风和输出设备的终端，以完成：

1. 各自选择指定输入/输出，前台互相说话并确认双方实际听到声音。
2. 前台 Ctrl+C 后确认麦克风、扬声器和网络活动停止。
3. 启动原生后台服务，关闭终端后持续对话；另开 watch 观察，再退出 watch 确认语音继续。
4. 分别验证耳机全双工与扬声器半双工保护的延迟、回声、噪声和可理解性。
5. 验证真实音频设备拔插、系统权限、登录恢复，以及第三台终端加入群聊。

还未做真实跨机器 IPv6、32 成员负载及长期稳定性实测。当前只承诺需要可达 UDP
地址的直连方式，不提供 NAT 自动打洞或语音中继；跨 NAT 可先使用 wire-connect。
