package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkillHubPopularMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":0,"data":{"skills":[{"slug":"demo","name":"Demo","description":"EN","description_zh":"中文","ownerName":"u_42bbb65f","iconUrl":"https://cdn.example/logo.png","namespace":{"displayName":"ake"},"source":"official","version":"1.2.3","downloads":42},{"slug":"demo","name":"Other","ownerName":"u_other","namespace":{"displayName":"other"}}]}}`))
	}))
	defer server.Close()
	client := &SkillHubClient{http: server.Client(), baseURL: server.URL, cache: skillHubCache{entries: map[string]skillHubCacheEntry{}}}
	skills, err := client.Popular(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 || skills[0].ID == skills[1].ID || skills[0].SkillID != "demo" || skills[0].Description != "中文" || skills[0].Version != "1.2.3" || skills[0].Author != "ake" || skills[0].IconURL != "https://cdn.example/logo.png" || !skills[0].Official || skills[0].Installs != 42 {
		t.Fatalf("unexpected skills: %+v", skills)
	}
}

func TestSkillHubDetailReadsSkillFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/skills/demo/files":
			_, _ = writer.Write([]byte(`{"version":"1.0.0","files":[{"path":"references/a.md"},{"path":"SKILL.md"}]}`))
		case "/api/v1/skills/demo/file":
			_, _ = writer.Write([]byte("---\nname: demo\n---\n# Demo"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := &SkillHubClient{http: server.Client(), baseURL: server.URL}
	detail, err := client.Detail(context.Background(), "demo", "")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Version != "1.0.0" || len(detail.Files) != 2 || detail.Content == "" {
		t.Fatalf("detail = %+v", detail)
	}
}
