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

func TestIntegrationTrafficStats(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = PresetYouTube.String()

	srv := NewServer(dns.NewResolver(dns.ProviderCloudflare), func() config.Settings { return settings })

	stats := NewTrafficStats()
	srv.SetStats(stats)

	stats.RecordConnection("www.youtube.com", 1024, 2048)
	stats.RecordConnection("r1---sn.googlevideo.com", 512, 1024)

	snapshot := stats.Snapshot()
	if snapshot.Connections != 2 {
		t.Fatalf("connections = %d, want 2", snapshot.Connections)
	}
	if snapshot.BytesIn != 1536 {
		t.Fatalf("bytesIn = %d, want 1536", snapshot.BytesIn)
	}
	if snapshot.BytesOut != 3072 {
		t.Fatalf("bytesOut = %d, want 3072", snapshot.BytesOut)
	}
	if len(snapshot.DomainHits) != 2 {
		t.Fatalf("domainHits = %d, want 2", len(snapshot.DomainHits))
	}
	if snapshot.DomainHits["www.youtube.com"] != 1 {
		t.Fatalf("youtube hits = %d, want 1", snapshot.DomainHits["www.youtube.com"])
	}
}

func TestIntegrationTrafficStatsActiveConnections(t *testing.T) {
	stats := NewTrafficStats()
	stats.ActiveConns.Add(1)
	stats.ActiveConns.Add(1)
	stats.ActiveConns.Add(-1)

	snapshot := stats.Snapshot()
	if snapshot.ActiveConns != 1 {
		t.Fatalf("activeConns = %d, want 1", snapshot.ActiveConns)
	}
	if snapshot.Uptime < 0 {
		t.Fatalf("uptime = %d, should be >= 0", snapshot.Uptime)
	}
}

func TestIntegrationDirectDialer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := DirectDialer(ctx, "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("DirectDialer: %v", err)
	}
	conn.Close()
}

func TestIntegrationSOCKS5StartStop(t *testing.T) {
	srv := NewSOCKS5Server(DirectDialer, func() config.Settings { return config.DefaultSettings() })
	ctx := context.Background()

	if err := srv.Start(ctx, "127.0.0.1", 0); err != nil {
		t.Fatalf("SOCKS5 start: %v", err)
	}

	port := srv.Port()
	if port == 0 {
		t.Fatal("SOCKS5 port should not be 0")
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("SOCKS5 stop: %v", err)
	}
}

func TestIntegrationSOCKS5DoubleStart(t *testing.T) {
	srv := NewSOCKS5Server(DirectDialer, func() config.Settings { return config.DefaultSettings() })
	ctx := context.Background()

	if err := srv.Start(ctx, "127.0.0.1", 0); err != nil {
		t.Fatalf("SOCKS5 first start: %v", err)
	}
	firstPort := srv.Port()

	if err := srv.Start(ctx, "127.0.0.1", firstPort+1); err != nil {
		t.Fatalf("SOCKS5 second start: %v", err)
	}
	if srv.Port() != firstPort {
		t.Fatalf("SOCKS5 port changed: %d != %d", srv.Port(), firstPort)
	}

	srv.Stop()
}

func TestIntegrationSOCKS5StopNotStarted(t *testing.T) {
	srv := NewSOCKS5Server(DirectDialer, func() config.Settings { return config.DefaultSettings() })
	if err := srv.Stop(); err != nil {
		t.Fatalf("SOCKS5 stop without start: %v", err)
	}
}

func TestIntegrationMASQUEStartStop(t *testing.T) {
	m := NewMASQUEServer()
	ctx := context.Background()

	if err := m.Start(ctx, "127.0.0.1", 0); err != nil {
		t.Fatalf("MASQUE start: %v", err)
	}

	if !m.IsEnabled() {
		t.Fatal("MASQUE should be enabled after start")
	}

	if m.Port() == 0 {
		t.Fatal("MASQUE port should not be 0")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("MASQUE stop: %v", err)
	}

	if m.IsEnabled() {
		t.Fatal("MASQUE should be disabled after stop")
	}
}

func TestIntegrationMASQUEDoubleStart(t *testing.T) {
	m := NewMASQUEServer()
	ctx := context.Background()

	if err := m.Start(ctx, "127.0.0.1", 0); err != nil {
		t.Fatalf("MASQUE first start: %v", err)
	}
	firstPort := m.Port()

	if err := m.Start(ctx, "127.0.0.1", firstPort+1); err != nil {
		t.Fatalf("MASQUE second start: %v", err)
	}
	if m.Port() != firstPort {
		t.Fatalf("MASQUE port changed: %d != %d", m.Port(), firstPort)
	}

	m.Stop()
}

func TestIntegrationUpstreamProxyConfig(t *testing.T) {
	up := &UpstreamProxy{
		Type:     "http",
		Host:     "proxy.example.com",
		Port:     3128,
		User:     "user",
		Password: "pass",
	}

	if !up.IsConfigured() {
		t.Fatal("UpstreamProxy should be configured")
	}
	if up.Address() != "proxy.example.com:3128" {
		t.Fatalf("Address = %s", up.Address())
	}
}

func TestIntegrationUpstreamProxyEmpty(t *testing.T) {
	up := &UpstreamProxy{}

	if up.IsConfigured() {
		t.Fatal("empty UpstreamProxy should not be configured")
	}
}

func TestIntegrationGeneratePAC(t *testing.T) {
	pac := GeneratePAC("127.0.0.1", 31284, []string{"youtube.com", "googlevideo.com"})
	if !strings.Contains(pac, "FindProxyForURL") {
		t.Fatal("PAC should contain FindProxyForURL")
	}
	if !strings.Contains(pac, "youtube.com") {
		t.Fatal("PAC should contain youtube.com")
	}
	if !strings.Contains(pac, "PROXY 127.0.0.1:31284") {
		t.Fatal("PAC should contain PROXY directive")
	}
}

func TestIntegrationGeneratePACEmptyDomains(t *testing.T) {
	pac := GeneratePAC("127.0.0.1", 31284, nil)
	if !strings.Contains(pac, "FindProxyForURL") {
		t.Fatal("PAC should contain FindProxyForURL")
	}
	if !strings.Contains(pac, "PROXY 127.0.0.1:31284") {
		t.Fatal("PAC should contain PROXY directive for empty domains")
	}
}

func TestIntegrationSpeedTestInvalidURL(t *testing.T) {
	ctx := context.Background()
	result := RunSpeedTest(ctx, "://invalid")
	if result.Error == "" {
		t.Fatal("expected error for invalid URL")
	}
}
