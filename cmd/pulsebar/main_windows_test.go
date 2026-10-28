//go:build windows && (amd64 || arm64)

package main

import "testing"

func TestLatestCustomSelectionReplacesQueuedSelection(t *testing.T) {
	ch := make(chan string, 1)
	sendLatestGroup(ch, "old")
	sendLatestGroup(ch, "custom_600519_000858")
	sendLatestGroup(ch, "voo_qqq")
	if got := <-ch; got != "voo_qqq" {
		t.Fatalf("stale queued selection: %s", got)
	}
}
