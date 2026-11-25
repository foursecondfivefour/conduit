package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/i18n"
	"github.com/foursecondfivefour/conduit/internal/proxy"
	"github.com/foursecondfivefour/conduit/internal/update"
	"github.com/foursecondfivefour/conduit/internal/winproxy"
	yt "github.com/foursecondfivefour/conduit/internal/youtube"
)

// RunInput bundles dependencies for the GUI application loop.
type RunInput struct {
	Ctx      context.Context
	Paths    RuntimePaths
	Prefs    *preferenceStore
	Proxy    *proxy.Server
	Resolver *dns.Resolver
	Settings *settingsStore
	WinProxy *winproxy.Manager
	Updater  *update.Service
	UI       UIOptions
}

// Run starts the WebView window, system tray, and startup flow.
func Run(in RunInput) error {
	port := in.Proxy.Port()
	if port == 0 {
		return fmt.Errorf("proxy is not running")
	}

	if in.UI.Debug {
		slog.Debug("frontend debug enabled", "inspector_on_startup", in.UI.DebugInspector, "remote_debugging_port", 9222)
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

	BackgroundAudioEnabled = p.BackgroundAudio

	app := application.New(application.Options{
		Name:        "Conduit",
		Description: "Local CONNECT proxy with TLS fragmentation and YouTube viewer",
		Icon:        appIcon,
		Windows:     windowsWebViewOptions(in.Paths.WebViewDataDir(), proxyURL, in.UI.Debug),
	})

	lang := i18n.ParseLang(p.Language)
	youtubeOpts := application.WebviewWindowOptions{
		Name:             "conduit",
		Title:            "YouTube",
		Width:            width,
		Height:           height,
		MinWidth:         480,
		MinHeight:        360,
		Hidden:           true,
		BackgroundColour: application.NewRGB(18, 18, 18),
		HTML:             renderLoadingHTML(lang),
		DevToolsEnabled:  in.UI.devToolsEnabled(),
	}
	applyLightweightWebviewDefaults(&youtubeOpts)
	youtube := app.Window.NewWithOptions(youtubeOpts)

	youtubeReady := StartYouTubePreload(youtube, lang)

	setupWindowCloseToTray(youtube, func() bool {
		return in.Prefs.Get().MinimizeToTray
	})

	// Create PiP (Picture-in-Picture) toggle for YouTube window
	pip := NewPiPWindow(youtube)

	// Create KillSwitch and wire it up
	killSwitch := proxy.NewKillSwitch()
	in.Proxy.SetKillSwitch(killSwitch)
	if p.KillSwitch {
		if err := killSwitch.Enable(); err != nil {
			slog.Warn("killswitch enable failed", "err", err)
		}
	}

	// Create DNS leak detector and blocker
	dnsLeak := &dns.LeakDetector{}
	dnsBlocker := winproxy.NewDNSBlocker()

	// Create TrafficStats
	stats := proxy.NewTrafficStats()
	in.Proxy.SetStats(stats)

	// Feature 4.6: Auto-Connect on startup
	if p.AutoConnect {
		if err := in.WinProxy.Enable(config.ListenHost, port); err != nil {
			slog.Warn("auto-connect system proxy enable failed", "err", err)
		}
		if !p.KillSwitch {
			if err := killSwitch.Enable(); err != nil {
				slog.Warn("auto-connect kill switch enable failed", "err", err)
			}
		}
	}

	// Create DNS ad blocker and wire it to the resolver
	adBlocker := dns.NewAdBlocker()
	in.Resolver.SetAdBlocker(adBlocker)
	if p.DNSAdBlock {
		adBlocker.SetEnabled(true)
		blockURL := p.DNSAdBlockURL
		if blockURL == "" {
			blockURL = config.DefaultAdBlockURL
		}
		go func() {
			if err := adBlocker.LoadFromURL(blockURL); err != nil {
				slog.Warn("adblock load failed", "err", err)
			}
		}()
	}

	control := NewControlService(in.Proxy, in.Resolver, in.Settings, in.Prefs)
	control.SetKillSwitch(killSwitch)
	control.SetDNSBlocker(dnsBlocker)
	control.SetLeakDetector(dnsLeak)

	flow := newStartupFlow(app, youtube, in.Prefs, youtubeReady, in.UI)

	// SponsorBlock event handler: fetch segments when JS requests them
	app.Event.On("sponsorblock:fetch", func(e *application.CustomEvent) {
		videoID, _ := e.Data.(string)
		if videoID == "" {
			return
		}
		segments, err := yt.FetchSegments(videoID, yt.DefaultCategories(), nil)
		if err != nil {
			slog.Debug("sponsorblock fetch failed", "videoID", videoID, "err", err)
			return
		}
		if b, err := json.Marshal(segments); err == nil {
			youtube.ExecJS("window.__sponsorblock_setSegments && window.__sponsorblock_setSegments(" + string(b) + ")")
		}
	})

	// RYD event handler: fetch dislike counts when JS requests them
	app.Event.On("ryd:fetch", func(e *application.CustomEvent) {
		videoID, _ := e.Data.(string)
		if videoID == "" {
			return
		}
		data, err := yt.FetchDislikes(videoID, nil)
		if err != nil {
			slog.Debug("ryd fetch failed", "videoID", videoID, "err", err)
			return
		}
		if b, err := json.Marshal(data); err == nil {
			youtube.ExecJS("window.__ryd_setDislikes && window.__ryd_setDislikes(" + string(b) + ")")
		}
	})

	// Inject YouTube feature scripts after navigation completes
	go func() {
		<-youtubeReady
		if p.SponsorBlock {
			youtube.ExecJS(yt.SponsorBlockJS())
		}
		if p.RYD {
			youtube.ExecJS(yt.RYDInjectJS())
		}
	}()

	if p.SystemProxy {
		if err := in.WinProxy.Enable(config.ListenHost, port); err != nil {
			slog.Warn("system proxy enable failed", "err", err)
		}
	}

	deps := trayDeps{
		app:       app,
		youtube:   youtube,
		proxy:     in.Proxy,
		control:   control,
		prefs:     in.Prefs,
		flow:      flow,
		updater:   in.Updater,
		winProxy:  in.WinProxy,
		paths:     in.Paths,
		ui:        in.UI,
		pip:       pip,
		adBlocker: adBlocker,
		stats:     stats,
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
