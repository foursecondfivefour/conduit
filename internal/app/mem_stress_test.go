package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

func TestMemProxyConnectBurstNoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	store := NewSettingsStore(Preferences{AllowlistPreset: string(proxy.PresetYouTube)})
	resolver := dns.NewResolver(dns.ProviderCloudflare)
	srv := proxy.NewServer(resolver, store.Snapshot)

	ctx := context.Background()
	port, err := srv.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(ctx)

	proxyURL, _ := url.Parse(fmt.Sprintf("http://%s:%d", config.ListenHost, port))
	client := &http.Client{Timeout: 500 * time.Millisecond, Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	defer client.CloseIdleConnections()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				req, _ := http.NewRequest(http.MethodConnect, "http://www.youtube.com:443", nil)
				req.Host = "www.youtube.com:443"
				_, _ = client.Do(req)
			}
		}()
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}

func TestMemSettingsToggleHeapStable(t *testing.T) {
	store := NewSettingsStore(Preferences{
		Strategy:        string(dpi.StrategyTCPSegmentation),
		AllowlistPreset: string(proxy.PresetYouTube),
	})
	resolver := dns.NewResolver(dns.ProviderCloudflare)
	srv := proxy.NewServer(resolver, store.Snapshot)
	ctrl := NewControlService(srv, resolver, store, nil)

	ctx := context.Background()
	if _, err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(ctx)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	strategies := ctrl.ListStrategies()
	presets := ctrl.ListAllowlistPresets()
	for i := 0; i < 500; i++ {
		_ = ctrl.SetStrategy(strategies[i%len(strategies)])
		_ = ctrl.SetAllowlistPreset(presets[i%len(presets)])
		_ = store.Snapshot()
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	growth := int64(after.HeapInuse) - int64(before.HeapInuse)
	if growth > 2<<20 {
		t.Fatalf("heap grew %d bytes after 500 setting toggles", growth)
	}
}

func TestMemHealthCheckManyCalls(t *testing.T) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	ctx := context.Background()
	for i := 0; i < 200; i++ {
		_ = CheckYouTube(ctx, "http://127.0.0.1:1")
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	growth := int64(after.HeapInuse) - int64(before.HeapInuse)
	if growth > 8<<20 {
		t.Fatalf("heap grew %d bytes after 200 health checks", growth)
	}
}
