package mcp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"larder/internal/core/model"
)

func TestInspectTransportSessionUsesSDK(t *testing.T) {
	ctx := context.Background()
	server := sdk.NewServer(&sdk.Implementation{Name: "mock", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "read_file", Description: "Read a file", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		return &sdk.CallToolResult{}, nil
	})
	server.AddResource(&sdk.Resource{Name: "info", URI: "embedded:info", MIMEType: "text/plain"}, func(context.Context, *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{{URI: "embedded:info", Text: "hello"}}}, nil
	})
	server.AddResourceTemplate(&sdk.ResourceTemplate{Name: "file", URITemplate: "file:///{path}"}, func(context.Context, *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		return &sdk.ReadResourceResult{}, nil
	})
	server.AddPrompt(&sdk.Prompt{Name: "review", Arguments: []*sdk.PromptArgument{{Name: "code", Required: true}}}, func(context.Context, *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
		return &sdk.GetPromptResult{Messages: []*sdk.PromptMessage{{Role: "user", Content: &sdk.TextContent{Text: "review"}}}}, nil
	})
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	out := &model.MCPInspection{}
	if err := inspectTransportSession(ctx, clientTransport, nil, out, "", "", "2025-06-18"); err != nil {
		t.Fatal(err)
	}
	if len(out.Tools) != 1 || out.Tools[0].Name != "read_file" {
		t.Fatalf("tools = %#v", out.Tools)
	}
	if len(out.Capabilities) == 0 {
		t.Fatal("expected capabilities")
	}
	if len(out.Logs) == 0 {
		t.Fatal("expected JSON-RPC logs")
	}
	if out.ToolError != "" {
		t.Fatalf("tool error = %q", out.ToolError)
	}
	if len(out.Resources) != 1 || out.Resources[0].URI != "embedded:info" {
		t.Fatalf("resources = %#v", out.Resources)
	}
	if len(out.ResourceTemplates) != 1 || out.ResourceTemplates[0].URITemplate != "file:///{path}" {
		t.Fatalf("templates = %#v", out.ResourceTemplates)
	}
	if len(out.Prompts) != 1 || out.Prompts[0].Name != "review" {
		t.Fatalf("prompts = %#v", out.Prompts)
	}
}

func TestInspectTransportSessionTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	out := &model.MCPInspection{}
	err := inspectTransportSession(ctx, &blockingTransport{}, nil, out, "", "", "2025-06-18")
	if err == nil {
		t.Fatal("expected timeout")
	}
}

type blockingTransport struct{}

func (blockingTransport) Connect(ctx context.Context) (sdk.Connection, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSanitizeErrorRemovesURLSecrets(t *testing.T) {
	endpoint := "https://user:pass@example.com/mcp?api_key=secret"
	source := &url.Error{Op: "Post", URL: endpoint, Err: errors.New("connection refused")}
	got := sanitizeError(source, endpoint).Error()
	for _, secret := range []string{"user", "pass", "api_key", "secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked: %s", secret, got)
		}
	}
	if !strings.Contains(got, "https://example.com/mcp") {
		t.Fatalf("host lost: %s", got)
	}
}

func TestHeaderTransportDoesNotLeakAcrossHosts(t *testing.T) {
	h := &headerTransport{headers: map[string]string{"Authorization": "secret", "Content-Type": "application/custom"}, host: "trusted.example.com"}
	req, _ := http.NewRequest("POST", "https://other.example.com/mcp", nil)
	seen := http.Header{}
	h.base = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r.Header.Clone()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, err := h.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if seen.Get("Authorization") != "" {
		t.Fatal("authorization leaked to another host")
	}
	if seen.Get("Content-Type") != "" {
		t.Fatal("content type was overwritten")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestResolveCommandUsesUserBinDirs(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".asdf", "shims")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "larder-resolve-test"
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := resolveCommand(name, home, "")
	if got != filepath.Join(bin, name) {
		t.Fatalf("resolved %q", got)
	}
}

func TestExpandArg(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LARDER_MCP_TEST_VALUE", "ok")
	if got := expandArg("${LARDER_MCP_TEST_VALUE}", home); got != "ok" {
		t.Fatalf("env = %q", got)
	}
	if got := expandArg("~/skills", home); got != filepath.Join(home, "skills") {
		t.Fatalf("tilde = %q", got)
	}
}

func TestInspectTransportNormalizesRemote(t *testing.T) {
	target := &client{name: "remote", transport: "remote", url: "https://example.com/mcp"}
	transport, cleanup, err := inspectTransport(target)
	if err != nil {
		t.Fatal(err)
	}
	if transport == nil {
		t.Fatal("expected HTTP transport")
	}
	if cleanup != nil {
		cleanup()
	}
}

func TestExpandPercentEnv(t *testing.T) {
	t.Setenv("LARDER_PERCENT_TEST", "ok")
	if got := expandPercentEnv("a%LARDER_PERCENT_TEST%b"); got != "aokb" { t.Fatalf("got %q", got) }
	if got := expandPercentEnv("no-vars"); got != "no-vars" { t.Fatalf("got %q", got) }
}
