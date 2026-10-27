//go:build windows && (amd64 || arm64)

package network

import (
	"context"
	"testing"
	"time"
)

func TestHTTPSLatencyCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := MeasureHTTPSLatency(ctx)
	if result.Error != context.Canceled.Error() || result.RTT != 0 || result.Valid(time.Now()) {
		t.Fatalf("canceled request produced a usable latency: %+v", result)
	}
}
