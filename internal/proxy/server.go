package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
)

// hostValidRegex matches valid hostnames (alphanumeric, dots, colons, hyphens only).
var hostValidRegex = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

// Server is a local HTTP CONNECT proxy with DPI-aware TLS fragmentation.
type Server struct {
	addr     string
	resolver *dns.Resolver
	settings func() config.Settings
	dialFunc func(context.Context, string, string) (net.Conn, error)

	stats      *TrafficStats
	killSwitch *KillSwitch

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
		ReadTimeout:       config.ReadHeaderTimeout,
		WriteTimeout:      config.IdleTimeout,
		IdleTimeout:       config.IdleTimeout,
	}
	s.mu.Unlock()

	s.running.Store(true)
	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("proxy: serve error", "err", err)
		}
		s.running.Store(false)
	}()

	return port, nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.RLock()
	srv := s.httpSrv
	s.mu.RUnlock()

	if srv == nil {
		return nil
	}
	var lastErr error
	if err := srv.Shutdown(ctx); err != nil && lastErr == nil {
		lastErr = err
	}
	return lastErr
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
	// Serve PAC file if requested
	if r.Method == http.MethodGet && r.URL.Path == "/proxy.pac" {
		st := s.settings()
		domains := st.CustomDomains
		preset := AllowlistPreset(st.AllowlistPreset)
		if preset.Valid() {
			domains = append(domains, preset.Domains()...)
		}
		PACHandler(config.ListenHost, s.Port(), domains)(w, r)
		return
	}

	if r.Method != http.MethodConnect {
		http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
		return
	}

	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}

	// A03: validate hostname to prevent host-header injection
	if !hostValidRegex.MatchString(host) {
		slog.Warn("proxy: invalid host characters rejected", "host", host)
		http.Error(w, "invalid host", http.StatusBadRequest)
		return
	}
	settings := s.settings()
	if !AllowedHostForSettings(host, settings) {
		http.Error(w, "host not allowed", http.StatusForbidden)
		return
	}

	// Determine the fragmentation strategy for this connection.
	strategy := settings.Strategy
	if !strategy.Valid() {
		strategy = dpi.DefaultStrategy
	}

	dial := s.dialFunc
	if dial == nil {
		dial = s.resolver.DialContext
	}

	upstream, err := dial(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, "upstream dial failed", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		if err := upstream.Close(); err != nil {
			slog.Warn("proxy: close upstream after hijack failure", "err", err)
		}
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		if err := upstream.Close(); err != nil {
			slog.Warn("proxy: close upstream after hijack error", "err", err)
		}
		return
	}
	defer client.Close()

	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		slog.Warn("proxy: write CONNECT response failed", "err", err)
		upstream.Close()
		client.Close()
		return
	}

	client = dpi.NewIdleConn(client, config.IdleTimeout)
	upstream = dpi.NewIdleConn(upstream, config.IdleTimeout)

	policy := dpi.NewDomainPolicy(strategy)
	if settings.DomainStrategies != nil {
		for domain, strategyVal := range settings.DomainStrategies {
			st := dpi.Strategy(strategyVal)
			if st.Valid() {
				policy.Set(domain, st)
			}
		}
	}
	finalStrategy := policy.Get(host)
	writer := dpi.NewFragmentWriter(finalStrategy)

	if s.stats != nil {
		s.stats.ActiveConns.Add(1)
	}
	sent, recv, relayErr := dpi.Relay(client, upstream, writer)
	if relayErr != nil {
		slog.Warn("proxy: relay error", "err", relayErr, "host", host, "sent", sent, "recv", recv)
	}
	if s.stats != nil {
		s.stats.RecordConnection(host, sent, recv)
		s.stats.ActiveConns.Add(-1)
	}
}

func (s *Server) SetKillSwitch(ks *KillSwitch) {
	s.killSwitch = ks
}

func (s *Server) ProxyURL() string {
	port := s.Port()
	if port == 0 {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", config.ListenHost, port)
}

// SetStats attaches a TrafficStats tracker to this server.
func (s *Server) SetStats(stats *TrafficStats) {
	s.stats = stats
}

// Stats returns the attached traffic statistics tracker.
func (s *Server) Stats() *TrafficStats {
	return s.stats
}
