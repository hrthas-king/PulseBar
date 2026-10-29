//go:build windows && (amd64 || arm64)

package network

import (
	"context"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/hrthas-king/Samuel/internal/monitor"
	"github.com/hrthas-king/Samuel/internal/winapi"
)

const HTTPSLatencyTarget = "https://www.cloudflare.com/cdn-cgi/trace"

var (
	latencyHTTP    = winapi.DLL("winhttp.dll")
	latencyOpen    = latencyHTTP.NewProc("WinHttpOpen")
	latencyConnect = latencyHTTP.NewProc("WinHttpConnect")
	latencyRequest = latencyHTTP.NewProc("WinHttpOpenRequest")
	latencyTimeout = latencyHTTP.NewProc("WinHttpSetTimeouts")
	latencyOption  = latencyHTTP.NewProc("WinHttpSetOption")
	latencySend    = latencyHTTP.NewProc("WinHttpSendRequest")
	latencyReceive = latencyHTTP.NewProc("WinHttpReceiveResponse")
	latencyHeaders = latencyHTTP.NewProc("WinHttpQueryHeaders")
	latencyClose   = latencyHTTP.NewProc("WinHttpCloseHandle")
)

// MeasureHTTPSLatency measures a fresh HTTPS GET request through the system proxy/TUN.
// Timing includes DNS, connection establishment, TLS and receipt of response headers.
// The trace endpoint has a small body; fresh connections avoid retaining a previous proxy exit.
func MeasureHTTPSLatency(ctx context.Context) (result monitor.Latency) {
	result.Method = "https"
	result.Target = HTTPSLatencyTarget
	defer func() { result.At = time.Now() }()
	rtt, err := measureHTTPSLatency(ctx)
	if err != nil {
		result.Error = err.Error()
	} else {
		result.RTT = rtt
	}
	return result
}

func measureHTTPSLatency(ctx context.Context) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	agent := winapi.UTF16("Samuel HTTPS latency")
	session, _, err := latencyOpen.Call(uintptr(unsafe.Pointer(agent)), 4, 0, 0, 0)
	if session == 0 {
		return 0, winapi.Error("WinHttpOpen", err)
	}
	defer latencyClose.Call(session)
	if ok, _, err := latencyTimeout.Call(session, 800, 1200, 1200, 1200); ok == 0 {
		return 0, winapi.Error("WinHttpSetTimeouts", err)
	}
	host := winapi.UTF16("www.cloudflare.com")
	conn, _, err := latencyConnect.Call(session, uintptr(unsafe.Pointer(host)), 443, 0)
	if conn == 0 {
		return 0, winapi.Error("WinHttpConnect", err)
	}
	defer latencyClose.Call(conn)
	verb, path := winapi.UTF16("GET"), winapi.UTF16("/cdn-cgi/trace")
	request, _, err := latencyRequest.Call(conn, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(path)), 0, 0, 0, 0x00800000)
	if request == 0 {
		return 0, winapi.Error("WinHttpOpenRequest", err)
	}
	defer latencyClose.Call(request)
	// Disable cookies, redirects and keep-alive; retain normal TLS certificate validation.
	var disable uint32 = 1 | 2 | 8
	if ok, _, err := latencyOption.Call(request, 63, uintptr(unsafe.Pointer(&disable)), 4); ok == 0 {
		return 0, winapi.Error("WinHttpSetOption", err)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	start := time.Now()
	if ok, _, err := latencySend.Call(request, 0, 0, 0, 0, 0, 0); ok == 0 {
		return 0, winapi.Error("WinHttpSendRequest", err)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if ok, _, err := latencyReceive.Call(request, 0); ok == 0 {
		return 0, winapi.Error("WinHttpReceiveResponse", err)
	}
	elapsed := time.Since(start)
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var status uint32
	size := uint32(4)
	if ok, _, err := latencyHeaders.Call(request, 19|0x20000000, 0, uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&size)), 0); ok == 0 {
		return 0, winapi.Error("WinHttpQueryHeaders", err)
	}
	if status != 200 {
		return 0, fmt.Errorf("HTTPS 延迟探针返回 HTTP %d", status)
	}
	runtime.KeepAlive(agent)
	runtime.KeepAlive(host)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(path)
	return elapsed, nil
}
