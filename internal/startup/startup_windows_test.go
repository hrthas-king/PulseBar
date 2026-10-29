//go:build windows && (amd64 || arm64)

package startup

import "testing"

func TestLegacyStartupMigrationOnlyMatchesSameDirectory(t *testing.T) {
	exe := `C:\Apps\Samuel.exe`
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{`"C:\Apps\PulseBar.exe"`, true},
		{`"c:\apps\PULSEBAR.EXE"`, true},
		{`"C:\Other\PulseBar.exe"`, false},
		{`"C:\Apps\PulseBar.exe" --extra`, false},
		{`C:\Apps\PulseBar.exe`, false},
	} {
		if got := legacyStartupMatches(tc.value, exe); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.value, got, tc.want)
		}
	}
}
