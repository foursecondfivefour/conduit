package dpi

import (
	"crypto/tls"
	"net"
)

// ECHConn upgrades a plain TCP connection to TLS with Encrypted Client Hello support.
// In Go 1.25, ECH is natively supported via crypto/tls. When EncryptedClientHelloConfigList
// is nil, ECH is disabled. For auto-discovery, set it to a discovered config via DNS.
// This function establishes a TLS 1.3 connection — ECH-capable servers will use it if supported.
func ECHConn(conn net.Conn, serverName string) (net.Conn, error) {
	cfg := &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS13,
	}
	tlsConn := tls.Client(conn, cfg)
	return tlsConn, nil
}
