package dpi

import (
	"log/slog"
	"net"
	"syscall"
	"time"
)

const (
	defaultTLSRecordPayload = 5
	defaultTLSRecordDelay   = 2 * time.Millisecond
	tlsRecordDelay2         = 5 * time.Millisecond
)

// FragmentWriter sends the first TLS ClientHello using the selected DPI evasion strategy.
type FragmentWriter struct {
	strategy Strategy
}

func NewFragmentWriter(strategy Strategy) *FragmentWriter {
	if !strategy.Valid() {
		strategy = DefaultStrategy
	}
	return &FragmentWriter{strategy: strategy}
}

func (f *FragmentWriter) WriteFirst(conn net.Conn, data []byte) error {
	if f.strategy == StrategyNone {
		_, err := conn.Write(data)
		return err
	}

	if err := setTCPNoDelay(conn); err != nil {
		slog.Warn("fragment: set TCP_NODELAY failed", "err", err)
	}

	switch f.strategy {
	case StrategyTLSRecordFrag2:
		return writeTLSRecordFragments(conn, data, 2, tlsRecordDelay2)
	case StrategyTLSRecordFrag:
		return writeTLSRecordFragments(conn, data, defaultTLSRecordPayload, defaultTLSRecordDelay)
	default:
		return writeTLSRecordFragments(conn, data, defaultTLSRecordPayload, defaultTLSRecordDelay)
	}
}

func setTCPNoDelay(conn net.Conn) error {
	type tcpNoDelayer interface {
		SetNoDelay(bool) error
	}
	if c, ok := conn.(tcpNoDelayer); ok {
		return c.SetNoDelay(true)
	}
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return nil
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return err
	}
	var setErr error
	err = raw.Control(func(fd uintptr) {
		setErr = setNoDelayFD(fd)
	})
	if err != nil {
		return err
	}
	return setErr
}

func writeTLSRecordFragments(conn net.Conn, data []byte, chunk int, delay time.Duration) error {
	if len(data) < 5 {
		_, err := conn.Write(data)
		return err
	}

	version := data[1:3]
	payload := data[5:]
	if chunk <= 0 {
		chunk = defaultTLSRecordPayload
	}

	for offset := 0; offset < len(payload); {
		end := offset + chunk
		if end > len(payload) {
			end = len(payload)
		}
		part := payload[offset:end]
		record := make([]byte, 5+len(part))
		record[0] = 0x16
		copy(record[1:3], version)
		record[3] = byte((len(part) >> 8) & 0xff)
		record[4] = byte(len(part) & 0xff)
		copy(record[5:], part)

		if _, err := conn.Write(record); err != nil {
			return err
		}
		offset = end
		if offset < len(payload) {
			time.Sleep(delay)
		}
	}
	return nil
}

// IsTLSClientHello reports whether data begins with a TLS handshake record.
func IsTLSClientHello(data []byte) bool {
	return len(data) >= 5 && data[0] == 0x16
}

// ClientHelloSize returns the expected full ClientHello size or 0 if unknown yet.
func ClientHelloSize(data []byte) int {
	if len(data) < 5 || data[0] != 0x16 {
		return 0
	}
	recordLen := int(data[3])<<8 | int(data[4])
	if recordLen <= 0 || recordLen > 16384 {
		return 0
	}
	return 5 + recordLen
}
