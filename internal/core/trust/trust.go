// Package trust reads security audit results published by skills.sh.
package trust

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"larder/internal/core/model"
)

const auditEndpoint = "https://www.skills.sh/api/v1/skills/audit"

type auditResponse struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Slug   string `json:"slug"`
	Audits []struct {
		Provider   string   `json:"provider"`
		Slug       string   `json:"slug"`
		Status     string   `json:"status"`
		Summary    string   `json:"summary"`
		RiskLevel  string   `json:"riskLevel"`
		AuditedAt  string   `json:"auditedAt"`
		Categories []string `json:"categories"`
	} `json:"audits"`
}

type cacheEntry struct {
	value   model.SkillAudit
	expires time.Time
}

// Client fetches and caches skills.sh audits.
type Client struct {
	http     *http.Client
	endpoint string
	mu       sync.Mutex
	cache    map[string]cacheEntry
}

func New() *Client {
	return newClient(auditEndpoint)
}

func newClient(endpoint string) *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}, cache: map[string]cacheEntry{}, endpoint: endpoint}
}

// Lookup returns upstream audits for source/skill. A 404 means the Skill has
// not been audited yet; it is not an error.
func (c *Client) Lookup(ctx context.Context, source, skill string) (model.SkillAudit, error) {
	source = strings.Trim(strings.TrimSpace(source), "/")
	skill = strings.TrimSpace(skill)
	if source == "" || skill == "" {
		return model.SkillAudit{Skill: skill, Source: source, Available: false, Verdict: "unreviewed", Reason: "no_source", CheckedAt: time.Now()}, nil
	}
	key := source + "\x00" + skill
	now := time.Now()
	c.mu.Lock()
	if entry, ok := c.cache[key]; ok && now.Before(entry.expires) {
		c.mu.Unlock()
		return entry.value, nil
	}
	c.mu.Unlock()

	endpoint := c.endpoint + "/" + escapePath(source) + "/" + url.PathEscape(skill)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return model.SkillAudit{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "larder-audit-check")
	resp, err := c.http.Do(req)
	if err != nil {
		return model.SkillAudit{}, fmt.Errorf("skills.sh audit request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		value := model.SkillAudit{Source: source, Skill: skill, Available: false, Verdict: "unreviewed", Reason: "not_audited", CheckedAt: now}
		c.store(key, value)
		return value, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.SkillAudit{}, fmt.Errorf("skills.sh audit: HTTP %d", resp.StatusCode)
	}
	var payload auditResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return model.SkillAudit{}, fmt.Errorf("skills.sh audit response: %w", err)
	}
	value := model.SkillAudit{Source: source, Skill: skill, Available: len(payload.Audits) > 0, Verdict: "unreviewed", CheckedAt: now}
	if !value.Available {
		value.Reason = "not_audited"
	}
	for _, entry := range payload.Audits {
		auditedAt, _ := time.Parse(time.RFC3339, entry.AuditedAt)
		value.Entries = append(value.Entries, model.SkillAuditEntry{
			Provider: entry.Provider, Slug: entry.Slug, Status: strings.ToLower(entry.Status),
			Summary: entry.Summary, RiskLevel: strings.ToUpper(entry.RiskLevel), AuditedAt: auditedAt, Categories: entry.Categories,
		})
		value.Verdict = worseVerdict(value.Verdict, entry.Status)
		value.RiskLevel = worseRisk(value.RiskLevel, entry.RiskLevel)
	}
	if value.Available && value.Verdict == "unreviewed" {
		value.Verdict = "pass"
	}
	c.store(key, value)
	return value, nil
}

func (c *Client) store(key string, value model.SkillAudit) {
	c.mu.Lock()
	c.cache[key] = cacheEntry{value: value, expires: time.Now().Add(6 * time.Hour)}
	c.mu.Unlock()
}

func escapePath(value string) string {
	parts := strings.Split(value, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func worseVerdict(current, next string) string {
	severity := map[string]int{"unreviewed": 0, "pass": 1, "warn": 2, "fail": 3}
	next = strings.ToLower(strings.TrimSpace(next))
	if severity[next] > severity[current] {
		return next
	}
	return current
}

func worseRisk(current, next string) string {
	severity := map[string]int{"": 0, "NONE": 1, "SAFE": 1, "LOW": 2, "MEDIUM": 3, "HIGH": 4, "CRITICAL": 5}
	next = strings.ToUpper(strings.TrimSpace(next))
	if severity[next] > severity[current] {
		return next
	}
	return current
}

// NormalizeSource turns a GitHub URL or repo field into owner/repo.
func NormalizeSource(source, sourceURL string) string {
	value := strings.TrimSpace(source)
	if value == "" {
		value = strings.TrimSpace(sourceURL)
	}
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.TrimPrefix(value, "http://github.com/")
	value = strings.TrimSuffix(value, ".git")
	value = strings.Trim(value, "/")
	if strings.Contains(value, "github.com/") {
		value = value[strings.Index(value, "github.com/")+len("github.com/"):]
	}
	return value
}
