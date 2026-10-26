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

const requestHeaders = "Content-Type: application/json\r\n" +
	"Accept: application/json\r\n" +
	"Origin: https://www.tradingview.com\r\n" +
	"Referer: https://www.tradingview.com/\r\n" +
	"Accept-Encoding: identity\r\n" +
	"Cache-Control: no-cache\r\n" +
	"Pragma: no-cache\r\n"

// Fetch 通过 WinHTTP 请求 TradingView 期货接口，沿用系统代理与系统 TLS 校验，限制最大正文 2047 字节。
func Fetch(ctx context.Context) (map[string]SymbolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	start := time.Now()
	body, err := postScanner(ctx)
	if err != nil {
		return nil, err
	}
	rttMs := float64(time.Since(start).Microseconds()) / 1000.0
	return ParseTradingView(body, rttMs)
}

func postScanner(ctx context.Context) ([]byte, error) {
	agent := winapi.UTF16("Mozilla/5.0 PulseBar/1.0")
	session, _, err := open.Call(uintptr(unsafe.Pointer(agent)), 4, 0, 0, 0)
	if session == 0 {
		return nil, winapi.Error("WinHttpOpen", err)
	}
	defer closeHandle.Call(session)

	if ok, _, err := setTimeouts.Call(session, 800, TimeoutMillis, TimeoutMillis, TimeoutMillis); ok == 0 {
		return nil, winapi.Error("WinHttpSetTimeouts", err)
	}
	hostPtr := winapi.UTF16(ScannerHost)
	conn, _, err := connect.Call(session, uintptr(unsafe.Pointer(hostPtr)), 443, 0)
	if conn == 0 {
		return nil, winapi.Error("WinHttpConnect", err)
	}
	defer closeHandle.Call(conn)

	verb, path := winapi.UTF16("POST"), winapi.UTF16(ScannerPath)
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
	payloadLen := uintptr(len(RequestPayload))
	if ok, _, err := sendRequest.Call(
		request,
		uintptr(unsafe.Pointer(hdrPtr)),
		^uintptr(0),
		uintptr(unsafe.Pointer(&RequestPayload[0])),
		payloadLen,
		payloadLen,
		0,
	); ok == 0 {
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
	// 检查 Content-Encoding 是否为未压缩
	var encBuf [64]uint16
	encSize := uint32(unsafe.Sizeof(encBuf))
	if ok, _, _ := queryHeaders.Call(request, 29, 0, uintptr(unsafe.Pointer(&encBuf[0])), uintptr(unsafe.Pointer(&encSize)), 0); ok != 0 {
		enc := strings.ToLower(strings.TrimSpace(utf16ToString(encBuf[:])))
		if enc != "" && enc != "identity" {
			return nil, fmt.Errorf("拒绝压缩响应 (%s)", enc)
		}
	}

	body := make([]byte, 0, 512)
	var chunk [512]byte
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
	runtime.KeepAlive(agent)
	runtime.KeepAlive(hostPtr)
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
