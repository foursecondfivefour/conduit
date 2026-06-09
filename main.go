//go:generate go-winres simply --icon assets/windows/icon.ico --manifest gui --product-name Conduit --file-description Conduit --copyright Copyright (c) foursecondfivefour --out rsrc

package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
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
	pprofAddr := flag.String("pprof", "", "enable pprof HTTP server on 127.0.0.1:port (e.g. :6060)")
	memProfile := flag.String("memprofile", "", "write heap profile to path on exit")
	debug := flag.Bool("debug", false, "enable debug logging and WebView developer tools")
	debugInspector := flag.Bool("debug-inspector", false, "open WebView inspector on YouTube at startup (requires -debug, non-production build)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if closer, err := startPprof(*pprofAddr); err != nil {
		log.Fatalf("pprof: %v", err)
	} else if closer != nil {
		defer closer()
	}
	if *memProfile != "" {
		defer writeMemProfile(*memProfile)
	}

	paths, err := app.ResolveRuntimePaths(*portable)
	if err != nil {
		log.Fatalf("paths: %v", err)
	}

	if logCloser, err := filelog.Setup(paths.LogDir, *debug); err != nil {
		log.Printf("file log setup failed: %v", err)
	} else {
		defer func() { _ = logCloser.Close() }()
		slog.Info("conduit starting", "version", config.Version, "portable", *portable, "debug", *debug)
	}

	prefs, err := app.NewPreferenceStore(paths)
	if err != nil {
		log.Fatalf("preferences: %v", err)
	}
	defer func() { _ = prefs.Close() }()

	settingsStore := app.NewSettingsStore(prefs.Get())
	resolver := dns.NewResolver(settingsStore.Snapshot().DoHProvider)
	proxyServer := proxy.NewServer(resolver, settingsStore.Snapshot)

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
			Settings: settingsStore,
			WinProxy: winProxy,
			Updater:  updater,
			UI: app.UIOptions{
				Debug:          *debug,
				DebugInspector: *debugInspector,
			},
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

func startPprof(addr string) (func(), error) {
	if addr == "" {
		return nil, nil
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	host := "127.0.0.1"
	if strings.HasPrefix(addr, ":") {
		addr = host + addr
	} else if !strings.HasPrefix(addr, host) {
		addr = host + ":" + strings.TrimPrefix(addr, ":")
	}

	srv := &http.Server{Addr: addr, Handler: http.DefaultServeMux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("pprof server error", "err", err)
		}
	}()
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), config.DialTimeout)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}, nil
}

func writeMemProfile(path string) {
	f, err := os.Create(path)
	if err != nil {
		slog.Warn("memprofile create failed", "err", err)
		return
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		slog.Warn("memprofile write failed", "err", err)
	}
}
