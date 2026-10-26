//go:build windows && (amd64 || arm64)

package market

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/hrthas-king/PulseBar/internal/winapi"
)

var (
	winhttp         = winapi.DLL("winhttp.dll")
	open            = winhttp.NewProc("WinHttpOpen")
	connect         = winhttp.NewProc("WinHttpConnect")
	openRequest     = winhttp.NewProc("WinHttpOpenRequest")
	setTimeouts     = winhttp.NewProc("WinHttpSetTimeouts")
	setOption       = winhttp.NewProc("WinHttpSetOption")
	sendRequest     = winhttp.NewProc("WinHttpSendRequest")
	receiveResponse = winhttp.NewProc("WinHttpReceiveResponse")
	queryHeaders    = winhttp.NewProc("WinHttpQueryHeaders")
	readData        = winhttp.NewProc("WinHttpReadData")
	closeHandle     = winhttp.NewProc("WinHttpCloseHandle")
)

const requestHeaders = "Accept: application/json\r\n" +
	"Accept-Encoding: identity\r\n" +
	"Cache-Control: no-cache\r\n" +
	"Pragma: no-cache\r\n"

// Fetch 通过 WinHTTP 获取 VOO 与 QQQ 的 24 小时全时段（夜盘/盘前/盘中/盘后）实时行情。
func Fetch(ctx context.Context) (map[string]SymbolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	agent := winapi.UTF16("Mozilla/5.0 (Windows NT 10.0; Win64; x64) PulseBar/1.1")
	session, _, err := open.Call(uintptr(unsafe.Pointer(agent)), 4, 0, 0, 0)
	if session == 0 {
		return nil, winapi.Error("WinHttpOpen", err)
	}
	defer closeHandle.Call(session)

	if ok, _, err := setTimeouts.Call(session, 800, TimeoutMillis, TimeoutMillis, TimeoutMillis); ok == 0 {
		return nil, winapi.Error("WinHttpSetTimeouts", err)
	}

	// 优先复用同一个主机连接句柄，减少第二次请求的 TLS 握手开销
	primaryPtr := winapi.UTF16(PrimaryHost)
	primaryConn, _, _ := connect.Call(session, uintptr(unsafe.Pointer(primaryPtr)), 443, 0)
	if primaryConn != 0 {
		defer closeHandle.Call(primaryConn)
	}

	results := make(map[string]SymbolResult, len(Symbols))
	var anySuccess bool
	var lastErr error

	for _, sym := range Symbols {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		q, fetchErr := fetchSymbolWithFallback(ctx, session, primaryConn, sym)
		if fetchErr != nil {
			lastErr = fetchErr
			results[sym] = SymbolResult{Err: fmt.Sprintf("行情读取失败: %v", fetchErr)}
		} else {
			anySuccess = true
			qCopy := q
			results[sym] = SymbolResult{Quote: &qCopy}
		}
	}
	runtime.KeepAlive(agent)
	runtime.KeepAlive(primaryPtr)
	if !anySuccess && lastErr != nil {
		return nil, lastErr
	}
	return results, nil
}

func fetchSymbolWithFallback(ctx context.Context, session, primaryConn uintptr, sym string) (Quote, error) {
	if primaryConn != 0 && ctx.Err() == nil {
		start := time.Now()
		if body, err := getChartJSONOnConn(ctx, primaryConn, sym); err == nil {
			rttMs := float64(time.Since(start).Microseconds()) / 1000.0
			if q, parseErr := ParseYahooChart(sym, body, rttMs, time.Now()); parseErr == nil {
				return q, nil
			}
		}
	}
	var lastErr error
	for _, host := range []string{PrimaryHost, FallbackHost} {
		if err := ctx.Err(); err != nil {
			return Quote{}, err
		}
		start := time.Now()
		body, err := getChartJSON(ctx, session, host, sym)
		if err == nil {
			rttMs := float64(time.Since(start).Microseconds()) / 1000.0
			q, parseErr := ParseYahooChart(sym, body, rttMs, time.Now())
			if parseErr == nil {
				return q, nil
			}
			err = parseErr
		}
		lastErr = err
	}
	return Quote{}, lastErr
}

func getChartJSON(ctx context.Context, session uintptr, host, sym string) ([]byte, error) {
	hostPtr := winapi.UTF16(host)
	conn, _, err := connect.Call(session, uintptr(unsafe.Pointer(hostPtr)), 443, 0)
	if conn == 0 {
		return nil, winapi.Error("WinHttpConnect", err)
	}
	defer closeHandle.Call(conn)
	body, err := getChartJSONOnConn(ctx, conn, sym)
	runtime.KeepAlive(hostPtr)
	return body, err
}

func getChartJSONOnConn(ctx context.Context, conn uintptr, sym string) ([]byte, error) {

	verb := winapi.UTF16("GET")
	path := winapi.UTF16("/v8/finance/chart/" + sym + "?interval=1d&range=1d&includePrePost=true")
	request, _, err := openRequest.Call(conn, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(path)), 0, 0, 0, 0x00800000)
	if request == 0 {
		return nil, winapi.Error("WinHttpOpenRequest", err)
	}
	defer closeHandle.Call(request)

	// 禁用重定向 (2) 与 Cookie (1)
	var disable uint32 = 2 | 1
	if ok, _, err := setOption.Call(request, 63, uintptr(unsafe.Pointer(&disable)), 4); ok == 0 {
		return nil, winapi.Error("WinHttpSetOption", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hdrPtr := winapi.UTF16(requestHeaders)
	if ok, _, err := sendRequest.Call(request, uintptr(unsafe.Pointer(hdrPtr)), ^uintptr(0), 0, 0, 0, 0); ok == 0 {
		return nil, winapi.Error("WinHttpSendRequest", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ok, _, err := receiveResponse.Call(request, 0); ok == 0 {
		return nil, winapi.Error("WinHttpReceiveResponse", err)
	}
	var status uint32
	size := uint32(4)
	if ok, _, err := queryHeaders.Call(request, 19|0x20000000, 0, uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&size)), 0); ok == 0 {
		return nil, winapi.Error("WinHttpQueryHeaders", err)
	}
	if status != 200 {
		return nil, HTTPError{StatusCode: int(status)}
	}
	var encBuf [64]uint16
	encSize := uint32(unsafe.Sizeof(encBuf))
	if ok, _, _ := queryHeaders.Call(request, 29, 0, uintptr(unsafe.Pointer(&encBuf[0])), uintptr(unsafe.Pointer(&encSize)), 0); ok != 0 {
		enc := strings.ToLower(strings.TrimSpace(utf16ToString(encBuf[:])))
		if enc != "" && enc != "identity" {
			return nil, fmt.Errorf("拒绝压缩响应 (%s)", enc)
		}
	}

	body := make([]byte, 0, 1536)
	var chunk [1024]byte
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var n uint32
		ok, _, err := readData.Call(request, uintptr(unsafe.Pointer(&chunk[0])), uintptr(len(chunk)), uintptr(unsafe.Pointer(&n)))
		if ok == 0 {
			return nil, winapi.Error("WinHttpReadData", err)
		}
		if n == 0 {
			break
		}
		if n > uint32(len(chunk)) || len(body)+int(n) > MaxBody {
			return nil, fmt.Errorf("行情响应超过 %d 字节上限", MaxBody)
		}
		body = append(body, chunk[:n]...)
	}
	runtime.KeepAlive(verb)
	runtime.KeepAlive(path)
	runtime.KeepAlive(hdrPtr)
	return body, nil
}

func utf16ToString(u []uint16) string {
	for i, v := range u {
		if v == 0 {
			u = u[:i]
			break
		}
	}
	var sb strings.Builder
	for _, r := range u {
		sb.WriteRune(rune(r))
	}
	return sb.String()
}
