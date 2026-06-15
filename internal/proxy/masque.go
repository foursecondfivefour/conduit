package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
)

// MASQUEServer provides HTTP/3 (QUIC) proxy support.
// It listens on a UDP port and serves as a stub for future QUIC-based proxying.
// Full MASQUE requires a QUIC library (e.g., quic-go) for HTTP/3 datagram support.
type MASQUEServer struct {
	host    string
	port    int
	running bool
	conn    net.PacketConn
}

// NewMASQUEServer creates a new MASQUE/QUIC proxy stub.
func NewMASQUEServer() *MASQUEServer {
	return &MASQUEServer{}
}

// Start begins listening on the given UDP host:port for QUIC connections.
func (m *MASQUEServer) Start(ctx context.Context, host string, port int) error {
	if m.running {
		return nil
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("masque: UDP listen failed: %w", err)
	}
	m.conn = conn
	m.host = host
	m.port = conn.LocalAddr().(*net.UDPAddr).Port
	m.running = true
	log.Printf("masque: QUIC listener stub started on %s", addr)
	return nil
}

// Stop closes the UDP listener and shuts down the server.
func (m *MASQUEServer) Stop() error {
	m.running = false
	if m.conn != nil {
		return m.conn.Close()
	}
	return nil
}

// IsEnabled returns whether the MASQUE server is currently running.
func (m *MASQUEServer) IsEnabled() bool { return m.running }

// Port returns the UDP port the server is listening on.
func (m *MASQUEServer) Port() int { return m.port }
