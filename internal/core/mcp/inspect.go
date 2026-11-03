package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"larder/internal/core/model"
	"larder/internal/platform"
)

// Inspect connects to one already-configured MCP server and lists its
// capabilities. It is read-only: tools are listed, never called. Secrets from
// env/header config are used only for the connection and are never returned.
func Inspect(ctx context.Context, serverID string, projects []string, clientVersion string) (model.MCPInspection, error) {
	target, spec, err := findInspectTarget(serverID, projects)
	if err != nil {
		return model.MCPInspection{}, err
	}
	_ = spec
	result := model.MCPInspection{ServerID: serverID, Name: target.name, Transport: target.transport, AgentLabel: target.agent.Label, AgentPath: target.agent.Path, Disabled: !target.enabled}
	if result.Name == "" {
		result.Name = serverID
	}
	if err := inspectTarget(ctx, target, clientVersion, &result); err != nil {
		return result, err
	}
	return result, nil
}

func inspectTarget(ctx context.Context, target *client, clientVersion string, result *model.MCPInspection) error {
	try := func(protocol string) error {
		transport, cleanup, err := inspectTransport(target)
		if err != nil {
			return err
		}
		return inspectTransportSession(ctx, transport, cleanup, result, target.url, clientVersion, protocol)
	}
	err := try("2025-06-18")
	if err != nil && isProtocolError(err) {
		*result = model.MCPInspection{ServerID: result.ServerID, Name: result.Name, Transport: result.Transport, AgentLabel: result.AgentLabel, AgentPath: result.AgentPath, Disabled: result.Disabled}
		return try("")
	}
	return err
}

func inspectTransportSession(ctx context.Context, transport sdk.Transport, cleanup func(), result *model.MCPInspection, endpoint, clientVersion, protocolVersion string) error {
	var resultLogs []model.MCPLogEntry
	if cleanup != nil {
		defer cleanup()
	}
	if clientVersion == "" {
		clientVersion = "0.0.0"
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "Larder", Version: clientVersion}, nil)
	var logMu sync.Mutex
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
			logMu.Lock()
			resultLogs = append(resultLogs, entry)
			if len(resultLogs) > 160 {
				resultLogs = resultLogs[len(resultLogs)-160:]
			}
			logMu.Unlock()
			return result, err
		}
	})
	session, err := client.Connect(ctx, transport, &sdk.ClientSessionOptions{ProtocolVersion: protocolVersion})
	if err != nil {
		return explainConnectionError(sanitizeError(err, endpoint))
	}
	defer session.Close()
	if init := session.InitializeResult(); init != nil {
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
		for tool, listErr := range session.Tools(ctx, nil) {
			if listErr != nil {
				result.ToolError = sanitizeError(listErr, endpoint).Error()
				break
			}
			result.Tools = append(result.Tools, convertTool(tool))
		}
	} else {
		result.ToolError = "tools are not advertised by this server"
	}
	if encoded, marshalErr := json.Marshal(result.Tools); marshalErr == nil {
		result.ToolTokens = len(encoded) / 4
	}
	if has("resources") {
		for resource, listErr := range session.Resources(ctx, nil) {
			if listErr != nil {
				result.ResourceError = sanitizeError(listErr, endpoint).Error()
				break
			}
			result.Resources = append(result.Resources, model.MCPResource{URI: resource.URI, Name: resource.Name, Title: resource.Title, Description: resource.Description, MimeType: resource.MIMEType})
		}
	} else {
		result.ResourceError = "resources are not advertised by this server"
	}
	if has("resources") {
		for template, listErr := range session.ResourceTemplates(ctx, nil) {
			if listErr != nil {
				result.ResourceError = sanitizeError(listErr, endpoint).Error()
				break
			}
			result.ResourceTemplates = append(result.ResourceTemplates, model.MCPResourceTemplate{URITemplate: template.URITemplate, Name: template.Name, Title: template.Title, Description: template.Description, MimeType: template.MIMEType})
		}
	}
	if has("prompts") {
		for prompt, listErr := range session.Prompts(ctx, nil) {
			if listErr != nil {
				result.PromptError = sanitizeError(listErr, endpoint).Error()
				break
			}
			result.Prompts = append(result.Prompts, convertPrompt(prompt))
		}
	} else {
		result.PromptError = "prompts are not advertised by this server"
	}
	result.Logs = resultLogs
	return nil
}

// InspectLaunch connects to a launch target that is not present in a current
// config file, such as an MCP entry detached by Larder.
func InspectLaunch(ctx context.Context, target model.MCPInspectTarget, clientVersion string) (model.MCPInspection, error) {
	entry := &client{name: target.Name, transport: target.Transport, command: target.Command, args: target.Args, url: target.URL, enabled: true, agent: model.MCPAgent{Label: target.AgentLabel, Path: target.AgentPath, Scope: target.Scope, Enabled: true}}
	result := model.MCPInspection{ServerID: target.ServerID, Name: target.Name, Transport: target.Transport, AgentLabel: target.AgentLabel, AgentPath: target.AgentPath, Disabled: target.Disabled}
	if err := inspectTarget(ctx, entry, clientVersion, &result); err != nil {
		return result, err
	}
	return result, nil
}

// ReadResource opens a short-lived MCP session and reads one resource.
func ReadResource(ctx context.Context, serverID string, projects []string, uri, clientVersion string) (model.MCPValueResult, error) {
	target, _, err := findInspectTarget(serverID, projects)
	if err != nil {
		return model.MCPValueResult{}, err
	}
	return readResourceTarget(ctx, target, uri, clientVersion)
}

// ReadResourceLaunch reads a resource from a detached launch target.
func ReadResourceLaunch(ctx context.Context, target model.MCPInspectTarget, uri, clientVersion string) (model.MCPValueResult, error) {
	return readResourceTarget(ctx, launchClient(target), uri, clientVersion)
}

func readResourceTarget(ctx context.Context, target *client, uri, clientVersion string) (model.MCPValueResult, error) {
	session, cleanup, err := connectTarget(ctx, target, clientVersion)
	if err != nil {
		return model.MCPValueResult{}, explainConnectionError(sanitizeError(err, target.url))
	}
	defer func() {
		session.Close()
		if cleanup != nil {
			cleanup()
		}
	}()
	result, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: uri})
	if err != nil {
		return model.MCPValueResult{}, sanitizeError(err, target.url)
	}
	raw, _ := json.Marshal(result.Contents)
	return model.MCPValueResult{Content: string(raw)}, nil
}

// GetPrompt opens a short-lived MCP session and renders one prompt.
func GetPrompt(ctx context.Context, serverID string, projects []string, name string, arguments map[string]string, clientVersion string) (model.MCPValueResult, error) {
	target, _, err := findInspectTarget(serverID, projects)
	if err != nil {
		return model.MCPValueResult{}, err
	}
	return getPromptTarget(ctx, target, name, arguments, clientVersion)
}

// GetPromptLaunch renders a prompt from a detached launch target.
func GetPromptLaunch(ctx context.Context, target model.MCPInspectTarget, name string, arguments map[string]string, clientVersion string) (model.MCPValueResult, error) {
	return getPromptTarget(ctx, launchClient(target), name, arguments, clientVersion)
}

func getPromptTarget(ctx context.Context, target *client, name string, arguments map[string]string, clientVersion string) (model.MCPValueResult, error) {
	session, cleanup, err := connectTarget(ctx, target, clientVersion)
	if err != nil {
		return model.MCPValueResult{}, explainConnectionError(sanitizeError(err, target.url))
	}
	defer func() {
		session.Close()
		if cleanup != nil {
			cleanup()
		}
	}()
	result, err := session.GetPrompt(ctx, &sdk.GetPromptParams{Name: name, Arguments: arguments})
	if err != nil {
		return model.MCPValueResult{}, sanitizeError(err, target.url)
	}
	raw, _ := json.Marshal(result)
	return model.MCPValueResult{Content: string(raw)}, nil
}

func launchClient(target model.MCPInspectTarget) *client {
	return &client{name: target.Name, transport: target.Transport, command: target.Command, args: target.Args, url: target.URL, enabled: true, agent: model.MCPAgent{Label: target.AgentLabel, Path: target.AgentPath, Scope: target.Scope, Enabled: true}}
}

// CallTool opens a short-lived MCP session and invokes exactly one tool. It is
// never called automatically; the UI must require an explicit user action.
func CallTool(ctx context.Context, serverID string, projects []string, toolName string, arguments map[string]any, clientVersion string) (model.MCPToolCallResult, error) {
	target, _, err := findInspectTarget(serverID, projects)
	if err != nil {
		return model.MCPToolCallResult{}, err
	}
	return callToolTarget(ctx, target, toolName, arguments, clientVersion)
}

// CallToolLaunch calls a tool on a detached launch target.
func CallToolLaunch(ctx context.Context, target model.MCPInspectTarget, toolName string, arguments map[string]any, clientVersion string) (model.MCPToolCallResult, error) {
	entry := &client{name: target.Name, transport: target.Transport, command: target.Command, args: target.Args, url: target.URL, enabled: true, agent: model.MCPAgent{Label: target.AgentLabel, Path: target.AgentPath, Scope: target.Scope, Enabled: true}}
	return callToolTarget(ctx, entry, toolName, arguments, clientVersion)
}

func callToolTarget(ctx context.Context, target *client, toolName string, arguments map[string]any, clientVersion string) (model.MCPToolCallResult, error) {
	session, cleanup, err := connectTarget(ctx, target, clientVersion)
	if err != nil {
		return model.MCPToolCallResult{}, explainConnectionError(sanitizeError(err, target.url))
	}
	defer func() {
		session.Close()
		if cleanup != nil {
			cleanup()
		}
	}()
	result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: toolName, Arguments: arguments})
	if err != nil {
		return model.MCPToolCallResult{}, sanitizeError(err, target.url)
	}
	content, _ := json.Marshal(result.Content)
	structured, _ := json.Marshal(result.StructuredContent)
	return model.MCPToolCallResult{Content: string(content), StructuredContent: string(structured), IsError: result.IsError}, nil
}

func connectTarget(ctx context.Context, target *client, clientVersion string) (*sdk.ClientSession, func(), error) {
	if clientVersion == "" {
		clientVersion = "0.0.0"
	}
	try := func(protocol string) (*sdk.ClientSession, func(), error) {
		transport, cleanup, err := inspectTransport(target)
		if err != nil {
			return nil, nil, err
		}
		client := sdk.NewClient(&sdk.Implementation{Name: "Larder", Version: clientVersion}, nil)
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
		return try("")
	}
	return session, cleanup, err
}

func findInspectTarget(serverID string, projects []string) (*client, configSpec, error) {
	specs := configs()
	for _, project := range projects {
		specs = append(specs, projectConfigs(project)...)
	}
	for _, spec := range specs {
		raw, err := os.ReadFile(spec.path)
		if err != nil {
			continue
		}
		var entries []client
		switch spec.format {
		case "claude-json":
			entries = parseClaudeJSON(raw, spec)
		case "opencode":
			entries = parseOpenCodeJSON(raw, spec)
		case "codex":
			entries = parseCodexTOML(raw, spec)
		default:
			entries = parseJSONConfig(raw, spec)
		}
		for index := range entries {
			entry := &entries[index]
			if hashedID(serverKey(*entry)) == serverID {
				return entry, spec, nil
			}
		}
	}
	return nil, configSpec{}, errors.New("MCP server not found; it may have been disabled or removed from its config")
}

// InspectTarget returns the exact configuration Inspect would use, without
// connecting to or launching the server.
func InspectTarget(serverID string, projects []string) (model.MCPInspectTarget, error) {
	target, spec, err := findInspectTarget(serverID, projects)
	if err != nil {
		return model.MCPInspectTarget{}, err
	}
	return model.MCPInspectTarget{
		ServerID: serverID, Name: target.name, Transport: target.transport,
		Command: target.command, Args: append([]string(nil), target.args...), URL: target.url,
		EnvKeys: append([]string(nil), target.envKeys...), HeaderKeys: append([]string(nil), target.headerKeys...),
		AgentLabel: target.agent.Label, AgentPath: target.agent.Path, Scope: target.agent.Scope,
		Project: spec.scope != "" && spec.scope != "global", Disabled: !target.enabled,
	}, nil
}

func inspectTransport(target *client) (sdk.Transport, func(), error) {
	transport := strings.ToLower(strings.TrimSpace(target.transport))
	if transport == "remote" || transport == "streamable" || transport == "streamablehttp" {
		transport = "http"
	}
	switch transport {
	case "http", "streamable-http":
		oauth, oauthCleanup, err := newOAuthHandler(target.url)
		if err != nil {
			return nil, nil, err
		}
		return &sdk.StreamableClientTransport{Endpoint: target.url, HTTPClient: inspectHTTPClient(target), DisableStandaloneSSE: true, OAuthHandler: oauth}, oauthCleanup, nil
	case "sse":
		return &sdk.SSEClientTransport{Endpoint: target.url, HTTPClient: inspectHTTPClient(target)}, nil, nil
	case "stdio", "":
		return newStdioTransport(target)
	default:
		return nil, nil, fmt.Errorf("unsupported MCP transport: %s (url=%t, command=%t)", target.transport, target.url != "", target.command != "")
	}
}

type headerTransport struct {
	headers map[string]string
	host    string
	base    http.RoundTripper
}

func (h *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := h.base
	if base == nil {
		base = http.DefaultTransport
	}
	clone := req.Clone(req.Context())
	if clone.Header == nil {
		clone.Header = make(http.Header)
	}
	if h.host == "" || strings.EqualFold(req.URL.Host, h.host) {
		for key, value := range h.headers {
			switch http.CanonicalHeaderKey(key) {
			case "Content-Type", "Accept", "Mcp-Session-Id":
				continue
			}
			clone.Header.Set(key, value)
		}
	}
	return base.RoundTrip(clone)
}

func inspectHTTPClient(target *client) *http.Client {
	host := ""
	if parsed, err := url.Parse(target.url); err == nil {
		host = parsed.Host
	}
	return &http.Client{
		Transport: &headerTransport{headers: target.headers, host: host},
		Timeout:   20 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Do not forward configured secrets to a redirect target.
			return http.ErrUseLastResponse
		},
	}
}

func convertTool(tool *sdk.Tool) model.MCPTool {
	if tool == nil {
		return model.MCPTool{}
	}
	result := model.MCPTool{Name: tool.Name, Title: tool.Title, Description: tool.Description, InputSchema: tool.InputSchema, OutputSchema: tool.OutputSchema}
	if tool.Annotations != nil {
		result.Annotations = tool.Annotations
	}
	return result
}

func convertPrompt(prompt *sdk.Prompt) model.MCPPrompt {
	if prompt == nil {
		return model.MCPPrompt{}
	}
	result := model.MCPPrompt{Name: prompt.Name, Title: prompt.Title, Description: prompt.Description}
	for _, argument := range prompt.Arguments {
		result.Arguments = append(result.Arguments, model.MCPPromptArgument{Name: argument.Name, Description: argument.Description, Required: argument.Required})
	}
	return result
}

func sanitizeError(err error, endpoint string) error {
	if err == nil {
		return nil
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		clean := *urlErr
		clean.URL = sanitizedURL(urlErr.URL)
		clean.Err = errors.New(redactEndpoint(urlErr.Err.Error(), endpoint))
		return &clean
	}
	return errors.New(redactEndpoint(err.Error(), endpoint))
}

func sanitizedURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func redactEndpoint(message, endpoint string) string {
	if endpoint == "" {
		return message
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return message
	}
	clean := sanitizedURL(endpoint)
	message = strings.ReplaceAll(message, endpoint, clean)
	message = strings.ReplaceAll(message, parsed.Redacted(), clean)
	message = strings.ReplaceAll(message, parsed.String(), clean)
	if parsed.RawQuery != "" {
		message = strings.ReplaceAll(message, parsed.RawQuery, "")
	}
	if parsed.User != nil {
		message = strings.ReplaceAll(message, parsed.User.String()+"@", "")
	}
	return message
}

// lockedBuffer keeps only the tail of noisy server output.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.b.Write(data)
	if b.b.Len() > 64<<10 {
		tail := append([]byte(nil), b.b.Bytes()[b.b.Len()-(64<<10):]...)
		b.b.Reset()
		_, _ = b.b.Write(tail)
	}
	return n, err
}
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }
func (b *lockedBuffer) Len() int       { b.mu.Lock(); defer b.mu.Unlock(); return b.b.Len() }

// processConnection owns a stdio child process so Close kills the process
// group rather than leaving npx/node children behind.
type processConnection struct {
	sdk.Connection
	cmd      *exec.Cmd
	stderr   *lockedBuffer
	closeOne sync.Once
}

func (p *processConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	message, err := p.Connection.Read(ctx)
	if err != nil && p.stderr.Len() > 0 {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(p.stderr.String()))
	}
	return message, err
}

func (p *processConnection) Close() error {
	var err error
	p.closeOne.Do(func() {
		err = p.Connection.Close()
		killProcess(p.cmd)
	})
	if err != nil && p.stderr.Len() > 0 {
		return fmt.Errorf("close MCP process: %w: %s", err, strings.TrimSpace(p.stderr.String()))
	}
	return err
}

type processTransport struct {
	transport sdk.Transport
	cmd       *exec.Cmd
	stderr    *lockedBuffer
}

func (p *processTransport) Connect(ctx context.Context) (sdk.Connection, error) {
	conn, err := p.transport.Connect(ctx)
	if err != nil {
		killProcess(p.cmd)
		return nil, err
	}
	return &processConnection{Connection: conn, cmd: p.cmd, stderr: p.stderr}, nil
}

func newStdioTransport(target *client) (sdk.Transport, func(), error) {
	home, _ := os.UserHomeDir()
	cwd := target.agent.Scope
	if cwd != "" {
		if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
			cwd = ""
		}
	}
	commandPath := resolveCommand(target.command, home, cwd)
	args := make([]string, 0, len(target.args))
	for _, arg := range target.args {
		args = append(args, expandArg(arg, home))
	}
	command := platform.NewCommand(commandPath, args...)
	command.Env = mcpEnv(os.Environ(), filepath.Dir(commandPath), platform.LocalPaths{}.PathDirs(home))
	for key, value := range target.env {
		command.Env = append(command.Env, key+"="+value)
	}
	if cwd != "" {
		command.Dir = cwd
	}
	prepareProcess(command)
	stderr := &lockedBuffer{}
	command.Stderr = stderr
	transport := &processTransport{
		transport: &sdk.CommandTransport{Command: command},
		cmd:       command,
		stderr:    stderr,
	}
	return transport, func() { killProcess(command) }, nil
}

func mcpEnv(base []string, commandDir string, extraDirs []string) []string {
	pathDirs := []string{}
	if commandDir != "" && commandDir != "." {
		pathDirs = append(pathDirs, commandDir)
	}
	pathDirs = append(pathDirs, extraDirs...)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.TrimSpace(dir) != "" {
			pathDirs = append(pathDirs, dir)
		}
	}
	seen := map[string]bool{}
	unique := pathDirs[:0]
	for _, dir := range pathDirs {
		dir = filepath.Clean(dir)
		if dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		unique = append(unique, dir)
	}
	env := make([]string, 0, len(base)+1)
	for _, entry := range base {
		if equalFoldKey(entry, "PATH") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "PATH="+strings.Join(unique, string(os.PathListSeparator)))
	return env
}

func equalFoldKey(entry, key string) bool {
	index := strings.IndexByte(entry, '=')
	return index >= 0 && strings.EqualFold(entry[:index], key)
}

func explainConnectionError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		return fmt.Errorf("MCP authentication required (401); check this server's headers or OAuth setup: %s", message)
	case strings.Contains(lower, "403") || strings.Contains(lower, "forbidden"):
		return fmt.Errorf("MCP server refused access (403); check its configured headers: %s", message)
	case strings.Contains(lower, "context deadline") || strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout"):
		return fmt.Errorf("MCP connection timed out; the first run may still be downloading dependencies: %s", message)
	case strings.Contains(lower, "executable file not found") || strings.Contains(lower, "command not found") || strings.Contains(lower, "no such file or directory"):
		return fmt.Errorf("MCP command could not be found; check the configured command or GUI PATH: %s", message)
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "dial tcp"):
		return fmt.Errorf("MCP server refused the connection; check the URL and that the service is running: %s", message)
	case isProtocolError(err):
		return fmt.Errorf("MCP protocol version is not supported by this server: %s", message)
	case strings.Contains(lower, "eof") || strings.Contains(lower, "connection closed") || strings.Contains(lower, "process closed"):
		return fmt.Errorf("MCP server closed the connection during initialization; check stderr and the launch command: %s", message)
	}
	return err
}

// ResolveCommand resolves an MCP command using the desktop PATH fallbacks.
func ResolveCommand(name, cwd string) string {
	home, _ := os.UserHomeDir()
	return resolveCommand(name, home, cwd)
}

func resolveCommand(name, home, cwd string) string {
	name = expandArg(name, home)
	if filepath.IsAbs(name) {
		return name
	}
	if hasPathSeparator(name) && cwd != "" {
		return filepath.Join(cwd, name)
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	for _, dir := range (platform.LocalPaths{}).PathDirs(home) {
		if candidate := findCommandInDir(dir, name); candidate != "" {
			return candidate
		}
	}
	return name
}

func findCommandInDir(dir, name string) string {
	candidates := []string{filepath.Join(dir, name)}
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		for _, ext := range []string{".exe", ".cmd", ".bat", ".ps1", ".com"} {
			candidates = append(candidates, filepath.Join(dir, name+ext))
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func hasPathSeparator(value string) bool {
	return strings.ContainsAny(value, `/\`)
}

func expandArg(value, home string) string {
	value = os.ExpandEnv(value)
	if runtime.GOOS == "windows" {
		value = expandPercentEnv(value)
	}
	if value == "~" {
		return home
	}
	if strings.HasPrefix(value, "~/") || (runtime.GOOS == "windows" && strings.HasPrefix(value, `~\`)) {
		return filepath.Join(home, value[2:])
	}
	return value
}

func expandPercentEnv(value string) string {
	var builder strings.Builder
	for {
		start := strings.IndexByte(value, '%')
		if start < 0 {
			builder.WriteString(value)
			break
		}
		end := strings.IndexByte(value[start+1:], '%')
		if end < 0 {
			builder.WriteString(value)
			break
		}
		builder.WriteString(value[:start])
		builder.WriteString(os.Getenv(value[start+1 : start+1+end]))
		value = value[start+end+2:]
	}
	return builder.String()
}

func isProtocolError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unsupported protocol") || strings.Contains(message, "protocol version") || strings.Contains(message, "server/discover") || strings.Contains(message, "unsupportedprotocolversion")
}

func truncateLog(value string) string {
	const limit = 4000
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
