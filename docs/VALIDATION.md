# 验证记录 (Validation)

日期：2026-10-25。测试环境：Windows 11 amd64，Go 1.27.2，`CGO_ENABLED=0`，零第三方运行时依赖单文件构建。

## 1. 功能与单元测试验证

- **连通性与速率状态机 (`internal/monitor`)**：单次失败转黄色 `Degraded`、连续两次失败转红色 `Offline`、单次成功恢复绿色 `Online`；网卡 LUID/Index 变更或计数器回滚自动重建基线。
- **公网 IP 与国家码解析 (`internal/ipinfo`)**：验证 Cloudflare `/cdn-cgi/trace` 解析，拒绝私有 IP、回环地址、无效国家代码或重复字段。
- **TradingView 期货行情校验 (`internal/market`)**：
  - 独立校验 `CME_MINI:ES1!` 与 `CME_MINI:NQ1!`；单个标的异常不影响另一有效标的。
  - 校验 `futures` 类型、`USD` 币种、有限数值、`ChangeAbs / Previous * 100` 与 `ChangePercent` 一致性（`rel_tol=1e-6, abs_tol=0.02`）、2047 字节响应上限及看门狗过期变红。
- **双核聚合调度 (`internal/app`)**：验证 1s 网卡采样、1s ICMP 心跳、事件驱动 Geo 定位与 TradingView 轮询并发协同。
- **Win32 64 位 ABI 与图标方向 (`internal/network`, `internal/ui`)**：
  - 验证 `MIB_IF_ROW2`（1352 字节）、`PROCESSENTRY32W`（568 字节）、`NOTIFYICONDATAW`（976 字节）、`PAINTSTRUCT`（72 字节）、`TTTOOLINFOW`（72 字节）结构对齐。
  - 通过 GDI `GetIconInfo` + `GetDIBits` 像素回读验证 32×32 动态托盘图标方向无倒置。

## 2. 本机真实环境冒烟与资源实测

执行 `./dist/PulseBar-windows-amd64.exe --run-for=6s --output .artifacts/smoke.json` 实测结果（公网 IP 已脱敏）：

- **网络感知**：成功识别活跃出口网卡 `Nano` 与 `tun_or_proxy_active=true`，ICMP 探针确认在线，Cloudflare 出口定位成功解析国家码 `US`。
- **期货行情**：通过 WinHTTP 成功读取 `CME_MINI:ES1!` 与 `CME_MINI:NQ1!`（`delayed_streaming_600`），两项均通过字段与涨跌幅一致性校验，HTTP RTT 约 258 ms。

| 指标 | 实测数值 |
| :--- | ---: |
| `PulseBar-windows-amd64.exe` 体积 | 3,356,672 字节（约 3.20 MiB） |
| `PulseBar-windows-arm64.exe` 体积 | 3,114,496 字节（约 2.97 MiB） |
| GUI 运行期私有工作集 (`private_working_set_bytes`) | 4,780,032 字节（约 **4.56 MiB**） |
| GUI 运行期总工作集 (`working_set_bytes`，含系统共享 DLL) | 20,922,368 字节（约 **19.95 MiB**） |
| 峰值总工作集 (`peak_working_set_bytes`) | 21,762,048 字节（约 **20.75 MiB**） |
| 私有提交内存 (`private_bytes`) | 16,662,528 字节（约 **15.89 MiB**） |
| GDI / USER 对象数 | 17 / 11 |

## 3. 复现命令

```powershell
./scripts/verify.ps1
./scripts/package.ps1 -Version v0.1.0
Start-Process ./dist/PulseBar-windows-amd64.exe -ArgumentList '--run-for=10s --output .artifacts/smoke.json' -Wait
```
