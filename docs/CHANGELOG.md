# 更新记录 (Changelog)

## v0.2.1 (2026-10-26)

- **修复夜盘/扩展时段实时性与涨跌幅基准，并同屏展示实时价格**：
  - 切换主行情源至带 `overnightPrice=true` 的 `/v7/finance/quote` 接口（WinHTTP 原生会话自动维护 Cookie 与 Crumb），直接获取 **Blue Ocean ATS (`BOATS Real Time Price`)** 的 `overnightMarketPrice`、`overnightMarketChange`、`overnightMarketChangePercent` 与 `overnightMarketTime` 实时成交时间戳。
  - 修复此前 `/v8/finance/chart` 在夜盘（`20:00–04:00 ET`）期间沿用两日前收盘价（`chartPreviousClose`）计算 `fulldayChangePercent` 导致涨跌幅失真的问题；现严格以最近一个常规交易日收盘价为基准计算夜盘/盘前/盘后涨跌幅。
  - 任务栏悬浮条直接同屏展示 **`实时成交价 + 涨跌幅`**（如 `VOO 699.48 +0.13% | QQQ 704.35 -0.01%`），使每 `$0.01` 的微小实时跳动都能在任务栏立即显现。
  - 悬停提示与诊断面板新增展示数据源名称（如 `BOATS Real Time Price`）与最近一笔交易所成交距今秒数（`最近成交 Xs 前`）。

## v0.2.0 (2026-10-25)

- **升级追踪 `VOO` 与 `QQQ` 24×5 全时段无缝实时行情**：
  - 将行情标的从延时 10 分钟的期货切换为 **`VOO`（Vanguard S&P 500 ETF）** 与 **`QQQ`（Invesco QQQ Trust）**。
  - 采用覆盖美东时间周日 20:00 至周五 20:00 全天 24 小时（**夜盘 Overnight 20:00–04:00 + 盘前 Pre-Market 04:00–09:30 + 盘中 Regular 09:30–16:00 + 盘后 Post-Market 16:00–20:00**）的 `fulldayPrice` 实时流，实现秒级跳动且无 10/15 分钟延时。
  - 悬停 Tooltip 与诊断面板新增当前交易时段自动识别（`夜盘` / `盘前` / `盘中` / `盘后` / `周末休市`）及常规盘收盘价对比。
  - 优化任务栏悬浮条三字母代码（`VOO` / `QQQ`）排版宽度，杜绝高 DPI 下文字拥挤。

## v0.1.0 (2026-10-25)

- **全新双核整合**：将 **GeoPulse**（秒级网络连通性、上下行流量、出口 IP/国家监控）与 **TickBar** 行情条整合为单一纯 Go 原生应用 **PulseBar**。
- **任务栏悬浮条 + 系统托盘双视图**：支持紧凑模式与实时网速展开模式一键切换、位置锁定穿透、自由拖拽、停靠最左侧、配色切换与当前用户开机自启。
