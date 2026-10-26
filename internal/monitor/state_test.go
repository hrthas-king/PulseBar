package monitor

import (
	"testing"
	"time"
)

func TestTrackerStateTransitions(t *testing.T) {
	var tr Tracker
	tr.Reset()
	if tr.Status != Unknown || tr.Failures != 0 {
		t.Fatalf("expected Unknown/0, got %s/%d", tr.Status, tr.Failures)
	}
	// First failure from Unknown -> Degraded
	if tr.Apply(Probe{Success: false}) {
		t.Fatal("failure should not report recovery")
	}
	if tr.Status != Degraded || tr.Failures != 1 {
		t.Fatalf("expected Degraded/1, got %s/%d", tr.Status, tr.Failures)
	}
	// Second & third consecutive failures -> stay Degraded
	tr.Apply(Probe{Success: false})
	tr.Apply(Probe{Success: false})
	if tr.Status != Degraded || tr.Failures != 3 {
		t.Fatalf("expected Degraded/3, got %s/%d", tr.Status, tr.Failures)
	}
	// Fourth consecutive failure -> Offline
	tr.Apply(Probe{Success: false})
	if tr.Status != Offline || tr.Failures != 4 {
		t.Fatalf("expected Offline/4, got %s/%d", tr.Status, tr.Failures)
	}
	// Single success -> Online & reports recovery
	if !tr.Apply(Probe{Success: true, Uplink: UplinkConfirmed, RTT: 12 * time.Millisecond}) {
		t.Fatal("expected recovery on first success after Offline")
	}
	if tr.Status != Online || tr.Failures != 0 {
		t.Fatalf("expected Online/0, got %s/%d", tr.Status, tr.Failures)
	}
	// Single transient failure while already Online -> stays Online (anti-jitter)
	tr.Apply(Probe{Success: false})
	if tr.Status != Online || tr.Failures != 1 {
		t.Fatalf("expected transient failure from Online to stay Online/1, got %s/%d", tr.Status, tr.Failures)
	}
}

func TestSamplerCounterValidation(t *testing.T) {
	var s Sampler
	now := time.Now()
	r0 := s.Sample(Counters{LUID: 10, Index: 1, Sent: 1000, Received: 2000, At: now})
	if r0.Valid {
		t.Fatal("first sample must only establish baseline")
	}
	r1 := s.Sample(Counters{LUID: 10, Index: 1, Sent: 2024, Received: 6096, At: now.Add(time.Second)})
	if !r1.Valid || r1.Up != 1024 || r1.Down != 4096 {
		t.Fatalf("unexpected rate: %+v", r1)
	}
	// Counter rollback or LUID switch must reset baseline
	rRollback := s.Sample(Counters{LUID: 10, Index: 1, Sent: 500, Received: 6096, At: now.Add(2 * time.Second)})
	if rRollback.Valid {
		t.Fatal("counter rollback must invalidate rate sample")
	}
	rSwitch := s.Sample(Counters{LUID: 99, Index: 2, Sent: 1000, Received: 8000, At: now.Add(3 * time.Second)})
	if rSwitch.Valid {
		t.Fatal("interface LUID switch must invalidate rate sample")
	}
}
