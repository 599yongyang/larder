package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
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
	app := application.New(application.Options{
		Name:        "Larder",
		Description: "本机 Skills、MCP、CLI 与指令管理",
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Larder",
		Width:            1440,
		Height:           900,
		MinWidth:         1080,
		MinHeight:        640,
		BackgroundColour: application.NewRGB(247, 246, 243),
		URL:              "/",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 36,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Larder")
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
	service.Startup()
	if err := app.Run(); err != nil {
		service.Shutdown()
		log.Fatal(err)
	}
	service.Shutdown()
}
