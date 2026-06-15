package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dpi"
)

var (
	errSOCKS5Version = errors.New("socks5: unsupported version")
	errSOCKS5Method  = errors.New("socks5: unsupported auth method")
	errSOCKS5Command = errors.New("socks5: unsupported command")
	errSOCKS5Address = errors.New("socks5: unsupported address type")
)

// SOCKS5Server is a SOCKS5 proxy (RFC 1928) that tunnels TCP through the DoH resolver.
type SOCKS5Server struct {
	mu       sync.Mutex
	listener net.Listener
	port     int
	running  atomic.Bool
	resolver func(ctx context.Context, network, address string) (net.Conn, error)
	settings func() config.Settings
}

// NewSOCKS5Server creates a SOCKS5 proxy server.
func NewSOCKS5Server(resolver func(ctx context.Context, network, address string) (net.Conn, error), settings func() config.Settings) *SOCKS5Server {
	return &SOCKS5Server{
		resolver: resolver,
		settings: settings,
	}
}

// Start begins listening on the given host:port.
func (s *SOCKS5Server) Start(ctx context.Context, host string, port int) error {
	if s.running.Load() {
		return nil
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		ln, err = net.Listen("tcp", net.JoinHostPort(host, "0"))
		if err != nil {
			return err
		}
	}
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.listener = ln
	s.running.Store(true)

	// A07: warn if SOCKS5 is exposed on a non-localhost interface without authentication.
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		slog.Warn("SOCKS5 proxy started without authentication on non-localhost interface",
			"host", host, "port", s.port)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
			if s.running.Load() {
				slog.Warn("socks5: accept error", "err", err)
			}
				return
			}
			go s.handleConn(conn)
		}
	}()
	return nil
}

// Stop closes the listener and stops accepting new connections.
func (s *SOCKS5Server) Stop() error {
	s.running.Store(false)
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Port returns the port the server is listening on.
func (s *SOCKS5Server) Port() int { return s.port }

func (s *SOCKS5Server) handleConn(client net.Conn) {
	defer client.Close()

	buf := make([]byte, 258)

	// Auth negotiation: read version + methods
	n, err := client.Read(buf[:2])
	if err != nil || n < 2 || buf[0] != 5 {
		return
	}
	nmethods := int(buf[1])
	_, err = io.ReadFull(client, buf[:nmethods])
	if err != nil {
		return
	}
	// Reply: no auth required
	if _, err := client.Write([]byte{5, 0}); err != nil {
		slog.Warn("socks5: write auth response failed", "err", err)
		return
	}

	// Request: read VER, CMD, RSV, ATYP
	n, err = client.Read(buf[:4])
	if err != nil || n < 4 || buf[0] != 5 {
		return
	}
	cmd := buf[1]
	atyp := buf[3]

	if cmd != 1 { // CONNECT only
		if _, err := client.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
			slog.Warn("socks5: write command not supported response failed", "err", err)
		}
		return
	}

	var host string
	var port uint16
	switch atyp {
	case 1: // IPv4
		_, err = io.ReadFull(client, buf[:4])
		if err != nil {
			return
		}
		host = net.IP(buf[:4]).String()
		_, err = io.ReadFull(client, buf[:2])
		if err != nil {
			return
		}
		port = uint16(buf[0])<<8 | uint16(buf[1])
	case 3: // Domain name
		_, err = io.ReadFull(client, buf[:1])
		if err != nil {
			return
		}
		domainLen := int(buf[0])
		_, err = io.ReadFull(client, buf[:domainLen])
		if err != nil {
			return
		}
		host = string(buf[:domainLen])
		_, err = io.ReadFull(client, buf[:2])
		if err != nil {
			return
		}
		port = uint16(buf[0])<<8 | uint16(buf[1])
	case 4: // IPv6
		_, err = io.ReadFull(client, buf[:16])
		if err != nil {
			return
		}
		host = net.IP(buf[:16]).String()
		_, err = io.ReadFull(client, buf[:2])
		if err != nil {
			return
		}
		port = uint16(buf[0])<<8 | uint16(buf[1])
	default:
		if _, err := client.Write([]byte{5, 8, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
			slog.Warn("socks5: write address not supported response failed", "err", err)
		}
		return
	}

	// Check allowlist
	st := s.settings()
	if !AllowedHostForSettings(host, st) {
		if _, err := client.Write([]byte{5, 2, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
			slog.Warn("socks5: write not allowed response failed", "err", err)
		}
		return
	}

	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	upstream, err := s.resolver(context.Background(), "tcp", address)
	if err != nil {
		if _, err := client.Write([]byte{5, 4, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
			slog.Warn("socks5: write host unreachable response failed", "err", err)
		}
		return
	}
	defer upstream.Close()

	// Success reply
	if _, err := client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		slog.Warn("socks5: write success response failed", "err", err)
		upstream.Close()
		return
	}

	// Relay with DPI
	client = dpi.NewIdleConn(client, config.IdleTimeout)
	upstream = dpi.NewIdleConn(upstream, config.IdleTimeout)
	writer := dpi.NewFragmentWriter(st.Strategy)
	sent, recv, relayErr := dpi.Relay(client, upstream, writer)
	if relayErr != nil {
		slog.Warn("socks5: relay error", "err", relayErr, "sent", sent, "recv", recv)
	}
}
