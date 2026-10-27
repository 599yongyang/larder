package larder

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type contextMenuPayload struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Project string `json:"project"`
	ID      string `json:"id"`
}

type contextMenuLabels struct {
	CopyName        string
	CopyPath        string
	CopyProjectPath string
	CopyID          string
	Reveal          string
}

func localizedContextMenuLabels(locale string) contextMenuLabels {
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return contextMenuLabels{
			CopyName:        "复制名称",
			CopyPath:        "复制路径",
			CopyProjectPath: "复制项目路径",
			CopyID:          "复制 ID",
			Reveal:          "在 Finder 中显示",
		}
	}
	return contextMenuLabels{
		CopyName:        "Copy name",
		CopyPath:        "Copy path",
		CopyProjectPath: "Copy project path",
		CopyID:          "Copy ID",
		Reveal:          "Reveal in Finder",
	}
}

func parseContextMenuPayload(ctx *application.Context) contextMenuPayload {
	var payload contextMenuPayload
	_ = json.Unmarshal([]byte(ctx.ContextMenuData()), &payload)
	return payload
}

// InstallNativeContextMenu registers the resource context menu. The menu is
// rebuilt when the frontend reports a language change so native labels follow
// the in-app locale.
func (s *AppService) InstallNativeContextMenu() {
	s.refreshNativeContextMenu()
}

func (s *AppService) SetNativeLocale(locale string) {
	locale = strings.TrimSpace(locale)
	s.mu.Lock()
	if s.locale == locale {
		s.mu.Unlock()
		return
	}
	s.locale = locale
	s.mu.Unlock()
	s.refreshNativeContextMenu()
}

func (s *AppService) refreshNativeContextMenu() {
	s.mu.RLock()
	app := s.app
	locale := s.locale
	s.mu.RUnlock()
	if app == nil {
		return
	}
	if existing, ok := app.ContextMenu.Get("larder-resource"); ok {
		existing.Destroy()
	}
	labels := localizedContextMenuLabels(locale)
	menu := application.NewContextMenu("larder-resource")
	addCopy := func(label string, value func(contextMenuPayload) string) {
		menu.Add(label).OnClick(func(ctx *application.Context) {
			text := value(parseContextMenuPayload(ctx))
			if text != "" {
				app.Clipboard.SetText(text)
			}
		})
	}
	addCopy(labels.CopyName, func(p contextMenuPayload) string { return p.Name })
	addCopy(labels.CopyPath, func(p contextMenuPayload) string { return p.Path })
	addCopy(labels.CopyProjectPath, func(p contextMenuPayload) string { return p.Project })
	addCopy(labels.CopyID, func(p contextMenuPayload) string { return p.ID })
	menu.AddSeparator()
	menu.Add(labels.Reveal).OnClick(func(ctx *application.Context) {
		payload := parseContextMenuPayload(ctx)
		if payload.Path == "" {
			return
		}
		if _, err := os.Stat(payload.Path); err == nil {
			_ = app.Env.OpenFileManager(payload.Path, true)
		}
	})
	menu.Update()
}
