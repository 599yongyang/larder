package platform

type RuntimeStatus struct {
	Name      string `json:"name"`
	Supported bool   `json:"supported"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Source    string `json:"source,omitempty"`
	Error     string `json:"error,omitempty"`
}

func DetectWebView2() RuntimeStatus { return detectWebView2() }
