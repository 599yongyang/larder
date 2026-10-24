//go:build !windows

package platform

func detectWebView2() RuntimeStatus {
	return RuntimeStatus{
		Name: "WebView2", Supported: false, Installed: true,
		Version: "system-webview", Source: "native-platform",
	}
}
