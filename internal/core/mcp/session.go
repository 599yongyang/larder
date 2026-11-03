package mcp

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"larder/internal/core/model"
)

// Session keeps an MCP connection alive so tool/resource/prompt operations do
// not restart the same stdio process for every action.
type Session struct {
	session    *sdk.ClientSession
	cleanup    func()
	serverID   string
	name       string
	transport  string
	agentLabel string
	agentPath  string
	logs       *sessionLogs
	mu         sync.Mutex
}

type sessionLogs struct {
	mu      sync.Mutex
	entries []model.MCPLogEntry
}

func (l *sessionLogs) add(entry model.MCPLogEntry) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > 160 {
		l.entries = l.entries[len(l.entries)-160:]
	}
}

func (l *sessionLogs) snapshot() []model.MCPLogEntry {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]model.MCPLogEntry(nil), l.entries...)
}

func OpenSession(ctx context.Context, serverID string, projects []string, clientVersion string) (*Session, error) {
	target, _, err := findInspectTarget(serverID, projects)
	if err != nil {
		return nil, err
	}
	return openSessionTarget(ctx, target, clientVersion)
}

func OpenSessionLaunch(ctx context.Context, target model.MCPInspectTarget, clientVersion string) (*Session, error) {
	return openSessionTarget(ctx, launchClient(target), clientVersion)
}

func openSessionTarget(ctx context.Context, target *client, clientVersion string) (*Session, error) {
	if clientVersion == "" {
		clientVersion = "0.0.0"
	}
	logs := &sessionLogs{}
	try := func(protocol string) (*sdk.ClientSession, func(), error) {
		transport, cleanup, err := inspectTransport(target)
		if err != nil {
			return nil, nil, err
		}
		client := sdk.NewClient(&sdk.Implementation{Name: "Larder", Version: clientVersion}, nil)
		client.AddSendingMiddleware(func(next sdk.MethodHandler) sdk.MethodHandler {
			return func(callCtx context.Context, method string, request sdk.Request) (sdk.Result, error) {
				started := time.Now()
				result, err := next(callCtx, method, request)
				entry := model.MCPLogEntry{At: started.UTC().Format(time.RFC3339Nano), Method: method, DurationMs: time.Since(started).Milliseconds()}
				if raw, marshalErr := json.Marshal(request); marshalErr == nil {
					entry.Request = truncateLog(string(raw))
				}
				if raw, marshalErr := json.Marshal(result); marshalErr == nil {
					entry.Response = truncateLog(string(raw))
				}
				if err != nil {
					entry.Error = err.Error()
				}
				logs.add(entry)
				return result, err
			}
		})
		session, err := client.Connect(ctx, transport, &sdk.ClientSessionOptions{ProtocolVersion: protocol})
		if err != nil {
			if cleanup != nil {
				cleanup()
			}
			return nil, nil, err
		}
		return session, cleanup, nil
	}
	session, cleanup, err := try("2025-06-18")
	if err != nil && isProtocolError(err) {
		session, cleanup, err = try("")
	}
	if err != nil {
		return nil, explainConnectionError(sanitizeError(err, target.url))
	}
	return &Session{session: session, cleanup: cleanup, serverID: hashedID(serverKey(*target)), name: target.name, transport: target.transport, agentLabel: target.agent.Label, agentPath: target.agent.Path, logs: logs}, nil
}

// Inspect lists capabilities over the already-open session and records the
// JSON-RPC trace for the same connection.
func (s *Session) Inspect(ctx context.Context) (model.MCPInspection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := model.MCPInspection{ServerID: s.serverID, Name: s.name, Transport: s.transport, AgentLabel: s.agentLabel, AgentPath: s.agentPath}
	if init := s.session.InitializeResult(); init != nil {
		result.ProtocolVersion = init.ProtocolVersion
		if init.ServerInfo != nil {
			result.ServerName = init.ServerInfo.Name
			result.ServerVersion = init.ServerInfo.Version
		}
		if init.Capabilities != nil {
			if init.Capabilities.Tools != nil {
				result.Capabilities = append(result.Capabilities, "tools")
			}
			if init.Capabilities.Resources != nil {
				result.Capabilities = append(result.Capabilities, "resources")
			}
			if init.Capabilities.Prompts != nil {
				result.Capabilities = append(result.Capabilities, "prompts")
			}
		}
	}
	has := func(name string) bool {
		for _, item := range result.Capabilities {
			if item == name {
				return true
			}
		}
		return false
	}
	if has("tools") {
		for tool, err := range s.session.Tools(ctx, nil) {
			if err != nil {
				result.ToolError = sanitizeError(err, "").Error()
				break
			}
			result.Tools = append(result.Tools, convertTool(tool))
		}
	} else {
		result.ToolError = "tools are not advertised by this server"
	}
	if encoded, err := json.Marshal(result.Tools); err == nil {
		result.ToolTokens = len(encoded) / 4
	}
	if has("resources") {
		for resource, err := range s.session.Resources(ctx, nil) {
			if err != nil {
				result.ResourceError = sanitizeError(err, "").Error()
				break
			}
			result.Resources = append(result.Resources, model.MCPResource{URI: resource.URI, Name: resource.Name, Title: resource.Title, Description: resource.Description, MimeType: resource.MIMEType})
		}
	} else {
		result.ResourceError = "resources are not advertised by this server"
	}
	if has("resources") {
		for template, err := range s.session.ResourceTemplates(ctx, nil) {
			if err != nil {
				result.ResourceError = sanitizeError(err, "").Error()
				break
			}
			result.ResourceTemplates = append(result.ResourceTemplates, model.MCPResourceTemplate{URITemplate: template.URITemplate, Name: template.Name, Title: template.Title, Description: template.Description, MimeType: template.MIMEType})
		}
	}
	if has("prompts") {
		for prompt, err := range s.session.Prompts(ctx, nil) {
			if err != nil {
				result.PromptError = sanitizeError(err, "").Error()
				break
			}
			result.Prompts = append(result.Prompts, convertPrompt(prompt))
		}
	} else {
		result.PromptError = "prompts are not advertised by this server"
	}
	result.Logs = s.logs.snapshot()
	return result, nil
}

func (s *Session) Logs() []model.MCPLogEntry {
	if s == nil {
		return nil
	}
	return s.logs.snapshot()
}

func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.session != nil {
		_ = s.session.Close()
	}
	if s.cleanup != nil {
		s.cleanup()
	}
}

func (s *Session) CallTool(ctx context.Context, tool string, arguments map[string]any) (model.MCPToolCallResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.session.CallTool(ctx, &sdk.CallToolParams{Name: tool, Arguments: arguments})
	if err != nil {
		return model.MCPToolCallResult{}, err
	}
	content, _ := json.Marshal(result.Content)
	structured, _ := json.Marshal(result.StructuredContent)
	return model.MCPToolCallResult{Content: string(content), StructuredContent: string(structured), IsError: result.IsError}, nil
}

func (s *Session) ReadResource(ctx context.Context, uri string) (model.MCPValueResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.session.ReadResource(ctx, &sdk.ReadResourceParams{URI: uri})
	if err != nil {
		return model.MCPValueResult{}, err
	}
	raw, _ := json.Marshal(result.Contents)
	return model.MCPValueResult{Content: string(raw)}, nil
}

func (s *Session) GetPrompt(ctx context.Context, name string, arguments map[string]string) (model.MCPValueResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.session.GetPrompt(ctx, &sdk.GetPromptParams{Name: name, Arguments: arguments})
	if err != nil {
		return model.MCPValueResult{}, err
	}
	raw, _ := json.Marshal(result)
	return model.MCPValueResult{Content: string(raw)}, nil
}
