//go:build windows && (amd64 || arm64)

package market

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"sync"
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

const (
	requestHeaders = "Accept: application/json, text/plain, */*\r\n" +
		"Accept-Encoding: identity\r\n" +
		"Cache-Control: no-cache\r\n" +
		"Pragma: no-cache\r\n"

	quoteFields = "symbol,shortName,longName,currency,quoteType,marketState,quoteSourceName,gmtOffSetMilliseconds," +
		"regularMarketPrice,regularMarketChange,regularMarketChangePercent,regularMarketTime,regularMarketPreviousClose," +
		"preMarketPrice,preMarketChange,preMarketChangePercent,preMarketTime," +
		"postMarketPrice,postMarketChange,postMarketChangePercent,postMarketTime," +
		"overnightMarketPrice,overnightMarketChange,overnightMarketChangePercent,overnightMarketTime," +
		"fulldayPrice,fulldayChange,fulldayChangePercent"
)

var (
	clientMu    sync.Mutex
	sessHandle  uintptr
	cachedCrumb string
)

func ensureSessionLocked() (uintptr, error) {
	if sessHandle != 0 {
		return sessHandle, nil
	}
	agent := winapi.UTF16("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	s, _, err := open.Call(uintptr(unsafe.Pointer(agent)), 4, 0, 0, 0)
	runtime.KeepAlive(agent)
	if s == 0 {
		return 0, winapi.Error("WinHttpOpen", err)
	}
	if ok, _, err := setTimeouts.Call(s, 800, TimeoutMillis, TimeoutMillis, TimeoutMillis); ok == 0 {
		closeHandle.Call(s)
		return 0, winapi.Error("WinHttpSetTimeouts", err)
	}
	sessHandle = s
	return sessHandle, nil
}

func resetSessionLocked() {
	if sessHandle != 0 {
		closeHandle.Call(sessHandle)
		sessHandle = 0
	}
	cachedCrumb = ""
}

func refreshCrumbLocked(ctx context.Context, session uintptr) (string, error) {
	// 1. 访问 fc.yahoo.com 获取会话 Cookie（返回 404/200 均会写入 Set-Cookie 到 WinHTTP 会话 Cookie 罐）
	_, _, _ = doHTTPGet(ctx, session, CookieHost, "/", true)

	// 2. 从 query1 / query2 获取 crumb 令牌
	var lastErr error
	for _, host := range []string{PrimaryHost, FallbackHost} {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		status, body, err := doHTTPGet(ctx, session, host, "/v1/test/getcrumb", true)
		if err != nil {
			lastErr = err
			continue
		}
		if status != 200 {
			lastErr = HTTPError{StatusCode: status}
			continue
		}
		c := strings.TrimSpace(string(body))
		if c != "" && len(c) <= 64 && !strings.ContainsAny(c, "<>{}\r\n") {
			cachedCrumb = c
			return c, nil
		}
		lastErr = fmt.Errorf("无效 crumb 响应")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("无法获取 crumb")
	}
	return "", lastErr
}

// Fetch 通过 WinHTTP 获取 VOO 与 QQQ 的 24 小时全时段（夜盘 BOATS / 盘前 / 盘中 / 盘后）实时行情。
func Fetch(ctx context.Context) (map[string]SymbolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	clientMu.Lock()
	defer clientMu.Unlock()

	session, err := ensureSessionLocked()
	if err != nil {
		return nil, err
	}

	// 优先走带 overnightPrice=true 的 /v7/finance/quote 实时流（包含 Blue Ocean ATS 夜盘实时成交价与时间戳）
	batch, err := fetchV7OvernightLocked(ctx, session)
	if err == nil {
		return batch, nil
	}

	// 若代理节点切换或连接失效导致失败，重建会话句柄并在独立短超时下回退到 /v8/finance/chart 接口
	resetSessionLocked()
	fbSession, sErr := ensureSessionLocked()
	if sErr != nil {
		return nil, err
	}
	fbCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return fetchV8FallbackLocked(fbCtx, fbSession, err)
}

func fetchV7OvernightLocked(ctx context.Context, session uintptr) (map[string]SymbolResult, error) {
	crumb := cachedCrumb
	if crumb == "" {
		var err error
		crumb, err = refreshCrumbLocked(ctx, session)
		if err != nil {
			return nil, err
		}
	}

	batch, err := tryV7Batch(ctx, session, crumb)
	if err == nil {
		return batch, nil
	}

	var httpErr HTTPError
	if errors.As(err, &httpErr) && (httpErr.StatusCode == 401 || httpErr.StatusCode == 403) {
		resetSessionLocked()
		newSession, sErr := ensureSessionLocked()
		if sErr != nil {
			return nil, sErr
		}
		newCrumb, cErr := refreshCrumbLocked(ctx, newSession)
		if cErr != nil {
			return nil, cErr
		}
		return tryV7Batch(ctx, newSession, newCrumb)
	}
	return nil, err
}

func tryV7Batch(ctx context.Context, session uintptr, crumb string) (map[string]SymbolResult, error) {
	path := "/v7/finance/quote?symbols=" + strings.Join(Symbols, ",") +
		"&overnightPrice=true&fields=" + quoteFields +
		"&crumb=" + url.QueryEscape(crumb)

	var lastErr error
	for _, host := range []string{PrimaryHost, FallbackHost} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		start := time.Now()
		status, body, err := doHTTPGet(ctx, session, host, path, true)
		if err != nil {
			lastErr = err
			continue
		}
		if status != 200 {
			lastErr = HTTPError{StatusCode: status}
			if status == 401 || status == 403 {
				return nil, lastErr
			}
			continue
		}
		rttMs := float64(time.Since(start).Microseconds()) / 1000.0
		batch, parseErr := ParseYahooQuoteBatch(body, rttMs, time.Now())
		if parseErr == nil {
			return batch, nil
		}
		lastErr = parseErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("行情请求失败")
	}
	return nil, lastErr
}

func fetchV8FallbackLocked(ctx context.Context, session uintptr, primaryErr error) (map[string]SymbolResult, error) {
	results := make(map[string]SymbolResult, len(Symbols))
	var anySuccess bool
	lastErr := primaryErr

	for _, sym := range Symbols {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		q, fetchErr := fetchChartSymbol(ctx, session, sym)
		if fetchErr != nil {
			lastErr = fetchErr
			results[sym] = SymbolResult{Err: fmt.Sprintf("行情读取失败: %v", fetchErr)}
		} else {
			anySuccess = true
			qCopy := q
			results[sym] = SymbolResult{Quote: &qCopy}
		}
	}
	if !anySuccess && lastErr != nil {
		return nil, lastErr
	}
	return results, nil
}

func fetchChartSymbol(ctx context.Context, session uintptr, sym string) (Quote, error) {
	path := "/v8/finance/chart/" + sym + "?interval=1d&range=1d&includePrePost=true"
	var lastErr error
	for _, host := range []string{PrimaryHost, FallbackHost} {
		if err := ctx.Err(); err != nil {
			return Quote{}, err
		}
		start := time.Now()
		status, body, err := doHTTPGet(ctx, session, host, path, false)
		if err == nil && status != 200 {
			err = HTTPError{StatusCode: status}
		}
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

func doHTTPGet(ctx context.Context, session uintptr, host, path string, enableCookies bool) (int, []byte, error) {
	hostPtr := winapi.UTF16(host)
	conn, _, err := connect.Call(session, uintptr(unsafe.Pointer(hostPtr)), 443, 0)
	runtime.KeepAlive(hostPtr)
	if conn == 0 {
		return 0, nil, winapi.Error("WinHttpConnect", err)
	}
	defer closeHandle.Call(conn)

	verb := winapi.UTF16("GET")
	pathPtr := winapi.UTF16(path)
	request, _, err := openRequest.Call(conn, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(pathPtr)), 0, 0, 0, 0x00800000)
	if request == 0 {
		return 0, nil, winapi.Error("WinHttpOpenRequest", err)
	}
	defer closeHandle.Call(request)

	if !enableCookies {
		var disable uint32 = 2 | 1
		if ok, _, err := setOption.Call(request, 63, uintptr(unsafe.Pointer(&disable)), 4); ok == 0 {
			return 0, nil, winapi.Error("WinHttpSetOption", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	hdrPtr := winapi.UTF16(requestHeaders)
	if ok, _, err := sendRequest.Call(request, uintptr(unsafe.Pointer(hdrPtr)), ^uintptr(0), 0, 0, 0, 0); ok == 0 {
		return 0, nil, winapi.Error("WinHttpSendRequest", err)
	}
	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	if ok, _, err := receiveResponse.Call(request, 0); ok == 0 {
		return 0, nil, winapi.Error("WinHttpReceiveResponse", err)
	}
	var status uint32
	size := uint32(4)
	if ok, _, err := queryHeaders.Call(request, 19|0x20000000, 0, uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&size)), 0); ok == 0 {
		return 0, nil, winapi.Error("WinHttpQueryHeaders", err)
	}
	var encBuf [64]uint16
	encSize := uint32(unsafe.Sizeof(encBuf))
	if ok, _, _ := queryHeaders.Call(request, 29, 0, uintptr(unsafe.Pointer(&encBuf[0])), uintptr(unsafe.Pointer(&encSize)), 0); ok != 0 {
		enc := strings.ToLower(strings.TrimSpace(utf16ToString(encBuf[:])))
		if enc != "" && enc != "identity" {
			return int(status), nil, fmt.Errorf("拒绝压缩响应 (%s)", enc)
		}
	}

	body := make([]byte, 0, 2048)
	var chunk [1024]byte
	for {
		if err := ctx.Err(); err != nil {
			return int(status), nil, err
		}
		var n uint32
		ok, _, err := readData.Call(request, uintptr(unsafe.Pointer(&chunk[0])), uintptr(len(chunk)), uintptr(unsafe.Pointer(&n)))
		if ok == 0 {
			return int(status), nil, winapi.Error("WinHttpReadData", err)
		}
		if n == 0 {
			break
		}
		if n > uint32(len(chunk)) || len(body)+int(n) > MaxBody {
			return int(status), nil, fmt.Errorf("行情响应超过 %d 字节上限", MaxBody)
		}
		body = append(body, chunk[:n]...)
	}
	runtime.KeepAlive(verb)
	runtime.KeepAlive(pathPtr)
	runtime.KeepAlive(hdrPtr)
	return int(status), body, nil
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
