// Package watchlist defines user-selected stock and ETF pairs without UI or network dependencies.
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
	code = strings.ToUpper(strings.TrimSpace(code))
	if isUSTicker(strings.ReplaceAll(code, ".", "-")) {
		return strings.ReplaceAll(code, ".", "-"), nil
	}
	if len(code) != 6 {
		return "", fmt.Errorf("请输入6位沪深北股票/ETF代码，或美股代码（如AAPL）")
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return "", fmt.Errorf("请输入6位数字或有效的美股字母代码")
		}
	}
	switch code[0] {
	case '5', '6':
		return code + ".SS", nil
	case '0', '1', '3':
		return code + ".SZ", nil
	case '4', '8':
		return code + ".BJ", nil
	case '9':
		if strings.HasPrefix(code, "92") {
			return code + ".BJ", nil
		}
	}
	return "", fmt.Errorf("不支持该代码号段，请输入沪深北股票或场内ETF代码")
}

func isUSTicker(code string) bool {
	base, share, hasShare := strings.Cut(code, "-")
	if len(base) < 1 || len(base) > 5 || (hasShare && len(share) != 1) {
		return false
	}
	for _, c := range base + share {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func IsUSSymbol(symbol string) bool { return isUSTicker(symbol) }

func canonicalCode(code, symbol string) string {
	if IsUSSymbol(symbol) {
		return symbol
	}
	return strings.TrimSpace(code)
}

func NewPair(a, b string) (Pair, error) {
	sa, err := NormalizeCode(a)
	if err != nil {
		return Pair{}, fmt.Errorf("股票1：%w", err)
	}
	sb, err := NormalizeCode(b)
	if err != nil {
		return Pair{}, fmt.Errorf("股票2：%w", err)
	}
	if sa == sb {
		return Pair{}, fmt.Errorf("请输入两个不同的股票或ETF代码")
	}
	return Pair{canonicalCode(a, sa), canonicalCode(b, sb)}, nil
}

func (p Pair) ID() string    { return "custom_" + p.Code1 + "_" + p.Code2 }
func (p Pair) Label() string { return p.Code1 + " & " + p.Code2 }
func (p Pair) Symbols() []string {
	a, _ := NormalizeCode(p.Code1)
	b, _ := NormalizeCode(p.Code2)
	return []string{a, b}
}

func IsSymbol(symbol string) bool {
	return IsUSSymbol(symbol) || IsMainlandSymbol(symbol)
}

func IsMainlandSymbol(symbol string) bool {
	if len(symbol) != 9 {
		return false
	}
	normal, err := NormalizeCode(symbol[:6])
	return err == nil && normal == symbol
}

func IsMainlandETF(symbol string) bool {
	return IsMainlandSymbol(symbol) && (symbol[0] == '1' || symbol[0] == '5')
}

func TencentSymbol(symbol string) (string, bool) {
	if !IsMainlandSymbol(symbol) {
		return "", false
	}
	prefix := map[string]string{".SS": "sh", ".SZ": "sz", ".BJ": "bj"}[symbol[6:]]
	return prefix + symbol[:6], true
}
