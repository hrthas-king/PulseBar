# 验证记录 (Validation)

## 1. v0.5.1 验证范围与结论

- 验证日期：**2026-10-27（美东时间）**。
- 应用源码基线：[`v0.5.1` / `372b14f`](https://github.com/hrthas-king/PulseBar/commit/372b14f61364106170059384c6d2020946d27da0)。
- 本机环境：Windows 11 amd64（系统版本 `10.0.26200`）、Go 1.27.2，`CGO_ENABLED=0`。
- 项目最低 Go 版本为 1.24；GitHub CI 与发布流水线使用 Go 1.24.x。本机和 GitHub 构建的文件体积及哈希可能因工具链不同而不同。

**结论：19 项自动测试、静态检查、格式检查及 Windows amd64 / arm64 构建均通过；根目录运行副本已同步到 v0.5.1。实际联网、GUI 资源占用和行情源延迟的最新验证尚未完成。**

| 检查项 | 结果 | 证据与范围 |
| :--- | :--- | :--- |
| `go test -count=1 -json ./...` | 通过 | 7 个测试包，19 项测试，0 失败 |
| `go vet ./...` | 通过 | 退出码 0 |
| `gofmt -l cmd internal` | 通过 | 无待格式化文件 |
| Windows amd64 构建 | 通过 | `CGO_ENABLED=0`，版本参数 `v0.5.1` |
| Windows arm64 构建 | 通过 | 交叉编译通过，尚未在 ARM64 设备上运行 |
| 根目录 `PulseBar.exe --version` | 通过 | 输出 `PulseBar v0.5.1` |
| 根目录与 dist 的 amd64 程序比对 | 通过 | 文件大小与 SHA-256 一致 |
| v0.5.1 GitHub CI / Release | 通过 | [CI](https://github.com/hrthas-king/PulseBar/actions/runs/40462052842)、[Release 流水线](https://github.com/hrthas-king/PulseBar/actions/runs/40462060700) |

受限执行环境的默认临时目录曾在配置文件重命名时返回权限错误。本轮将 `TMP`、`TEMP`、`GOTMPDIR` 指向项目内临时目录后，完整测试一次性通过。详细日志保存在本机 `.artifacts/validation-v0.5.1-20261027/`，不随源码提交。

## 2. 当前自动测试覆盖

| 模块 | 测试数 | 已验证内容 |
| :--- | ---: | :--- |
| `internal/app` | 1 | 模拟网络、定位与行情结果聚合；运行中从 VOO / QQQ 切换到 A 股分组 |
| `internal/config` | 1 | 配置默认值及读写；位置、刷新间隔、市场分组、85% 缩放等字段持久化 |
| `internal/ipinfo` | 1 | Cloudflare trace 解析；拒绝私有 IP、回环地址、无效国家码和重复字段 |
| `internal/market` | 8 | Yahoo v7 / v8 样例解析、多市场币种和类型、五态时段、腾讯查询映射及 A股 / 港股 / 日韩 / 商品样例解析 |
| `internal/monitor` | 2 | 网络状态防抖；网卡切换、计数回滚及采样时间异常处理 |
| `internal/network` | 3 | 网卡与进程结构 ABI；代理远端端点切换指纹 |
| `internal/ui` | 3 | Win32 结构 ABI、32×32 托盘图标像素方向、悬停和诊断文字格式 |
| **合计** | **19** | **均通过** |

当前网络状态规则为：已在线时单次探针失败保持 `Online`；连续 2～3 次失败进入 `Degraded`；连续 4 次失败进入 `Offline`；任意一次成功恢复 `Online`。初始未知状态下首次失败进入 `Degraded`。

行情分组配置共 8 组：`voo_qqq`、`soxx_smh`、`top2_us`、`gold_oil`、`cn_indices`、`hk_indices`、`asia_indices`、`eu_indices`。腾讯查询映射覆盖 A股、港股、黄金 / 布油、日韩和欧洲指数；解析测试使用固定样例，不代表已经完成所有标的的线上验证。

行情校验覆盖最新价、涨跌额、涨跌幅、有限数值、标的、币种和资产类型；当前响应体上限为 **32,768 字节**。日韩测试覆盖给定时间下的盘中、东京午休及收盘状态；这些检查不等于交易所节假日历或夏令时的完整验证。

## 3. 本地 v0.5.1 运行文件

以下为现有本地运行文件的检查结果。根目录 amd64 程序由 `dist/PulseBar-windows-amd64.exe` 同步，旧版 v0.4.1 副本已备份至本机验证目录。

| 文件 | 版本 | 大小 |
| :--- | :--- | ---: |
| `PulseBar.exe` | v0.5.1，已执行 `--version` | 3,469,824 字节（约 3.31 MiB） |
| `dist/PulseBar-windows-amd64.exe` | 与根目录程序完全一致 | 3,469,824 字节（约 3.31 MiB） |
| `dist/PulseBar-windows-arm64.exe` | 本地 v0.5.1 分发文件，仅检查构建信息 | 3,210,752 字节（约 3.06 MiB） |

本地文件 SHA-256：

```text
E3CF7976CD3FF3EA8FEBBD64CB934E0DF6187E387263A5A1CF30F23AF997C817  PulseBar.exe / dist/PulseBar-windows-amd64.exe
3F2ECDD05EE3BBB253C978CBF0EF84F838D6E822E56B249FA3BA9BB26B9C40E2  dist/PulseBar-windows-arm64.exe
```

根目录运行副本和 `dist/` 按 `.gitignore` 不进入源码仓库。GitHub 已发布 [v0.5.1](https://github.com/hrthas-king/PulseBar/releases/tag/v0.5.1)，包含 amd64、arm64 压缩包和 `SHA256SUMS.txt`；下载包应使用该发布页的校验文件核对，不能与上面的本地 EXE 哈希直接比较。

## 4. 尚待完成的 v0.5.1 实测

- **当前环境的联网诊断未完成**：执行 8 秒无窗口诊断时，`IcmpCreateFile` 返回 `Access is denied`，程序在初始化阶段退出，未生成新报告。该次结果不能证明网络、定位或行情源的实际运行状态。
- **真实行情延迟**：需要在对应市场交易时间，对照返回的成交时间戳和可靠的市场参考，分别记录数据年龄、请求往返耗时及变化频率。“秒级轮询”与“行情零延迟”不是同一指标，当前自动测试不证明零延迟。
- **跨时段与异常恢复**：仍需实测各市场开收盘、午休、法定节假日、夏令时、腾讯 / Yahoo 回退、节点切换和断网恢复。
- **界面与资源占用**：仍需重新测量 v0.5.1 的 GUI 内存、GDI / USER 对象数，并检查不同 DPI、多屏、缩放和任务栏位置。结构 ABI 与图标测试不证明所有屏幕布局均正确。
- **Windows ARM64**：当前仅完成交叉编译，仍需真机运行验证。

## 5. 历史参考：v0.2.0 GUI 与联网记录

以下保留旧验证文档的原始结果，**仅代表 v0.2.0 的一次历史运行，不作为 v0.5.1 的性能或行情准确性结论**。

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

## 6. 复现命令

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
./scripts/build.ps1 -Arch amd64 -Version v0.5.1
./scripts/build.ps1 -Arch arm64 -Version v0.5.1
Copy-Item ./dist/PulseBar-windows-amd64.exe ./PulseBar.exe -Force
./PulseBar.exe --version
Get-FileHash ./PulseBar.exe, ./dist/PulseBar-windows-amd64.exe -Algorithm SHA256
```

在允许网络探针的正常桌面环境中执行新的运行实测：

```powershell
Start-Process ./PulseBar.exe -ArgumentList '--diagnose=10s --output .artifacts/diagnose-v0.5.1.json' -WindowStyle Hidden -Wait
Start-Process ./PulseBar.exe -ArgumentList '--run-for=10s --output .artifacts/smoke-v0.5.1.json' -WindowStyle Hidden -Wait
```

GUI 冒烟前需先退出已有实例，避免单实例机制跳过新运行。诊断 JSON 可能包含公网 IP，提交或公开分享前应脱敏。

如需重新生成本地分发包：

```powershell
./scripts/package.ps1 -Version v0.5.1
```
