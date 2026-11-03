package mcp

import "testing"

func TestMapRegistryEntryRuntimeAndAuth(t *testing.T) {
	var entry registryEntry
	entry.Server.Name = "com.figma.mcp/mcp"
	entry.Server.Title = "Figma"
	entry.Server.Description = "Design"
	entry.Server.Packages = []struct {
		RegistryType string `json:"registryType"`
		Identifier   string `json:"identifier"`
		Version      string `json:"version"`
		RuntimeHint  string `json:"runtimeHint"`
		RuntimeArgs  []struct {
			Value string `json:"value"`
			Type  string `json:"type"`
		} `json:"runtimeArguments"`
		Transport struct {
			Type string `json:"type"`
		} `json:"transport"`
		Environment []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			IsRequired  bool   `json:"isRequired"`
			IsSecret    bool   `json:"isSecret"`
		} `json:"environmentVariables"`
	}{{RegistryType: "npm", Identifier: "@figma/mcp", RuntimeHint: "npx"}}
	item, ok := mapRegistryEntry(entry)
	if !ok || item.Runtime != "npx" || item.Transport != "stdio" || item.Publisher != "Figma" {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestMapRegistryEntryRemoteAuth(t *testing.T) {
	var entry registryEntry
	entry.Server.Name = "com.notion/mcp"
	entry.Server.Remotes = []struct {
		Type    string `json:"type"`
		URL     string `json:"url"`
		Headers []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			IsRequired  bool   `json:"isRequired"`
			IsSecret    bool   `json:"isSecret"`
		} `json:"headers"`
		Variables map[string]struct {
			IsRequired bool `json:"isRequired"`
		} `json:"variables"`
	}{{Type: "streamable-http", URL: "https://mcp.notion.com/mcp", Headers: []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		IsRequired  bool   `json:"isRequired"`
		IsSecret    bool   `json:"isSecret"`
	}{{Name: "Authorization", IsRequired: true, IsSecret: true}}}}
	item, ok := mapRegistryEntry(entry)
	if !ok || !item.RequiresAuth || item.Transport != "http" || len(item.Headers) != 1 || item.Headers[0].Name != "Authorization" {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestConfigDirForPlatform(t *testing.T) {
	if got := configDirFor("windows", `C:\Users\a`, `C:\Users\a\AppData\Roaming`, ""); got != `C:\Users\a\AppData\Roaming` {
		t.Fatalf("windows = %q", got)
	}
	if got := configDirFor("darwin", "/Users/a", "", ""); got != "/Users/a/Library/Application Support" {
		t.Fatalf("darwin = %q", got)
	}
	if got := configDirFor("linux", "/home/a", "", "/home/a/.config/custom"); got != "/home/a/.config/custom" {
		t.Fatalf("linux = %q", got)
	}
}
