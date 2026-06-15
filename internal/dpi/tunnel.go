package dpi

import (
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024)
		return &buf
	},
}

// idleConn resets the read deadline before each read to enforce idle timeouts.
type idleConn struct {
	net.Conn
	idle time.Duration
}

// NewIdleConn wraps a connection with per-read idle deadline refresh.
func NewIdleConn(conn net.Conn, idle time.Duration) net.Conn {
	if conn == nil || idle <= 0 {
		return conn
	}
	return &idleConn{Conn: conn, idle: idle}
}

func (c *idleConn) Read(b []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.idle)); err != nil {
		slog.Warn("idleConn: set read deadline failed", "err", err)
	}
	return c.Conn.Read(b)
}

// Relay copies traffic between client and server, fragmenting only the first client→server write.
// Returns the byte counts: sent (client→server) and recv (server→client).
func Relay(client, server net.Conn, writer *FragmentWriter) (sent int64, recv int64, err error) {
	errCh := make(chan error, 2)
	var c2s int64

	go func() {
		n, e := copyClientFirst(client, server, writer)
		c2s = n
		errCh <- e
	}()

	go func() {
		bufPtr := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(bufPtr)
		n, e := io.CopyBuffer(client, server, *bufPtr)
		recv = n
		errCh <- e
	}()

	err = <-errCh
	if err := client.Close(); err != nil {
		slog.Warn("Relay: close client failed", "err", err)
	}
	if err := server.Close(); err != nil {
		slog.Warn("Relay: close server failed", "err", err)
	}
	<-errCh
	sent = c2s
	return
}

func copyClientFirst(client, server net.Conn, writer *FragmentWriter) (int64, error) {
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)
	buf := *bufPtr
	n, err := client.Read(buf)
	if err != nil {
		return 0, err
	}
	data := buf[:n]

	if writer != nil && IsTLSClientHello(data) {
		needed := ClientHelloSize(data)
		for needed > 0 && len(data) < needed {
			m, readErr := client.Read(buf)
			if readErr != nil {
				return 0, readErr
			}
			data = append(data, buf[:m]...)
		}
		if err := writer.WriteFirst(server, data); err != nil {
			return 0, err
		}
	} else {
		if _, err := server.Write(data); err != nil {
			return 0, err
		}
	}

	written, err := io.CopyBuffer(server, client, buf)
	return int64(len(data)) + written, err
}
