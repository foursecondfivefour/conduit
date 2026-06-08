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

	if !waitBool(t, func() bool { return rec.hijacked }, 2*time.Second) {
		t.Fatal("expected hijacked connection")
	}

	reader := bufio.NewReader(rec.conn)
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
	h.hijacked = true
	h.conn = client
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}

var _ http.Hijacker = (*hijackRecorder)(nil)
