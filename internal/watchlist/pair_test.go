package watchlist

import "testing"

func TestNormalizeMainlandInputs(t *testing.T) {
	for code, want := range map[string]string{"600519": "600519.SS", " 000858 ": "000858.SZ", "300750": "300750.SZ", "688001": "688001.SS", "430047": "430047.BJ", "830799": "830799.BJ", "920002": "920002.BJ"} {
		got, err := NormalizeCode(code)
		if err != nil || got != want {
			t.Errorf("%q: %q %v", code, got, err)
		}
	}
	for _, code := range []string{"", "60051", "6005190", "６００５１９", "60051A", "sh600519", "159632", "900001", "200001", "6 0519"} {
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
