package proxy

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
)

// TestConnectBurstNoGoroutineLeak drives many CONNECT requests through the proxy
// against a local dialer, so the check stays offline and still exercises the
// hijack/relay path.
func TestConnectBurstNoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	settings := config.DefaultSettings()
	settings.AllowlistPreset = PresetYouTube.String()
	srv := NewServer(dns.NewResolver(dns.ProviderCloudflare), func() config.Settings { return settings })
	srv.dialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		client, server := net.Pipe()
		// Drain and close the far end so every relay terminates immediately.
		go func() {
			buf := make([]byte, 1024)
			for {
				if _, err := server.Read(buf); err != nil {
					break
				}
			}
			_ = server.Close()
		}()
		return client, nil
	}

	ctx := context.Background()
	if _, err := srv.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop(ctx)

	proxyURL, _ := url.Parse(srv.ProxyURL())
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
