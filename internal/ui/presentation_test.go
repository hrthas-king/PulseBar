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
	voo := market.Quote{Symbol: "VOO", Price: 699.05, ChangePercent: 0.07, ChangeAbs: 0.47, RegularPrice: 698.58, TradeTime: 1792997930, QuoteSource: "BOATS Real Time Price", UpdateMode: "overnight_boats_realtime", Session: "夜盘 (Overnight BOATS)", Description: "Vanguard S&P 500 ETF"}
	qqq := market.Quote{Symbol: "QQQ", Price: 703.55, ChangePercent: -0.12, ChangeAbs: -0.85, RegularPrice: 704.40, TradeTime: 1792997935, QuoteSource: "BOATS Real Time Price", UpdateMode: "overnight_boats_realtime", Session: "夜盘 (Overnight BOATS)", Description: "Invesco QQQ Trust"}
	age := 1.2
	snap := app.Snapshot{
		Status: monitor.Online,
		Geo:    ipinfo.Info{IP: "198.51.100.99", Country: "SG"},
		Rates:  monitor.Rates{Up: 12800, Down: 1572864, Valid: true},
		Probe:  monitor.Probe{Success: true, Uplink: monitor.UplinkConfirmed, RTT: 18 * time.Millisecond},
		NIC:    "WLAN",
		Quotes: []market.View{
			{Symbol: "VOO", Quote: &voo, Healthy: true, ReceivedAge: &age},
			{Symbol: "QQQ", Quote: &qqq, Healthy: true, ReceivedAge: &age},
		},
	}

	trayTip := TrayTooltip(snap)
	if !strings.Contains(trayTip, "PulseBar [SG] 正常") || !strings.Contains(trayTip, "VOO 699.05 +0.07%") || !strings.Contains(trayTip, "QQQ 703.55 -0.12%") {
		t.Fatalf("unexpected tray tooltip:\n%s", trayTip)
	}

	overlayTip := OverlayTooltip(snap)
	if !strings.Contains(overlayTip, "198.51.100.99") || !strings.Contains(overlayTip, "NYSEArca:VOO") || !strings.Contains(overlayTip, "BOATS Real Time Price") {
		t.Fatalf("unexpected overlay tooltip:\n%s", overlayTip)
	}

	diag := DiagnosticsText("v0.2.1", snap)
	if !strings.Contains(diag, "GeoPulse") || !strings.Contains(diag, "VOO") || !strings.Contains(diag, "BOATS Real Time Price") {
		t.Fatalf("unexpected diagnostics text:\n%s", diag)
	}
}
