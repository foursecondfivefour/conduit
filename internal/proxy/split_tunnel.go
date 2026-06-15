package proxy

import (
	"context"
	"net"
	"time"
)

// DirectDialer creates TCP connections without going through the DPI proxy.
// Used for split-tunneling: traffic that should bypass the proxy.
func DirectDialer(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, network, address)
}
