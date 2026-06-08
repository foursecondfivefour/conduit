package app

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

// Run starts a single WebView window and a lightweight system-tray menu.
func Run(ctx context.Context, proxyServer *proxy.Server, settings *config.Settings) error {
	port := proxyServer.Port()
	if port == 0 {
		return fmt.Errorf("proxy is not running")
	}

	proxyURL := fmt.Sprintf("http://%s:%d", config.ListenHost, port)

	app := application.New(application.Options{
		Name:        "Conduit",
		Description: "Local CONNECT proxy with TLS fragmentation and YouTube viewer",
		Icon:        appIcon,
		Windows: application.WindowsOptions{
			AdditionalBrowserArgs: chromiumArgs(proxyURL),
		},
	})

	setupTray(app, proxyServer, settings)

	youtube := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                     "conduit",
		Title:                    "YouTube",
		Width:                    config.WindowWidth,
		Height:                   config.WindowHeight,
		MinWidth:                 480,
		MinHeight:                360,
		BackgroundColour:         application.NewRGB(18, 18, 18),
		URL:                      config.YouTubeURL,
		DevToolsEnabled:          false,
		DefaultContextMenuDisabled: true,
		OpenInspectorOnStartup:   false,
	})

	youtube.Center()
	youtube.Show()

	return app.Run()
}
