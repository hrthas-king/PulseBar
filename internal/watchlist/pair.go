// Package watchlist defines user-selected mainland stock pairs without UI or network dependencies.
package watchlist

import (
	"fmt"
	"strings"
)

type Pair struct {
	Code1 string `json:"code1"`
	Code2 string `json:"code2"`
}

func NormalizeCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return "", fmt.Errorf("请输入6位数字股票代码")
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return "", fmt.Errorf("股票代码只能包含6位数字")
		}
	}
	switch code[0] {
	case '6':
		return code + ".SS", nil
	case '0', '3':
		return code + ".SZ", nil
	case '4', '8':
		return code + ".BJ", nil
	case '9':
		if strings.HasPrefix(code, "92") {
			return code + ".BJ", nil
		}
	}
	return "", fmt.Errorf("不支持该代码号段，请输入沪、深或北交所A股代码")
}

func NewPair(a, b string) (Pair, error) {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if _, err := NormalizeCode(a); err != nil {
		return Pair{}, fmt.Errorf("股票1：%w", err)
	}
	if _, err := NormalizeCode(b); err != nil {
		return Pair{}, fmt.Errorf("股票2：%w", err)
	}
	if a == b {
		return Pair{}, fmt.Errorf("请输入两只不同的股票")
	}
	return Pair{a, b}, nil
}

func (p Pair) ID() string    { return "custom_" + p.Code1 + "_" + p.Code2 }
func (p Pair) Label() string { return p.Code1 + " & " + p.Code2 }
func (p Pair) Symbols() []string {
	a, _ := NormalizeCode(p.Code1)
	b, _ := NormalizeCode(p.Code2)
	return []string{a, b}
}

func IsSymbol(symbol string) bool {
	if len(symbol) != 9 {
		return false
	}
	normal, err := NormalizeCode(symbol[:6])
	return err == nil && normal == symbol
}

func TencentSymbol(symbol string) (string, bool) {
	if !IsSymbol(symbol) {
		return "", false
	}
	prefix := map[string]string{".SS": "sh", ".SZ": "sz", ".BJ": "bj"}[symbol[6:]]
	return prefix + symbol[:6], true
}
