package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// UpstreamProxy configures an external proxy to chain through.
type UpstreamProxy struct {
	Type     string // "http", "socks5"
	Host     string
	Port     int
	User     string
	Password string
}

func (u *UpstreamProxy) Address() string {
	return net.JoinHostPort(u.Host, fmt.Sprintf("%d", u.Port))
}

func (u *UpstreamProxy) IsConfigured() bool {
	return u.Host != "" && u.Port > 0
}

// DialChain connects to the target through an upstream HTTP CONNECT proxy.
func DialChain(ctx context.Context, upstream *UpstreamProxy, targetNetwork, targetAddress string) (net.Conn, error) {
	if !upstream.IsConfigured() {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, targetNetwork, targetAddress)
	}

	// Connect to upstream proxy
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", upstream.Address())
	if err != nil {
		return nil, err
	}

	// Send CONNECT request
	req, _ := http.NewRequest(http.MethodConnect, "http://"+targetAddress, nil)
	if upstream.User != "" {
		req.SetBasicAuth(upstream.User, upstream.Password)
	}
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}

	// Read response
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("upstream proxy returned %d", resp.StatusCode)
	}

	return conn, nil
}
