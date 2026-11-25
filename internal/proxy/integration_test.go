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
