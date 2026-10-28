package catalog

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed popular.json
var popularSnapshot []byte

const (
	baseURL    = "https://www.skills.sh"
	searchURL  = "https://skills.sh/api/search"
	rawBaseURL = "https://raw.githubusercontent.com"
)

type Skill struct {
	ID                string `json:"id"`
	Source            string `json:"source"`
	SkillID           string `json:"skillId"`
	Name              string `json:"name"`
	Installs          int    `json:"installs"`
	InstallsYesterday int    `json:"installsYesterday,omitempty"`
	Change            int    `json:"change,omitempty"`
	Official          bool   `json:"isOfficial,omitempty"`
}

type cacheEntry struct {
	skills  []Skill
	expires time.Time
}

type pathEntry struct {
	path    string
	expires time.Time
}

type Client struct {
	http *http.Client

	mu           sync.Mutex
	topCache     map[string]cacheEntry
	search       map[string]cacheEntry
	paths        map[string]pathEntry
	descriptions map[string]string
	popular      []Skill
	popularAt    time.Time
}

func NewClient() *Client {
	client := &Client{
		http:         &http.Client{Timeout: 15 * time.Second},
		topCache:     map[string]cacheEntry{},
		search:       map[string]cacheEntry{},
		paths:        map[string]pathEntry{},
		descriptions: map[string]string{},
	}
	client.loadAboutCache()
	return client
}

// Popular returns skills.sh's front-page most-installed list. The official
// API for this view needs a Vercel OIDC token, so the public page is read
// instead. It is cached for six hours, then on disk, then falls back to the
// snapshot shipped with the app.
// LocalPopular returns the bundled skills.sh snapshot without network access.
// It is intended for resolving source metadata during a local scan.
func LocalPopular() []Skill {
	var skills []Skill
	if json.Unmarshal(popularSnapshot, &skills) != nil {
		return nil
	}
	return tidyPopular(skills)
}

func (c *Client) Popular(ctx context.Context) ([]Skill, error) {
	c.mu.Lock()
	if c.popular != nil && time.Since(c.popularAt) < 6*time.Hour {
		skills := c.popular
		c.mu.Unlock()
		return skills, nil
	}
	c.mu.Unlock()
	if skills, err := c.fetchPopular(ctx); err == nil && len(skills) > 0 {
		c.mu.Lock()
		c.popular, c.popularAt = skills, time.Now()
		c.mu.Unlock()
		c.writePopularCache(skills)
		return skills, nil
	}
	if skills := c.readPopularCache(); len(skills) > 0 {
		c.mu.Lock()
		c.popular, c.popularAt = skills, time.Now()
		c.mu.Unlock()
		return skills, nil
	}
	var skills []Skill
	if err := json.Unmarshal(popularSnapshot, &skills); err == nil {
		skills = tidyPopular(skills)
		c.mu.Lock()
		c.popular, c.popularAt = skills, time.Now().Add(-6*time.Hour+5*time.Minute)
		c.mu.Unlock()
		return skills, nil
	}
	return nil, errors.New("skills.sh popular list is unavailable")
}

func (c *Client) fetchPopular(ctx context.Context) ([]Skill, error) {
	body, err := c.get(ctx, baseURL+"/")
	if err != nil {
		return nil, err
	}
	skills, err := extractInitialSkills(body)
	if err != nil {
		return nil, err
	}
	return tidyPopular(skills), nil
}

func tidyPopular(skills []Skill) []Skill {
	return tidySkills(skills, 60, 4)
}

func tidySkills(skills []Skill, limit, perRepository int) []Skill {
	repoPattern := regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)
	out := make([]Skill, 0, min(len(skills), limit))
	perSource := map[string]int{}
	for _, skill := range skills {
		if !repoPattern.MatchString(skill.Source) || skill.SkillID == "" {
			continue
		}
		if perSource[skill.Source] >= perRepository {
			continue
		}
		perSource[skill.Source]++
		if skill.ID == "" {
			skill.ID = skill.Source + "/" + skill.SkillID
		}
		out = append(out, skill)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func popularCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "larder", "popular-skills.json")
}

func (c *Client) writePopularCache(skills []Skill) {
	path := popularCachePath()
	if path == "" {
		return
	}
	body, err := json.Marshal(skills)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, body, 0o644)
}

func (c *Client) readPopularCache() []Skill {
	path := popularCachePath()
	if path == "" {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var skills []Skill
	if json.Unmarshal(body, &skills) != nil {
		return nil
	}
	return tidyPopular(skills)
}

// Top returns the public hot/trending leaderboard rendered by skills.sh. The
// public JSON API for these views requires a Vercel OIDC token, so the site's
// server-rendered data is decoded instead of relying on undocumented auth.
func (c *Client) Top(ctx context.Context, view string) ([]Skill, error) {
	if view != "hot" && view != "trending" {
		return nil, errors.New("view must be hot or trending")
	}
	c.mu.Lock()
	if cached, ok := c.topCache[view]; ok && time.Now().Before(cached.expires) {
		c.mu.Unlock()
		return cached.skills, nil
	}
	c.mu.Unlock()

	body, err := c.get(ctx, baseURL+"/"+view)
	if err != nil {
		return nil, err
	}
	skills, err := extractInitialSkills(body)
	if err != nil {
		return nil, err
	}
	skills = tidySkills(skills, 60, 8)
	c.mu.Lock()
	c.topCache[view] = cacheEntry{skills: skills, expires: time.Now().Add(10 * time.Minute)}
	c.mu.Unlock()
	return skills, nil
}

// Search uses the public search endpoint used by the skills CLI.
func (c *Client) Search(ctx context.Context, query string) ([]Skill, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, errors.New("search query must be at least 2 characters")
	}
	c.mu.Lock()
	if cached, ok := c.search[query]; ok && time.Now().Before(cached.expires) {
		c.mu.Unlock()
		return cached.skills, nil
	}
	c.mu.Unlock()

	endpoint := searchURL + "?q=" + url.QueryEscape(query) + "&limit=40"
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Skills []struct {
			ID       string `json:"id"`
			Source   string `json:"source"`
			SkillID  string `json:"skillId"`
			Name     string `json:"name"`
			Installs int    `json:"installs"`
			Official bool   `json:"isOfficial"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	skills := make([]Skill, 0, len(payload.Skills))
	for _, item := range payload.Skills {
		skills = append(skills, Skill{ID: item.ID, Source: item.Source, SkillID: item.SkillID, Name: item.Name, Installs: item.Installs, Official: item.Official})
	}
	c.mu.Lock()
	c.search[query] = cacheEntry{skills: skills, expires: time.Now().Add(5 * time.Minute)}
	c.mu.Unlock()
	return skills, nil
}

// Resolve finds the SKILL.md path for a catalog entry. It probes the common
// repository layouts instead of spending the low anonymous GitHub API quota.
func (c *Client) Resolve(ctx context.Context, source, skillID string) (string, error) {
	source = strings.TrimSpace(source)
	skillID = strings.TrimSpace(skillID)
	if source == "" || skillID == "" {
		return "", errors.New("source and skill are required")
	}
	key := source + "/" + skillID
	c.mu.Lock()
	if cached, ok := c.paths[key]; ok && time.Now().Before(cached.expires) {
		c.mu.Unlock()
		return cached.path, nil
	}
	c.mu.Unlock()

	candidates := candidatePaths(skillID)
	type result struct {
		path string
		ok   bool
	}
	probeContext, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan result, len(candidates))
	for _, candidate := range candidates {
		go func(path string) {
			ok, err := c.exists(probeContext, rawBaseURL+"/"+source+"/HEAD/"+path)
			results <- result{path: path, ok: err == nil && ok}
		}(candidate)
	}
	for range candidates {
		result := <-results
		if !result.ok {
			continue
		}
		c.cachePath(key, result.path)
		return result.path, nil
	}
	return "", fmt.Errorf("could not locate SKILL.md for %s/%s", source, skillID)
}

func candidatePaths(skillID string) []string {
	return []string{
		"skills/" + skillID + "/SKILL.md",
		skillID + "/SKILL.md",
		"SKILL.md",
		".agents/skills/" + skillID + "/SKILL.md",
	}
}

func (c *Client) cachePath(key, path string) {
	c.mu.Lock()
	c.paths[key] = pathEntry{path: path, expires: time.Now().Add(24 * time.Hour)}
	c.mu.Unlock()
}

// Description reads the description skills.sh already publishes for the
// skill page, then falls back to the SKILL.md frontmatter. It is fetched
// lazily by the UI so lists do not block on many repository requests.
func (c *Client) Description(ctx context.Context, source, skillID string) (string, error) {
	key := source + "/" + skillID
	c.mu.Lock()
	if description, ok := c.descriptions[key]; ok {
		c.mu.Unlock()
		return description, nil
	}
	c.mu.Unlock()

	if description := c.pageDescription(ctx, source, skillID); description != "" {
		c.cacheDescription(key, description)
		return description, nil
	}

	// Descriptions are decorative metadata. Catalog entries can disappear or
	// point at repositories that no longer resolve; that must not surface as an
	// application error while the user is merely scrolling the discovery list.
	// Probe candidate paths in parallel so the card does not wait on several
	// sequential 404s before it gets its description.
	candidates := candidatePaths(skillID)
	type result struct {
		path string
		body []byte
		ok   bool
	}
	fetchContext, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan result, len(candidates))
	for _, candidate := range candidates {
		go func(path string) {
			body, err := c.get(fetchContext, rawBaseURL+"/"+source+"/HEAD/"+path)
			results <- result{path: path, body: body, ok: err == nil}
		}(candidate)
	}
	for range candidates {
		result := <-results
		if !result.ok {
			continue
		}
		description := frontmatterDescription(string(result.body))
		c.cachePath(key, result.path)
		c.cacheDescription(key, description)
		return description, nil
	}
	c.cacheDescription(key, "")
	return "", nil
}

func (c *Client) pageDescription(ctx context.Context, source, skillID string) string {
	body, err := c.get(ctx, baseURL+"/"+source+"/"+url.PathEscape(skillID))
	if err != nil {
		return ""
	}
	return metaDescription(string(body))
}

var metaDescriptionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<meta[^>]+(?:name|property)=["'](?:description|og:description)["'][^>]+content=["']([^"']*)["']`),
	regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']*)["'][^>]+(?:name|property)=["'](?:description|og:description)["']`),
}

func metaDescription(page string) string {
	for _, pattern := range metaDescriptionPatterns {
		match := pattern.FindStringSubmatch(page)
		if len(match) < 2 {
			continue
		}
		description := html.UnescapeString(strings.TrimSpace(match[1]))
		if description != "" {
			return strings.Join(strings.Fields(description), " ")
		}
	}
	return ""
}

func (c *Client) cacheDescription(key, description string) {
	c.mu.Lock()
	if _, exists := c.descriptions[key]; exists && description == "" {
		c.mu.Unlock()
		return
	}
	c.descriptions[key] = description
	body, err := json.Marshal(c.descriptions)
	c.mu.Unlock()
	if err != nil {
		return
	}
	path := aboutCachePath()
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, body, 0o644)
}

func aboutCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "larder", "skill-about.json")
}

func (c *Client) loadAboutCache() {
	path := aboutCachePath()
	if path == "" {
		return
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var about map[string]string
	if json.Unmarshal(body, &about) != nil {
		return
	}
	c.mu.Lock()
	for key, description := range about {
		c.descriptions[key] = description
	}
	c.mu.Unlock()
}

func frontmatterDescription(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for index := 1; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "---" {
			break
		}
		if !strings.HasPrefix(line, "description:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		if value == "|" || value == ">" || value == "|-" || value == ">-" {
			var parts []string
			for next := index + 1; next < len(lines); next++ {
				nextLine := lines[next]
				if strings.TrimSpace(nextLine) == "---" || (strings.TrimSpace(nextLine) != "" && !strings.HasPrefix(nextLine, " ") && !strings.HasPrefix(nextLine, "\t")) {
					break
				}
				parts = append(parts, strings.TrimSpace(nextLine))
			}
			value = strings.Join(parts, " ")
		}
		value = strings.Trim(value, "\"'")
		return strings.Join(strings.Fields(value), " ")
	}
	return firstParagraph(markdown)
}

func firstParagraph(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	inFrontmatter := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
	if inFrontmatter {
		closed := false
		for index := 1; index < len(lines); index++ {
			if strings.TrimSpace(lines[index]) == "---" {
				lines = lines[index+1:]
				closed = true
				break
			}
		}
		if !closed {
			return ""
		}
	}
	var parts []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "- ") {
			if len(parts) > 0 {
				break
			}
			continue
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

func (c *Client) get(ctx context.Context, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json, text/html")
	request.Header.Set("User-Agent", "larder")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills.sh returned %s", response.Status)
	}
	return io.ReadAll(io.LimitReader(response.Body, 8<<20))
}

func (c *Client) exists(ctx context.Context, endpoint string) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("User-Agent", "larder")
	response, err := c.http.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK, nil
}

var rscPushPattern = regexp.MustCompile(`self\.__next_f\.push\(\[1,("(?:[^"\\]|\\.)*")\]\)`)

// extractInitialSkills decodes the RSC payload server-rendered by skills.sh and
// pulls out the initialSkills array. The payload is a JS string, so unquote it
// before looking for the JSON fragment.
func extractInitialSkills(body []byte) ([]Skill, error) {
	payload := new(bytes.Buffer)
	for _, match := range rscPushPattern.FindAllSubmatch(body, -1) {
		decoded, err := strconv.Unquote(string(match[1]))
		if err != nil {
			continue
		}
		payload.WriteString(decoded)
	}
	marker := []byte(`"initialSkills":[`)
	index := bytes.Index(payload.Bytes(), marker)
	if index == -1 {
		return nil, errors.New("skills.sh page did not contain a skills list")
	}
	start := index + len(marker) - 1
	end, err := arrayEnd(payload.Bytes(), start)
	if err != nil {
		return nil, err
	}
	var skills []Skill
	if err := json.Unmarshal(payload.Bytes()[start:end], &skills); err != nil {
		return nil, fmt.Errorf("decode skills list: %w", err)
	}
	for index := range skills {
		if skills[index].ID == "" {
			skills[index].ID = skills[index].Source + "/" + skills[index].SkillID
		}
		if skills[index].Name == "" {
			skills[index].Name = skills[index].SkillID
		}
	}
	return skills, nil
}

func arrayEnd(data []byte, start int) (int, error) {
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(data); index++ {
		character := data[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
				continue
			}
			if character == '"' {
				inString = false
			}
			continue
		}
		switch character {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return index + 1, nil
			}
		}
	}
	return 0, errors.New("unterminated skills list")
}
