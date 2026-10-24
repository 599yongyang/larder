package platform_test

import (
	"os"
	"runtime"
	"testing"

	"larder/internal/platform"
)

func TestWebView2ProbeIsExplicit(t *testing.T) {
	status := platform.DetectWebView2()
	if status.Name != "WebView2" {
		t.Fatalf("runtime name = %q", status.Name)
	}
	if runtime.GOOS == "windows" && os.Getenv("LARDER_REQUIRE_WEBVIEW2") == "1" && !status.Installed {
		t.Fatalf("WebView2 not detected: %s", status.Error)
	}
	if runtime.GOOS != "windows" && status.Installed == false {
		t.Fatal("native non-Windows WebView status must remain available")
	}
}
