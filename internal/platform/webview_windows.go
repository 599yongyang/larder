//go:build windows

package platform

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const webView2ClientID = `{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

func detectWebView2() RuntimeStatus {
	status := RuntimeStatus{Name: "WebView2", Supported: true, Source: "registry"}
	paths := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\` + webView2ClientID},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + webView2ClientID},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + webView2ClientID},
	}
	for _, candidate := range paths {
		key, err := registry.OpenKey(candidate.root, candidate.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		version, _, valueErr := key.GetStringValue("pv")
		_ = key.Close()
		if valueErr == nil && strings.TrimSpace(version) != "" && version != "0.0.0.0" {
			status.Installed = true
			status.Version = version
			return status
		}
	}
	if folder := strings.TrimSpace(os.Getenv("WEBVIEW2_BROWSER_EXECUTABLE_FOLDER")); folder != "" {
		if _, err := os.Stat(folder); err == nil {
			status.Installed = true
			status.Source = "WEBVIEW2_BROWSER_EXECUTABLE_FOLDER"
			return status
		}
	}
	status.Error = fmt.Sprintf("WebView2 Evergreen runtime registry key %s not found", webView2ClientID)
	return status
}
