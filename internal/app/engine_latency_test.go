//go:build windows && (amd64 || arm64)

package app_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hrthas-king/PulseBar/internal/app"
	"github.com/hrthas-king/PulseBar/internal/ipinfo"
	"github.com/hrthas-king/PulseBar/internal/monitor"
	"github.com/hrthas-king/PulseBar/internal/ui"
)

func TestOnlineEvidenceDoesNotInventLatency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	geoReady := make(chan struct{})
	var readyOnce sync.Once
	probeAt := time.Now()
	var probeCalls atomic.Int32
	engine := app.New(app.Dependencies{
		ReadCounters: func(string) (monitor.Counters, error) {
			return monitor.Counters{LUID: 1, Index: 1, At: time.Now()}, nil
		},
		Ping: func(target string) monitor.Probe {
			select {
			case <-geoReady:
			case <-ctx.Done():
			}
			if probeCalls.Add(1) == 1 {
				return monitor.Probe{Success: true, Method: "icmp", Target: target, Uplink: monitor.UplinkConfirmed, RTT: time.Millisecond, At: probeAt.Add(-time.Millisecond)}
			}
			return monitor.Probe{Target: target, Uplink: monitor.UplinkUnknown, At: probeAt}
		},
		FetchGeo: func(context.Context) (ipinfo.Info, error) {
			return ipinfo.Info{IP: "198.51.100.8", Country: "KR", At: time.Now()}, nil
		},
	})
	result := make(chan app.Snapshot, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Run(ctx, 1, func(s app.Snapshot) {
			if s.Geo.Country == "KR" {
				readyOnce.Do(func() { close(geoReady) })
			}
			if s.Status == monitor.Online && s.Probe.At.Equal(probeAt) {
				select {
				case result <- s:
				default:
				}
			}
		})
	}()
	defer func() { cancel(); <-done }()
	select {
	case s := <-result:
		if s.Probe.RTT != 0 {
			t.Errorf("failed probe retained old heartbeat RTT: %v", s.Probe.RTT)
		}
		if got := ui.CompactRTT(s); got != "--" {
			t.Fatalf("failed probe with only recent HTTPS connectivity evidence displays invented latency %q; want --", got)
		}
	case <-ctx.Done():
		t.Fatal("engine did not retain online status after a failed probe with recent HTTPS evidence")
	}
}

func TestLatencyDropsPreviousEgressMeasurement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	egress := make(chan monitor.EgressEvent, 1)
	firstStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	var releaseOnce sync.Once
	var calls atomic.Int32
	engine := app.New(app.Dependencies{
		ReadCounters: func(string) (monitor.Counters, error) {
			return monitor.Counters{LUID: 1, Index: 1, At: time.Now()}, nil
		},
		Ping: func(target string) monitor.Probe {
			return monitor.Probe{Success: true, Method: "icmp", RTT: time.Millisecond, At: time.Now(), Target: target}
		},
		FetchGeo: func(context.Context) (ipinfo.Info, error) {
			return ipinfo.Info{IP: "198.51.100.8", Country: "KR", At: time.Now()}, nil
		},
		MeasureLatency: func(context.Context) monitor.Latency {
			if calls.Add(1) == 1 {
				close(firstStarted)
				<-releaseOld // Emulate a native request completing after cancellation.
				return monitor.Latency{Method: "https", Target: "old-exit", RTT: time.Millisecond, At: time.Now()}
			}
			return monitor.Latency{Method: "https", Target: "new-exit", RTT: 245 * time.Millisecond, At: time.Now()}
		},
		EgressEvents: egress,
	})
	snapshots := make(chan app.Snapshot, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Run(ctx, 1, func(s app.Snapshot) { snapshots <- s })
	}()
	defer func() { releaseOnce.Do(func() { close(releaseOld) }); cancel(); <-done }()
	select {
	case <-firstStarted:
	case <-ctx.Done():
		t.Fatal("first latency request did not start")
	}
	egress <- monitor.EgressEvent{Active: true, Changed: true}
	for {
		select {
		case s := <-snapshots:
			if s.Latency.Target == "new-exit" {
				released := time.Now()
				releaseOnce.Do(func() { close(releaseOld) })
				for {
					select {
					case after := <-snapshots:
						if after.At.After(released.Add(100 * time.Millisecond)) {
							if after.Latency.Target != "new-exit" || ui.CompactRTT(after) != "245ms" {
								t.Fatalf("old egress overwrote fresh HTTPS latency: %+v", after.Latency)
							}
							return
						}
					case <-ctx.Done():
						t.Fatal("no snapshot after the old request completed")
					}
				}
			}
		case <-ctx.Done():
			t.Fatal("new egress latency did not arrive while the old request was pending")
		}
	}
}
