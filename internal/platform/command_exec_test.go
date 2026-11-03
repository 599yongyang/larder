package platform

import "testing"

func TestIsWindowsBatch(t *testing.T) {
	for _, name := range []string{"npx.cmd", `C:\Users\a\AppData\Roaming\npm\npx.cmd`, "foo.BAT"} {
		if !isWindowsBatch(name) {
			t.Fatalf("expected batch: %s", name)
		}
	}
	for _, name := range []string{"npx.exe", "node", "foo.ps1"} {
		if isWindowsBatch(name) {
			t.Fatalf("unexpected batch: %s", name)
		}
	}
}
