package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/proxy"
	"github.com/foursecondfivefour/conduit/internal/update"
	"github.com/foursecondfivefour/conduit/internal/winproxy"
)

// RunInput bundles dependencies for the GUI application loop.
type RunInput struct {
	Ctx        context.Context
	Paths      RuntimePaths
	Prefs      *preferenceStore
	Proxy      *proxy.Server
	Resolver   *dns.Resolver
	Settings   *config.Settings
	WinProxy   *winproxy.Manager
	Updater    *update.Service
}

// Run starts the WebView window, system tray, and startup flow.
func Run(in RunInput) error {
	port := in.Proxy.Port()
	if port == 0 {
		return fmt.Errorf("proxy is not running")
	}

	proxyURL := fmt.Sprintf("http://%s:%d", config.ListenHost, port)
	p := in.Prefs.Get()
	width := config.WindowWidth
	height := config.WindowHeight
	if p.WindowWidth > 0 {
		width = p.WindowWidth
	}
	if p.WindowHeight > 0 {
		height = p.WindowHeight
	}

	app := application.New(application.Options{
		Name:        "Conduit",
		Description: "Local CONNECT proxy with TLS fragmentation and YouTube viewer",
		Icon:        appIcon,
		Windows: application.WindowsOptions{
			AdditionalBrowserArgs: chromiumArgs(proxyURL),
		},
	})

	youtube := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                       "conduit",
		Title:                      "YouTube",
		Width:                      width,
		Height:                     height,
		MinWidth:                   480,
		MinHeight:                  360,
		Hidden:                     true,
		BackgroundColour:           application.NewRGB(18, 18, 18),
		URL:                        config.YouTubeURL,
		DevToolsEnabled:            false,
		DefaultContextMenuDisabled: true,
		OpenInspectorOnStartup:     false,
	})

	setupWindowCloseToTray(youtube, func() bool {
		return in.Prefs.Get().MinimizeToTray
	})
	control := NewControlService(in.Proxy, in.Resolver, in.Settings, in.Prefs)
	flow := newStartupFlow(app, youtube, in.Prefs)

	if p.SystemProxy {
		if err := in.WinProxy.Enable(config.ListenHost, port); err != nil {
			slog.Warn("system proxy enable failed", "err", err)
		}
	}

	deps := trayDeps{
		app:      app,
		youtube:  youtube,
		proxy:    in.Proxy,
		control:  control,
		prefs:    in.Prefs,
		flow:     flow,
		updater:  in.Updater,
		winProxy: in.WinProxy,
		paths:    in.Paths,
	}
	tray := setupTray(deps)
	setupHotkeyToggle(youtube, tray.stopCh)
	flow.begin()

	if in.Ctx != nil {
		go func() {
			<-in.Ctx.Done()
			tray.stop()
		}()
	}

	return app.Run()
}
