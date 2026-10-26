package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hrthas-king/PulseBar/internal/ipinfo"
	"github.com/hrthas-king/PulseBar/internal/market"
	"github.com/hrthas-king/PulseBar/internal/monitor"
)

func TestEngineGeoAndQuotesAggregation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	netEvents := make(chan struct{}, 1)
	egressEvents := make(chan monitor.EgressEvent, 1)

	var geoCalls atomic.Int32
	var quoteCalls atomic.Int32

	engine := New(Dependencies{
		ReadCounters: func(target string) (monitor.Counters, error) {
			return monitor.Counters{LUID: 1, Index: 1, Name: "Ethernet", Sent: 1024, Received: 2048, At: time.Now()}, nil
		},
		Ping: func(target string) monitor.Probe {
			return monitor.Probe{Success: true, Uplink: monitor.UplinkConfirmed, RTT: 10 * time.Millisecond, Target: target, At: time.Now()}
		},
		FetchGeo: func(ctx context.Context) (ipinfo.Info, error) {
			geoCalls.Add(1)
			return ipinfo.Info{IP: "198.51.100.8", Country: "US", At: time.Now()}, nil
		},
		FetchQuotes: func(ctx context.Context) (map[string]market.SymbolResult, error) {
			quoteCalls.Add(1)
			voo := market.Quote{Symbol: "VOO", Price: 699.16, ChangePercent: -0.447, ChangeAbs: -3.14, UpdateMode: "24h_fullday_realtime", Session: "夜盘 (Overnight)", Description: "Vanguard S&P 500 ETF"}
			qqq := market.Quote{Symbol: "QQQ", Price: 703.68, ChangePercent: -0.112, ChangeAbs: -0.79, UpdateMode: "24h_fullday_realtime", Session: "夜盘 (Overnight)", Description: "Invesco QQQ Trust"}
			return map[string]market.SymbolResult{
				"VOO": {Quote: &voo},
				"QQQ": {Quote: &qqq},
			}, nil
		},
		NetworkEvents: netEvents,
		EgressEvents:  egressEvents,
	})

	ready := make(chan Snapshot, 16)
	go engine.Run(ctx, 1, func(s Snapshot) {
		select {
		case ready <- s:
		default:
		}
	})

	deadline := time.After(1500 * time.Millisecond)
	var gotOnlineAndQuotes bool
	for !gotOnlineAndQuotes {
		select {
		case s := <-ready:
			if s.Status == monitor.Online && s.Geo.Country == "US" && len(s.Quotes) == 2 && s.Quotes[0].Healthy && s.Quotes[1].Healthy {
				gotOnlineAndQuotes = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for unified snapshot (geoCalls=%d, quoteCalls=%d)", geoCalls.Load(), quoteCalls.Load())
		}
	}
}
