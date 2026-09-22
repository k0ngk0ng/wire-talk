# wire-talk

通过本地麦克风和扬声器进行端到端加密的实时对话。终端命令为
`wirectl talk`，也可以直接运行 `wirectl-talk`。不转录、不保存音频，
不使用云端语音服务。

## 安装

数字配对码从 0.1.3 起提供：

```sh
# macOS / Linux，Apple Silicon / Intel，amd64 / arm64
brew install k0ngk0ng/tap/wire-talk
```

新版 Homebrew 若提示尚未信任 `wirectl` 依赖，先执行下面的信任命令，再重试安装：

```sh
brew trust --formula k0ngk0ng/tap/wirectl
brew install k0ngk0ng/tap/wire-talk
```

```powershell
# Windows amd64
scoop bucket add k0ngk0ng https://github.com/k0ngk0ng/scoop-bucket
scoop install k0ngk0ng/wire-talk
```

包名为 `wire-talk`，运行命令仍为 `wirectl talk`。旧的 Homebrew 名称 `talk` 保留为
别名；已安装旧包时，停止语音后运行 `brew update`、`brew migrate talk`，再使用
`brew upgrade k0ngk0ng/tap/wire-talk`。Scoop 的旧安装先 `scoop uninstall talk`，再
`scoop install k0ngk0ng/wire-talk`；房间配置保存在独立的用户配置目录，不随卸载删除。
迁移完成后重新启动；使用原生自启服务的用户重新执行 `daemon install`。

手动安装：下载 [Releases](https://github.com/k0ngk0ng/wire-talk/releases)
中对应平台的包，核对 `SHA256SUMS`，将 `bin` 中两个可执行文件放入同一 PATH
目录。已安装 wirectl 时可只添加 wirectl-talk 插件。Windows 使用 `.exe`。
无需另装 ffmpeg、Python 或音频编解码器；Linux 需要可用的 ALSA/PulseAudio
音频环境，macOS/Windows 需要允许麦克风权限。

## 用临时数字码加入房间

房主在自己电脑上生成配对码，对方通过 **IP:端口 + 6 位数字码** 加入。不需要部署
独立配对服务，也不用复制密钥文件。房间仍使用随机 256 位密钥，由程序在配对时
自动交换并保存；数字码不是语音加密密钥。

机器 A（房主）：

```sh
wirectl talk invite
# Pairing code: 482731（示例，以实际输出为准）
```

首次 `invite` 会自动创建默认房间，使用默认音频设备和端口 `51830`。已有配置时
沿用现有房间。保持该命令运行，将自己的可达 IP、端口和临时码告诉对方。
如果要自选设备，先执行 `devices`，再用 `init --input ID --output ID` 创建房间；
戴耳机时可加 `--headphones`，自选端口可加 `--listen 0.0.0.0:端口`。

机器 B（假设 A 的可达 IP 为 `192.168.1.10`，B 尚未初始化房间）：

```sh
wirectl talk join 192.168.1.10:51830 --code 482731
```

配对后 B 自动保存房间和房主地址，并启动前台音频。A 的 `invite` 随后退出，A 执行：

```sh
wirectl talk join
```

现在双方直接说话即可。首次启动需允许麦克风权限。B 的首次 `join` 也可以添加
`--input ID --output ID --headphones` 等选项。若只想完成配对、稍后再启动音频：

```sh
wirectl talk pair 192.168.1.10:51830 --code 482731
wirectl talk daemon start
```

以后两端直接 `join` 或 `daemon start`，不需要再次输入地址或数字码。
已有房间配置不会被配对覆盖；加入另一个房间时用 `--state-dir DIR` 选择新配置目录。
配对后若音频启动失败，修复设备/权限后直接运行 `join`，无需重新配对。

数字码随机生成，**5 分钟有效、一次使用**；5 次失败连接后关闭邀请，需重新运行
`invite`。Ctrl+C 立即取消邀请，不影响已运行的语音会话。认证成功即消耗邀请，
即使随后网络中断或保存失败，也需要生成新码。码到期不会踢出已经配对的成员。

群聊时，任一已有成员运行 `invite` 邀请下一个成员；后台对讲可以继续运行。
其他成员使用该邀请者的地址和新码加入。发现成员后音频直接在各参与者之间传送，
邀请者不负责转发语音。前台 `join` 按 Ctrl+C 会关闭本次麦克风、扬声器、网络和
状态接口；远端成员不受影响。

配对使用房主监听地址上的 **TCP**，语音使用同一端口号的 **UDP**（默认均为
`51830`）。TCP 只在 `invite` 等待期间监听；防火墙/路由器端口映射需放行对应协议。
需要所有成员之间 UDP 地址可达；支持局域网、公网地址、VPN，或 `wirectl connect`
建立的虚拟 IP。数字码不会解决 NAT 连通问题，当前不提供自动打洞/中继。
`0.0.0.0:51830` 默认接收 IPv4；IPv6 可使用 `[::]:51830`，对端地址写为 `[IPv6]:51830`。

旧的 `init --key-file ./room.key --peers IP:PORT` 方式仍可使用，与数字配对加入的成员
使用同一种语音协议；日常使用优先选择临时数字码。

## 音频设备和扬声器防啸叫

`devices` 输出设备名、稳定 ID、方向和是否默认。初始化时绑定一套设备：

```sh
wirectl talk init --input INPUT_ID --output OUTPUT_ID
```

已有配置可在停止服务后编辑 `config.json` 中的 `input`、`output`，空字符串表示
启动时选择默认设备。明确指定的设备不存在时会报错，不会悄悄切换到其他设备。
每个运行实例只打开一套输入和输出；同一配置不能同时启动前台和后台实例。

默认开启**半双工扬声器保护**：播放远端明显可闻的音频期间及结束后约 200ms 暂停
上传本地麦克风，避免喇叭回声不断传回对端。这不是声学回声消除。佩戴耳机时可在
初始化加 `--headphones`，或将配置 `headphones` 改为 `true`，实现双方同时说话的
全双工模式。直接用外放全双工可能产生回声/啸叫。

格式为 16kHz 单声道 16-bit PCM，每帧 20ms；单个远端约 282kbit/s（另加 IP/UDP
开销），群聊发送带宽随成员数线性增长，最多 32 个远端。不做语音转录和有损压缩。
接收端保留有界短队列，多人同时说话时混音并限幅，丢失的帧用静音填充。

## 后台在线与状态

```sh
wirectl talk daemon start
wirectl talk watch
wirectl talk mute
wirectl talk unmute
wirectl talk daemon status
wirectl talk daemon stop
```

`daemon start` 分离为后台进程，确认音频设备成功启动后才报告在线。可关闭终端；
`watch` 每秒打印 JSON，Ctrl+C 只退出观察，不停止后台音频。状态包含输入/输出
设备、成员、最近消息时间、发送/接收帧、丢帧、拒绝报文计数和静音状态。
无音频设备、权限拒绝或端口冲突时不会假报启动成功，详情在状态目录 `daemon.log`。

需要登录后自动启动及异常恢复时，先停止手动会话，再注册原生用户服务：

```sh
wirectl talk daemon stop
wirectl talk daemon install
wirectl talk watch
```

macOS 使用 LaunchAgent，Linux 使用 systemd user service，Windows 使用登录计划
任务（交互用户身份，避免 session 0 无法访问音频）。必须先在前台授予麦克风权限。
注册成功表示系统服务已安装，实际设备在线状态用 `watch` 确认。设备不可用或端口
暂时占用时，服务每 3 秒重试；整个进程异常退出时由系统服务管理器重启。退出登录期间不承诺
音频可用；重新登录后自动启动。`daemon stop` 同时移除当前配置的服务注册，保留
房间配置。再次 `daemon install` 恢复自启。系统服务实测情况见 [验收记录](docs/VALIDATION.md)。

默认状态目录：macOS 为 `~/Library/Application Support/wirectl/talk`，Linux 为
`$XDG_CONFIG_HOME/wirectl/talk` 或 `~/.config/wirectl/talk`，Windows 为
`%AppData%\wirectl\talk`。可用 `--state-dir DIR` 或 `WIRE_TALK_HOME` 指定。
配置目录限制为当前用户：Unix 使用 0700，Windows 使用受保护的用户/SYSTEM ACL。

## 更新

停止当前实例后，手动安装可自更新：

```sh
wirectl talk daemon stop
wirectl talk update
wirectl talk daemon start
```

`update --version 0.1.0` 可选具体版本；在线下载只接受官方 GitHub Release 及可信
下载域名，同时核对 GitHub asset SHA-256 和独立 `SHA256SUMS` 后替换插件。
包管理器安装使用 `brew upgrade k0ngk0ng/tap/wire-talk` / `scoop update wire-talk`。
内置更新不会改写带包管理标记的安装。wirectl 主程序单独由其包管理器更新。
原生服务绑定启动时的程序路径；包管理器升级前先 `daemon stop`，升级后重新
`daemon install`，以免旧安装目录被清理后服务仍引用旧版本。

离线更新：

```sh
wirectl talk update --archive ./wire-talk-0.1.0-linux-amd64.tar.gz --checksums ./SHA256SUMS
```

离线校验只能证明包与提供的校验文件一致，校验文件应从可信渠道获取。Windows
更新时会先重命名旧程序；仍在使用的旧 `.talk-old-*.exe` 可在所有进程退出后清理。
不同 `--state-dir` 下运行的其他实例需要分别停止、重启才能使用新版本。

## 安全边界

临时数字码通过 P-256 PAKE、双方密钥确认和 AES-256-GCM 传递房间密钥；数字码与
房间密钥不会明文发送。配对接口限制报文大小、连接时长和总尝试次数。知道有效码
且能连接邀请者的人可以抢先加入，因此只把码交给预期成员。对配对端口的恶意连接
可以耗尽次数，届时生成新码即可。配对协议及其集成尚未经过独立安全审计。

随机房间密钥用于 AES-256-GCM 加密和认证；随机报文 nonce、防重放窗口与时间戳
阻止重放。各机器时间须同步，最大时差约 15 秒。持有房间密钥者均为受信任成员，
可听到房间语音并邀请其他人；不提供成员级身份、撤销或前向保密。移除成员需要
停止所有实例并创建新密钥。元数据（IP、流量、时序）不隐藏。

控制接口只监听本机回环地址，并使用独立随机令牌认证。音频不写盘；`daemon.log`
只记运行信息。密码协议尚未经过独立审计。

## 开发和发布

需要 Go 1.25+ 与 C 编译器；CI 使用 Go 1.26.2。音频通过 malgo/miniaudio 访问
CoreAudio、WASAPI、ALSA/PulseAudio。`third_party/wirectl` 固定了 CLI 基座快照，
因此构建无需修改相邻项目。

```sh
make check
make build
./bin/wirectl talk --help
python3 scripts/package.py --version 0.1.0
python3 scripts/test-package.py
```

缓存和测试临时文件都留在当前目录 `.cache`。测试覆盖加密与重放、三节点真实 UDP
通信、混音、实例锁、状态接口权限、更新校验与安装包实际升级，以及数字码配对、
错误码/过期/重用/篡改拒绝、配对后保存与重启、多成员配对后收发音频帧。

推送 `vX.Y.Z` 标签触发 GitHub Actions：五个平台分别原生测试、构建、安装包
自更新测试，通过后发布二进制包、校验和、构建证明、Homebrew Formula 和 Scoop
Manifest，并执行实际 Homebrew/Scoop 安装检查。

Homebrew tap 与 Scoop bucket 各自的 `Sync talk releases` Actions 每小时检查正式
Release，独立核对 GitHub 摘要和 SHA256SUMS，再做原生安装测试；通过才提交包定义。
可手动触发同步。这个流程使用包仓库自己的 GITHUB_TOKEN，不需要跨仓库个人令牌。
同步可能受 GitHub 调度延迟影响。维护用工作流源文件在 `packaging/`。
