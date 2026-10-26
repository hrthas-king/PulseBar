# PulseBar

<img src="assets/icon.png" alt="PulseBar 应用图标" width="160">

![Windows](https://img.shields.io/badge/Windows-10%20%2F%2011-0078D4)
![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8)
![CGO](https://img.shields.io/badge/CGO-disabled-brightgreen)
![Dependencies](https://img.shields.io/badge/runtime_dependencies-0-brightgreen)
![License](https://img.shields.io/badge/license-MIT-blue)
[![CI](https://github.com/hrthas-king/PulseBar/actions/workflows/ci.yml/badge.svg)](https://github.com/hrthas-king/PulseBar/actions/workflows/ci.yml)

**合二为一的极轻量 Windows 任务栏网络脉搏（GeoPulse）与 VOO / QQQ 24小时全时段实时行情条（TickBar）。**

双击单个 `~3.2 MB` 的 `.exe` 即可静默运行，同时在 **任务栏最左侧悬浮条** 与 **系统托盘** 展示：
1. **GeoPulse 秒级网络感知**：公网出口两位国家代码、连通性状态灯（绿/黄/红）、上下行实时流量活动箭头（或一键展开实时网速与 ICMP RTT）、TUN/系统代理节点切换自动感知。
2. **TickBar VOO / QQQ 24×5 全时段无缝实时行情**：追踪 `VOO`（标普 500 ETF）与 `QQQ`（纳斯达克 100 ETF），无缝覆盖美东时间周日 20:00 至周五 20:00 的 **夜盘（Overnight）+ 盘前（Pre-Market）+ 盘中（Regular）+ 盘后（Post-Market）** 24 小时连续实时价格流与日涨跌幅。

纯 Go 标准库直接调用 Win32 GDI、Shell_NotifyIcon、IP Helper 与 WinHTTP 系统 API；无 CGo、无 Python 解释器打包、无 Electron/WebView，运行期第三方依赖为零，私有工作集仅约 **4.5 MiB**。

---

## 核心特性

### 1. 任务栏悬浮条 + 系统托盘双视图
- **紧凑模式（默认）**：`[● US ↑↓ | ● VOO 699.48 +0.13% | ● QQQ 704.35 -0.01%]`
  - 左侧显示出口国家代码、连通状态圆点与随流量亮起的上下行箭头；右侧同屏显示 `VOO` 与 `QQQ` 的 24h 实时成交价与当前时段涨跌幅（每 `$0.01` 跳动即刻可见）。
- **详细网速模式（右键菜单一键切换）**：`[● US ↑12K ↓1.5M 18ms | ● VOO 699.48 +0.13% | ● QQQ 704.35 -0.01%]`
  - 在任务栏悬浮条上直接展开实时上行速率、下行速率与 ICMP 延迟。
- **系统托盘动态图标**：32×32 像素级实时绘制两位国家代码、状态点与上下行闪烁箭头。当悬浮条开启**锁定穿透**时，仍可通过悬停或右键系统托盘查看完整信息。
- **交互与布局**：
  - 默认停靠在主屏幕任务栏最左侧（边距 8 逻辑像素，支持高 DPI 自动缩放）。
  - 解锁状态下按住左键可自由拖拽并记忆坐标；右键菜单支持**“锁定位置（鼠标穿透）”**与**“移到任务栏最左侧”**。
  - 支持一键切换 **绿涨红跌 / 红涨绿跌** 配色、**1s / 2s / 5s** 刷新间隔及当前用户 **开机自启**。

### 2. VOO / QQQ 24×5 全时段实时无缝衔接 (TickBar 内核)
- **四时段 24 小时无缝覆盖（美东时间 ET，周日 20:00 — 周五 20:00）**：
  - **夜盘 (Overnight, Blue Ocean ATS / BOATS)**：`20:00 – 04:00`（8 小时）
  - **盘前 / 早盘 (Pre-Market)**：`04:00 – 09:30`（5.5 小时）
  - **盘中 (Regular Market)**：`09:30 – 16:00`（6.5 小时）
  - **盘后 (Post-Market)**：`16:00 – 20:00`（4 小时）
- **BOATS 夜盘实时成交价与跨时段无缝切换**：通过原生 WinHTTP 长会话（自动管理 Cookie 与 Crumb 令牌）单次并发拉取 `/v7/finance/quote?symbols=VOO,QQQ&overnightPrice=true`（双标的总响应仅约 `2.4 KB`，耗时 `~170ms`），直接读取 Blue Ocean ATS (`BOATS Real Time Price`) 的 `overnightMarketPrice` / `preMarketPrice` / `regularMarketPrice` / `postMarketPrice` 及各时段精确成交时间戳（`overnightMarketTime` 等），按最新真实成交时间戳无缝衔接四个时段，并在悬停提示与诊断面板中展示当前时段、数据源与最近一笔成交距今秒数。

### 3. 秒级网络感知与代理节点切换检测 (GeoPulse 内核)
- **64 位出口网卡计数**：通过 `GetBestInterface` + `GetIfEntry2` 每秒读取当前有效路由出口网卡字节计数，避免物理网卡与 VPN 虚拟网卡重复统计。
- **极低开销 ICMP 探针**：每秒最多发送 1 个 32 字节 `IcmpSendEcho`（650ms 超时），失败下一秒自动交替探测 `1.1.1.1` / `8.8.8.8`；单次失败变黄（待确认），连续两次失败变红（断网警告），收到单次响应立即恢复绿色。
- **TUN / 代理出口感知**：同时监听系统路由/地址变更事件与本地代理内核（`mihomo`、`sing-box`、`xray`、`clash` 等）连接端点指纹，节点切换后自动复核 Cloudflare `/cdn-cgi/trace`。

---

## 快速构建与验证

要求 Windows 10 / 11（64 位）与 Go 1.24+：

```powershell
git clone https://github.com/hrthas-king/PulseBar.git
cd PulseBar
./scripts/verify.ps1
./dist/PulseBar-windows-amd64.exe
```

常用命令行参数：

```powershell
# 停靠回任务栏最左侧（对已运行实例同样生效）
./dist/PulseBar-windows-amd64.exe --dock-left

# 打开网络与行情详细诊断窗口
./dist/PulseBar-windows-amd64.exe --diagnostics

# 终端执行一次探针并打印完整诊断文本
./dist/PulseBar-windows-amd64.exe --probe

# 请求正在运行的实例优雅退出
./dist/PulseBar-windows-amd64.exe --quit
```

更换图标或打包发布归档：

```powershell
./scripts/generate-icons.ps1
./scripts/package.ps1 -Version v0.2.0
```

---

## 目录结构

```text
PulseBar/
├── assets/                         # 现代应用图标 PNG、多尺寸 ICO 与 winres 配置
├── cmd/pulsebar/                   # 入口与双架构 Windows 图标资源 (.syso)
├── internal/
│   ├── app/                        # 四线并发调度（网卡采样、心跳、定位、VOO/QQQ 行情）
│   ├── config/                     # 持久化配置 (%LOCALAPPDATA%\PulseBar\config.json)
│   ├── ipinfo/                     # WinHTTP Cloudflare trace 查询与解析
│   ├── market/                     # WinHTTP VOO/QQQ 24h 全时段实时行情抓取与校验
│   ├── monitor/                    # 连通性状态机与 64 位速率计算
│   ├── network/                    # ICMP、NIC 计数、路由事件与本地 TUN/代理指纹
│   ├── startup/                    # HKCU\...\Run\PulseBar 注册表自启管理
│   ├── ui/                         # GDI 分层任务栏悬浮条、系统托盘、动态图标与菜单
│   └── winapi/                     # 系统 DLL 安全加载、单实例互斥与内存统计
├── scripts/
│   ├── build.ps1
│   ├── generate-icons.ps1
│   ├── package.ps1
│   └── verify.ps1
├── docs/
│   ├── ARCHITECTURE.md
│   ├── VALIDATION.md
│   └── CHANGELOG.md
└── .github/workflows/
    ├── ci.yml                      # 自动测试、静态检查与双架构构建
    └── release.yml                 # Tag 触发交叉编译与 GitHub Release 发布
```

## 许可证

采用 [MIT License](LICENSE)。行情数据访问与使用遵循相关数据源及交易所条款，本软件不提供交易执行或行情再分发授权。
