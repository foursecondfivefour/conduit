package dpi

import (
	"net"
	"syscall"
	"time"
)

const (
	defaultTCPSegmentSize   = 2
	defaultTCPSegmentDelay  = 2 * time.Millisecond
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
		strategy = StrategyTCPSegmentation
	}
	return &FragmentWriter{strategy: strategy}
}

func (f *FragmentWriter) WriteFirst(conn net.Conn, data []byte) error {
	if f.strategy == StrategyNone {
		_, err := conn.Write(data)
		return err
	}

	if err := setTCPNoDelay(conn); err != nil {
		_ = err
	}

	switch f.strategy {
	case StrategyTLSRecordFrag:
		return writeTLSRecordFragments(conn, data, defaultTLSRecordPayload, defaultTLSRecordDelay)
	case StrategyTLSRecordFrag2:
		return writeTLSRecordFragments(conn, data, 2, tlsRecordDelay2)
	case StrategyTCPSegmentation1:
		return writeTCPSegments(conn, data, 1, defaultTCPSegmentDelay)
	case StrategyTCPSegmentation8:
		return writeTCPSegments(conn, data, 8, defaultTCPSegmentDelay)
	case StrategyTCPSegmentation:
		return writeTCPSegments(conn, data, defaultTCPSegmentSize, defaultTCPSegmentDelay)
	default:
		return writeTCPSegments(conn, data, defaultTCPSegmentSize, defaultTCPSegmentDelay)
	}
}

func setTCPNoDelay(conn net.Conn) error {
	type tcpNoDelayer interface {
		SetNoDelay(bool) error
	}
	if c, ok := conn.(tcpNoDelayer); ok {
		return c.SetNoDelay(true)
	}
	if sc, ok := conn.(syscall.Conn); ok {
		raw, err := sc.SyscallConn()
		if err != nil {
			return err
		}
		var setErr error
		err = raw.Control(func(fd uintptr) {
			setErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
		})
		if err != nil {
			return err
		}
		return setErr
	}
	return nil
}

func writeTCPSegments(conn net.Conn, data []byte, size int, delay time.Duration) error {
	if size <= 0 {
		size = defaultTCPSegmentSize
	}
	for offset := 0; offset < len(data); {
		end := offset + size
		if end > len(data) {
			end = len(data)
		}
		if _, err := conn.Write(data[offset:end]); err != nil {
			return err
		}
		offset = end
		if offset < len(data) {
			time.Sleep(delay)
		}
	}
	return nil
}

func writeTLSRecordFragments(conn net.Conn, data []byte, chunk int, delay time.Duration) error {
	if len(data) < 5 {
		return writeTCPSegments(conn, data, defaultTCPSegmentSize, defaultTCPSegmentDelay)
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
