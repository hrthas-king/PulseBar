//go:build windows && (amd64 || arm64)

package network

import (
	"syscall"
	"testing"
	"unsafe"
)

func TestTrafficRecognizesTunnelAdapters(t *testing.T) {
	for _, tc := range []struct {
		name        string
		kind        uint32
		description string
	}{
		{"proprietary_virtual", 53, "phantom"},
		{"tunnel", 131, "VPN"},
		{"wireguard_ethernet", 6, "WireGuard Adapter"},
		{"tap_ethernet", 6, "TAP-Windows Adapter V9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := ifRow2{Type: tc.kind, OperStatus: 1}
			copy(row.Description[:], syscall.StringToUTF16(tc.description))
			if trafficInterfaceUsable(row) {
				t.Fatal("TUN adapter accepted as an underlying uplink")
			}
		})
	}
}

func TestTrafficCountersUseUnderlayInsteadOfMirroredTUN(t *testing.T) {
	tun := ifRow2{LUID: 46, Index: 46, Type: 53, OperStatus: 1, InOctets: 1286563, OutOctets: 1286469}
	wifi := ifRow2{LUID: 4, Index: 4, Type: 71, Flags: 1, OperStatus: 1, InOctets: 1352778, OutOctets: 109418}
	routes := []ipv4ForwardRow{{Index: 46, Type: 4, Metric: 0}, {Index: 4, Type: 4, Metric: 50}}
	got, err := selectTrafficInterface(tun, []ifRow2{tun, wifi}, routes)
	if err != nil || got.Index != 4 || got.InOctets != 1352778 || got.OutOctets != 109418 {
		t.Fatalf("download still mirrored through virtual interface: index=%d in=%d out=%d err=%v", got.Index, got.InOctets, got.OutOctets, err)
	}
}

func TestTrafficCountersKeepTheDirectRoute(t *testing.T) {
	wifi := ifRow2{Index: 4, Type: 71, Flags: 1, OperStatus: 1}
	wired := ifRow2{Index: 8, Type: 6, Flags: 1, OperStatus: 1}
	got, err := selectTrafficInterface(wifi, []ifRow2{wifi, wired}, []ipv4ForwardRow{{Index: 8, Type: 4, Metric: 0}})
	if err != nil || got.Index != wifi.Index {
		t.Fatal("overrode the OS-selected direct route")
	}
}

func TestTrafficUnderlayRequiresAUsableDefaultRoute(t *testing.T) {
	tun := ifRow2{Index: 46, Type: 53, OperStatus: 1}
	rows := []ifRow2{{Index: 4, Type: 71, Flags: 1, OperStatus: 1}, {Index: 5, Type: 6, Flags: 1, OperStatus: 2}, {Index: 6, Type: 6, Flags: 3, OperStatus: 1}, {Index: 1, Type: 24, OperStatus: 1}}
	routes := []ipv4ForwardRow{{Index: 4, Destination: 1, Mask: 0xffffffff, Type: 4}, {Index: 5, Type: 4}, {Index: 6, Type: 4}, {Index: 1, Type: 3}}
	if got, err := selectTrafficInterface(tun, rows, routes); err == nil {
		t.Fatalf("accepted an unrelated, disconnected or filter interface: index=%d", got.Index)
	}
}

func TestTrafficUnderlayUsesRoutePreferenceWithoutSummingAdapters(t *testing.T) {
	tun := ifRow2{Index: 46, Type: 53, OperStatus: 1}
	wifi := ifRow2{Index: 4, Type: 71, Flags: 1, OperStatus: 1, InOctets: 500}
	wired := ifRow2{Index: 8, Type: 6, Flags: 1, OperStatus: 1, InOctets: 900}
	vm := ifRow2{Index: 43, Type: 6, OperStatus: 1}
	routes := []ipv4ForwardRow{{Index: 4, Type: 4, Metric: 50}, {Index: 8, Type: 4, Metric: 20}, {Index: 43, Type: 4, Metric: 20}}
	got, err := selectTrafficInterface(tun, []ifRow2{tun, wifi, wired, vm}, routes)
	if err != nil || got.Index != 8 || got.InOctets != 900 {
		t.Fatalf("selected wrong underlay or combined counters: index=%d in=%d err=%v", got.Index, got.InOctets, err)
	}
}

func TestIPv4ForwardLayout(t *testing.T) {
	var r ipv4ForwardRow
	if unsafe.Sizeof(r) != 56 || unsafe.Offsetof(r.Index) != 16 || unsafe.Offsetof(r.Metric) != 36 {
		t.Fatal("IPv4 route ABI mismatch")
	}
}
