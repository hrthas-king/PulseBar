# 验证记录 (Validation)

## v0.6.0：新增深交所 ETF 行情（最新）

验证日期：**2026-10-27 美东时间**（对应下述行情日期为 **2026-10-28 北京时间**）。本轮源码全量测试 **31 项**（含子项共 **49 项**）通过，`go vet`、`gofmt` 检查与 Windows amd64 / arm64 构建均通过。ARM64 尚未在真机运行。

- 新增分组 `etf_159632_159655`：腾讯 `sz159632` 对应纳斯达克ETF华安，`sz159655` 对应标普500ETF华夏。均为深交所场内 ETF；价格以人民币显示三位小数，涨跌幅保留两位。
- 自动测试覆盖菜单动作与缩放动作不冲突、分组配置保存、三位精度、行情代码及时间校验、两只标的独立失败、深交所交易时段、午休和休市，以及 IPv6 场景下托盘提示不截断第二只基金。
- 腾讯行情请求失败时，两只 ETF 不会回退到时效未经核实的 Yahoo 行情。已有缓存仍遵循程序原有的短时容错与过期规则；悬停或诊断可查看行情时间与数据源。
- 在独立配置下运行 v0.6.0 两次 7 秒联网诊断，均获取到两只 `ETF / CNY` 行情且状态正常；两轮返回的北京时间从 **10:05:48** 更新到 **10:05:57**。本次报价分别为 `159632 2.506`、`159655 2.013`，仅作为历史测试样本。
- 最终 v0.6.0 构建的 10 秒原生 GUI 冒烟通过（`gui_started=true`），两只行情均正常；北京时间 **10:12:12** 的样本为 `159632 2.502`、`159655 2.013`。GUI 私有工作集为 **6,414,336 字节（约 6.12 MiB）**，GDI / USER 对象数为 **21 / 12**。
- 同期独立读取腾讯与新浪公开行情：例如 `159632` 分别返回 **10:05:15 / 2.507** 与 **10:05:18 / 2.506**，表明不同采样时刻可能得到不同成交价。行情时间距本机接收时刻的差值可能包含无新成交的时间，不能单凭一次采样证明持续零延迟。

本轮本机独立构建产物 SHA-256（与 GitHub Actions 使用不同 Go 工具链时可能不同）：

```text
B4933C0B06E3B28A638FDA915C7811BA0125067C48754A12D3379CDC65AD3A33  PulseBar.exe / dist/PulseBar-windows-amd64.exe
E73876F84F1F1A280E32BE803D2B7E91FFF8FA3FC6CB4D99006649EF02394C72  PulseBar-v0.6.0-arm64.exe
```

**时段说明**：这两只基金在深交所场内交易，工作日通常为北京时间 09:30–11:30、13:00–15:00，盘前集合竞价和休市状态单独显示。节假日或行情日期尚未更新时，界面显示休市或日期未更新提示；美国市场夜盘不意味着这两只深交所基金同时成交。

## 以下为 v0.5.2 历史验证记录

历史记录中的文件版本、哈希与测试数量只代表当时结果；当前 v0.6.0 请以上方记录为准。

### 1. v0.5.2 验证范围与结论

- 验证日期：**2026-10-27（美东时间）**。
- 应用版本：[`v0.5.2`](https://github.com/hrthas-king/PulseBar/tree/v0.5.2)，修复 TUN 模式下网络延迟固定为 1ms 的问题。
- 本机环境：Windows 11 amd64（系统版本 `10.0.26200`）、Go 1.27.2，`CGO_ENABLED=0`。
- 项目最低 Go 版本为 1.24；GitHub CI 与发布流水线使用 Go 1.24.x。本机和 GitHub 构建的文件体积及哈希可能因工具链不同而不同。

**当时结论：23 项自动测试（含子项共 29 项）、静态检查、格式检查及 Windows amd64 / arm64 构建均通过。正常 Windows 会话中的联网诊断与 10 秒 GUI 冒烟通过；当时根目录运行副本为 v0.5.2。全市场行情实际延迟、完整屏幕布局及 ARM64 真机运行仍待验证。**

| 检查项 | 结果 | 证据与范围 |
| :--- | :--- | :--- |
| `go test -count=1 -json ./...` | 通过 | 7 个测试包，23 项测试（含子项 29 项），0 失败 |
| `go vet ./...` | 通过 | 退出码 0 |
| `gofmt -l cmd internal` | 通过 | 无待格式化文件 |
| Windows amd64 构建 | 通过 | `CGO_ENABLED=0`，版本参数 `v0.5.2` |
| Windows arm64 构建 | 通过 | 交叉编译通过，尚未在 ARM64 设备上运行 |
| 根目录 `PulseBar.exe --version` | 通过 | 输出 `PulseBar v0.5.2` |
| 根目录与 dist 的 amd64 程序比对 | 通过 | 文件大小与 SHA-256 一致 |
| 正常 Windows 会话联网诊断 | 通过 | KR 出口、Nano TUN，独立 HTTPS 测量成功 |
| 10 秒 GUI 冒烟 | 通过 | `gui_started=true`，采样得到 248ms HTTPS 请求延迟 |

受限执行环境的默认临时目录曾在配置文件重命名时返回权限错误。本轮将 `TMP`、`TEMP`、`GOTMPDIR` 指向项目内临时目录后，完整测试一次性通过。受限环境也曾拒绝 `IcmpCreateFile`，本轮改在正常 Windows 会话复测成功。详细日志保存在本机 `.artifacts/latency-debug/`，诊断 JSON 含公网 IP，不随源码提交。

### 2. v0.5.2 自动测试覆盖

| 模块 | 测试数 | 已验证内容 |
| :--- | ---: | :--- |
| `internal/app` | 3 | 网络/定位/行情聚合与分组切换；连通性旁证不生成/沿用延迟；出口切换后迟到测量不能覆盖新结果 |
| `internal/config` | 1 | 配置默认值及读写；位置、刷新间隔、市场分组、85% 缩放等字段持久化 |
| `internal/ipinfo` | 1 | Cloudflare trace 解析；拒绝私有 IP、回环地址、无效国家码和重复字段 |
| `internal/market` | 8 | Yahoo v7 / v8 样例解析、多市场币种和类型、五态时段、腾讯查询映射及 A股 / 港股 / 日韩 / 商品样例解析 |
| `internal/monitor` | 2 | 网络状态防抖；网卡切换、计数回滚及采样时间异常处理 |
| `internal/network` | 4 | 网卡与进程结构 ABI；代理端点指纹；已取消的 HTTPS 测量不返回有效延迟 |
| `internal/ui` | 4 | Win32 ABI、托盘图标、提示文字；1ms 心跳不得覆盖 HTTPS 测量，失败/过期/非 HTTPS 数值不得显示 |
| **合计** | **23** | **含子项 29 项，均通过** |

当前网络状态规则为：已在线时单次探针失败保持 `Online`；连续 2～3 次失败进入 `Degraded`；连续 4 次失败进入 `Offline`；任意一次成功恢复 `Online`。初始未知状态下首次失败进入 `Degraded`。

v0.5.2 行情分组配置共 8 组：`voo_qqq`、`soxx_smh`、`top2_us`、`gold_oil`、`cn_indices`、`hk_indices`、`asia_indices`、`eu_indices`。腾讯查询映射覆盖 A股、港股、黄金 / 布油、日韩和欧洲指数；解析测试使用固定样例，不代表已经完成所有标的的线上验证。

行情校验覆盖最新价、涨跌额、涨跌幅、有限数值、标的、币种和资产类型；当前响应体上限为 **32,768 字节**。日韩测试覆盖给定时间下的盘中、东京午休及收盘状态；这些检查不等于交易所节假日历或夏令时的完整验证。

### 3. 当时的本地 v0.5.2 运行文件

以下为当时本地运行文件的检查结果。根目录 amd64 程序当时由 `dist/PulseBar-windows-amd64.exe` 同步，旧版 v0.5.1 副本已备份至本机验证目录；以下哈希不是当前 v0.6.0 文件的哈希。

| 文件 | 版本 | 大小 |
| :--- | :--- | ---: |
| `PulseBar.exe` | v0.5.2，已执行 `--version` | 3,487,232 字节（约 3.33 MiB） |
| `dist/PulseBar-windows-amd64.exe` | 与根目录程序完全一致 | 3,487,232 字节（约 3.33 MiB） |
| `dist/PulseBar-windows-arm64.exe` | 本地 v0.5.2 分发文件，尚未真机运行 | 3,225,600 字节（约 3.08 MiB） |

本地文件 SHA-256：

```text
4C05E5948F0DB100311E8437E63B036927A750C369C710ACCC1AB7FF672BE6A7  PulseBar.exe / dist/PulseBar-windows-amd64.exe
B56C90A65D73F22EFA9FA2E15DF1DA9D970B3EF496A6B0228548A124EF189509  dist/PulseBar-windows-arm64.exe
```

根目录运行副本和 `dist/` 按 `.gitignore` 不进入源码仓库。GitHub 程序通过 [v0.5.2 Releases](https://github.com/hrthas-king/PulseBar/releases/tag/v0.5.2) 提供；下载包应使用发布页的 `SHA256SUMS.txt` 核对，不能与上面的本地 EXE 哈希直接比较。

### 4. TUN 延迟问题复现与修复实测

环境为 KR 出口、Nano TUN 活跃。旧版原始 ICMP 回包的系统 RTT 为 **0ms**，随后被程序强制改成 **1ms**；独立 TCP 握手也约为 1ms。多轮 HTTPS GET 对照请求为约 **241～551ms**，说明旧显示值不能代表该外网请求耗时。

修复版将心跳连通性与显示延迟分开，每 5 秒向 `https://www.cloudflare.com/cdn-cgi/trace` 发起独立 HTTPS GET。计时从发送前到收到响应头，包含 DNS、建连和 TLS；按 [WinHttpReceiveResponse 文档](https://learn.microsoft.com/en-us/windows/win32/api/winhttp/nf-winhttp-winhttpreceiveresponse)，该调用成功后状态码与响应头已可读取。本接口的 HEAD 请求实际返回 404，因此最终使用已验证返回 200 的 GET 请求。

| 运行 | 原始心跳 | HTTPS 测量 | 结果 |
| :--- | :--- | :--- | :--- |
| v0.5.1 复现 | ICMP 0ms 被改成 1ms | 未独立测量 | 悬浮条显示 1ms |
| v0.5.2 无窗口诊断 12 秒 | ICMP 0ms | 336.34ms | 显示逻辑使用 336ms |
| v0.5.2 GUI 冒烟 10 秒 | ICMP 0ms | 248.18ms | `gui_started=true`，显示逻辑使用 248ms |

这些数值是不同时间的采样，不保证后续延迟恒定。此指标表示指定 HTTPS 目标的请求耗时；目标、网络路径和握手成本都会影响数值。

本轮 GUI 资源快照：

| 指标 | v0.5.2 实测数值 |
| :--- | ---: |
| GUI 私有工作集 | 6,381,568 字节（约 6.09 MiB） |
| GUI 总工作集（含共享 DLL） | 24,592,384 字节（约 23.45 MiB） |
| 峰值总工作集 | 25,350,144 字节（约 24.18 MiB） |
| 私有提交内存 | 18,604,032 字节（约 17.74 MiB） |
| GDI / USER 对象数 | 21 / 13 |

### 5. v0.5.2 当时尚待完成的实测

- **真实行情延迟**：需要在对应市场交易时间，对照返回的成交时间戳和可靠的市场参考，分别记录数据年龄、请求往返耗时及变化频率。“秒级轮询”与“行情零延迟”不是同一指标，当前自动测试不证明零延迟。
- **跨时段与异常恢复**：仍需实测各市场开收盘、午休、法定节假日、夏令时、腾讯 / Yahoo 回退，以及真实节点切换和断网恢复。迟到测量丢弃已通过确定性的模拟切换回归测试。
- **界面与长期运行**：已记录上述 GUI 资源快照，仍需长时间观察，并检查不同 DPI、多屏、缩放和任务栏位置。结构 ABI 与图标测试不证明所有屏幕布局均正确。
- **Windows ARM64**：当前仅完成交叉编译，仍需真机运行验证。

### 6. 历史参考：v0.2.0 GUI 与联网记录

以下保留旧验证文档的原始结果，**仅代表 v0.2.0 的一次历史运行，不作为 v0.5.2 的性能或行情准确性结论**。

原记录在美东时间凌晨 02:35 执行 `--run-for=5s`：识别出口网卡 `Nano` 与 `tun_or_proxy_active=true`，ICMP 确认在线，出口国家码为 `US`；VOO 为 `699.00 / -0.47%`，QQQ 为 `703.43 / -0.148%`，HTTP RTT 约 166 ms。当时报告使用 `24h_fullday_realtime`，后续版本已调整行情源和夜盘涨跌幅基准。

| 历史指标 | v0.2.0 实测数值 |
| :--- | ---: |
| amd64 EXE | 3,361,792 字节（约 3.21 MiB） |
| arm64 EXE | 3,118,080 字节（约 2.97 MiB） |
| GUI 私有工作集 | 4,620,288 字节（约 4.41 MiB） |
| GUI 总工作集（含共享 DLL） | 20,774,912 字节（约 19.81 MiB） |
| 峰值总工作集 | 21,843,968 字节（约 20.83 MiB） |
| 私有提交内存 | 16,293,888 字节（约 15.54 MiB） |
| GDI / USER 对象数 | 20 / 11 |

## 当前版本复现命令

要求 Windows 10 / 11、Go 1.24+，在项目根目录执行。常规验证可运行 `./scripts/verify.ps1`；如默认临时目录受权限限制，可先将临时目录设在项目内：

```powershell
$validationTemp = Join-Path (Get-Location) '.artifacts/validation-temp'
New-Item -ItemType Directory -Force -Path $validationTemp | Out-Null
$env:TMP = $validationTemp
$env:TEMP = $validationTemp
$env:GOTMPDIR = $validationTemp
./scripts/verify.ps1
```

`verify.ps1` 会生成 `dev` 版本的检查程序。完成后使用明确版本重新构建，并同步根目录运行副本：

```powershell
./scripts/build.ps1 -Arch amd64 -Version v0.6.0
./scripts/build.ps1 -Arch arm64 -Version v0.6.0
Copy-Item ./dist/PulseBar-windows-amd64.exe ./PulseBar.exe -Force
./PulseBar.exe --version
Get-FileHash ./PulseBar.exe, ./dist/PulseBar-windows-amd64.exe -Algorithm SHA256
```

在允许网络探针的正常桌面环境中执行新的运行实测：

```powershell
Start-Process ./PulseBar.exe -ArgumentList '--diagnose=10s --output .artifacts/diagnose-v0.6.0.json' -WindowStyle Hidden -Wait
Start-Process ./PulseBar.exe -ArgumentList '--run-for=10s --output .artifacts/smoke-v0.6.0.json' -WindowStyle Hidden -Wait
```

GUI 冒烟前需先退出已有实例，避免单实例机制跳过新运行。诊断 JSON 可能包含公网 IP，提交或公开分享前应脱敏。

如需重新生成本地分发包：

```powershell
./scripts/package.ps1 -Version v0.6.0
```
