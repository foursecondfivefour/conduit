package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/i18n"
	"github.com/foursecondfivefour/conduit/internal/proxy"
	"github.com/foursecondfivefour/conduit/internal/tun"
	"github.com/foursecondfivefour/conduit/internal/update"
	"github.com/foursecondfivefour/conduit/internal/winproxy"
	yt "github.com/foursecondfivefour/conduit/internal/youtube"
)

// RunInput bundles dependencies for the GUI application loop.
type RunInput struct {
	Ctx        context.Context
	Paths      RuntimePaths
	Prefs      *preferenceStore
	Proxy      *proxy.Server
	Resolver   *dns.Resolver
	Settings   *settingsStore
	WinProxy   *winproxy.Manager
	Updater    *update.Service
	UI         UIOptions
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
		Windows: windowsWebViewOptions(in.Paths.WebViewDataDir(), proxyURL, in.UI.Debug),
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

	// Create SOCKS5 server
	socks5 := proxy.NewSOCKS5Server(in.Resolver.DialContext, in.Settings.Snapshot)
	socks5Port := config.DefaultSOCKS5Port
	if p.SOCKS5Port > 0 {
		socks5Port = p.SOCKS5Port
	} else if p.SOCKS5 {
		socks5Port = port + config.SOCKS5PortOffset
	}
	if p.SOCKS5 {
		if err := socks5.Start(context.Background(), config.ListenHost, socks5Port); err != nil {
			slog.Warn("SOCKS5 start failed", "err", err)
		}
	}

	// Start extra proxy ports from preferences
	if len(p.ProxyPorts) > 0 {
		in.Proxy.AddExtraPorts(p.ProxyPorts)
	}

	// Feature 4.2: Upstream Proxy Chaining
	if p.UpstreamHost != "" && p.UpstreamPort > 0 {
		upstream := &proxy.UpstreamProxy{
			Type:     p.UpstreamType,
			Host:     p.UpstreamHost,
			Port:     p.UpstreamPort,
			User:     p.UpstreamUser,
			Password: p.UpstreamPassword,
		}
		in.Proxy.SetUpstream(upstream)
	}

	// Feature 4.1: Split Tunneling (bypass proxy on port+2)
	var splitTunnelSrv *http.Server
	splitTunnelPort := 0
	if p.SplitTunnel {
		splitTunnelPort = port + config.SplitTunnelPortOffset
		splitAddr := net.JoinHostPort(config.ListenHost, fmt.Sprintf("%d", splitTunnelPort))
		splitTunnelListener, listenErr := net.Listen("tcp", splitAddr)
	if listenErr != nil {
		slog.Warn("split tunnel listen failed", "err", listenErr, "addr", splitAddr)
	} else if splitTunnelListener != nil {
			splitTunnelPort = splitTunnelListener.Addr().(*net.TCPAddr).Port
			splitTunnelSrv = &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodConnect {
						http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
						return
					}
					hijacker, ok := w.(http.Hijacker)
					if !ok {
						http.Error(w, "hijack not supported", http.StatusInternalServerError)
						return
					}
					upConn, err := proxy.DirectDialer(r.Context(), "tcp", r.Host)
					if err != nil {
						http.Error(w, "upstream dial failed", http.StatusBadGateway)
						return
					}
					client, _, err := hijacker.Hijack()
					if err != nil {
						upConn.Close()
						return
					}
					defer client.Close()
					if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
						slog.Warn("split tunnel: write CONNECT response failed", "err", err)
						upConn.Close()
						return
					}
					go func() {
						defer upConn.Close()
						defer client.Close()
						if _, err := io.Copy(upConn, client); err != nil {
							slog.Debug("split tunnel: client->upstream copy done", "err", err)
						}
					}()
					if _, err := io.Copy(client, upConn); err != nil {
						slog.Debug("split tunnel: upstream->client copy done", "err", err)
					}
				}),
				ReadHeaderTimeout: config.ReadHeaderTimeout,
				ReadTimeout:       config.ReadHeaderTimeout,
				WriteTimeout:      config.IdleTimeout,
				IdleTimeout:       config.IdleTimeout,
			}
			go func() {
				if err := splitTunnelSrv.Serve(splitTunnelListener); err != nil && err != http.ErrServerClosed {
					slog.Warn("split tunnel serve error", "err", err)
				}
			}()
		}
	}

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

	// Feature 4.7: TUN Mode
	tunMode := tun.NewTUNMode()
	if p.TUNMode {
		exePath, _ := os.Executable()
		if exePath != "" {
			if err := tunMode.Enable(exePath); err != nil {
				slog.Warn("tun mode enable failed", "err", err)
			}
		}
	}

	// Feature 5.2: MASQUE (HTTP/3) Proxy
	masqueServer := proxy.NewMASQUEServer()
	if p.MASQUE {
		if err := masqueServer.Start(context.Background(), config.ListenHost, config.DefaultMASQUEPort); err != nil {
			slog.Warn("MASQUE start failed", "err", err)
		}
	}

	// Feature 5.3: AI DPI Strategy Selector
	var aiSelector *dpi.AIStrategySelector
	if p.AIDPI {
		aiSelector = dpi.NewAIStrategySelector(in.Settings.Snapshot().Strategy)
		in.Proxy.SetAISelector(aiSelector)
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
		app:          app,
		youtube:      youtube,
		proxy:        in.Proxy,
		control:      control,
		prefs:        in.Prefs,
		flow:         flow,
		updater:      in.Updater,
		winProxy:     in.WinProxy,
		paths:        in.Paths,
		ui:           in.UI,
		pip:          pip,
		adBlocker:    adBlocker,
		socks5:       socks5,
		stats:        stats,
		tunMode:      tunMode,
		masque:       masqueServer,
		splitTunnelPort: splitTunnelPort,
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
