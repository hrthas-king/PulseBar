//go:build windows && (amd64 || arm64)

package ipinfo

import (
	"context"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/hrthas-king/Samuel/internal/winapi"
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

// Fetch 通过 WinHTTP 请求 Cloudflare trace，跟随系统代理与 TUN 路由，禁用连接复用以保证切换节点后探测最新出口。
func Fetch(ctx context.Context) (Info, error) {
	var lastErr error
	for _, host := range []string{"www.cloudflare.com", "cloudflare.com"} {
		if err := ctx.Err(); err != nil {
			return Info{}, err
		}
		body, err := fetchHost(ctx, host)
		if err == nil {
			info, parseErr := ParseTrace(body)
			if parseErr == nil {
				return info, nil
			}
			err = parseErr
		}
		lastErr = fmt.Errorf("%s: %w", host, err)
	}
	return Info{}, lastErr
}

func fetchHost(ctx context.Context, host string) ([]byte, error) {
	agent := winapi.UTF16("Samuel/1.0")
	session, _, err := open.Call(uintptr(unsafe.Pointer(agent)), 4, 0, 0, 0)
	if session == 0 {
		return nil, winapi.Error("WinHttpOpen", err)
	}
	defer closeHandle.Call(session)
	if ok, _, err := setTimeouts.Call(session, 800, 1200, 1200, 1200); ok == 0 {
		return nil, winapi.Error("WinHttpSetTimeouts", err)
	}
	hostPtr := winapi.UTF16(host)
	conn, _, err := connect.Call(session, uintptr(unsafe.Pointer(hostPtr)), 443, 0)
	if conn == 0 {
		return nil, winapi.Error("WinHttpConnect", err)
	}
	defer closeHandle.Call(conn)
	verb, path := winapi.UTF16("GET"), winapi.UTF16("/cdn-cgi/trace")
	request, _, err := openRequest.Call(conn, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(path)), 0, 0, 0, 0x00800000)
	if request == 0 {
		return nil, winapi.Error("WinHttpOpenRequest", err)
	}
	defer closeHandle.Call(request)
	var disable uint32 = 2 | 1 | 8
	if ok, _, err := setOption.Call(request, 63, uintptr(unsafe.Pointer(&disable)), 4); ok == 0 {
		return nil, winapi.Error("WinHttpSetOption", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ok, _, err := sendRequest.Call(request, 0, 0, 0, 0, 0, 0); ok == 0 {
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
		return nil, fmt.Errorf("HTTP %d", status)
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
			return nil, fmt.Errorf("定位响应超出限制")
		}
		body = append(body, chunk[:n]...)
	}
	runtime.KeepAlive(agent)
	runtime.KeepAlive(hostPtr)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(path)
	return body, nil
}
