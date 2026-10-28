package larder

import (
	"encoding/json"
	"os"
	"runtime"
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

type appMenuLabels struct {
	About         string
	Settings      string
	CheckUpdates  string
	Hide          string
	HideOthers    string
	ShowAll       string
	Quit          string
	File          string
	CloseWindow   string
	Edit          string
	Undo          string
	Redo          string
	Cut           string
	Copy          string
	Paste         string
	PasteAndMatch string
	Delete        string
	SelectAll     string
	Window        string
	Minimise      string
	Zoom          string
	BringToFront  string
}

func localizedAppMenuLabels(locale string) appMenuLabels {
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return appMenuLabels{About: "关于 Larder", Settings: "设置…", CheckUpdates: "检查更新…", Hide: "隐藏 Larder", HideOthers: "隐藏其他", ShowAll: "显示全部", Quit: "退出 Larder", File: "文件", CloseWindow: "关闭窗口", Edit: "编辑", Undo: "撤销", Redo: "重做", Cut: "剪切", Copy: "复制", Paste: "粘贴", PasteAndMatch: "粘贴并匹配样式", Delete: "删除", SelectAll: "全选", Window: "窗口", Minimise: "最小化", Zoom: "缩放", BringToFront: "前置全部窗口"}
	}
	return appMenuLabels{About: "About Larder", Settings: "Settings…", CheckUpdates: "Check for Updates…", Hide: "Hide Larder", HideOthers: "Hide Others", ShowAll: "Show All", Quit: "Quit Larder", File: "File", CloseWindow: "Close Window", Edit: "Edit", Undo: "Undo", Redo: "Redo", Cut: "Cut", Copy: "Copy", Paste: "Paste", PasteAndMatch: "Paste and Match Style", Delete: "Delete", SelectAll: "Select All", Window: "Window", Minimise: "Minimise", Zoom: "Zoom", BringToFront: "Bring All to Front"}
}

func (s *AppService) emitWindowEvent(name string) {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return
	}
	if window := app.Window.Current(); window != nil {
		window.Show()
		window.Focus()
		window.EmitEvent(name)
	}
}

func (s *AppService) emitNativeEvent(name string) {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return
	}
	if window := app.Window.Current(); window != nil {
		window.EmitEvent(name)
	}
}

func localizedRole(role application.Role, label string) *application.MenuItem {
	item := application.NewRole(role)
	if item != nil && label != "" {
		item.SetLabel(label)
	}
	return item
}

// InstallNativeMenus registers the application and resource menus. Menus are
// rebuilt when the frontend reports a language change so native labels follow
// the in-app locale.
func (s *AppService) InstallNativeMenus() {
	s.refreshNativeMenus()
}

func (s *AppService) refreshNativeMenus() {
	s.refreshNativeContextMenu()
	s.refreshNativeApplicationMenu()
}

func (s *AppService) SetNativeLocale(locale string) {
	locale = strings.TrimSpace(locale)
	s.mu.Lock()
	if s.locale == locale {
		s.mu.Unlock()
		return
	}
	s.locale = locale
	listener := s.localeListener
	s.mu.Unlock()
	s.refreshNativeMenus()
	if listener != nil {
		listener(locale)
	}
}

// SetLocaleListener lets the host refresh host-owned menus, such as the tray
// menu, without coupling the service to the application bootstrap.
func SetLocaleListener(service *AppService, listener func(string)) {
	service.mu.Lock()
	service.localeListener = listener
	service.mu.Unlock()
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
			if text != "" && app.Clipboard.SetText(text) {
				s.emitNativeEvent("larder:native-copied")
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
			if err := app.Env.OpenFileManager(payload.Path, true); err == nil {
				s.emitNativeEvent("larder:native-revealed")
			}
		}
	})
	menu.Update()
}

func (s *AppService) refreshNativeApplicationMenu() {
	s.mu.RLock()
	app := s.app
	locale := s.locale
	s.mu.RUnlock()
	if app == nil || runtime.GOOS != "darwin" {
		return
	}
	labels := localizedAppMenuLabels(locale)
	appMenu := app.NewMenu()
	settings := application.NewMenuItem(labels.Settings).SetAccelerator("CmdOrCtrl+,")
	settings.OnClick(func(_ *application.Context) { s.emitWindowEvent("larder:open-settings") })
	checkUpdates := application.NewMenuItem(labels.CheckUpdates)
	checkUpdates.OnClick(func(_ *application.Context) { s.emitWindowEvent("larder:check-app-update") })
	appMenu.Append(application.NewMenuFromItems(
		localizedRole(application.About, labels.About),
		application.NewMenuItemSeparator(),
		settings,
		checkUpdates,
		application.NewMenuItemSeparator(),
		localizedRole(application.Hide, labels.Hide),
		localizedRole(application.HideOthers, labels.HideOthers),
		localizedRole(application.UnHide, labels.ShowAll),
		application.NewMenuItemSeparator(),
		localizedRole(application.Quit, labels.Quit),
	))
	appSub := application.NewSubMenuItem("Larder")
	appSub.GetSubmenu().Append(appMenu)
	fileMenu := app.NewMenu()
	fileMenu.Append(application.NewMenuFromItems(
		localizedRole(application.CloseWindow, labels.CloseWindow),
	))
	fileItem := application.NewSubMenuItem(labels.File)
	fileItem.GetSubmenu().Append(fileMenu)
	editMenu := app.NewMenu()
	editMenu.Append(application.NewMenuFromItems(
		localizedRole(application.Undo, labels.Undo),
		localizedRole(application.Redo, labels.Redo),
		application.NewMenuItemSeparator(),
		localizedRole(application.Cut, labels.Cut),
		localizedRole(application.Copy, labels.Copy),
		localizedRole(application.Paste, labels.Paste),
		localizedRole(application.PasteAndMatchStyle, labels.PasteAndMatch),
		localizedRole(application.Delete, labels.Delete),
		localizedRole(application.SelectAll, labels.SelectAll),
	))
	editItem := application.NewSubMenuItem(labels.Edit)
	editItem.GetSubmenu().Append(editMenu)
	windowMenu := app.NewMenu()
	windowMenu.Append(application.NewMenuFromItems(
		localizedRole(application.Minimise, labels.Minimise),
		localizedRole(application.Zoom, labels.Zoom),
		application.NewMenuItemSeparator(),
		localizedRole(application.Front, labels.BringToFront),
	))
	windowItem := application.NewSubMenuItem(labels.Window)
	windowItem.GetSubmenu().Append(windowMenu)
	menu := app.NewMenu()
	menu.Append(application.NewMenuFromItems(appSub, fileItem, editItem, windowItem))
	app.Menu.SetApplicationMenu(menu)
}
