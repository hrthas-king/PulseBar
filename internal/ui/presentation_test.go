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
	voo := market.Quote{Symbol: "VOO", Price: 699.16, ChangePercent: -0.45, ChangeAbs: -3.14, UpdateMode: "24h_fullday_realtime", Session: "夜盘 (Overnight)", Description: "Vanguard S&P 500 ETF"}
	qqq := market.Quote{Symbol: "QQQ", Price: 703.68, ChangePercent: -0.11, ChangeAbs: -0.79, UpdateMode: "24h_fullday_realtime", Session: "夜盘 (Overnight)", Description: "Invesco QQQ Trust"}
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
	if !strings.Contains(trayTip, "PulseBar [SG] 正常") || !strings.Contains(trayTip, "VOO -0.45%") || !strings.Contains(trayTip, "QQQ -0.11%") {
		t.Fatalf("unexpected tray tooltip:\n%s", trayTip)
	}

	overlayTip := OverlayTooltip(snap)
	if !strings.Contains(overlayTip, "198.51.100.99") || !strings.Contains(overlayTip, "NYSEArca:VOO") || !strings.Contains(overlayTip, "NASDAQ:QQQ") {
		t.Fatalf("unexpected overlay tooltip:\n%s", overlayTip)
	}

	diag := DiagnosticsText("v0.2.0", snap)
	if !strings.Contains(diag, "GeoPulse") || !strings.Contains(diag, "VOO") || !strings.Contains(diag, "QQQ") {
		t.Fatalf("unexpected diagnostics text:\n%s", diag)
	}
}
