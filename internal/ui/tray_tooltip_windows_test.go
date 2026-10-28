//go:build windows && (amd64 || arm64)

package ui

import (
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/hrthas-king/PulseBar/internal/app"
	"github.com/hrthas-king/PulseBar/internal/ipinfo"
	"github.com/hrthas-king/PulseBar/internal/market"
	"github.com/hrthas-king/PulseBar/internal/monitor"
)

func TestTrayTooltipPreservesETFQuotesWithLongIPv6(t *testing.T) {
	first := market.Quote{Symbol: "159632.SZ", Alias: "159632", Price: 1.234, ChangePercent: 0.82}
	second := market.Quote{Symbol: "159655.SZ", Alias: "159655", Price: 1.876, ChangePercent: -0.05}
	snap := app.Snapshot{
		Status:      monitor.Online,
		MarketGroup: "etf_159632_159655",
		Geo:         ipinfo.Info{Country: "KR", IP: "2001:db8:1234:5678:9abc:def0:1234:5678"},
		Rates:       monitor.Rates{Valid: true, Up: 12800, Down: 1572864},
		Latency:     monitor.Latency{Method: "https", Target: "https://www.cloudflare.com/cdn-cgi/trace", RTT: 248 * time.Millisecond, At: time.Now()},
		Quotes: []market.View{
			{Symbol: first.Symbol, Alias: first.Alias, Quote: &first, Healthy: true},
			{Symbol: second.Symbol, Alias: second.Alias, Quote: &second, Healthy: true},
		},
	}
	tip := TrayTooltip(snap)
	var native [128]uint16
	copyText(native[:], tip)
	shown := syscall.UTF16ToString(native[:])
	for _, part := range []string{"[KR] 正常", "HTTPS: 248ms", "↑12.5K ↓1.5M", "159632 1.234 +0.82%", "159655 1.876 -0.05%", "诊断"} {
		if !strings.Contains(shown, part) {
			t.Errorf("native tray tooltip lost %q: %q (original units=%d)", part, shown, len(utf16.Encode([]rune(tip))))
		}
	}
	if got := len(utf16.Encode([]rune(tip))); got > 127 {
		t.Errorf("tray tooltip length=%d UTF-16 units, exceeds Windows limit", got)
	}
	if strings.Contains(shown, snap.Geo.IP) {
		t.Errorf("long IP was not shortened in native tray tooltip: %q", shown)
	}

	snap.GeoError = "lookup failed"
	copyText(native[:], TrayTooltip(snap))
	shown = syscall.UTF16ToString(native[:])
	if !strings.Contains(shown, "…*(见诊断)") || !strings.Contains(shown, "159655 1.876 -0.05%") {
		t.Errorf("truncated IP lost its error marker or second ETF: %q", shown)
	}
}

func TestTrayTooltipKeepsShortIPUnchanged(t *testing.T) {
	snap := app.Snapshot{Status: monitor.Online, Geo: ipinfo.Info{Country: "US", IP: "198.51.100.1"}}
	tip := TrayTooltip(snap)
	want := "PulseBar [US] 正常\nIP: 198.51.100.1 | HTTPS: --\n↑-- ↓--\nVOO -- | QQQ --"
	if tip != want {
		t.Fatalf("short tray tooltip changed: %q", tip)
	}
	var native [128]uint16
	copyText(native[:], tip)
	if got := syscall.UTF16ToString(native[:]); got != tip {
		t.Fatalf("short tray tooltip was truncated: %q != %q", got, tip)
	}
}
