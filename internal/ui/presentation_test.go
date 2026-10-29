package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/hrthas-king/Samuel/internal/app"
	"github.com/hrthas-king/Samuel/internal/ipinfo"
	"github.com/hrthas-king/Samuel/internal/market"
	"github.com/hrthas-king/Samuel/internal/monitor"
)

func TestTooltipsAndFormatting(t *testing.T) {
	voo := market.Quote{Symbol: "VOO", Alias: "VOO", State: market.StateOvernight, Price: 699.05, ChangePercent: 0.07, ChangeAbs: 0.47, RegularPrice: 698.58, TradeTime: 1792997930, QuoteSource: "BOATS Real Time Price", UpdateMode: "overnight_boats_realtime", Session: "夜盘 (Overnight BOATS)", Description: "Vanguard S&P 500 ETF"}
	qqq := market.Quote{Symbol: "QQQ", Alias: "QQQ", State: market.StateOvernight, Price: 703.55, ChangePercent: -0.12, ChangeAbs: -0.85, RegularPrice: 704.40, TradeTime: 1792997935, QuoteSource: "BOATS Real Time Price", UpdateMode: "overnight_boats_realtime", Session: "夜盘 (Overnight BOATS)", Description: "Invesco QQQ Trust"}
	age := 1.2
	snap := app.Snapshot{
		Status:      monitor.Online,
		MarketGroup: market.GroupVOOQQQ,
		Geo:         ipinfo.Info{IP: "198.51.100.99", Country: "SG"},
		Rates:       monitor.Rates{Up: 12800, Down: 1572864, Valid: true},
		Probe:       monitor.Probe{Success: true, Uplink: monitor.UplinkConfirmed, RTT: 18 * time.Millisecond},
		Latency:     monitor.Latency{Method: "https", Target: "https://www.cloudflare.com/cdn-cgi/trace", RTT: 18 * time.Millisecond, At: time.Now()},
		NIC:         "WLAN",
		Quotes: []market.View{
			{Symbol: "VOO", Alias: "VOO", State: market.StateOvernight, Quote: &voo, Healthy: true, ReceivedAge: &age},
			{Symbol: "QQQ", Alias: "QQQ", State: market.StateOvernight, Quote: &qqq, Healthy: true, ReceivedAge: &age},
		},
	}

	trayTip := TrayTooltip(snap)
	if !strings.Contains(trayTip, "Samuel [SG] 正常") || !strings.Contains(trayTip, "VOO 699.05 +0.07%") || !strings.Contains(trayTip, "QQQ 703.55 -0.12%") {
		t.Fatalf("unexpected tray tooltip:\n%s", trayTip)
	}

	overlayTip := OverlayTooltip(snap)
	if !strings.Contains(overlayTip, "198.51.100.99") || !strings.Contains(overlayTip, "NYSEArca:VOO") || !strings.Contains(overlayTip, "BOATS Real Time Price") || !strings.Contains(overlayTip, "🌙") {
		t.Fatalf("unexpected overlay tooltip:\n%s", overlayTip)
	}

	diag := DiagnosticsText("v0.3.0", snap)
	if !strings.Contains(diag, "GeoPulse") || !strings.Contains(diag, "VOO") || !strings.Contains(diag, "BOATS Real Time Price") {
		t.Fatalf("unexpected diagnostics text:\n%s", diag)
	}

	// Verify alias summary for Chinese/HK indices and unhealthy '!' marker
	hsi := market.Quote{Symbol: "^HSI", Alias: "恒指", State: market.StatePreMarket, Price: 23211.81, ChangePercent: 0.51}
	hstech := market.Quote{Symbol: "^HSTECH", Alias: "恒科", State: market.StatePreMarket, Price: 3857.53, ChangePercent: -0.36}
	hkSnap := app.Snapshot{
		MarketGroup: market.GroupHKIndices,
		Quotes: []market.View{
			{Symbol: "^HSI", Alias: "恒指", State: market.StatePreMarket, Quote: &hsi, Healthy: true},
			{Symbol: "^HSTECH", Alias: "恒科", State: market.StatePreMarket, Quote: &hstech, Healthy: false, Reason: "连接超时"},
		},
	}
	summary := QuoteSummary(hkSnap)
	if summary != "恒指 23211.81 +0.51% | 恒科 3857.53 -0.36%!" {
		t.Fatalf("unexpected HK QuoteSummary: %q", summary)
	}
}

func TestLatencyDoesNotUseTunnelHeartbeat(t *testing.T) {
	now := time.Now()
	measured := monitor.Latency{
		RTT: 245 * time.Millisecond, Target: "https://www.cloudflare.com/cdn-cgi/trace",
		Method: "https", At: now,
	}
	for _, tc := range []struct {
		name    string
		latency monitor.Latency
		want    string
	}{
		{"measured_https", measured, "245ms"},
		{"no_measurement", monitor.Latency{}, "--"},
		{"request_failed", monitor.Latency{Method: "https", Target: measured.Target, At: now, Error: "timeout"}, "--"},
		{"expired", monitor.Latency{Method: "https", Target: measured.Target, At: now.Add(-monitor.LatencyMaxAge - time.Second), RTT: measured.RTT}, "--"},
		{"tcp_is_not_https", monitor.Latency{Method: "tcp", Target: measured.Target, At: now, RTT: time.Millisecond}, "--"},
		{"sub_millisecond_precision", monitor.Latency{Method: "https", Target: measured.Target, At: now, RTT: 700 * time.Microsecond}, "<1ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := app.Snapshot{
				Status: monitor.Online, EgressActive: true, Geo: ipinfo.Info{Country: "KR"},
				Probe:   monitor.Probe{Success: true, Method: "icmp", RTT: time.Millisecond},
				Latency: tc.latency,
			}
			if got := CompactRTT(s); got != tc.want {
				t.Fatalf("KR/TUN heartbeat=1ms, HTTPS=%+v: displayed %q; want %q", tc.latency, got, tc.want)
			}
		})
	}
}

func TestShenzhenETFsDisplayThreeDecimalPrices(t *testing.T) {
	first := market.Quote{Symbol: "159632.SZ", Alias: "纳指ETF", Description: "纳斯达克ETF华安 (QDII)", Price: 1.234, RegularPrice: 1.224, ChangeAbs: 0.010, ChangePercent: 0.82}
	second := market.Quote{Symbol: "159655.SZ", Alias: "标普500ETF", Description: "标普500ETF华夏 (QDII)", Price: 1.876, RegularPrice: 1.877, ChangeAbs: -0.001, ChangePercent: -0.05}
	views := []market.View{
		{Symbol: first.Symbol, Alias: first.Alias, Quote: &first, Healthy: true},
		{Symbol: second.Symbol, Alias: second.Alias, Quote: &second, Healthy: true},
	}
	snap := app.Snapshot{MarketGroup: "etf_159632_159655", Quotes: views}
	if got := quotePriceChangeText(first); got != "1.234 +0.82%" {
		t.Fatalf("taskbar ETF price=%q", got)
	}
	if got := QuoteSummary(snap); got != "纳指ETF 1.234 +0.82% | 标普500ETF 1.876 -0.05%" {
		t.Fatalf("tray ETF summary=%q", got)
	}
	diag := DiagnosticsText("test", snap)
	overlay := OverlayTooltip(snap)
	for _, expected := range []string{
		"当前最新价: 1.234 (常规盘收盘: 1.224) | 涨跌额: +0.010",
		"当前最新价: 1.876 (常规盘收盘: 1.877) | 涨跌额: -0.001",
		"参考基准价: 1.224",
		"参考基准价: 1.877",
		"纳指ETF",
		"标普500ETF",
		"纳斯达克ETF华安 (QDII)",
		"标普500ETF华夏 (QDII)",
	} {
		if !strings.Contains(diag, expected) {
			t.Errorf("diagnostics missing %q:\n%s", expected, diag)
		}
	}
	for _, shown := range []string{diag, overlay, TrayTooltip(snap)} {
		for _, code := range []string{"159632", "159655"} {
			if strings.Contains(shown, code) {
				t.Errorf("user-facing presentation exposed %s:\n%s", code, shown)
			}
		}
	}
	if !strings.Contains(overlay, "纳指ETF") || !strings.Contains(overlay, "标普500ETF") {
		t.Fatalf("overlay missing ETF names:\n%s", overlay)
	}
	// Before the first response, the two pending quote labels must also be names.
	pending := app.Snapshot{MarketGroup: market.GroupETF159632159655}
	if got := QuoteSummary(pending); got != "纳指ETF -- | 标普500ETF --" {
		t.Fatalf("pending quote summary=%q", got)
	}
	for _, sym := range []string{first.Symbol, second.Symbol} {
		pendingView := market.View{Symbol: sym, Healthy: false, Reason: "等待行情"}
		text := pendingView.Tooltip()
		if strings.Contains(text, "159632") || strings.Contains(text, "159655") {
			t.Errorf("pending tooltip exposed ETF code: %s", text)
		}
	}
	if got := quotePriceChangeText(market.Quote{Symbol: "VOO", Price: 699.051, ChangePercent: 0.07}); got != "699.05 +0.07%" {
		t.Fatalf("existing ETF precision changed: %q", got)
	}
}
