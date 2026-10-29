package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hrthas-king/Samuel/internal/ipinfo"
	"github.com/hrthas-king/Samuel/internal/market"
	"github.com/hrthas-king/Samuel/internal/monitor"
)

func TestEngineGeoAndQuotesAggregation(t *testing.T) {
	defer market.SetActiveGroup(market.DefaultGroup)
	market.SetActiveGroup(market.GroupVOOQQQ)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
			syms := market.ActiveSymbols()
			out := make(map[string]market.SymbolResult, len(syms))
			for _, sym := range syms {
				q := market.Quote{
					Symbol:        sym,
					Alias:         market.AliasFor(sym),
					Price:         699.16,
					ChangePercent: 0.45,
					ChangeAbs:     3.14,
					State:         market.StateRegular,
					UpdateMode:    "regular_realtime",
					Session:       "盘中 (Regular)",
					Description:   sym,
				}
				out[sym] = market.SymbolResult{Quote: &q}
			}
			return out, nil
		},
		NetworkEvents: netEvents,
		EgressEvents:  egressEvents,
	})

	ready := make(chan Snapshot, 32)
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

	// Switch market group dynamically to A-share indices
	engine.Group <- market.GroupCNIndices
	switchDeadline := time.After(1200 * time.Millisecond)
	var gotCNGroup bool
	for !gotCNGroup {
		select {
		case s := <-ready:
			if s.MarketGroup == market.GroupCNIndices && len(s.Quotes) == 2 && s.Quotes[0].Alias == "上证" && s.Quotes[1].Alias == "创指" && s.Quotes[0].Healthy {
				gotCNGroup = true
			}
		case <-switchDeadline:
			t.Fatalf("timed out waiting for CN group switch")
		}
	}
}
