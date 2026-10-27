package trust

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupNormalizesAuditResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vercel-labs/skills/find-skills" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"vercel-labs/skills/find-skills","audits":[{"provider":"Socket","slug":"socket","status":"pass","summary":"No alerts","auditedAt":"2026-09-15T08:00:19Z"},{"provider":"Snyk","slug":"snyk","status":"warn","summary":"Risk: MEDIUM","riskLevel":"MEDIUM","auditedAt":"2026-09-15T07:59:46Z"}]}`))
	}))
	defer server.Close()
	client := newClient(server.URL)
	audit, err := client.Lookup(context.Background(), "vercel-labs/skills", "find-skills")
	if err != nil {
		t.Fatal(err)
	}
	if !audit.Available || audit.Verdict != "warn" || audit.RiskLevel != "MEDIUM" || len(audit.Entries) != 2 {
		t.Fatalf("audit = %#v", audit)
	}
}

func TestLookupTreats404AsUnreviewed(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	audit, err := newClient(server.URL).Lookup(context.Background(), "owner/repo", "missing")
	if err != nil {
		t.Fatal(err)
	}
	if audit.Available || audit.Verdict != "unreviewed" || audit.Reason != "not_audited" {
		t.Fatalf("audit = %#v", audit)
	}
}

func TestNormalizeSource(t *testing.T) {
	for input, want := range map[string]string{
		"https://github.com/owner/repo.git": "owner/repo",
		"owner/repo":                        "owner/repo",
		"https://github.com/owner/repo":     "owner/repo",
	} {
		if got := NormalizeSource(input, ""); got != want {
			t.Fatalf("NormalizeSource(%q) = %q, want %q", input, got, want)
		}
	}
}
