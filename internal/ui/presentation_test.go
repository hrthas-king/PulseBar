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
	es := market.Quote{Symbol: "ES", Price: 5050, ChangePercent: 1.0, ChangeAbs: 50, UpdateMode: "delayed_streaming_600", Description: "E-mini S&P 500"}
	nq := market.Quote{Symbol: "NQ", Price: 19800, ChangePercent: -0.5, ChangeAbs: -100, UpdateMode: "streaming", Description: "E-mini Nasdaq 100"}
	age := 1.2
	snap := app.Snapshot{
		Status: monitor.Online,
		Geo:    ipinfo.Info{IP: "198.51.100.99", Country: "SG"},
		Rates:  monitor.Rates{Up: 12800, Down: 1572864, Valid: true},
		Probe:  monitor.Probe{Success: true, Uplink: monitor.UplinkConfirmed, RTT: 18 * time.Millisecond},
		NIC:    "WLAN",
		Quotes: []market.View{
			{Symbol: "ES", Quote: &es, Healthy: true, ReceivedAge: &age},
			{Symbol: "NQ", Quote: &nq, Healthy: true, ReceivedAge: &age},
		},
	}

	trayTip := TrayTooltip(snap)
	if !strings.Contains(trayTip, "PulseBar [SG] 正常") || !strings.Contains(trayTip, "ES +1.00%") || !strings.Contains(trayTip, "NQ -0.50%") {
		t.Fatalf("unexpected tray tooltip:\n%s", trayTip)
	}

	overlayTip := OverlayTooltip(snap)
	if !strings.Contains(overlayTip, "198.51.100.99") || !strings.Contains(overlayTip, "CME_MINI:ES1!") || !strings.Contains(overlayTip, "CME_MINI:NQ1!") {
		t.Fatalf("unexpected overlay tooltip:\n%s", overlayTip)
	}

	diag := DiagnosticsText("v0.1.0", snap)
	if !strings.Contains(diag, "GeoPulse") || !strings.Contains(diag, "TickBar") {
		t.Fatalf("unexpected diagnostics text:\n%s", diag)
	}
}
