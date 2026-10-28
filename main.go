package main

import (
	"embed"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
	"larder/cmd/larder"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIcon []byte

func init() {
	// The frontend subscribes to this event instead of polling the full
	// snapshot every 1.5 seconds.
	application.RegisterEvent[larder.AppState]("larder:state")
}

func main() {
	service, err := larder.NewAppService()
	if err != nil {
		log.Fatal(err)
	}
	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "Larder",
		Description: "本机 Skills、MCP、CLI 与指令管理",
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "io.github.599yongyang.larder",
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if window == nil {
					return
				}
				window.Show()
				window.Focus()
			},
		},
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})
	// Use the official Wails updater on macOS. Releases ship a single-top-level
	// app zip and a checksums.txt sidecar; Windows keeps the manual installer.
	if runtime.GOOS == "darwin" {
		providers := make([]updater.Provider, 0, 2)
		if githubProvider, githubErr := github.New(github.Config{
			Repository:    "599yongyang/larder",
			ChecksumAsset: "checksums.txt",
		}); githubErr != nil {
			log.Printf("updater github: %v", githubErr)
		} else {
			providers = append(providers, githubProvider)
		}
		if endpointProvider, endpointErr := endpoint.New(endpoint.Config{
			URL:     "https://raw.githubusercontent.com/599yongyang/larder/main/updates/macos-arm64.json",
			Channel: "stable",
		}); endpointErr != nil {
			log.Printf("updater endpoint: %v", endpointErr)
		} else {
			providers = append(providers, endpointProvider)
		}
		if len(providers) == 0 {
			log.Printf("updater: no providers configured")
		} else if initErr := app.Updater.Init(updater.Config{
			CurrentVersion: larder.Version,
			Providers:      providers,
			Window:         updater.WindowNone,
		}); initErr != nil {
			log.Printf("updater: %v", initErr)
		}
	}
	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Larder",
		Width:            1440,
		Height:           900,
		MinWidth:         1080,
		MinHeight:        640,
		BackgroundColour: application.NewRGB(247, 246, 243),
		URL:              "/",
		EnableFileDrop:   true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 36,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})
	window.RegisterHook(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		var folders []string
		for _, path := range e.Context().DroppedFiles() {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				folders = append(folders, path)
			}
		}
		if len(folders) > 0 {
			window.EmitEvent("larder:files-dropped", folders)
		}
	})
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Larder")
	tray.OnClick(func() {
		window.Show()
		window.Focus()
	})
	trayMenu := app.NewMenu()
	setTrayLocale := func(locale string) {
		openLabel, quitLabel := "Open Larder", "Quit Larder"
		if strings.HasPrefix(strings.ToLower(locale), "zh") {
			openLabel, quitLabel = "打开 Larder", "退出 Larder"
		}
		trayMenu.Clear()
		trayMenu.Add(openLabel).OnClick(func(_ *application.Context) {
			window.Show()
			window.Focus()
		})
		trayMenu.Add(quitLabel).OnClick(func(_ *application.Context) {
			app.Quit()
		})
		tray.SetMenu(trayMenu)
	}
	setTrayLocale("en")
	service.SetApp(app)
	larder.SetLocaleListener(service, setTrayLocale)
	service.InstallNativeMenus()
	service.Startup()
	if err := app.Run(); err != nil {
		service.Shutdown()
		log.Fatal(err)
	}
	service.Shutdown()
}
