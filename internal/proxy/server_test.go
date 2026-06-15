package proxy

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
)

func TestConnectForbiddenHost(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	req := httptest.NewRequest(http.MethodConnect, "http://example.com:443", nil)
	req.Host = "example.com:443"
	rec := httptest.NewRecorder()
	srv.serve(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestConnectAllowedHost(t *testing.T) {
	serverConn, _ := net.Pipe()
	t.Cleanup(func() { _ = serverConn.Close() })

	srv := newTestServer(t, func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "www.youtube.com:443" {
			t.Fatalf("dial address = %q", address)
		}
		return serverConn, nil
	})
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
		t.Fatalf("response = %q, want 200", line)
	}
}

func waitBool(t *testing.T, fn func() bool, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fn()
}

func TestConnectNonConnectMethod(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	req := httptest.NewRequest(http.MethodGet, "http://www.youtube.com/", nil)
	rec := httptest.NewRecorder()
	srv.serve(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestServerPortAndAddr(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	port := srv.Port()
	if port == 0 {
		t.Fatal("Port() should not be 0")
	}
	addr := srv.Addr()
	if addr == "" {
		t.Fatal("Addr() should not be empty")
	}
	if !srv.Running() {
		t.Fatal("server should be running")
	}
}

func TestServerProxyURL(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	url := srv.ProxyURL()
	if url == "" {
		t.Fatal("ProxyURL should not be empty")
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("unexpected ProxyURL: %s", url)
	}
}

func TestServerStopNotRunning(t *testing.T) {
	srv := newTestServer(t, nil)
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("double Stop error: %v", err)
	}
}

func TestServerTrafficStats(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	stats := NewTrafficStats()
	srv.SetStats(stats)
	if srv.Stats() != stats {
		t.Fatal("Stats() should return the set stats")
	}
}

func TestServerKillSwitch(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	ks := NewKillSwitch()
	srv.SetKillSwitch(ks)
}

func TestServerSetUpstream(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	up := &UpstreamProxy{Host: "proxy.example.com", Port: 8080, Type: "http"}
	srv.SetUpstream(up)
}

func TestServerRestart(t *testing.T) {
	srv := newTestServer(t, nil)
	ctx := context.Background()

	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	port, err := srv.Restart(ctx)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if port == 0 {
		t.Fatal("restart port should not be 0")
	}
	if !srv.Running() {
		t.Fatal("server should be running after restart")
	}
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("stop after restart: %v", err)
	}
}

func TestServerStartTwice(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	port1, err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	port2, err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("second start: %v", err)
	}
	if port1 != port2 {
		t.Fatalf("second start should return same port: %d != %d", port2, port1)
	}
}

func TestServerExtraPortsStart(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	cfg := []config.ProxyPortConfig{
		{Port: 0, TCPPort: true},
	}
	srv.AddExtraPorts(cfg)

	extraPorts := srv.ExtraPorts()
	if len(extraPorts) == 0 {
		t.Fatal("expected at least one extra port")
	}
}

func TestServerPACFile(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/proxy.pac", nil)
	rec := httptest.NewRecorder()
	srv.serve(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PAC status = %d, want 200", rec.Code)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "x-ns-proxy-autoconfig") && !strings.Contains(contentType, "text/plain") {
		t.Errorf("unexpected content type: %s", contentType)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "FindProxyForURL") {
		t.Fatal("PAC body should contain FindProxyForURL")
	}
}

func TestConnectWithUpstreamDial(t *testing.T) {
	serverConn, _ := net.Pipe()
	t.Cleanup(func() { _ = serverConn.Close() })

	srv := newTestServer(t, nil)
	defer srv.Stop(context.Background())

	up := &UpstreamProxy{Host: "localhost", Port: 3128, Type: "http"}
	srv.SetUpstream(up)

	srv.dialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		return serverConn, nil
	}

	req := httptest.NewRequest(http.MethodConnect, "http://www.youtube.com:443", nil)
	req.Host = "www.youtube.com:443"
	rec := newHijackRecorder()
	go srv.serve(rec, req)

	if !waitBool(t, func() bool { return rec.Hijacked() }, 2*time.Second) {
		t.Fatal("expected hijacked connection with upstream")
	}

	reader := bufio.NewReader(rec.ClientConn())
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.Contains(line, "200") {
		t.Fatalf("response = %q", line)
	}

	_ = rec.ClientConn().Close()
	_ = serverConn.Close()
}

func newTestServer(t *testing.T, dial func(context.Context, string, string) (net.Conn, error)) *Server {
	t.Helper()
	settings := config.DefaultSettings()
	srv := NewServer(dns.NewResolver(dns.ProviderCloudflare), func() config.Settings { return settings })
	srv.dialFunc = dial
	if _, err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	return srv
}

type hijackRecorder struct {
	mu       sync.Mutex
	hijacked bool
	conn     net.Conn
}

func newHijackRecorder() *hijackRecorder {
	return &hijackRecorder{}
}

func (h *hijackRecorder) Header() http.Header         { return http.Header{} }
func (h *hijackRecorder) Write([]byte) (int, error)   { return 0, http.ErrNotSupported }
func (h *hijackRecorder) WriteHeader(int)             {}

func (h *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	client, server := net.Pipe()
	h.mu.Lock()
	h.hijacked = true
	h.conn = client
	h.mu.Unlock()
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}

func (h *hijackRecorder) Hijacked() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hijacked
}

func (h *hijackRecorder) ClientConn() net.Conn {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.conn
}

var _ http.Hijacker = (*hijackRecorder)(nil)
