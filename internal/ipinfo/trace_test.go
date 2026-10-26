package ipinfo

import "testing"

func TestParseTrace(t *testing.T) {
	valid := []byte("fl=29f1\nip=198.51.100.42\nts=1700000000.000\nloc=US\n")
	info, err := ParseTrace(valid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IP != "198.51.100.42" || info.Country != "US" {
		t.Fatalf("unexpected info: %+v", info)
	}

	cases := [][]byte{
		[]byte("ip=127.0.0.1\nloc=US\n"),
		[]byte("ip=192.168.1.1\nloc=US\n"),
		[]byte("ip=198.51.100.42\nloc=XX\n"),
		[]byte("ip=198.51.100.42\nloc=USA\n"),
		[]byte("ip=198.51.100.42\nip=198.51.100.43\nloc=US\n"),
	}
	for i, c := range cases {
		if _, err := ParseTrace(c); err == nil {
			t.Fatalf("case %d: expected error for invalid trace", i)
		}
	}
}
