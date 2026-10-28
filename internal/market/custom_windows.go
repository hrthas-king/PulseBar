//go:build windows && (amd64 || arm64)

package market

import (
	"context"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/hrthas-king/PulseBar/internal/watchlist"
	"github.com/hrthas-king/PulseBar/internal/winapi"
)

var gbkToWide = winapi.DLL("kernel32.dll").NewProc("MultiByteToWideChar")

func decodeQuoteName(s string) (string, error) {
	if utf8.ValidString(s) {
		return s, nil
	}
	b := []byte(s)
	if len(b) == 0 {
		return "", fmt.Errorf("空名称")
	}
	n, _, _ := gbkToWide.Call(936, 8, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, 0)
	if n == 0 {
		return "", fmt.Errorf("名称编码无效")
	}
	w := make([]uint16, n)
	r, _, _ := gbkToWide.Call(936, 8, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&w[0])), n)
	if r == 0 {
		return "", fmt.Errorf("名称编码无效")
	}
	return string(utf16.Decode(w)), nil
}

func ValidateCustomPair(ctx context.Context, a, b string) (watchlist.Pair, error) {
	p, err := watchlist.NewPair(a, b)
	if err != nil {
		return p, err
	}
	syms := p.Symbols()
	query, _ := TencentQueryForSymbols(syms)
	clientMu.Lock()
	defer clientMu.Unlock()
	if err := ctx.Err(); err != nil {
		return watchlist.Pair{}, err
	}
	session, err := ensureSessionLocked()
	if err != nil {
		return watchlist.Pair{}, err
	}
	batch, err := fetchTencentRealtimeLocked(ctx, session, syms, query)
	for i, s := range syms {
		r := batch[s]
		if r.Quote == nil {
			detail := r.Err
			if detail == "" && err != nil {
				detail = err.Error()
			}
			return watchlist.Pair{}, fmt.Errorf("股票%d：%s", i+1, detail)
		}
	}
	if err := ctx.Err(); err != nil {
		return watchlist.Pair{}, err
	}
	return p, nil
}
