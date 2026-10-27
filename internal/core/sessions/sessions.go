// Package sessions reads read-only metadata from local Agent session files.
// It intentionally does not return transcript bodies; callers only receive
// enough to list a project's recent sessions.
package sessions

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	_ "modernc.org/sqlite"

	"larder/internal/core/model"
)

const (
	maxSessions        = 50
	maxAgentSessions   = 500
	maxAgentCandidates = 500
	maxClaudeFiles     = 40
	maxCodexFiles      = 120
	maxLines           = 4000
	maxLineSize        = 8 << 20
	maxWorkers         = 6
	maxCodexIndexFiles = 2500
	codexIndexTTL      = 60 * time.Second
)

// Scan returns recent sessions whose working directory is projectPath.
// An empty projectPath scans every project: AgentSession uses that.
func Scan(ctx context.Context, home, projectPath string) []model.Session {
	return scan(ctx, home, projectPath, maxSessions)
}

// ScanAgent returns recent sessions for one Agent across every project.
func ScanAgent(ctx context.Context, home, agent string) []model.Session {
	var found []model.Session
	switch agent {
	case "Claude Code":
		found = scanClaude(ctx, home, "")
	case "DSH":
		found = scanDSH(ctx, home, "")
	case "Codex":
		found = scanCodex(ctx, home, "")
	case "OpenCode":
		found = scanOpenCode(ctx, home, "")
	case "Cursor":
		found = scanCursor(ctx, home, "")
	case "Cline":
		found = scanCline(ctx, home, "")
	case "Goose":
		found = scanGoose(ctx, home, "")
	case "OMP":
		found = scanOMP(ctx, home, "")
	case "WorkBuddy":
		found = scanWorkBuddy(ctx, home, "")
	case "Qoder":
		found = scanQoder(ctx, home, "")
	case "CodeBuddy":
		found = scanCodeBuddy(ctx, home, "")
	case "Gemini CLI":
		found = scanGemini(ctx, home, "")
	}
	return finalizeSessions(found, maxAgentSessions)
}

func scan(ctx context.Context, home, projectPath string, limit int) []model.Session {
	if projectPath != "" {
		projectPath = filepath.Clean(projectPath)
	}
	var found []model.Session
	found = append(found, scanClaude(ctx, home, projectPath)...)
	found = append(found, scanDSH(ctx, home, projectPath)...)
	found = append(found, scanCodex(ctx, home, projectPath)...)
	found = append(found, scanOpenCode(ctx, home, projectPath)...)
	found = append(found, scanCursor(ctx, home, projectPath)...)
	found = append(found, scanCline(ctx, home, projectPath)...)
	found = append(found, scanGoose(ctx, home, projectPath)...)
	found = append(found, scanOMP(ctx, home, projectPath)...)
	found = append(found, scanWorkBuddy(ctx, home, projectPath)...)
	found = append(found, scanQoder(ctx, home, projectPath)...)
	found = append(found, scanCodeBuddy(ctx, home, projectPath)...)
	found = append(found, scanGemini(ctx, home, projectPath)...)
	return finalizeSessions(found, limit)
}

func finalizeSessions(found []model.Session, limit int) []model.Session {
	sort.SliceStable(found, func(i, j int) bool { return found[i].LastAt.After(found[j].LastAt) })
	seen := map[string]bool{}
	unique := found[:0]
	for _, session := range found {
		if session.Messages == 0 {
			continue
		}
		key := session.Agent + "\x00" + session.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		if isTranscriptPath(session.Path) {
			if info, err := os.Stat(session.Path); err == nil && info.ModTime().After(session.LastAt) {
				session.LastAt = info.ModTime()
			}
		}
		session.Title = usableTitle(session.Title)
		if weakTitle(session.Title) || session.Title == session.ID {
			session.Title = fallbackTitle(session)
		}
		unique = append(unique, session)
	}
	found = unique
	if len(found) > limit {
		found = found[:limit]
	}
	return found
}

func scanClaude(ctx context.Context, home, projectPath string) []model.Session {
	return scanClaudeLike(ctx, projectPath, "Claude Code", filepath.Join(home, ".claude", "projects"))
}

func scanQoder(ctx context.Context, home, projectPath string) []model.Session {
	out := scanClaudeLike(ctx, projectPath, "Qoder", filepath.Join(home, ".qoder", "projects"))
	out = append(out, scanClaudeLike(ctx, projectPath, "Qoder", filepath.Join(home, ".qoder-cn", "projects"))...)
	return out
}

// ToolUsage is aggregate evidence of Skills and MCP servers used by an Agent.
// It is derived from tool-call records only; transcript text is never returned.
type ToolUsage struct {
	Skills map[string]int
	MCP    map[string]int
	LastAt time.Time
}

var skillPathPattern = regexp.MustCompile(`(?:^|[/\\])skills[/\\]([^/\\\s"']+)[/\\]SKILL\.md`)

func ReadToolUsage(path string) (ToolUsage, error) {
	file, err := openSessionFile(path)
	if err != nil {
		return ToolUsage{}, err
	}
	defer file.Close()
	usage := ToolUsage{Skills: map[string]int{}, MCP: map[string]int{}}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for lines := 0; scanner.Scan() && lines < maxLines*25; lines++ {
		var line struct {
			Type      string `json:"type"`
			Time      int64  `json:"time"`
			Timestamp string `json:"timestamp"`
			Message   struct {
				Content []struct {
					Type  string          `json:"type"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				} `json:"content"`
			} `json:"message"`
			Payload struct {
				Type      string          `json:"type"`
				Name      string          `json:"name"`
				Arguments string          `json:"arguments"`
				Input     json.RawMessage `json:"input"`
			} `json:"payload"`
			Data struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		at := time.Time{}
		if line.Time > 0 {
			at = time.UnixMilli(line.Time)
		} else if parsed, parseErr := time.Parse(time.RFC3339, line.Timestamp); parseErr == nil {
			at = parsed
		}
		if at.After(usage.LastAt) {
			usage.LastAt = at
		}
		tool := func(name, raw string) {
			if name == "Skill" || name == "skill" {
				var input struct {
					Skill string `json:"skill"`
				}
				if json.Unmarshal([]byte(raw), &input) == nil && input.Skill != "" {
					usage.Skills[input.Skill]++
				}
				return
			}
			if strings.HasPrefix(name, "mcp__") {
				parts := strings.Split(name, "__")
				if len(parts) >= 2 && parts[1] != "" {
					usage.MCP[parts[1]]++
				}
			}
			for _, match := range skillPathPattern.FindAllStringSubmatch(raw, -1) {
				if len(match) > 1 {
					usage.Skills[match[1]]++
				}
			}
		}
		switch line.Type {
		case "assistant":
			for _, part := range line.Message.Content {
				if part.Type != "tool_use" || part.Name == "" {
					continue
				}
				tool(part.Name, string(part.Input))
			}
		case "response_item":
			if line.Payload.Name != "" {
				tool(line.Payload.Name, line.Payload.Arguments)
			}
			if len(line.Payload.Input) > 0 {
				tool(line.Payload.Name, string(line.Payload.Input))
			}
		case "tool/call":
			tool(line.Data.Name, line.Data.Arguments)
		}
	}
	if err := scanner.Err(); err != nil {
		return usage, err
	}
	return usage, nil
}

// ReadMessages returns a short, on-demand preview of transcript text. It never
// writes to the source file and never exposes tool arguments or results.
func ReadMessages(path string, limit int) ([]model.SessionMessage, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	file, err := openSessionFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	messages := make([]model.SessionMessage, 0, limit)
	for lines := 0; scanner.Scan() && lines < maxLines*25; lines++ {
		var line struct {
			Type      string          `json:"type"`
			Role      string          `json:"role"`
			Timestamp string          `json:"timestamp"`
			Time      int64           `json:"time"`
			Content   json.RawMessage `json:"content"`
			Message   struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			Payload struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"payload"`
			Data struct {
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		role := line.Role
		var content json.RawMessage
		switch line.Type {
		case "user/message":
			role, content = "user", line.Data.Message.Content
		case "assistant/message":
			role, content = "assistant", line.Data.Message.Content
		case "user", "assistant":
			role, content = line.Type, line.Message.Content
			if len(content) == 0 {
				content = line.Content
			}
		case "response_item":
			role, content = line.Payload.Role, line.Payload.Content
		case "message":
			role, content = line.Role, line.Content
			if len(content) == 0 {
				content = line.Message.Content
			}
		}
		if role != "user" && role != "assistant" {
			continue
		}
		text := sessionText(content)
		if text == "" {
			continue
		}
		at := time.Time{}
		if line.Time > 0 {
			at = time.UnixMilli(line.Time)
		} else if parsed, parseErr := time.Parse(time.RFC3339, line.Timestamp); parseErr == nil {
			at = parsed
		}
		messages = append(messages, model.SessionMessage{Role: role, Text: text, At: at})
		if len(messages) > limit {
			messages = messages[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, errors.New("no readable messages in this session format")
	}
	return messages, nil
}

func sessionText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return clipSessionText(text)
		}
		return ""
	}
	if raw[0] == '[' {
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &parts) != nil {
			return ""
		}
		var builder strings.Builder
		for _, part := range parts {
			if part.Type != "text" && part.Type != "input_text" && part.Type != "output_text" {
				continue
			}
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(part.Text)
		}
		return clipSessionText(builder.String())
	}
	var nested struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &nested) == nil {
		return clipSessionText(nested.Text)
	}
	return ""
}

func clipSessionText(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	runes := []rune(value)
	if len(runes) > 2000 {
		return string(runes[:2000]) + "…"
	}
	return value
}

func scanDSH(ctx context.Context, home, projectPath string) []model.Session {
	root := filepath.Join(home, ".dsh", "sessions")
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil
	}
	var paths []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "session.v4.jsonl.zstd" {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		paths = append(paths, path)
		return nil
	})
	sort.Slice(paths, func(i, j int) bool {
		left, leftErr := os.Stat(paths[i])
		right, rightErr := os.Stat(paths[j])
		if leftErr != nil || rightErr != nil {
			return false
		}
		return left.ModTime().After(right.ModTime())
	})
	if len(paths) > maxAgentCandidates {
		paths = paths[:maxAgentCandidates]
	}
	return scanInParallel(ctx, paths, func(path string) (model.Session, bool) {
		return readSessionCached(path+"\x00"+projectPath, path, func() (model.Session, bool) { return readDSH(path, projectPath) })
	})
}

func readDSH(path, projectPath string) (model.Session, bool) {
	file, err := openSessionFile(path)
	if err != nil {
		return model.Session{}, false
	}
	defer file.Close()
	session := model.Session{ID: filepath.Base(filepath.Dir(path)), Agent: "DSH", Path: path}
	title := ""
	fallbackTitle := ""
	messages := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for lines := 0; scanner.Scan() && lines < maxLines*25; lines++ {
		var line struct {
			Type      string `json:"type"`
			Time      int64  `json:"time"`
			ID        string `json:"id"`
			CWD       string `json:"cwd"`
			CreatedAt int64  `json:"createdAt"`
			Origin    string `json:"origin"`
			Data      struct {
				Title  string `json:"title"`
				Model  string `json:"model"`
				Header struct {
					Config struct {
						Model string `json:"model"`
					} `json:"config"`
				} `json:"header"`
				Message struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		if line.Time > 0 {
			at := time.UnixMilli(line.Time)
			if session.StartedAt.IsZero() || at.Before(session.StartedAt) {
				session.StartedAt = at
			}
			if at.After(session.LastAt) {
				session.LastAt = at
			}
		}
		switch line.Type {
		case "session":
			if line.ID != "" {
				session.ID = line.ID
			}
			if line.CWD != "" {
				session.CWD = line.CWD
			}
			if line.CreatedAt > 0 {
				session.StartedAt = time.UnixMilli(line.CreatedAt)
			}
			if line.Origin == "subagent" {
				return model.Session{}, false
			}
		case "session/title":
			if next := usableTitle(line.Data.Title); next != "" {
				title = next
			}
		case "request/header":
			if line.Data.Header.Config.Model != "" {
				session.Model = line.Data.Header.Config.Model
			} else if line.Data.Model != "" {
				session.Model = line.Data.Model
			}
		case "user/message":
			messages++
			for _, part := range line.Data.Message.Content {
				if part.Type != "text" {
					continue
				}
				text := usableTitle(part.Text)
				if text != "" && !weakTitle(text) && fallbackTitle == "" {
					fallbackTitle = text
				}
			}
		case "assistant/message":
			messages++
		}
	}
	if messages == 0 || session.CWD == "" {
		return model.Session{}, false
	}
	if title == "" {
		title = fallbackTitle
	}
	if title == "" {
		title = session.ID
	}
	if projectPath != "" && filepath.Clean(session.CWD) != projectPath {
		return model.Session{}, false
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = session.LastAt
	}
	if info, statErr := os.Stat(path); statErr == nil {
		if info.ModTime().After(session.LastAt) {
			session.LastAt = info.ModTime()
		}
	}
	session.Title = title
	session.Messages = messages
	return session, true
}

func scanClaudeLike(ctx context.Context, projectPath, agent, projectsRoot string) []model.Session {
	if projectPath == "" {
		var paths []string
		_ = filepath.WalkDir(projectsRoot, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".jsonl") {
				paths = append(paths, path)
			}
			return nil
		})
		return scanInParallel(ctx, newestPaths(paths, maxAgentCandidates), func(path string) (model.Session, bool) {
			return readSessionCached(path+"\x00"+agent, path, func() (model.Session, bool) { return readClaudeAs(path, "", agent) })
		})
	}
	root := filepath.Join(projectsRoot, strings.ReplaceAll(projectPath, "/", "-"))
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		left, leftErr := entries[i].Info()
		right, rightErr := entries[j].Info()
		if leftErr != nil || rightErr != nil {
			return entries[i].Name() > entries[j].Name()
		}
		return left.ModTime().After(right.ModTime())
	})
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(root, entry.Name()))
		if len(paths) >= maxClaudeFiles {
			break
		}
	}
	return scanInParallel(ctx, paths, func(path string) (model.Session, bool) {
		return readClaudeAs(path, projectPath, agent)
	})
}

type claudeLine struct {
	Type      string `json:"type"`
	AITitle   string `json:"aiTitle"`
	CWD       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Message   struct {
		Role    string          `json:"role"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func readClaudeAs(path, projectPath, agent string) (model.Session, bool) {
	file, err := os.Open(path)
	if err != nil {
		return model.Session{}, false
	}
	defer file.Close()
	session := model.Session{ID: strings.TrimSuffix(filepath.Base(path), ".jsonl"), Agent: agent, Path: path}
	userTitle, assistantTitle, aiTitle := "", "", ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for lines := 0; scanner.Scan() && lines < maxLines; lines++ {
		var line claudeLine
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		if line.CWD != "" {
			session.CWD = line.CWD
		}
		if line.SessionID != "" {
			session.ID = line.SessionID
		}
		if at, err := time.Parse(time.RFC3339, line.Timestamp); err == nil {
			if session.StartedAt.IsZero() || at.Before(session.StartedAt) {
				session.StartedAt = at
			}
			if at.After(session.LastAt) {
				session.LastAt = at
			}
		}
		if line.Type == "ai-title" && line.AITitle != "" {
			aiTitle = usableTitle(line.AITitle)
		}
		if line.Type == "assistant" && line.Message.Model != "" {
			session.Model = line.Message.Model
		}
		if line.Type == "user" || line.Type == "assistant" {
			session.Messages++
			session.TokensIn += line.Message.Usage.InputTokens
			session.TokensOut += line.Message.Usage.OutputTokens
			text := usableTitle(firstText(line.Message.Content))
			if line.Type == "user" && userTitle == "" && !weakTitle(text) {
				userTitle = text
			}
			if line.Type == "assistant" && assistantTitle == "" && text != "" {
				assistantTitle = text
			}
		}
	}
	session.Title = firstNonEmpty(aiTitle, userTitle, assistantTitle)
	if session.CWD == "" && session.StartedAt.IsZero() {
		return model.Session{}, false
	}
	if session.Title == "" {
		session.Title = sessionFileID(path)
	}
	if session.LastAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			session.LastAt = info.ModTime()
		}
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = session.LastAt
	}
	return session, conditionPath(session.CWD, projectPath)
}

func scanCodex(ctx context.Context, home, projectPath string) []model.Session {
	paths := codexPathsForProject(ctx, home, projectPath)
	return scanInParallel(ctx, paths, func(path string) (model.Session, bool) {
		if projectPath == "" {
			return readSessionCached(path, path, func() (model.Session, bool) { return readCodex(path, "") })
		}
		cwd, source, ok := readCodexMeta(path)
		if !ok || codexInternalThread(source) || filepath.Clean(cwd) != projectPath {
			return model.Session{}, false
		}
		return readSessionCached(path+"\x00"+projectPath, path, func() (model.Session, bool) { return readCodex(path, projectPath) })
	})
}

type codexIndex struct {
	root      string
	builtAt   time.Time
	byProject map[string][]candidatePath
}

type candidatePath struct {
	path string
	mod  time.Time
}

var codexProjectIndex struct {
	sync.Mutex
	entry codexIndex
}

func codexPathsForProject(ctx context.Context, home, projectPath string) []string {
	root := filepath.Join(home, ".codex")
	codexProjectIndex.Lock()
	defer codexProjectIndex.Unlock()
	entry := codexProjectIndex.entry
	if entry.root != root || time.Since(entry.builtAt) > codexIndexTTL {
		entry = buildCodexIndex(ctx, home)
		codexProjectIndex.entry = entry
	}
	paths := make([]string, 0)
	if projectPath == "" {
		candidates := make([]candidatePath, 0)
		for _, projectCandidates := range entry.byProject {
			candidates = append(candidates, projectCandidates...)
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].mod.After(candidates[j].mod) })
		if len(candidates) > maxAgentCandidates {
			candidates = candidates[:maxAgentCandidates]
		}
		for _, candidate := range candidates {
			paths = append(paths, candidate.path)
		}
		return paths
	}
	candidates := entry.byProject[projectPath]
	for _, candidate := range candidates {
		paths = append(paths, candidate.path)
	}
	return paths
}

func buildCodexIndex(ctx context.Context, home string) codexIndex {
	entry := codexIndex{root: filepath.Join(home, ".codex"), builtAt: time.Now(), byProject: map[string][]candidatePath{}}
	type codexCandidate struct {
		path string
		mod  time.Time
	}
	var candidates []codexCandidate
	for _, root := range []string{filepath.Join(home, ".codex", "sessions"), filepath.Join(home, ".codex", "archived_sessions")} {
		_ = filepath.WalkDir(root, func(path string, item os.DirEntry, err error) error {
			if err != nil || item.IsDir() || !(strings.HasSuffix(path, ".jsonl") || strings.HasSuffix(path, ".jsonl.zst")) {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			info, infoErr := item.Info()
			if infoErr != nil {
				return nil
			}
			candidates = append(candidates, codexCandidate{path: path, mod: info.ModTime()})
			return nil
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].mod.After(candidates[j].mod) })
	if len(candidates) > maxCodexIndexFiles {
		candidates = candidates[:maxCodexIndexFiles]
	}
	sem := make(chan struct{}, maxWorkers)
	var wait sync.WaitGroup
	var mu sync.Mutex
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		wait.Add(1)
		go func(candidate codexCandidate) {
			defer wait.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			cwd, source, ok := readCodexMeta(candidate.path)
			if !ok || cwd == "" || codexInternalThread(source) {
				return
			}
			cwd = filepath.Clean(cwd)
			mu.Lock()
			entry.byProject[cwd] = append(entry.byProject[cwd], candidatePath{path: candidate.path, mod: candidate.mod})
			mu.Unlock()
		}(candidate)
	}
	wait.Wait()
	for cwd := range entry.byProject {
		sort.Slice(entry.byProject[cwd], func(i, j int) bool {
			return entry.byProject[cwd][i].mod.After(entry.byProject[cwd][j].mod)
		})
	}
	return entry
}

type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	Role         string `json:"role"`
	CWD          string `json:"cwd"`
	Model        string `json:"model"`
	ThreadSource string `json:"thread_source"`
	Content      []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	ThreadTokenUsage *codexUsage `json:"thread_token_usage"`
	Usage            *codexUsage `json:"usage"`
}

type codexUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type sessionReadCloser struct {
	io.Reader
	close func() error
}

func (r sessionReadCloser) Close() error { return r.close() }

type cachedSessionParse struct {
	mod     time.Time
	size    int64
	session model.Session
	ok      bool
}

type persistedSessionCache struct {
	Version int                              `json:"version"`
	Entries map[string]persistedSessionEntry `json:"entries"`
}

type persistedSessionEntry struct {
	ModNanos int64         `json:"modNanos"`
	Size     int64         `json:"size"`
	Session  model.Session `json:"session"`
	OK       bool          `json:"ok"`
}

var sessionParseCache = struct {
	sync.Mutex
	entries   map[string]cachedSessionParse
	loaded    bool
	loadOnce  sync.Once
	dirty     bool
	saveTimer *time.Timer
	savePath  string
}{entries: map[string]cachedSessionParse{}}

func sessionDiskCachePath() string {
	if sessionParseCache.savePath != "" {
		return sessionParseCache.savePath
	}
	if override := strings.TrimSpace(os.Getenv("LARDER_SESSION_CACHE")); override != "" {
		sessionParseCache.savePath = override
		return override
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	sessionParseCache.savePath = filepath.Join(dir, "Larder", "session-cache-v1.json")
	return sessionParseCache.savePath
}

func loadSessionDiskCache() {
	sessionParseCache.loadOnce.Do(func() {
		path := sessionDiskCachePath()
		if path == "" {
			sessionParseCache.loaded = true
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			sessionParseCache.loaded = true
			return
		}
		var cached persistedSessionCache
		if json.Unmarshal(raw, &cached) != nil || cached.Version != 2 {
			sessionParseCache.loaded = true
			return
		}
		sessionParseCache.Lock()
		for key, entry := range cached.Entries {
			sessionParseCache.entries[key] = cachedSessionParse{
				mod:     time.Unix(0, entry.ModNanos),
				size:    entry.Size,
				session: entry.Session,
				ok:      entry.OK,
			}
		}
		sessionParseCache.loaded = true
		sessionParseCache.Unlock()
	})
}

func scheduleSessionDiskSave() {
	sessionParseCache.Lock()
	if sessionParseCache.saveTimer != nil {
		sessionParseCache.Unlock()
		return
	}
	sessionParseCache.saveTimer = time.AfterFunc(2*time.Second, saveSessionDiskCache)
	sessionParseCache.Unlock()
}

func saveSessionDiskCache() {
	sessionParseCache.Lock()
	sessionParseCache.saveTimer = nil
	if !sessionParseCache.dirty {
		sessionParseCache.Unlock()
		return
	}
	sessionParseCache.dirty = false
	entries := make(map[string]persistedSessionEntry, len(sessionParseCache.entries))
	for key, entry := range sessionParseCache.entries {
		entries[key] = persistedSessionEntry{ModNanos: entry.mod.UnixNano(), Size: entry.size, Session: entry.session, OK: entry.ok}
	}
	path := sessionDiskCachePath()
	sessionParseCache.Unlock()
	if path == "" {
		return
	}
	raw, err := json.Marshal(persistedSessionCache{Version: 2, Entries: entries})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(temp, path)
}

func readSessionCached(key, path string, read func() (model.Session, bool)) (model.Session, bool) {
	loadSessionDiskCache()
	info, err := os.Stat(path)
	if err != nil {
		return read()
	}
	sessionParseCache.Lock()
	entry, found := sessionParseCache.entries[key]
	sessionParseCache.Unlock()
	if found && entry.mod.Equal(info.ModTime()) && entry.size == info.Size() {
		return entry.session, entry.ok
	}
	session, ok := read()
	sessionParseCache.Lock()
	sessionParseCache.entries[key] = cachedSessionParse{mod: info.ModTime(), size: info.Size(), session: session, ok: ok}
	sessionParseCache.dirty = true
	sessionParseCache.Unlock()
	scheduleSessionDiskSave()
	return session, ok
}

func openSessionFile(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(path, ".zst") && !strings.HasSuffix(path, ".zstd") {
		return file, nil
	}
	decoder, err := zstd.NewReader(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return sessionReadCloser{Reader: decoder, close: func() error {
		decoder.Close()
		return file.Close()
	}}, nil
}

type cachedCodexMeta struct {
	mod    time.Time
	size   int64
	cwd    string
	source string
	ok     bool
}

type persistedCodexMetaCache struct {
	Version int                                `json:"version"`
	Entries map[string]persistedCodexMetaEntry `json:"entries"`
}

type persistedCodexMetaEntry struct {
	ModNanos int64  `json:"modNanos"`
	Size     int64  `json:"size"`
	CWD      string `json:"cwd"`
	Source   string `json:"source"`
	OK       bool   `json:"ok"`
}

var codexMetaCache = struct {
	sync.Mutex
	entries   map[string]cachedCodexMeta
	loadOnce  sync.Once
	saveTimer *time.Timer
	dirty     bool
	savePath  string
}{entries: map[string]cachedCodexMeta{}}

func codexMetaDiskCachePath() string {
	if codexMetaCache.savePath != "" {
		return codexMetaCache.savePath
	}
	if override := strings.TrimSpace(os.Getenv("LARDER_CODEX_META_CACHE")); override != "" {
		codexMetaCache.savePath = override
		return override
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	codexMetaCache.savePath = filepath.Join(dir, "Larder", "codex-meta-cache-v1.json")
	return codexMetaCache.savePath
}

func loadCodexMetaDiskCache() {
	codexMetaCache.loadOnce.Do(func() {
		path := codexMetaDiskCachePath()
		if path == "" {
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var cached persistedCodexMetaCache
		if json.Unmarshal(raw, &cached) != nil || cached.Version != 1 {
			return
		}
		codexMetaCache.Lock()
		for key, entry := range cached.Entries {
			codexMetaCache.entries[key] = cachedCodexMeta{mod: time.Unix(0, entry.ModNanos), size: entry.Size, cwd: entry.CWD, source: entry.Source, ok: entry.OK}
		}
		codexMetaCache.Unlock()
	})
}

func scheduleCodexMetaDiskSave() {
	codexMetaCache.Lock()
	if codexMetaCache.saveTimer != nil {
		codexMetaCache.Unlock()
		return
	}
	codexMetaCache.saveTimer = time.AfterFunc(2*time.Second, saveCodexMetaDiskCache)
	codexMetaCache.Unlock()
}

func saveCodexMetaDiskCache() {
	codexMetaCache.Lock()
	codexMetaCache.saveTimer = nil
	if !codexMetaCache.dirty {
		codexMetaCache.Unlock()
		return
	}
	codexMetaCache.dirty = false
	entries := make(map[string]persistedCodexMetaEntry, len(codexMetaCache.entries))
	for key, entry := range codexMetaCache.entries {
		entries[key] = persistedCodexMetaEntry{ModNanos: entry.mod.UnixNano(), Size: entry.size, CWD: entry.cwd, Source: entry.source, OK: entry.ok}
	}
	path := codexMetaDiskCachePath()
	codexMetaCache.Unlock()
	if path == "" {
		return
	}
	raw, err := json.Marshal(persistedCodexMetaCache{Version: 1, Entries: entries})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(temp, path)
}

func readCodexMeta(path string) (string, string, bool) {
	loadCodexMetaDiskCache()
	info, err := os.Stat(path)
	if err != nil {
		return readCodexMetaFile(path)
	}
	codexMetaCache.Lock()
	entry, found := codexMetaCache.entries[path]
	codexMetaCache.Unlock()
	if found && entry.mod.Equal(info.ModTime()) && entry.size == info.Size() {
		return entry.cwd, entry.source, entry.ok
	}
	cwd, source, ok := readCodexMetaFile(path)
	codexMetaCache.Lock()
	codexMetaCache.entries[path] = cachedCodexMeta{mod: info.ModTime(), size: info.Size(), cwd: cwd, source: source, ok: ok}
	codexMetaCache.dirty = true
	codexMetaCache.Unlock()
	scheduleCodexMetaDiskSave()
	return cwd, source, ok
}

func readCodexMetaFile(path string) (string, string, bool) {
	file, err := openSessionFile(path)
	if err != nil {
		return "", "", false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 16*1024), 1<<20)
	for lines := 0; scanner.Scan() && lines < 8; lines++ {
		var line codexLine
		if json.Unmarshal(scanner.Bytes(), &line) != nil || line.Type != "session_meta" {
			continue
		}
		var payload codexPayload
		if json.Unmarshal(line.Payload, &payload) == nil && payload.CWD != "" {
			return payload.CWD, payload.ThreadSource, true
		}
		return "", "", false
	}
	return "", "", false
}

func codexInternalThread(source string) bool {
	source = strings.ToLower(source)
	return strings.Contains(source, "guardian") || strings.Contains(source, "review") || strings.Contains(source, "subagent") || strings.Contains(source, "agent_created")
}

func readCodex(path, projectPath string) (model.Session, bool) {
	file, err := openSessionFile(path)
	if err != nil {
		return model.Session{}, false
	}
	defer file.Close()
	session := model.Session{ID: sessionFileID(path), Agent: "Codex", Path: path}
	userTitle, assistantTitle := "", ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for lines := 0; scanner.Scan() && lines < maxLines; lines++ {
		var line codexLine
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		var payload codexPayload
		_ = json.Unmarshal(line.Payload, &payload)
		if at, err := time.Parse(time.RFC3339, line.Timestamp); err == nil {
			if session.StartedAt.IsZero() || at.Before(session.StartedAt) {
				session.StartedAt = at
			}
			if at.After(session.LastAt) {
				session.LastAt = at
			}
		}
		switch line.Type {
		case "token_usage_record":
			usage := payload.ThreadTokenUsage
			if usage == nil {
				usage = payload.Usage
			}
			if usage != nil {
				if usage.InputTokens > session.TokensIn {
					session.TokensIn = usage.InputTokens
				}
				if usage.OutputTokens > session.TokensOut {
					session.TokensOut = usage.OutputTokens
				}
			}
		case "session_meta":
			if payload.CWD != "" {
				session.CWD = payload.CWD
			}
			if payload.ID != "" {
				session.ID = payload.ID
			}
		case "turn_context":
			if payload.Model != "" {
				session.Model = payload.Model
			}
		case "response_item":
			if payload.Role != "user" && payload.Role != "assistant" {
				continue
			}
			session.Messages++
			for _, part := range payload.Content {
				if part.Type != "input_text" && part.Type != "output_text" {
					continue
				}
				text := usableTitle(part.Text)
				if payload.Role == "user" && userTitle == "" && !weakTitle(text) {
					userTitle = text
				}
				if payload.Role == "assistant" && assistantTitle == "" && text != "" {
					assistantTitle = text
				}
			}
		}
	}
	session.Title = firstNonEmpty(userTitle, assistantTitle)
	if session.CWD == "" && session.StartedAt.IsZero() {
		return model.Session{}, false
	}
	if session.Title == "" {
		session.Title = sessionFileID(path)
	}
	if session.LastAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			session.LastAt = info.ModTime()
		}
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = session.LastAt
	}
	if session.Model == "codex-auto-review" {
		return model.Session{}, false
	}
	return session, conditionPath(session.CWD, projectPath)
}

func scanOpenCode(ctx context.Context, home, projectPath string) []model.Session {
	storage := filepath.Join(home, ".local", "share", "opencode", "storage")
	type openCodeSession struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Directory string `json:"directory"`
		Time      struct {
			Created int64 `json:"created"`
			Updated int64 `json:"updated"`
		} `json:"time"`
	}
	projectIDs := map[string]bool{}
	if entries, err := os.ReadDir(filepath.Join(storage, "project")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			raw, readErr := os.ReadFile(filepath.Join(storage, "project", entry.Name()))
			if readErr != nil {
				continue
			}
			var meta struct {
				ID       string `json:"id"`
				Worktree string `json:"worktree"`
			}
			if json.Unmarshal(raw, &meta) == nil && meta.ID != "" && filepath.Clean(meta.Worktree) == projectPath {
				projectIDs[meta.ID] = true
			}
		}
	}
	roots := make([]string, 0, len(projectIDs)*2)
	if len(projectIDs) > 0 {
		for id := range projectIDs {
			roots = append(roots, filepath.Join(storage, "session", id), filepath.Join(storage, "session_v2", id))
		}
	} else {
		for _, folder := range []string{"session", "session_v2"} {
			roots = append(roots, filepath.Join(storage, folder))
		}
	}
	var out []model.Session
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".json") {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			var item openCodeSession
			if json.Unmarshal(raw, &item) != nil || item.Directory == "" || item.ID == "" || (projectPath != "" && filepath.Clean(item.Directory) != projectPath) {
				return nil
			}
			messages := 0
			title := usableTitle(item.Title)
			if strings.HasPrefix(title, "ACP Session ") {
				title = ""
			}
			sessionModel := ""
			tokensIn, tokensOut := 0, 0
			messageDir := filepath.Join(storage, "message", item.ID)
			if entries, readErr := os.ReadDir(messageDir); readErr == nil {
				sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
				for _, message := range entries {
					if message.IsDir() || !strings.HasSuffix(message.Name(), ".json") {
						continue
					}
					messages++
					raw, readErr := os.ReadFile(filepath.Join(messageDir, message.Name()))
					if readErr != nil {
						continue
					}
					var payload struct {
						Role    string `json:"role"`
						Summary struct {
							Title string `json:"title"`
						} `json:"summary"`
						Model struct {
							ModelID string `json:"modelID"`
						} `json:"model"`
						Tokens struct {
							Input  int `json:"input"`
							Output int `json:"output"`
						} `json:"tokens"`
					}
					if json.Unmarshal(raw, &payload) != nil {
						continue
					}
					if title == "" && payload.Role == "user" {
						title = usableTitle(payload.Summary.Title)
					}
					if sessionModel == "" && payload.Model.ModelID != "" {
						sessionModel = payload.Model.ModelID
					}
					if payload.Role == "assistant" {
						tokensIn += payload.Tokens.Input
						tokensOut += payload.Tokens.Output
					}
				}
			}
			if title == "" {
				title = item.ID
			}
			started := time.UnixMilli(item.Time.Created)
			last := time.UnixMilli(item.Time.Updated)
			if item.Time.Created == 0 {
				started = time.Time{}
			}
			if item.Time.Updated == 0 {
				last = time.Time{}
			}
			if info, statErr := os.Stat(path); statErr == nil {
				if started.IsZero() {
					started = info.ModTime()
				}
				if last.IsZero() {
					last = info.ModTime()
				}
			}
			out = append(out, model.Session{ID: item.ID, Agent: "OpenCode", CWD: item.Directory, Title: title, Model: sessionModel, Path: path, StartedAt: started, LastAt: last, Messages: messages, TokensIn: tokensIn, TokensOut: tokensOut})
			return nil
		})
	}
	return out
}

var cursorSlugBad = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func scanCursor(ctx context.Context, home, projectPath string) []model.Session {
	projects := filepath.Join(home, ".cursor", "projects")
	if projectPath == "" {
		var paths []string
		_ = filepath.WalkDir(projects, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".jsonl") {
				paths = append(paths, path)
			}
			return nil
		})
		return scanInParallel(ctx, newestPaths(paths, maxAgentCandidates), func(path string) (model.Session, bool) {
			return readSessionCached(path, path, func() (model.Session, bool) { return readCursor(path) })
		})
	}
	slug := strings.Trim(cursorSlugBad.ReplaceAllString(projectPath, "-"), "-")
	root := filepath.Join(projects, slug, "agent-transcripts")
	var paths []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".jsonl") {
			paths = append(paths, path)
		}
		return nil
	})
	return scanInParallel(ctx, newestPaths(paths, maxClaudeFiles), func(path string) (model.Session, bool) {
		session, ok := readCursor(path)
		if ok {
			session.CWD = projectPath
		}
		return session, ok
	})
}

func readCursor(path string) (model.Session, bool) {
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	file, err := os.Open(path)
	if err != nil {
		return model.Session{}, false
	}
	session := model.Session{ID: id, Agent: "Cursor", Path: path}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for lines := 0; scanner.Scan() && lines < maxLines; lines++ {
		var line struct {
			Role    string `json:"role"`
			Message struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) != nil || (line.Role != "user" && line.Role != "assistant") {
			continue
		}
		session.Messages++
		if line.Role == "user" && session.Title == "" {
			for _, part := range line.Message.Content {
				if part.Type == "text" && part.Text != "" {
					text := part.Text
					if start := strings.Index(text, "<user_query>"); start >= 0 {
						text = text[start+len("<user_query>"):]
						if end := strings.Index(text, "</user_query>"); end >= 0 {
							text = text[:end]
						}
					}
					session.Title = usableTitle(text)
					if session.Title != "" {
						break
					}
				}
			}
		}
	}
	_ = file.Close()
	if info, statErr := os.Stat(path); statErr == nil {
		session.StartedAt = info.ModTime()
		session.LastAt = info.ModTime()
	}
	if session.Title == "" {
		session.Title = id
	}
	return session, true
}

func scanCline(ctx context.Context, home, projectPath string) []model.Session {
	root := filepath.Join(home, ".cline", "data", "sessions")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []model.Session
	for _, entry := range entries {
		if ctx.Err() != nil || !entry.IsDir() || len(out) >= maxClaudeFiles {
			break
		}
		id := entry.Name()
		manifestPath := filepath.Join(root, id, id+".json")
		messagesPath := filepath.Join(root, id, id+".messages.json")
		var manifest struct {
			CWD       string `json:"cwd"`
			StartedAt string `json:"started_at"`
			Model     string `json:"model"`
			Prompt    string `json:"prompt"`
			Metadata  struct {
				Title string `json:"title"`
			} `json:"metadata"`
		}
		raw, readErr := os.ReadFile(manifestPath)
		if readErr != nil || json.Unmarshal(raw, &manifest) != nil || (projectPath != "" && filepath.Clean(manifest.CWD) != projectPath) {
			continue
		}
		session := model.Session{ID: id, Agent: "Cline", CWD: manifest.CWD, Title: usableTitle(manifest.Metadata.Title), Model: manifest.Model, Path: messagesPath}
		if session.Title == "" {
			session.Title = usableTitle(stripTag(manifest.Prompt, "user_input"))
		}
		if at, parseErr := time.Parse(time.RFC3339, manifest.StartedAt); parseErr == nil {
			session.StartedAt = at
		}
		var body struct {
			UpdatedAt string `json:"updated_at"`
			Messages  []struct {
				Role      string `json:"role"`
				TS        int64  `json:"ts"`
				ModelInfo *struct {
					ID string `json:"id"`
				} `json:"modelInfo"`
				Metrics *struct {
					Input  int `json:"inputTokens"`
					Output int `json:"outputTokens"`
				} `json:"metrics"`
			} `json:"messages"`
		}
		if bodyRaw, bodyErr := os.ReadFile(messagesPath); bodyErr == nil {
			if json.Unmarshal(bodyRaw, &body) == nil {
				for _, message := range body.Messages {
					if message.Role != "user" && message.Role != "assistant" {
						continue
					}
					session.Messages++
					if session.Model == "" && message.ModelInfo != nil {
						session.Model = message.ModelInfo.ID
					}
					if message.Metrics != nil {
						session.TokensIn += message.Metrics.Input
						session.TokensOut += message.Metrics.Output
					}
					if at := time.UnixMilli(message.TS); message.TS > 0 && at.After(session.LastAt) {
						session.LastAt = at
					}
				}
			}
			if at, parseErr := time.Parse(time.RFC3339, body.UpdatedAt); parseErr == nil && at.After(session.LastAt) {
				session.LastAt = at
			}
		}
		if session.StartedAt.IsZero() {
			if info, statErr := os.Stat(manifestPath); statErr == nil {
				session.StartedAt = info.ModTime()
			}
		}
		if session.LastAt.IsZero() {
			session.LastAt = session.StartedAt
		}
		if session.Title == "" {
			session.Title = id
		}
		out = append(out, session)
	}
	return out
}

func stripTag(value, tag string) string {
	open := "<" + tag
	close := "</" + tag + ">"
	start := strings.Index(value, open)
	if start < 0 {
		return value
	}
	if end := strings.Index(value[start:], ">"); end >= 0 {
		start += end + 1
	}
	value = value[start:]
	if end := strings.Index(value, close); end >= 0 {
		value = value[:end]
	}
	return value
}

func scanGoose(ctx context.Context, home, projectPath string) []model.Session {
	db, err := sql.Open("sqlite", filepath.Join(home, ".local", "share", "goose", "sessions", "sessions.db"))
	if err != nil {
		return nil
	}
	defer db.Close()
	query := `SELECT id, name, description, working_dir, created_at, updated_at, COALESCE(input_tokens, 0), COALESCE(output_tokens, 0), COALESCE(model_config_json, '') FROM sessions`
	args := []any{}
	if projectPath != "" {
		query += ` WHERE working_dir = ?`
		args = append(args, projectPath)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	if projectPath == "" {
		args = append(args, maxAgentCandidates)
	} else {
		args = append(args, maxClaudeFiles)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []model.Session
	for rows.Next() {
		var id, name, description, cwd, created, updated, modelConfig string
		var input, output int
		if rows.Scan(&id, &name, &description, &cwd, &created, &updated, &input, &output, &modelConfig) != nil {
			continue
		}
		title := name
		if strings.TrimSpace(title) == "" || title == "New Chat" {
			title = usableTitle(description)
		}
		if title == "" {
			title = id
		}
		started := parseSQLiteTime(created)
		last := parseSQLiteTime(updated)
		var messages int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE session_id = ?`, id).Scan(&messages)
		out = append(out, model.Session{ID: id, Agent: "Goose", CWD: cwd, Title: title, Path: filepath.Join(home, ".local", "share", "goose", "sessions", "sessions.db"), StartedAt: started, LastAt: last, Messages: messages, TokensIn: input, TokensOut: output})
	}
	return out
}

func parseSQLiteTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04:05.999999999"} {
		if at, err := time.Parse(layout, value); err == nil {
			return at
		}
	}
	return time.Time{}
}

func scanInParallel(ctx context.Context, paths []string, read func(string) (model.Session, bool)) []model.Session {
	results := make(chan model.Session, len(paths))
	sem := make(chan struct{}, maxWorkers)
	var wait sync.WaitGroup
	for _, path := range paths {
		if ctx.Err() != nil {
			break
		}
		wait.Add(1)
		go func(path string) {
			defer wait.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if session, ok := read(path); ok {
				results <- session
			}
		}(path)
	}
	go func() {
		wait.Wait()
		close(results)
	}()
	var out []model.Session
	for session := range results {
		out = append(out, session)
	}
	return out
}

func scanOMP(ctx context.Context, home, projectPath string) []model.Session {
	root := filepath.Join(home, ".omp", "agent", "sessions")
	paths := make([]string, 0)
	if projectPath == "" {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".jsonl") && !strings.HasPrefix(entry.Name(), ".") {
				paths = append(paths, path)
			}
			return nil
		})
		paths = newestPaths(paths, maxAgentCandidates)
	} else {
		root = ompProjectRoot(home, projectPath)
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			paths = append(paths, filepath.Join(root, entry.Name()))
			if len(paths) >= maxCodexFiles {
				break
			}
		}
	}
	return scanInParallel(ctx, paths, func(path string) (model.Session, bool) {
		return readSessionCached(path+"\x00"+projectPath, path, func() (model.Session, bool) { return readOMP(path, projectPath) })
	})
}

func ompProjectRoot(home, projectPath string) string {
	rel, err := filepath.Rel(home, projectPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return filepath.Join(home, ".omp", "agent", "sessions")
	}
	return filepath.Join(home, ".omp", "agent", "sessions", "-"+strings.ReplaceAll(filepath.ToSlash(rel), "/", "-"))
}

func readOMP(path, projectPath string) (model.Session, bool) {
	file, err := os.Open(path)
	if err != nil {
		return model.Session{}, false
	}
	defer file.Close()
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	if cut := strings.LastIndex(id, "_"); cut >= 0 {
		id = id[cut+1:]
	}
	session := model.Session{ID: id, Agent: "OMP", Path: path}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for lines := 0; scanner.Scan() && lines < maxLines; lines++ {
		var line struct {
			Type      string `json:"type"`
			Title     string `json:"title"`
			Timestamp string `json:"timestamp"`
			ID        string `json:"id"`
			CWD       string `json:"cwd"`
			Model     string `json:"model"`
			Message   struct {
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				Usage struct {
					Input  int `json:"input"`
					Output int `json:"output"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		if line.CWD != "" {
			session.CWD = line.CWD
		}
		if line.ID != "" {
			session.ID = line.ID
		}
		if line.Model != "" {
			session.Model = line.Model
		}
		if line.Type == "title" && session.Title == "" {
			session.Title = usableTitle(line.Title)
		}
		if at, err := time.Parse(time.RFC3339, line.Timestamp); err == nil {
			if session.StartedAt.IsZero() || at.Before(session.StartedAt) {
				session.StartedAt = at
			}
			if at.After(session.LastAt) {
				session.LastAt = at
			}
		}
		if line.Type == "message" && (line.Message.Role == "user" || line.Message.Role == "assistant") {
			session.Messages++
			if line.Message.Role == "assistant" {
				session.TokensIn += line.Message.Usage.Input
				session.TokensOut += line.Message.Usage.Output
			}
			if line.Message.Role == "user" && session.Title == "" {
				for _, part := range line.Message.Content {
					if part.Type == "text" {
						session.Title = usableTitle(part.Text)
						if session.Title != "" {
							break
						}
					}
				}
			}
		}
	}
	if !conditionPath(session.CWD, projectPath) {
		return model.Session{}, false
	}
	if session.Title == "" {
		session.Title = id
	}
	if session.StartedAt.IsZero() || session.LastAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			if session.StartedAt.IsZero() {
				session.StartedAt = info.ModTime()
			}
			if session.LastAt.IsZero() {
				session.LastAt = info.ModTime()
			}
		}
	}
	return session, true
}

func scanWorkBuddy(ctx context.Context, home, projectPath string) []model.Session {
	root := filepath.Join(home, ".workbuddy", "projects")
	paths := make([]string, 0)
	if projectPath == "" {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".jsonl") {
				paths = append(paths, path)
			}
			return nil
		})
		paths = newestPaths(paths, maxAgentCandidates)
	} else {
		root = filepath.Join(root, strings.Trim(cursorSlugBad.ReplaceAllString(projectPath, "-"), "-"))
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
				continue
			}
			paths = append(paths, filepath.Join(root, entry.Name()))
			if len(paths) >= maxCodexFiles {
				break
			}
		}
	}
	return scanInParallel(ctx, paths, func(path string) (model.Session, bool) {
		return readSessionCached(path+"\x00"+projectPath, path, func() (model.Session, bool) { return readWorkBuddy(path, projectPath) })
	})
}

func readWorkBuddy(path, projectPath string) (model.Session, bool) {
	file, err := os.Open(path)
	if err != nil {
		return model.Session{}, false
	}
	defer file.Close()
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	session := model.Session{ID: id, Agent: "WorkBuddy", Path: path}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for lines := 0; scanner.Scan() && lines < maxLines; lines++ {
		var line struct {
			Type      string `json:"type"`
			Role      string `json:"role"`
			Timestamp int64  `json:"timestamp"`
			SessionID string `json:"sessionId"`
			CWD       string `json:"cwd"`
			Model     string `json:"model"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			ProviderData struct {
				Model    string `json:"model"`
				RawUsage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
				} `json:"rawUsage"`
				Usage struct {
					InputTokens  int `json:"inputTokens"`
					OutputTokens int `json:"outputTokens"`
				} `json:"usage"`
			} `json:"providerData"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		if line.CWD != "" {
			session.CWD = line.CWD
		}
		if line.SessionID != "" {
			session.ID = line.SessionID
		}
		if line.Model != "" {
			session.Model = line.Model
		}
		if line.ProviderData.Model != "" {
			session.Model = line.ProviderData.Model
		}
		input := line.ProviderData.RawUsage.PromptTokens
		output := line.ProviderData.RawUsage.CompletionTokens
		if input == 0 && output == 0 {
			input = line.ProviderData.Usage.InputTokens
			output = line.ProviderData.Usage.OutputTokens
		}
		session.TokensIn += input
		session.TokensOut += output
		if line.Timestamp > 0 {
			at := time.UnixMilli(line.Timestamp)
			if session.StartedAt.IsZero() || at.Before(session.StartedAt) {
				session.StartedAt = at
			}
			if at.After(session.LastAt) {
				session.LastAt = at
			}
		}
		if line.Type == "message" && (line.Role == "user" || line.Role == "assistant") {
			session.Messages++
			if line.Role == "user" && session.Title == "" {
				for _, part := range line.Content {
					if part.Type != "input_text" || part.Text == "" || strings.Contains(part.Text, "<system-reminder") {
						continue
					}
					session.Title = usableTitle(part.Text)
					if session.Title != "" {
						break
					}
				}
			}
		}
	}
	if !conditionPath(session.CWD, projectPath) {
		return model.Session{}, false
	}
	if session.Title == "" {
		if base := filepath.Base(session.CWD); base != "" && base != "." && base != "/" {
			session.Title = base
		} else {
			session.Title = id
		}
	}
	if session.StartedAt.IsZero() || session.LastAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			if session.StartedAt.IsZero() {
				session.StartedAt = info.ModTime()
			}
			if session.LastAt.IsZero() {
				session.LastAt = info.ModTime()
			}
		}
	}
	return session, true
}

func newestPaths(paths []string, limit int) []string {
	type candidate struct {
		path string
		mod  time.Time
	}
	candidates := make([]candidate, 0, len(paths))
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil {
			candidates = append(candidates, candidate{path: path, mod: info.ModTime()})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].mod.After(candidates[j].mod) })
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]string, len(candidates))
	for index, candidate := range candidates {
		out[index] = candidate.path
	}
	return out
}

func scanCodeBuddy(ctx context.Context, home, projectPath string) []model.Session {
	root := filepath.Join(home, ".codebuddy", "projects")
	paths := make([]string, 0)
	if projectPath == "" {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".jsonl") {
				paths = append(paths, path)
			}
			return nil
		})
		paths = newestPaths(paths, maxAgentCandidates)
	} else {
		root = filepath.Join(root, strings.Trim(cursorSlugBad.ReplaceAllString(projectPath, "-"), "-"))
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
				continue
			}
			paths = append(paths, filepath.Join(root, entry.Name()))
			if len(paths) >= maxClaudeFiles {
				break
			}
		}
	}
	return scanInParallel(ctx, paths, func(path string) (model.Session, bool) {
		return readSessionCached(path+"\x00"+projectPath, path, func() (model.Session, bool) { return readCodeBuddy(path, projectPath) })
	})
}

func readCodeBuddy(path, projectPath string) (model.Session, bool) {
	file, err := os.Open(path)
	if err != nil {
		return model.Session{}, false
	}
	defer file.Close()
	session := model.Session{ID: strings.TrimSuffix(filepath.Base(path), ".jsonl"), Agent: "CodeBuddy", Path: path}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for lines := 0; scanner.Scan() && lines < maxLines; lines++ {
		var line struct {
			Type      string `json:"type"`
			Role      string `json:"role"`
			Timestamp int64  `json:"timestamp"`
			SessionID string `json:"sessionId"`
			CWD       string `json:"cwd"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			ProviderData struct {
				Model    string `json:"model"`
				RawUsage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
				} `json:"rawUsage"`
			} `json:"providerData"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		if line.CWD != "" {
			session.CWD = line.CWD
		}
		if line.SessionID != "" {
			session.ID = line.SessionID
		}
		if line.ProviderData.Model != "" {
			session.Model = line.ProviderData.Model
		}
		session.TokensIn += line.Usage.InputTokens + line.ProviderData.RawUsage.PromptTokens
		session.TokensOut += line.Usage.OutputTokens + line.ProviderData.RawUsage.CompletionTokens
		if line.Timestamp > 0 {
			at := time.UnixMilli(line.Timestamp)
			if session.StartedAt.IsZero() || at.Before(session.StartedAt) {
				session.StartedAt = at
			}
			if at.After(session.LastAt) {
				session.LastAt = at
			}
		}
		if line.Type == "message" && (line.Role == "user" || line.Role == "assistant") {
			session.Messages++
			if line.Role == "user" && session.Title == "" {
				for _, part := range line.Content {
					if part.Type == "input_text" {
						session.Title = usableTitle(part.Text)
						if session.Title != "" {
							break
						}
					}
				}
			}
		}
	}
	if !conditionPath(session.CWD, projectPath) {
		return model.Session{}, false
	}
	if session.Title == "" {
		session.Title = session.ID
	}
	if session.StartedAt.IsZero() || session.LastAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			if session.StartedAt.IsZero() {
				session.StartedAt = info.ModTime()
			}
			if session.LastAt.IsZero() {
				session.LastAt = info.ModTime()
			}
		}
	}
	return session, true
}

func scanGemini(ctx context.Context, home, projectPath string) []model.Session {
	tmp := filepath.Join(home, ".gemini", "tmp")
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return nil
	}
	var out []model.Session
	for _, entry := range entries {
		if ctx.Err() != nil || !entry.IsDir() {
			break
		}
		dir := filepath.Join(tmp, entry.Name())
		root, readErr := os.ReadFile(filepath.Join(dir, ".project_root"))
		if readErr != nil {
			continue
		}
		cwd := strings.TrimSpace(string(root))
		if projectPath != "" && filepath.Clean(cwd) != projectPath {
			continue
		}
		chats := filepath.Join(dir, "chats")
		files, readErr := os.ReadDir(chats)
		if readErr != nil {
			continue
		}
		for _, file := range files {
			if ctx.Err() != nil || file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			if session, ok := readSessionCached(filepath.Join(chats, file.Name())+"\x00"+cwd, filepath.Join(chats, file.Name()), func() (model.Session, bool) { return readGemini(filepath.Join(chats, file.Name()), cwd) }); ok {
				out = append(out, session)
			}
		}
	}
	return out
}

func readGemini(path, cwd string) (model.Session, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.Session{}, false
	}
	var payload struct {
		SessionID   string `json:"sessionId"`
		StartTime   string `json:"startTime"`
		LastUpdated string `json:"lastUpdated"`
		Messages    []struct {
			Timestamp string          `json:"timestamp"`
			Type      string          `json:"type"`
			Model     string          `json:"model"`
			Content   json.RawMessage `json:"content"`
			Tokens    struct {
				Input  int `json:"input"`
				Output int `json:"output"`
			} `json:"tokens"`
		} `json:"messages"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return model.Session{}, false
	}
	session := model.Session{ID: payload.SessionID, Agent: "Gemini CLI", CWD: cwd, Path: path}
	if session.ID == "" {
		session.ID = strings.TrimSuffix(filepath.Base(path), ".json")
	}
	for _, message := range payload.Messages {
		if message.Type != "user" && message.Type != "gemini" && message.Type != "assistant" {
			continue
		}
		session.Messages++
		if message.Model != "" {
			session.Model = message.Model
		}
		if message.Tokens.Input > session.TokensIn {
			session.TokensIn = message.Tokens.Input
		}
		session.TokensOut += message.Tokens.Output
		if session.Title == "" && message.Type == "user" {
			session.Title = geminiText(message.Content)
		}
		if at, parseErr := time.Parse(time.RFC3339, message.Timestamp); parseErr == nil {
			if session.StartedAt.IsZero() || at.Before(session.StartedAt) {
				session.StartedAt = at
			}
			if at.After(session.LastAt) {
				session.LastAt = at
			}
		}
	}
	if at, parseErr := time.Parse(time.RFC3339, payload.StartTime); parseErr == nil {
		session.StartedAt = at
	}
	if at, parseErr := time.Parse(time.RFC3339, payload.LastUpdated); parseErr == nil {
		session.LastAt = at
	}
	if session.Title == "" {
		session.Title = session.ID
	}
	if session.StartedAt.IsZero() || session.LastAt.IsZero() {
		if info, statErr := os.Stat(path); statErr == nil {
			if session.StartedAt.IsZero() {
				session.StartedAt = info.ModTime()
			}
			if session.LastAt.IsZero() {
				session.LastAt = info.ModTime()
			}
		}
	}
	return session, true
}

func geminiText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return usableTitle(text)
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		for _, part := range parts {
			if value := usableTitle(part.Text); value != "" {
				return value
			}
		}
	}
	return ""
}

func conditionPath(cwd, projectPath string) bool {
	return cwd != "" && (projectPath == "" || filepath.Clean(cwd) == projectPath)
}

func firstText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return trimTitle(text)
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		for _, part := range parts {
			if part.Text != "" {
				return trimTitle(part.Text)
			}
		}
	}
	return ""
}

var uuidTitle = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var weakSessionTitles = map[string]bool{
	"可以": true, "可以可以": true, "好": true, "好的": true, "好呀": true, "嗯": true, "行": true, "收到": true,
	"继续": true, "继续吧": true, "开始": true, "开始吧": true, "谢谢": true, "ok": true, "okay": true, "yes": true, "no": true,
	"1.": true, "1": true, "。": true, "...": true,
}

func weakTitle(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "" || uuidTitle.MatchString(value) || weakSessionTitles[value]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fallbackTitle(session model.Session) string {
	if base := filepath.Base(session.CWD); base != "" && base != "." && base != "/" {
		return "未命名会话 · " + base
	}
	return "未命名会话"
}

func usableTitle(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "#>-* \t")
	if value == "" || strings.HasPrefix(value, "<") || strings.HasPrefix(value, "# AGENTS.md instructions") || strings.Contains(value, "<INSTRUCTIONS>") || strings.HasPrefix(value, "You are ") || strings.HasPrefix(value, "The following is the Codex agent history") || strings.HasPrefix(value, "Base directory for this skill:") || strings.HasPrefix(value, "RETAINED USER INSTRUCTIONS START") || strings.HasPrefix(value, "ROOT CONVERSATION START") || strings.HasPrefix(value, "rollout-") || strings.HasPrefix(value, "Host notice:") || strings.HasPrefix(value, "SENDER USER MESSAGES START") || strings.HasPrefix(value, "Within the root conversation") || strings.HasPrefix(value, "Files mentioned by the user:") || strings.HasPrefix(value, "Files pasted by the user:") || strings.HasPrefix(value, "Another language model started") {
		return ""
	}
	return trimTitle(value)
}

func isTranscriptPath(path string) bool {
	return strings.HasSuffix(path, ".jsonl") || strings.HasSuffix(path, ".jsonl.zst") || strings.HasSuffix(path, ".jsonl.zstd")
}

func sessionFileID(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".zst")
	return strings.TrimSuffix(name, ".jsonl")
}

func trimTitle(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	for _, separator := range []string{"。", "！", "？", ". "} {
		if index := strings.Index(value, separator); index > 0 {
			value = strings.TrimSpace(value[:index])
			break
		}
	}
	runes := []rune(value)
	if len(runes) > 80 {
		value = string(runes[:80]) + "…"
	}
	return value
}
