package proxy

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
)

func TestIntegrationConnectAllowlistAndRelay(t *testing.T) {
	upstream, clientSide := net.Pipe()
	t.Cleanup(func() {
		_ = upstream.Close()
		_ = clientSide.Close()
	})

	settings := config.DefaultSettings()
	settings.AllowlistPreset = PresetYouTube.String()

	srv := NewServer(dns.NewResolver(dns.ProviderCloudflare), func() config.Settings { return settings })
	srv.dialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		return upstream, nil
	}
	if _, err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop(context.Background())

	req := httptest.NewRequest(http.MethodConnect, "http://www.youtube.com:443", nil)
	req.Host = "www.youtube.com:443"
	rec := newHijackRecorder()
	go srv.serve(rec, req)

	if !waitBool(t, func() bool { return rec.Hijacked() }, 2*time.Second) {
		t.Fatal("expected hijacked connection")
	}

	reader := bufio.NewReader(rec.ClientConn())
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.Contains(line, "200") {
		t.Fatalf("response = %q", line)
	}

	forbidden := httptest.NewRequest(http.MethodConnect, "http://example.com:443", nil)
	forbidden.Host = "example.com:443"
	rec2 := httptest.NewRecorder()
	srv.serve(rec2, forbidden)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d", rec2.Code)
	}
}
