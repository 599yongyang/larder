package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const skillHubBaseURL = "https://api.skillhub.cn"

type skillHubEntry struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DescriptionZH string `json:"description_zh"`
	OwnerName     string `json:"ownerName"`
	Namespace     struct {
		DisplayName string `json:"displayName"`
	} `json:"namespace"`
	Publisher struct {
		Name string `json:"name"`
	} `json:"publisher"`
	Source    string            `json:"source"`
	Version   string            `json:"version"`
	Homepage  string            `json:"homepage"`
	IconURL   string            `json:"iconUrl"`
	Downloads int               `json:"downloads"`
	Stars     int               `json:"stars"`
	Installs  int               `json:"installs"`
	Score     float64           `json:"score"`
	UpdatedAt int64             `json:"updated_at"`
	Labels    map[string]string `json:"labels"`
}

type skillHubCache struct {
	entries map[string]skillHubCacheEntry
	mu      sync.Mutex
}

type skillHubCacheEntry struct {
	skills  []Skill
	hasMore bool
	expires time.Time
}

type SkillHubDetail struct {
	Skill   Skill    `json:"skill"`
	Version string   `json:"version"`
	Files   []string `json:"files"`
	Content string   `json:"content,omitempty"`
}

type SkillHubClient struct {
	http     *http.Client
	baseURL  string
	apiKey   string
	configMu sync.RWMutex
	cache    skillHubCache
}

// Detail returns file metadata and the SKILL.md preview for one SkillHub skill.
func (c *SkillHubClient) Detail(ctx context.Context, slug, version string) (SkillHubDetail, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return SkillHubDetail{}, fmt.Errorf("skill slug is required")
	}
	filesURL := c.base() + "/api/v1/skills/" + url.PathEscape(slug) + "/files"
	if version != "" {
		filesURL += "?version=" + url.QueryEscape(version)
	}
	resp, err := c.doGet(ctx, filesURL)
	if err != nil {
		return SkillHubDetail{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SkillHubDetail{}, fmt.Errorf("SkillHub returned %s", resp.Status)
	}
	var payload struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return SkillHubDetail{}, fmt.Errorf("decode SkillHub files: %w", err)
	}
	detail := SkillHubDetail{Version: payload.Version}
	if detail.Version == "" {
		detail.Version = version
	}
	for _, file := range payload.Files {
		detail.Files = append(detail.Files, file.Path)
	}
	contentPath := ""
	for _, file := range detail.Files {
		if strings.EqualFold(file, "SKILL.md") {
			contentPath = file
			break
		}
	}
	if contentPath == "" && len(detail.Files) > 0 {
		contentPath = detail.Files[0]
	}
	if contentPath != "" {
		fileURL := c.base() + "/api/v1/skills/" + url.PathEscape(slug) + "/file?path=" + url.QueryEscape(contentPath)
		if detail.Version != "" {
			fileURL += "&version=" + url.QueryEscape(detail.Version)
		}
		if fileResp, doErr := c.doGet(ctx, fileURL); doErr == nil {
			defer fileResp.Body.Close()
			if fileResp.StatusCode == http.StatusOK {
				body, readErr := io.ReadAll(io.LimitReader(fileResp.Body, 1<<20))
				if readErr == nil {
					detail.Content = string(body)
				}
			}
		}
	}
	return detail, nil
}

// Configure updates the optional base URL and API key.
func (c *SkillHubClient) Configure(baseURL, apiKey string) {
	c.configMu.Lock()
	if strings.TrimSpace(baseURL) != "" {
		c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
	c.apiKey = strings.TrimSpace(apiKey)
	c.configMu.Unlock()
}

func (c *SkillHubClient) base() string {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	if c.baseURL == "" {
		return skillHubBaseURL
	}
	return c.baseURL
}

func (c *SkillHubClient) key() string {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.apiKey
}

type SkillHubCategory struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	NameEN string `json:"nameEn"`
}

// Categories returns the public SkillHub taxonomy at runtime.
func (c *SkillHubClient) Categories(ctx context.Context) ([]SkillHubCategory, error) {
	resp, err := c.doGet(ctx, c.base()+"/api/v1/categories")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SkillHub returned %s", resp.Status)
	}
	var payload struct {
		Items []SkillHubCategory `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode SkillHub categories: %w", err)
	}
	return payload.Items, nil
}

// FileContent reads one file from a published SkillHub version.
func (c *SkillHubClient) FileContent(ctx context.Context, slug, version, filePath string) (string, error) {
	slug = strings.TrimSpace(slug)
	filePath = strings.TrimSpace(filePath)
	if slug == "" || filePath == "" {
		return "", fmt.Errorf("slug and file path are required")
	}
	endpoint := c.base() + "/api/v1/skills/" + url.PathEscape(slug) + "/file?path=" + url.QueryEscape(filePath)
	if version != "" {
		endpoint += "&version=" + url.QueryEscape(version)
	}
	resp, err := c.doGet(ctx, endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SkillHub returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// DownloadURL returns the public zip endpoint for a SkillHub slug.
func SkillHubDownloadURL(slug, version string) string {
	endpoint := skillHubBaseURL + "/api/v1/download?slug=" + url.QueryEscape(slug)
	if version != "" {
		endpoint += "&version=" + url.QueryEscape(version)
	}
	return endpoint
}

func NewSkillHubClient() *SkillHubClient {
	return &SkillHubClient{http: &http.Client{Timeout: 15 * time.Second}, baseURL: skillHubBaseURL, cache: skillHubCache{entries: map[string]skillHubCacheEntry{}}}
}

// Popular returns anonymous SkillHub results sorted by downloads.
func (c *SkillHubClient) Popular(ctx context.Context) ([]Skill, error) {
	skills, _, err := c.ListPage(ctx, "popular", 1)
	return skills, err
}

// ListPage returns one page of popular or trending SkillHub results.
func (c *SkillHubClient) ListPage(ctx context.Context, view string, page int) ([]Skill, bool, error) {
	return c.ListPageFiltered(ctx, view, page, "")
}

func (c *SkillHubClient) ListPageFiltered(ctx context.Context, view string, page int, category string) ([]Skill, bool, error) {
	sortBy, key := "downloads", "popular"
	if view == "trending" {
		sortBy, key = "score", "trending"
	}
	if page < 1 {
		page = 1
	}
	endpoint := c.base() + "/api/skills?sortBy=" + sortBy + "&order=desc&pageSize=40&page=" + fmt.Sprint(page)
	if category != "" {
		endpoint += "&category=" + url.QueryEscape(category)
	}
	return c.listPage(ctx, endpoint, key+":"+category+":"+fmt.Sprint(page), page)
}

// Trending uses SkillHub's composite score, which is its public popularity signal.
func (c *SkillHubClient) Trending(ctx context.Context) ([]Skill, error) {
	skills, _, err := c.ListPage(ctx, "trending", 1)
	return skills, err
}

func (c *SkillHubClient) Search(ctx context.Context, query string) ([]Skill, error) {
	skills, _, err := c.SearchPage(ctx, query, 1)
	return skills, err
}

func (c *SkillHubClient) SearchPage(ctx context.Context, query string, page int) ([]Skill, bool, error) {
	return c.SearchPageFiltered(ctx, query, page, "")
}

func (c *SkillHubClient) SearchPageFiltered(ctx context.Context, query string, page int, category string) ([]Skill, bool, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, false, fmt.Errorf("search query must be at least 2 characters")
	}
	if page < 1 {
		page = 1
	}
	endpoint := c.base() + "/api/skills?keyword=" + url.QueryEscape(query) + "&sortBy=score&order=desc&pageSize=40&page=" + fmt.Sprint(page)
	if category != "" {
		endpoint += "&category=" + url.QueryEscape(category)
	}
	return c.listPage(ctx, endpoint, "search:"+strings.ToLower(query)+":"+category+":"+fmt.Sprint(page), page)
}

func skillHubAuthor(entry skillHubEntry) string {
	if entry.Namespace.DisplayName != "" {
		return entry.Namespace.DisplayName
	}
	if entry.Publisher.Name != "" {
		return entry.Publisher.Name
	}
	return entry.OwnerName
}

func (c *SkillHubClient) doGet(ctx context.Context, endpoint string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "larder")
		req.Header.Set("X-Client-User-Id", "larder-desktop")
		if key := c.key(); key != "" {
			req.Header.Set("X-API-Key", key)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
		} else if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			return resp, nil
		} else {
			lastErr = fmt.Errorf("SkillHub returned %s", resp.Status)
			_ = resp.Body.Close()
		}
		if attempt < 2 {
			delay := time.Duration(250*(1<<attempt)) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, lastErr
}

func (c *SkillHubClient) listPage(ctx context.Context, endpoint, cacheKey string, page int) ([]Skill, bool, error) {
	c.cache.mu.Lock()
	if cached, ok := c.cache.entries[cacheKey]; ok && time.Now().Before(cached.expires) {
		c.cache.mu.Unlock()
		return cached.skills, cached.hasMore, nil
	}
	c.cache.mu.Unlock()
	resp, err := c.doGet(ctx, endpoint)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Skills []skillHubEntry `json:"skills"`
			Total  int             `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, false, fmt.Errorf("decode SkillHub: %w", err)
	}
	skills := make([]Skill, 0, len(payload.Data.Skills))
	for _, entry := range payload.Data.Skills {
		if entry.Slug == "" {
			continue
		}
		description := entry.DescriptionZH
		if description == "" {
			description = entry.Description
		}
		skills = append(skills, Skill{ID: "skillhub:" + skillHubAuthor(entry) + "/" + entry.Slug, Source: "skillhub.cn/" + entry.OwnerName, SkillID: entry.Slug, Name: entry.Name, Installs: entry.Downloads, Official: entry.Source == "official", Version: entry.Version, Description: description, Homepage: entry.Homepage, IconURL: entry.IconURL, Author: skillHubAuthor(entry)})
	}
	hasMore := page*40 < payload.Data.Total
	c.cache.mu.Lock()
	c.cache.entries[cacheKey] = skillHubCacheEntry{skills: skills, hasMore: hasMore, expires: time.Now().Add(10 * time.Minute)}
	c.cache.mu.Unlock()
	return skills, hasMore, nil
}
