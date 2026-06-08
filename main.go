//go:generate go-winres simply --icon assets/windows/icon.ico --manifest gui --product-name Conduit --file-description Conduit --copyright Copyright (c) foursecondfivefour --out rsrc

package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/foursecondfivefour/conduit/internal/app"
	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	filelog "github.com/foursecondfivefour/conduit/internal/log"
	"github.com/foursecondfivefour/conduit/internal/proxy"
	"github.com/foursecondfivefour/conduit/internal/update"
	"github.com/foursecondfivefour/conduit/internal/winproxy"
)

func main() {
	noGUI := flag.Bool("no-gui", false, "run proxy only (no WebView window, lowest RAM)")
	portable := flag.Bool("portable", false, "store settings next to the executable")
	systemProxy := flag.Bool("system-proxy", false, "enable Windows system proxy (no-gui mode)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	paths, err := app.ResolveRuntimePaths(*portable)
	if err != nil {
		log.Fatalf("paths: %v", err)
	}

	if _, err := filelog.Setup(paths.LogDir); err != nil {
		log.Printf("file log setup failed: %v", err)
	} else {
		slog.Info("conduit starting", "version", config.Version, "portable", *portable)
	}

	prefs, err := app.NewPreferenceStore(paths)
	if err != nil {
		log.Fatalf("preferences: %v", err)
	}
	defer func() { _ = prefs.Close() }()

	settings := app.SettingsFromPreferences(prefs.Get())
	resolver := dns.NewResolver(settings.DoHProvider)
	proxyServer := proxy.NewServer(resolver, func() config.Settings {
		return settings
	})

	winProxy := winproxy.NewManager()
	updater := update.NewService()

	port, err := proxyServer.Start(ctx)
	if err != nil {
		log.Fatalf("proxy start failed: %v", err)
	}
	slog.Info("proxy listening", "addr", config.ListenHost, "port", port)

	if *systemProxy || (!*noGUI && prefs.Get().SystemProxy) {
		if err := winProxy.Enable(config.ListenHost, port); err != nil {
			slog.Warn("system proxy enable failed", "err", err)
		}
	}
	defer func() {
		if err := winProxy.Disable(); err != nil {
			slog.Warn("system proxy disable failed", "err", err)
		}
	}()

	if *noGUI {
		slog.Info("no-gui mode", "proxy", proxyServer.ProxyURL(), "youtube", config.YouTubeURL)
		<-ctx.Done()
	} else {
		if err := app.Run(app.RunInput{
			Ctx:      ctx,
			Paths:    paths,
			Prefs:    prefs,
			Proxy:    proxyServer,
			Resolver: resolver,
			Settings: &settings,
			WinProxy: winProxy,
			Updater:  updater,
		}); err != nil {
			log.Fatalf("application error: %v", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.DialTimeout)
	defer shutdownCancel()
	if err := proxyServer.Stop(shutdownCtx); err != nil {
		slog.Warn("proxy shutdown error", "err", err)
	}
}
