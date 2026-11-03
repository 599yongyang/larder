package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const registryURL = "https://registry.modelcontextprotocol.io/v0/servers"

var registryHTTP = &http.Client{Timeout: 15 * time.Second}

// RegistryEnv is an environment variable a registry server needs.
type RegistryEnv struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

type RegistryHeader struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

// RegistryServer is a registry entry mapped to a config the installer can
// write. Package and remote entries that cannot be represented are skipped.
type RegistryServer struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	Transport    string           `json:"transport"`
	Command      string           `json:"command,omitempty"`
	Args         []string         `json:"args,omitempty"`
	URL          string           `json:"url,omitempty"`
	Runtime      string           `json:"runtime,omitempty"`
	RequiresAuth bool             `json:"requiresAuth,omitempty"`
	Publisher    string           `json:"publisher,omitempty"`
	Env          []RegistryEnv    `json:"env,omitempty"`
	Headers      []RegistryHeader `json:"headers,omitempty"`
	Unsupported  string           `json:"unsupported,omitempty"`
}

type registryEntry struct {
	Server struct {
		Name        string `json:"name"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Packages    []struct {
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
		} `json:"packages"`
		Remotes []struct {
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
		} `json:"remotes"`
	} `json:"server"`
	Meta struct {
		Official struct {
			IsLatest bool `json:"isLatest"`
		} `json:"io.modelcontextprotocol.registry/official"`
		PublisherProvided struct {
			Publisher string `json:"publisher"`
		} `json:"io.modelcontextprotocol.registry/publisher-provided"`
	} `json:"_meta"`
}

type registryCache struct {
	mu     sync.Mutex
	values map[string]registryCacheEntry
}

type registryCacheEntry struct {
	servers    []RegistryServer
	nextCursor string
	expires    time.Time
}

type RegistryPage struct {
	Servers    []RegistryServer `json:"servers"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

var registrySkills = registryCache{values: map[string]registryCacheEntry{}}

var registryNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// SearchRegistry searches the official MCP Registry and returns entries that
// can be written as an agent MCP config. Servers needing a required HTTP
// header are skipped because headers have no input UI yet.
func SearchRegistry(ctx context.Context, query string) ([]RegistryServer, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, errors.New("search query must be at least 2 characters")
	}
	endpoint := registryURL + "?search=" + url.QueryEscape(query) + "&limit=30&version=latest"
	servers, err := fetchRegistry(ctx, endpoint, query)
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 && len(strings.Fields(query)) > 1 {
		servers, _ = SearchRegistry(ctx, strings.Fields(query)[0])
	}
	return servers, nil
}

func SearchRegistryPage(ctx context.Context, query, cursor string) (RegistryPage, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return RegistryPage{}, errors.New("search query must be at least 2 characters")
	}
	endpoint := registryURL + "?search=" + url.QueryEscape(query) + "&limit=20&version=latest"
	if cursor != "" {
		endpoint += "&cursor=" + url.QueryEscape(cursor)
	}
	return fetchRegistryPage(ctx, endpoint, "page:"+query+":"+cursor)
}

func fetchRegistryPage(ctx context.Context, endpoint, cacheKey string) (RegistryPage, error) {
	registrySkills.mu.Lock()
	if cached, ok := registrySkills.values[cacheKey]; ok && time.Now().Before(cached.expires) {
		registrySkills.mu.Unlock()
		return RegistryPage{Servers: cached.servers, NextCursor: cached.nextCursor}, nil
	}
	registrySkills.mu.Unlock()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return RegistryPage{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "larder")
	response, err := registryHTTP.Do(request)
	if err != nil {
		return RegistryPage{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return RegistryPage{}, fmt.Errorf("MCP Registry returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return RegistryPage{}, err
	}
	var payload struct {
		Servers  []registryEntry `json:"servers"`
		Metadata struct {
			NextCursor string `json:"nextCursor"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return RegistryPage{}, fmt.Errorf("decode MCP Registry: %w", err)
	}
	page := RegistryPage{Servers: mapRegistryEntries(payload.Servers), NextCursor: payload.Metadata.NextCursor}
	registrySkills.mu.Lock()
	registrySkills.values[cacheKey] = registryCacheEntry{servers: page.Servers, nextCursor: page.NextCursor, expires: time.Now().Add(5 * time.Minute)}
	registrySkills.mu.Unlock()
	return page, nil
}

func fetchRegistry(ctx context.Context, endpoint, cacheKey string) ([]RegistryServer, error) {
	registrySkills.mu.Lock()
	if cached, ok := registrySkills.values[cacheKey]; ok && time.Now().Before(cached.expires) {
		registrySkills.mu.Unlock()
		return cached.servers, nil
	}
	registrySkills.mu.Unlock()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "larder")
	response, err := registryHTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP Registry returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Servers []registryEntry `json:"servers"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode MCP Registry: %w", err)
	}
	servers := mapRegistryEntries(payload.Servers)
	registrySkills.mu.Lock()
	registrySkills.values[cacheKey] = registryCacheEntry{servers: servers, expires: time.Now().Add(5 * time.Minute)}
	registrySkills.mu.Unlock()
	return servers, nil
}

var curatedQueries = []string{
	"figma", "notion", "linear", "sentry", "github", "playwright", "context7", "supabase", "stripe", "vercel",
}

// DiscoverRegistry returns live registry entries for well-known vendors. The
// server configs are not hardcoded; only the search terms are curated.
func DiscoverRegistry(ctx context.Context) ([]RegistryServer, error) {
	registrySkills.mu.Lock()
	if cached, ok := registrySkills.values["__curated__"]; ok && time.Now().Before(cached.expires) {
		registrySkills.mu.Unlock()
		return cached.servers, nil
	}
	registrySkills.mu.Unlock()
	type result struct {
		index   int
		servers []RegistryServer
	}
	results := make(chan result, len(curatedQueries))
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for index, query := range curatedQueries {
		index, query := index, query
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			endpoint := registryURL + "?search=" + url.QueryEscape(query) + "&limit=4&version=latest"
			servers, err := fetchRegistry(ctx, endpoint, "curated:"+query)
			if err == nil {
				results <- result{index: index, servers: servers}
			}
		}()
	}
	wg.Wait()
	close(results)
	byName := map[string]RegistryServer{}
	priority := map[string]int{}
	for item := range results {
		for _, server := range item.servers {
			key := strings.ToLower(server.Name)
			if existing, ok := byName[key]; ok {
				if existing.URL == "" && server.URL != "" {
					byName[key] = server
				}
				continue
			}
			byName[key] = server
			priority[key] = item.index
		}
	}
	servers := make([]RegistryServer, 0, len(byName))
	for _, server := range byName {
		servers = append(servers, server)
	}
	sort.Slice(servers, func(i, j int) bool {
		pi, pj := priority[strings.ToLower(servers[i].Name)], priority[strings.ToLower(servers[j].Name)]
		if pi == pj {
			return servers[i].Name < servers[j].Name
		}
		return pi < pj
	})
	if len(servers) > 48 {
		servers = servers[:48]
	}
	registrySkills.mu.Lock()
	registrySkills.values["__curated__"] = registryCacheEntry{servers: servers, expires: time.Now().Add(10 * time.Minute)}
	registrySkills.mu.Unlock()
	return servers, nil
}

func mapRegistryEntries(entries []registryEntry) []RegistryServer {
	latest := map[string]registryEntry{}
	order := []string{}
	for _, entry := range entries {
		name := strings.ToLower(entry.Server.Name)
		if name == "" || strings.HasPrefix(name, "ai.smithery/") || strings.Contains(name, "smithery") {
			continue
		}
		if _, exists := latest[name]; !exists {
			order = append(order, name)
			latest[name] = entry
			continue
		}
		if entry.Meta.Official.IsLatest && !latest[name].Meta.Official.IsLatest {
			latest[name] = entry
		}
	}
	seen := map[string]bool{}
	out := make([]RegistryServer, 0, len(order))
	for _, name := range order {
		entry := latest[name]
		item, ok := mapRegistryEntry(entry)
		if !ok {
			continue
		}
		key := strings.ToLower(item.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftSupported := out[i].Unsupported == ""
		rightSupported := out[j].Unsupported == ""
		if leftSupported != rightSupported {
			return leftSupported
		}
		leftPackage := out[i].Command != ""
		rightPackage := out[j].Command != ""
		if leftPackage != rightPackage {
			return leftPackage
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

func registryPublisher(entry registryEntry) string {
	if value := strings.TrimSpace(entry.Meta.PublisherProvided.Publisher); value != "" {
		return value
	}
	name := strings.ToLower(entry.Server.Name)
	namespace := name
	if index := strings.Index(name, "/"); index >= 0 {
		namespace = name[:index]
	}
	parts := strings.Split(namespace, ".")
	if len(parts) >= 3 && parts[0] == "io" && parts[1] == "github" {
		return titleCase(parts[2])
	}
	if len(parts) >= 2 && len(parts[0]) <= 3 {
		return titleCase(parts[1])
	}
	if len(parts) > 0 {
		return titleCase(parts[0])
	}
	return ""
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func mapRegistryEntry(entry registryEntry) (RegistryServer, bool) {
	server := entry.Server
	name := server.Name
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}
	name = registryNamePattern.ReplaceAllString(name, "-")
	name = strings.Trim(name, ".-")
	if name == "" {
		return RegistryServer{}, false
	}
	title := server.Title
	if title == "" {
		title = name
	}
	item := RegistryServer{ID: server.Name, Name: name, Title: title, Description: server.Description, Publisher: registryPublisher(entry)}

	for _, pkg := range server.Packages {
		command, args := packageCommand(pkg)
		if command == "" {
			continue
		}
		item.Transport = "stdio"
		item.Command = command
		item.Args = args
		item.Runtime = pkg.RuntimeHint
		if item.Runtime == "" {
			item.Runtime = command
		}
		for _, env := range pkg.Environment {
			item.Env = append(item.Env, RegistryEnv{Name: env.Name, Description: env.Description, Required: env.IsRequired, Secret: env.IsSecret})
		}
		return item, true
	}
	if len(server.Packages) > 0 {
		item.Unsupported = "不支持的包类型"
		return item, true
	}
	if len(server.Remotes) == 0 {
		item.Unsupported = "没有可写入的启动方式"
		return item, true
	}
	for _, remote := range server.Remotes {
		if remote.URL == "" {
			continue
		}
		requiredVariable := false
		for _, variable := range remote.Variables {
			if variable.IsRequired {
				requiredVariable = true
				break
			}
		}
		item.Transport = "http"
		item.Runtime = "remote"
		for _, header := range remote.Headers {
			item.Headers = append(item.Headers, RegistryHeader{Name: header.Name, Description: header.Description, Required: header.IsRequired, Secret: header.IsSecret})
			if header.IsRequired {
				item.RequiresAuth = true
			}
		}
		if requiredVariable {
			item.Unsupported = "需要 URL 变量"
			return item, true
		}
		item.URL = remote.URL
		return item, true
	}
	return RegistryServer{}, false
}

func packageCommand(pkg struct {
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
}) (string, []string) {
	command := pkg.RuntimeHint
	switch strings.ToLower(pkg.RegistryType) {
	case "npm":
		if command == "" {
			command = "npx"
		}
	case "pypi":
		if command == "" {
			command = "uvx"
		}
	case "oci", "docker":
		if command == "" {
			command = "docker"
		}
	default:
		return "", nil
	}
	args := make([]string, 0, len(pkg.RuntimeArgs)+4)
	for _, arg := range pkg.RuntimeArgs {
		if arg.Value != "" {
			args = append(args, arg.Value)
		}
	}
	if strings.EqualFold(pkg.RegistryType, "npm") && !contains(args, "-y") {
		args = append([]string{"-y"}, args...)
	}
	if strings.EqualFold(pkg.RegistryType, "oci") || strings.EqualFold(pkg.RegistryType, "docker") {
		if !contains(args, "run") {
			args = append([]string{"run", "-i", "--rm"}, args...)
		}
	}
	identifier := pkg.Identifier
	if identifier == "" {
		return "", nil
	}
	if pkg.Version != "" && !strings.Contains(identifier, "@") {
		identifier += "@" + pkg.Version
	}
	args = append(args, identifier)
	return command, args
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
