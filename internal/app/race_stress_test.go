package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

// Stresses shared settingsStore + proxy CONNECT (same wiring as main.go).
func TestRaceSettingsProxyConcurrent(t *testing.T) {
	store := NewSettingsStore(Preferences{Strategy: string(dpi.StrategyTCPSegmentation), DoHProvider: string(dns.ProviderCloudflare), AllowlistPreset: string(proxy.PresetYouTube)})
	resolver := dns.NewResolver(store.Snapshot().DoHProvider)
	srv := proxy.NewServer(resolver, store.Snapshot)
	ctrl := NewControlService(srv, resolver, store, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port, err := srv.Start(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop(context.Background())

	proxyURL, _ := url.Parse(fmt.Sprintf("http://%s:%d", config.ListenHost, port))
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	client := &http.Client{Timeout: 2 * time.Second, Transport: transport}

	strategies := dpi.AllStrategies()
	presets := ctrl.ListAllowlistPresets()
	providers := ctrl.ListDoHProviders()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					req, _ := http.NewRequest(http.MethodConnect, "http://www.youtube.com:443", nil)
					req.Host = "www.youtube.com:443"
					_, _ = client.Do(req)
				}
			}
		}()
	}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for j := seed; ; j++ {
				select {
				case <-stop:
					return
				default:
					_ = ctrl.SetStrategy(strategies[j%len(strategies)])
					_ = ctrl.SetAllowlistPreset(presets[j%len(presets)])
					_ = ctrl.SetDoHProvider(providers[j%len(providers)])
				}
			}
		}(i)
	}

	time.Sleep(3 * time.Second)
	close(stop)
	wg.Wait()
}
