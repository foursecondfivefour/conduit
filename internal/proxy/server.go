package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/dns"
)

// Server is a local HTTP CONNECT proxy with DPI-aware TLS fragmentation.
type Server struct {
	addr     string
	resolver *dns.Resolver
	settings func() config.Settings
	dialFunc func(context.Context, string, string) (net.Conn, error)

	mu       sync.RWMutex
	httpSrv  *http.Server
	listener net.Listener
	running  atomic.Bool
}

func NewServer(resolver *dns.Resolver, settings func() config.Settings) *Server {
	return &Server{
		resolver: resolver,
		settings: settings,
	}
}

func (s *Server) Start(ctx context.Context) (int, error) {
	if s.running.Load() {
		return s.Port(), nil
	}

	addr := net.JoinHostPort(config.ListenHost, fmt.Sprintf("%d", config.DefaultProxyPort))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		ln, err = net.Listen("tcp", net.JoinHostPort(config.ListenHost, "0"))
		if err != nil {
			return 0, err
		}
	}

	port := ln.Addr().(*net.TCPAddr).Port
	s.mu.Lock()
	s.listener = ln
	s.addr = ln.Addr().String()
	s.httpSrv = &http.Server{
		Handler:           http.HandlerFunc(s.serve),
		ReadHeaderTimeout: config.ReadHeaderTimeout,
	}
	s.mu.Unlock()

	s.running.Store(true)
	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("proxy: serve error: %v", err)
		}
		s.running.Store(false)
	}()

	return port, nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpSrv
	s.mu.Unlock()

	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

func (s *Server) Restart(ctx context.Context) (int, error) {
	if s.resolver != nil {
		s.resolver.ClearCache()
	}
	if s.running.Load() {
		return s.Port(), nil
	}
	return s.Start(ctx)
}

func (s *Server) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil {
		return 0
	}
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

func (s *Server) Running() bool {
	return s.running.Load()
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
		return
	}

	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	settings := s.settings()
	if !AllowedHostForSettings(host, settings) {
		http.Error(w, "host not allowed", http.StatusForbidden)
		return
	}

	dial := s.resolver.DialContext
	if s.dialFunc != nil {
		dial = s.dialFunc
	}
	upstream, err := dial(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, "upstream dial failed", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	defer client.Close()

	_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")

	writer := dpi.NewFragmentWriter(settings.Strategy)
	_ = dpi.Relay(client, upstream, writer)
}

func (s *Server) ProxyURL() string {
	port := s.Port()
	if port == 0 {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", config.ListenHost, port)
}
