package dpi

import (
	"io"
	"net"
	"time"
)

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
	_ = c.Conn.SetReadDeadline(time.Now().Add(c.idle))
	return c.Conn.Read(b)
}

// Relay copies traffic between client and server, fragmenting only the first client→server write.
func Relay(client, server net.Conn, writer *FragmentWriter) error {
	errCh := make(chan error, 2)

	go func() {
		errCh <- copyClientFirst(client, server, writer)
	}()

	go func() {
		_, err := io.Copy(client, server)
		errCh <- err
	}()

	err := <-errCh
	_ = client.Close()
	_ = server.Close()
	<-errCh
	return err
}

func copyClientFirst(client, server net.Conn, writer *FragmentWriter) error {
	buf := make([]byte, 32*1024)
	n, err := client.Read(buf)
	if err != nil {
		return err
	}
	data := buf[:n]

	if writer != nil && IsTLSClientHello(data) {
		needed := ClientHelloSize(data)
		for needed > 0 && len(data) < needed {
			m, readErr := client.Read(buf)
			if readErr != nil {
				return readErr
			}
			data = append(data, buf[:m]...)
		}
		if err := writer.WriteFirst(server, data); err != nil {
			return err
		}
	} else {
		if _, err := server.Write(data); err != nil {
			return err
		}
	}

	_, err = io.Copy(server, client)
	return err
}
