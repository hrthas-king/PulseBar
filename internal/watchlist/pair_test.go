package watchlist

import "testing"

func TestNormalizeMainlandInputs(t *testing.T) {
	for code, want := range map[string]string{"600519": "600519.SS", " 000858 ": "000858.SZ", "300750": "300750.SZ", "688001": "688001.SS", "430047": "430047.BJ", "830799": "830799.BJ", "920002": "920002.BJ"} {
		got, err := NormalizeCode(code)
		if err != nil || got != want {
			t.Errorf("%q: %q %v", code, got, err)
		}
	}
	for _, code := range []string{"", "60051", "6005190", "６００５１９", "60051A", "sh600519", "900001", "200001", "6 0519"} {
		if _, err := NormalizeCode(code); err == nil {
			t.Errorf("accepted %q", code)
		}
	}
	p, err := NewPair(" 600519 ", "000858")
	if err != nil || p.ID() != "custom_600519_000858" || p.Label() != "600519 & 000858" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := NewPair("600519", "600519"); err == nil {
		t.Fatal("accepted duplicate stocks")
	}
}

func TestNormalizeETFAndUSInputs(t *testing.T) {
	for code, want := range map[string]string{
		"159632": "159632.SZ", "159655": "159655.SZ", "513500": "513500.SS", "510300": "510300.SS",
		"aapl": "AAPL", " MSFT ": "MSFT", "VOO": "VOO", "qqq": "QQQ", "brk.b": "BRK-B",
	} {
		got, err := NormalizeCode(code)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", code, got, err, want)
		}
	}
	p, err := NewPair(" aapl ", "msft")
	if err != nil || p.Code1 != "AAPL" || p.Code2 != "MSFT" || p.ID() != "custom_AAPL_MSFT" {
		t.Fatalf("canonical pair: %+v, %v", p, err)
	}
	if _, err := NewPair("BRK.B", "brk-b"); err == nil {
		t.Fatal("accepted the same share class twice")
	}
	for _, code := range []string{"AAPL/USD", "AAPL?", "AAPL1", "ABCDEF", "BRK--B", "BRK-BC", "^GSPC", "纳指", "NASDAQ:AAPL"} {
		if _, err := NormalizeCode(code); err == nil {
			t.Errorf("accepted invalid ticker %q", code)
		}
	}
	for symbol, want := range map[string]string{"513500.SS": "sh513500", "159632.SZ": "sz159632"} {
		if got, ok := TencentSymbol(symbol); !ok || got != want {
			t.Errorf("mainland provider symbol: %s %v", got, ok)
		}
	}
	if _, ok := TencentSymbol("AAPL"); ok {
		t.Fatal("US ticker routed to the mainland provider")
	}
}
