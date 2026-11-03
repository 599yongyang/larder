package mcp

import (
	"context"
	"os"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv("LARDER_MCP_STDIO_HELPER") != "1" {
		return
	}
	server := sdk.NewServer(&sdk.Implementation{Name: "stdio-helper", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "echo", Description: "Echo", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "ok"}}}, nil
	})
	if err := server.Run(context.Background(), &sdk.StdioTransport{}); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestConnectRealStdioProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	target := &client{name: "stdio-helper", transport: "stdio", command: os.Args[0], args: []string{"-test.run=^TestMCPStdioHelperProcess$"}, env: map[string]string{"LARDER_MCP_STDIO_HELPER": "1"}}
	session, cleanup, err := connectTarget(ctx, target, "2025-06-18")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		session.Close()
		if cleanup != nil {
			cleanup()
		}
	}()
	tools, err := session.ListTools(ctx, &sdk.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "echo" {
		t.Fatalf("tools = %#v", tools.Tools)
	}
}
