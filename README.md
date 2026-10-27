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
  - 采用 GDI `GetTextExtentPoint32W` 动态紧凑流式排版，消除网速、延迟与行情槽位之间的多余留白，避免遮挡或撞上 Windows 11 居中开始按钮。
  - **可调节显示大小（75% ~ 130%）**：右键菜单支持 **`显示大小 ▶`** 六档无损缩放（`75% 迷你` / `85% 紧凑` / `90% 适中` / `100% 标准` / `115% 放大` / `130% 特大`），自动记忆至 `config.json` 的 `ui_scale_percent` 字段。
  - 解锁状态下按住左键可自由拖拽并记忆坐标；右键菜单支持**“锁定位置（鼠标穿透）”**与**“移到任务栏最左侧”**。
  - 支持一键切换 **绿涨红跌 / 红涨绿跌** 配色、**1s / 2s / 5s** 刷新间隔及当前用户 **开机自启**。

### 2. 全球 8 大市场标的折叠切换与五态交易时段信号 (TickBar 内核)
- **右键菜单可折叠切换 8 组主流标的对（自动持久化至 `config.json`）**：
  - 第一行直接展示当前选中的标的对（如 `✔ 标普 / 纳指 (VOO / QQQ)`），第二行提供可折叠子菜单 **`More (更多行情标的) ▶`**，展开即可切换其余 7 组全球标的：
  1. `voo_qqq`（默认）：**标普 / 纳指 (`VOO` / `QQQ`)** —— 别名 `VOO`、`QQQ`
  2. `soxx_smh`：**半导体 ETF (`SOXX` / `SMH`)** —— 别名 `SOXX`、`SMH`
  3. `top2_us`：**美股市值前二 (`NVDA` / `MSFT`)** —— 别名 `NVDA`、`MSFT`
  4. `gold_oil`：**黄金 / 布伦特原油 (`GC=F` / `BZ=F`)** —— 别名 `黄金`、`布油`
  5. `cn_indices`：**A股指数 (`000001.SS` / `399006.SZ`)** —— 别名 `上证`、`创指`
  6. `hk_indices`：**港股指数 (`^HSI` / `^HSTECH`)** —— 别名 `恒指`、`恒科`
  7. `asia_indices`：**日韩指数 (`^N225` / `^KS11`)** —— 别名 `日经`、`韩指`
  8. `eu_indices`：**欧洲指数 (`^FCHI` / `^GDAXI`)** —— 别名 `CAC40`、`DAX`
- **A 股 / 港股 / 日韩 / 大宗商品 0 延迟秒级实时直连**：
  - A 股、港股、日韩核心指数（日经225 / 韩国综合）及大宗商品标的直连腾讯财经行情通道（`qt.gtimg.cn`），彻底打破 Yahoo Finance 的 15~20 分钟延时限制，实现真正 0 延迟秒级 tick 刷新；
  - 黄金与布伦特原油期货支持 24 小时连续交易时段识别（周一至周五全天候连续监控）；日韩指数精准匹配东京证交所（含午休）与韩国证交所实时交易时钟。
- **五态交易时段状态信号灯**：
  - ☀️ **早盘 / 盘前 (`PRE`)**：明黄色实心太阳符号 `☀`（`RGB(255, 214, 10)`）
  - 🟢 **盘中 (`REGULAR`)**：高亮翠绿色实心圆点（`RGB(0, 230, 118)`）
  - 🔵 **盘后 (`POST`)**：天蓝色实心圆点（`RGB(56, 189, 248)`）
  - 🌙 **夜盘 (`OVERNIGHT`)**：亮紫色实心月牙符号 `🌙`（`RGB(192, 132, 252)`，Blue Ocean ATS 夜盘）
  - ⚪ **休市 / 周末 (`CLOSED`)**：浅灰色实心圆点（`RGB(148, 163, 184)`）
  - **异常提示**：连续抓取失败或超时时，在信号右侧保留红色感叹号 `!` 提示，并在悬停提示与诊断面板中展示具体原因。

### 3. 秒级网络感知与代理节点切换检测 (GeoPulse 内核)
- **64 位出口网卡计数**：通过 `GetBestInterface` + `GetIfEntry2` 每秒读取当前有效路由出口网卡字节计数，避免物理网卡与 VPN 虚拟网卡重复统计。
- **极低开销多层防抖探针（ICMP + TCP 443 兜底 + 流量旁路证明）**：优先发送 32 字节 `IcmpSendEcho`，若主目标失败立即同周期复核备用目标（`1.1.1.1` / `8.8.8.8`）；若 TUN/代理节点丢弃三层 ICMP，自动通过轻量 TCP 443 握手兜底并结合实时下行流量/HTTPS 成功通信作为旁路在线证明，杜绝 TUN 模式下误报断网闪烁。
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
