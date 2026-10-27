package sessions

import (
	"context"
	"fmt"
	"github.com/klauspost/compress/zstd"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"larder/internal/core/model"
)

func TestScanClaudeAndCodexSessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "demo")
	claudeDir := filepath.Join(home, ".claude", "projects", strings.ReplaceAll(filepath.ToSlash(project), "/", "-"))
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	claude := `{"type":"user","sessionId":"c1","cwd":"` + filepath.ToSlash(project) + `","timestamp":"2026-01-02T03:04:05Z","message":{"role":"user","content":[{"type":"text","text":"Fix the build"}]}}
{"type":"assistant","sessionId":"c1","cwd":"` + filepath.ToSlash(project) + `","timestamp":"2026-01-02T03:05:05Z","message":{"role":"assistant","model":"claude-sonnet","usage":{"input_tokens":10,"output_tokens":20},"content":[]}}
`
	if err := os.WriteFile(filepath.Join(claudeDir, "c1.jsonl"), []byte(claude), 0o600); err != nil {
		t.Fatal(err)
	}
	codexDir := filepath.Join(home, ".codex", "sessions", "2026", "01", "02")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	codex := `{"timestamp":"2026-01-02T04:00:00Z","type":"session_meta","payload":{"id":"x1","cwd":"` + filepath.ToSlash(project) + `"}}
{"timestamp":"2026-01-02T04:01:00Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"Optimize the query"}]}}
{"timestamp":"2026-01-02T04:02:00Z","type":"turn_context","payload":{"model":"gpt-5"}}
`
	if err := os.WriteFile(filepath.Join(codexDir, "rollout-x1.jsonl"), []byte(codex), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Scan(context.Background(), home, project)
	if len(result) != 2 {
		t.Fatalf("sessions = %#v", result)
	}
	byAgent := map[string]struct {
		title, model string
		messages     int
	}{}
	for _, session := range result {
		byAgent[session.Agent] = struct {
			title, model string
			messages     int
		}{session.Title, session.Model, session.Messages}
	}
	if got := byAgent["Claude Code"]; got.title != "Fix the build" || got.model != "claude-sonnet" || got.messages != 2 {
		t.Fatalf("claude = %#v", got)
	}
	if got := byAgent["Codex"]; got.title != "Optimize the query" || got.model != "gpt-5" || got.messages != 1 {
		t.Fatalf("codex = %#v", got)
	}
}

func TestScanOpenCodeSessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "opencode-demo")
	storage := filepath.Join(home, ".local", "share", "opencode", "storage")
	sessionDir := filepath.Join(storage, "session", "project1")
	messageDir := filepath.Join(storage, "message", "ses_1")
	for _, dir := range []string{sessionDir, messageDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	raw := `{"id":"ses_1","title":"Fix the parser","directory":"` + filepath.ToSlash(project) + `","time":{"created":1769157069409,"updated":1769157070000}}`
	if err := os.WriteFile(filepath.Join(sessionDir, "ses_1.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(messageDir, "m1.json"), []byte(`{"role":"user"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(messageDir, "m2.json"), []byte(`{"role":"assistant","tokens":{"input":100,"output":20}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Scan(context.Background(), home, project)
	if len(result) != 1 || result[0].Agent != "OpenCode" || result[0].Title != "Fix the parser" || result[0].Messages != 2 || result[0].TokensIn != 100 || result[0].TokensOut != 20 {
		t.Fatalf("sessions = %#v", result)
	}
}

func TestScanCursorSessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "cursor-demo")
	slug := strings.Trim(cursorSlugBad.ReplaceAllString(project, "-"), "-")
	dir := filepath.Join(home, ".cursor", "projects", slug, "agent-transcripts", "chat1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"role":"user","message":{"content":[{"type":"text","text":"<user_query>Fix the Cursor flow</user_query>"}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"Working"}]}}
`
	if err := os.WriteFile(filepath.Join(dir, "chat1.jsonl"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Scan(context.Background(), home, project)
	if len(result) != 1 || result[0].Agent != "Cursor" || result[0].Title != "Fix the Cursor flow" || result[0].Messages != 2 {
		t.Fatalf("sessions = %#v", result)
	}
}

func TestScanClineSessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "cline-demo")
	dir := filepath.Join(home, ".cline", "data", "sessions", "s1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"session_id":"s1","started_at":"2026-01-02T03:04:05Z","cwd":"` + filepath.ToSlash(project) + `","model":"claude-sonnet","metadata":{"title":"Fix the importer"}}`
	messages := `{"updated_at":"2026-01-02T03:06:00Z","messages":[{"role":"user","ts":1767323045000},{"role":"assistant","ts":1767323100000,"modelInfo":{"id":"claude-sonnet"},"metrics":{"inputTokens":10,"outputTokens":20}}]}`
	if err := os.WriteFile(filepath.Join(dir, "s1.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "s1.messages.json"), []byte(messages), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Scan(context.Background(), home, project)
	if len(result) != 1 || result[0].Agent != "Cline" || result[0].Title != "Fix the importer" || result[0].Messages != 2 || result[0].TokensOut != 20 {
		t.Fatalf("sessions = %#v", result)
	}
}

func TestScanOMPAndWorkBuddySessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "more-agents")
	ompDir := ompProjectRoot(home, project)
	wbDir := filepath.Join(home, ".workbuddy", "projects", strings.Trim(cursorSlugBad.ReplaceAllString(project, "-"), "-"))
	for _, dir := range []string{ompDir, wbDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	omp := `{"type":"title","title":"Refactor the agent"}
{"type":"session","id":"o1","timestamp":"2026-01-02T03:04:05Z","cwd":"` + filepath.ToSlash(project) + `"}
{"type":"message","timestamp":"2026-01-02T03:05:05Z","message":{"role":"user","content":[{"type":"text","text":"Do it"}]}}
{"type":"message","timestamp":"2026-01-02T03:06:05Z","message":{"role":"assistant","content":[],"usage":{"input":200,"output":30}}}
`
	if err := os.WriteFile(filepath.Join(ompDir, "2026-01-02_o1.jsonl"), []byte(omp), 0o600); err != nil {
		t.Fatal(err)
	}
	wb := `{"type":"message","role":"user","timestamp":1767323045000,"sessionId":"w1","cwd":"` + filepath.ToSlash(project) + `","content":[{"type":"input_text","text":"<system-reminder>ignore</system-reminder>"}]}
{"type":"message","role":"user","timestamp":1767323100000,"sessionId":"w1","cwd":"` + filepath.ToSlash(project) + `","content":[{"type":"input_text","text":"WorkBuddy task"}]}
{"type":"function_call","timestamp":1767323110000,"providerData":{"model":"glm-5","rawUsage":{"prompt_tokens":300,"completion_tokens":40}}}
`
	if err := os.WriteFile(filepath.Join(wbDir, "w1.jsonl"), []byte(wb), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Scan(context.Background(), home, project)
	byAgent := map[string]model.Session{}
	for _, session := range result {
		byAgent[session.Agent] = session
	}
	if byAgent["OMP"].Title != "Refactor the agent" || byAgent["OMP"].Messages != 2 || byAgent["OMP"].TokensIn != 200 || byAgent["OMP"].TokensOut != 30 {
		t.Fatalf("omp = %#v", byAgent["OMP"])
	}
	if byAgent["WorkBuddy"].Title != "WorkBuddy task" || byAgent["WorkBuddy"].Messages != 2 || byAgent["WorkBuddy"].TokensIn != 300 || byAgent["WorkBuddy"].TokensOut != 40 {
		t.Fatalf("workbuddy = %#v", byAgent["WorkBuddy"])
	}
}

func TestScanQoderSessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "qoder-demo")
	dir := filepath.Join(home, ".qoder", "projects", strings.ReplaceAll(filepath.ToSlash(project), "/", "-"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"type":"user","sessionId":"q1","cwd":"` + filepath.ToSlash(project) + `","timestamp":"2026-01-02T03:04:05Z","message":{"role":"user","content":[{"type":"text","text":"Qoder task"}]}}
{"type":"assistant","sessionId":"q1","cwd":"` + filepath.ToSlash(project) + `","timestamp":"2026-01-02T03:05:05Z","message":{"role":"assistant","model":"qoder-model","content":[]}}
`
	if err := os.WriteFile(filepath.Join(dir, "q1.jsonl"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Scan(context.Background(), home, project)
	if len(result) != 1 || result[0].Agent != "Qoder" || result[0].Title != "Qoder task" || result[0].Model != "qoder-model" {
		t.Fatalf("sessions = %#v", result)
	}
}

func TestScanCodeBuddySessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "codebuddy-demo")
	slug := strings.Trim(cursorSlugBad.ReplaceAllString(project, "-"), "-")
	dir := filepath.Join(home, ".codebuddy", "projects", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"type":"message","role":"user","timestamp":1767323045000,"sessionId":"cb1","cwd":"` + filepath.ToSlash(project) + `","content":[{"type":"input_text","text":"CodeBuddy task"}]}
{"type":"message","role":"assistant","timestamp":1767323100000,"sessionId":"cb1","cwd":"` + filepath.ToSlash(project) + `","content":[{"type":"output_text","text":"Done"}],"providerData":{"model":"cb-model","rawUsage":{"prompt_tokens":400,"completion_tokens":50}}}
`
	if err := os.WriteFile(filepath.Join(dir, "cb1.jsonl"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Scan(context.Background(), home, project)
	if len(result) != 1 || result[0].Agent != "CodeBuddy" || result[0].Title != "CodeBuddy task" || result[0].Messages != 2 || result[0].TokensIn != 400 || result[0].TokensOut != 50 || result[0].Model != "cb-model" {
		t.Fatalf("sessions = %#v", result)
	}
}

func TestScanGeminiSessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "gemini-demo")
	dir := filepath.Join(home, ".gemini", "tmp", "project", "chats")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gemini", "tmp", "project", ".project_root"), []byte(project), 0o600); err != nil {
		t.Fatal(err)
	}
	raw := `{"sessionId":"g1","startTime":"2026-01-02T03:04:05Z","lastUpdated":"2026-01-02T03:06:00Z","messages":[{"type":"user","timestamp":"2026-01-02T03:04:05Z","content":[{"text":"Gemini task"}]},{"type":"gemini","timestamp":"2026-01-02T03:05:00Z","model":"gemini-test","tokens":{"input":500,"output":60},"content":"Done"}]}`
	if err := os.WriteFile(filepath.Join(dir, "session-g1.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Scan(context.Background(), home, project)
	if len(result) != 1 || result[0].Agent != "Gemini CLI" || result[0].Title != "Gemini task" || result[0].Messages != 2 || result[0].TokensIn != 500 || result[0].TokensOut != 60 || result[0].Model != "gemini-test" {
		t.Fatalf("sessions = %#v", result)
	}
}

func TestScanAgentReturnsOnlyThatAgent(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "agent-filter")
	claudeDir := filepath.Join(home, ".claude", "projects", strings.ReplaceAll(filepath.ToSlash(project), "/", "-"))
	codexDir := filepath.Join(home, ".codex", "sessions", "2026", "01", "02")
	for _, dir := range []string{claudeDir, codexDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	claude := `{"type":"user","sessionId":"c1","cwd":"` + filepath.ToSlash(project) + `","timestamp":"2026-01-02T03:04:05Z","message":{"role":"user","content":[{"type":"text","text":"Claude only"}]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "c1.jsonl"), []byte(claude), 0o600); err != nil {
		t.Fatal(err)
	}
	codex := `{"timestamp":"2026-01-02T04:00:00Z","type":"session_meta","payload":{"id":"x1","cwd":"` + filepath.ToSlash(project) + `"}}
{"timestamp":"2026-01-02T04:01:00Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"Codex task"}]}}
{"timestamp":"2026-01-02T04:02:00Z","type":"token_usage_record","payload":{"thread_token_usage":{"input_tokens":1200,"output_tokens":80}}}
`
	if err := os.WriteFile(filepath.Join(codexDir, "rollout-x1.jsonl"), []byte(codex), 0o600); err != nil {
		t.Fatal(err)
	}
	claudeSessions := ScanAgent(context.Background(), home, "Claude Code")
	if len(claudeSessions) != 1 || claudeSessions[0].Agent != "Claude Code" {
		t.Fatalf("claude = %#v", claudeSessions)
	}
	codexSessions := ScanAgent(context.Background(), home, "Codex")
	if len(codexSessions) != 1 || codexSessions[0].Agent != "Codex" || codexSessions[0].TokensIn != 1200 || codexSessions[0].TokensOut != 80 {
		t.Fatalf("codex = %#v", codexSessions)
	}
}

func TestScanCodexCompressedArchivedSessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "codex-archive")
	dir := filepath.Join(home, ".codex", "archived_sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-01-02T04:00:00Z","type":"session_meta","payload":{"id":"archived-1","cwd":"` + filepath.ToSlash(project) + `","thread_source":"user"}}
{"timestamp":"2026-01-02T04:01:00Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"Archived Codex task"}]}}
{"timestamp":"2026-01-02T04:02:00Z","type":"token_usage_record","payload":{"thread_token_usage":{"input_tokens":900,"output_tokens":75}}}
`
	file, err := os.Create(filepath.Join(dir, "rollout-2026-01-02-archived-1.jsonl.zst"))
	if err != nil {
		t.Fatal(err)
	}
	writer, err := zstd.NewWriter(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	// Guardian review threads are internal and should not appear as user sessions.
	review := filepath.Join(home, ".codex", "sessions", "2026", "01", "02", "rollout-review.jsonl")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	reviewContent := `{"timestamp":"2026-01-02T04:00:00Z","type":"session_meta","payload":{"id":"review-1","cwd":"` + filepath.ToSlash(project) + `","thread_source":"guardian_review"}}
{"timestamp":"2026-01-02T04:01:00Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"Review task"}]}}
`
	if err := os.WriteFile(review, []byte(reviewContent), 0o600); err != nil {
		t.Fatal(err)
	}
	result := ScanAgent(context.Background(), home, "Codex")
	if len(result) != 1 {
		t.Fatalf("codex sessions = %#v", result)
	}
	if result[0].Title != "Archived Codex task" || result[0].TokensIn != 900 || result[0].TokensOut != 75 {
		t.Fatalf("session = %#v", result[0])
	}
}

func TestCodexPathsForProjectNewestFirst(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	codexProjectIndex.Lock()
	previous := codexProjectIndex.entry
	codexProjectIndex.entry = codexIndex{
		root:    filepath.Join(home, ".codex"),
		builtAt: now,
		byProject: map[string][]candidatePath{
			"/b": {{path: "/b/middle", mod: now.Add(-time.Minute)}},
			"/a": {{path: "/a/newest", mod: now}, {path: "/a/oldest", mod: now.Add(-time.Hour)}},
		},
	}
	codexProjectIndex.Unlock()
	t.Cleanup(func() {
		codexProjectIndex.Lock()
		codexProjectIndex.entry = previous
		codexProjectIndex.Unlock()
	})
	paths := codexPathsForProject(context.Background(), home, "")
	want := []string{"/a/newest", "/b/middle", "/a/oldest"}
	if fmt.Sprint(paths) != fmt.Sprint(want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestScanDSHSessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "dsh-demo")
	dir := filepath.Join(home, ".dsh", "sessions", "--work-dsh-demo--", "session-dsh-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	content := fmt.Sprintf(`{"type":"session","id":"session-dsh-1","cwd":%q,"createdAt":%d}
{"type":"request/header","time":%d,"data":{"header":{"config":{"model":"deepseek-flash"}}}}
{"type":"user/message","time":%d,"data":{"message":{"content":[{"type":"text","text":"Build the Larder DSH reader"}]}}}
{"type":"assistant/message","time":%d,"data":{"message":{"content":[{"type":"text","text":"Done"}]}}}
{"type":"session/title","time":%d,"data":{"title":"DSH session reader"}}
`, filepath.ToSlash(project), now, now, now, now, now)
	file, err := os.Create(filepath.Join(dir, "session.v4.jsonl.zstd"))
	if err != nil {
		t.Fatal(err)
	}
	writer, err := zstd.NewWriter(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	preview, err := ReadMessages(filepath.Join(dir, "session.v4.jsonl.zstd"), 20)
	if err != nil || len(preview) != 2 || preview[0].Role != "user" || preview[1].Role != "assistant" {
		t.Fatalf("DSH preview = %#v, %v", preview, err)
	}
	all := ScanAgent(context.Background(), home, "DSH")
	if len(all) != 1 || all[0].Title != "DSH session reader" || all[0].Messages != 2 || all[0].Model != "deepseek-flash" || all[0].CWD != project {
		t.Fatalf("DSH sessions = %#v", all)
	}
	projectSessions := Scan(context.Background(), home, project)
	if len(projectSessions) != 1 || projectSessions[0].Agent != "DSH" {
		t.Fatalf("project sessions = %#v", projectSessions)
	}
}

func TestReadToolUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	body := `{"type":"assistant","timestamp":"2026-01-02T04:00:00Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"cairn"}},{"type":"tool_use","name":"mcp__context7__query","input":{"query":"docs"}}]}}
{"type":"response_item","payload":{"type":"custom_tool_call","name":"exec","arguments":"cat .codex/skills/analyze/SKILL.md"}}
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	usage, err := ReadToolUsage(path)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Skills["cairn"] != 1 || usage.Skills["analyze"] != 1 {
		t.Fatalf("skills = %#v", usage.Skills)
	}
	if usage.MCP["context7"] != 1 {
		t.Fatalf("mcp = %#v", usage.MCP)
	}
	if usage.LastAt.IsZero() {
		t.Fatal("expected last activity")
	}
}
