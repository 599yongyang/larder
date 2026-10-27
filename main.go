package main

import (
	"context"
	"embed"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
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
		provider, providerErr := github.New(github.Config{
			Repository:    "599yongyang/larder",
			ChecksumAsset: "checksums.txt",
		})
		if providerErr != nil {
			log.Printf("updater: %v", providerErr)
		} else if initErr := app.Updater.Init(updater.Config{
			CurrentVersion: larder.Version,
			Providers:      []updater.Provider{provider},
			CheckInterval:  6 * time.Hour,
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
	if runtime.GOOS == "darwin" {
		appMenu := application.NewAppMenu()
		extra := app.NewMenu()
		settings := extra.Add("Settings…")
		settings.SetAccelerator("CmdOrCtrl+,")
		settings.OnClick(func(_ *application.Context) {
			window.Show()
			window.Focus()
			window.EmitEvent("larder:open-settings")
		})
		extra.Add("Check for Updates…").OnClick(func(_ *application.Context) {
			go func() {
				if err := app.Updater.CheckAndInstall(context.Background()); err != nil {
					log.Printf("updater: %v", err)
				}
			}()
		})
		appMenu.GetSubmenu().Prepend(extra)
		menu := app.NewMenu()
		menu.Append(application.NewMenuFromItems(
			appMenu,
			application.NewFileMenu(),
			application.NewEditMenu(),
			application.NewWindowMenu(),
			application.NewHelpMenu(),
		))
		app.Menu.SetApplicationMenu(menu)
	}
	if err := app.GlobalShortcut.Register("CmdOrCtrl+Shift+L", func() {
		window.Show()
		window.Focus()
		window.EmitEvent("larder:open-command")
	}); err != nil {
		log.Printf("global shortcut: %v", err)
	}
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Larder")
	tray.OnClick(func() {
		window.Show()
		window.Focus()
	})
	trayMenu := app.NewMenu()
	trayMenu.Add("打开 Larder").OnClick(func(_ *application.Context) {
		window.Show()
		window.Focus()
	})
	trayMenu.Add("退出").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(trayMenu)
	service.SetApp(app)
	service.InstallNativeContextMenu()
	service.Startup()
	if err := app.Run(); err != nil {
		service.Shutdown()
		log.Fatal(err)
	}
	service.Shutdown()
}
