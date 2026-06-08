package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/foursecondfivefour/conduit/internal/app"
	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

func main() {
	noGUI := flag.Bool("no-gui", false, "run proxy only (no WebView window, lowest RAM)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	settings := config.DefaultSettings()
	resolver := dns.NewResolver()
	proxyServer := proxy.NewServer(resolver, func() config.Settings {
		return settings
	})

	port, err := proxyServer.Start(ctx)
	if err != nil {
		log.Fatalf("proxy start failed: %v", err)
	}
	log.Printf("proxy listening on %s:%d", config.ListenHost, port)

	if *noGUI {
		log.Printf("proxy URL: http://%s:%d", config.ListenHost, port)
		log.Printf("no-gui mode: open %s in a browser with the proxy above", config.YouTubeURL)
		<-ctx.Done()
	} else {
		if err := app.Run(ctx, proxyServer, &settings); err != nil {
			log.Fatalf("application error: %v", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.DialTimeout)
	defer shutdownCancel()
	if err := proxyServer.Stop(shutdownCtx); err != nil {
		log.Printf("proxy shutdown error: %v", err)
	}
}
