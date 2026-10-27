package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/hrthas-king/PulseBar/internal/app"
	"github.com/hrthas-king/PulseBar/internal/ipinfo"
	"github.com/hrthas-king/PulseBar/internal/market"
	"github.com/hrthas-king/PulseBar/internal/monitor"
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
	if !strings.Contains(trayTip, "PulseBar [SG] 正常") || !strings.Contains(trayTip, "VOO 699.05 +0.07%") || !strings.Contains(trayTip, "QQQ 703.55 -0.12%") {
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
