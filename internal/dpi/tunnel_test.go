package dpi

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestRelayFragmentsFirstClientHello(t *testing.T) {
	client, proxySide := net.Pipe()
	upstreamClient, upstreamServer := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = proxySide.Close()
		_ = upstreamClient.Close()
		_ = upstreamServer.Close()
	})

	hello := []byte{0x16, 0x03, 0x01, 0x00, 0x03, 0x01, 0x02, 0x03}
	received := make(chan []byte, 1)

	go func() {
		raw, _ := io.ReadAll(upstreamServer)
		received <- raw
	}()

	go func() {
		_, _ = client.Write(hello)
		_ = client.Close()
	}()

	writer := NewFragmentWriter(StrategyTCPSegmentation)
	go func() { _, _, _ = Relay(proxySide, upstreamClient, writer) }()

	select {
	case got := <-received:
		if len(got) == 0 {
			t.Fatal("upstream received no data")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for upstream data")
	}
}

func TestRelayWithECHStrategy(t *testing.T) {
	client, proxySide := net.Pipe()
	upstreamClient, upstreamServer := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = proxySide.Close()
		_ = upstreamClient.Close()
		_ = upstreamServer.Close()
	})

	hello := []byte{0x16, 0x03, 0x01, 0x00, 0x0a, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a}
	received := make(chan []byte, 1)

	go func() {
		raw, _ := io.ReadAll(upstreamServer)
		received <- raw
	}()

	go func() {
		_, _ = client.Write(hello)
		_ = client.Close()
	}()

	writer := NewFragmentWriter(StrategyECH)
	go func() { _, _, _ = Relay(proxySide, upstreamClient, writer) }()

	select {
	case got := <-received:
		if len(got) == 0 {
			t.Fatal("upstream received no data for ECH")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for upstream data with ECH")
	}
}

func TestRelayNonTLSData(t *testing.T) {
	client, proxySide := net.Pipe()
	upstreamClient, upstreamServer := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = proxySide.Close()
		_ = upstreamClient.Close()
		_ = upstreamServer.Close()
	})

	data := []byte("GET / HTTP/1.1\r\nHost: test\r\n\r\n")
	received := make(chan []byte, 1)

	go func() {
		raw, _ := io.ReadAll(upstreamServer)
		received <- raw
	}()

	go func() {
		_, _ = client.Write(data)
		_ = client.Close()
	}()

	writer := NewFragmentWriter(StrategyTCPSegmentation)
	go func() { _, _, _ = Relay(proxySide, upstreamClient, writer) }()

	select {
	case got := <-received:
		if len(got) == 0 {
			t.Fatal("upstream should receive non-TLS data")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for non-TLS data")
	}
}

func TestNewIdleConnNil(t *testing.T) {
	if NewIdleConn(nil, 10*time.Second) != nil {
		t.Fatal("nil conn should return nil")
	}
	if NewIdleConn(nil, 0) != nil {
		t.Fatal("nil conn with zero idle should return nil")
	}
}

func TestNewIdleConnZeroTimeout(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })

	conn := NewIdleConn(client, 0)
	if conn != client {
		t.Fatal("zero-idle should return original conn")
	}
}

func TestNewIdleConnPositiveTimeout(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })

	conn := NewIdleConn(client, 10*time.Second)
	if conn == client {
		t.Fatal("positive-idle should wrap conn")
	}
}

func BenchmarkRelay(b *testing.B) {
	payload := make([]byte, 1400)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		client, proxySide := net.Pipe()
		upstreamClient, upstreamServer := net.Pipe()

		go func() {
			_, _ = io.ReadAll(upstreamServer)
			_ = upstreamServer.Close()
		}()
		go func() {
			_, _ = proxySide.Write(payload)
			_ = proxySide.Close()
		}()

		writer := NewFragmentWriter(StrategyNone)
		_, _, _ = Relay(proxySide, upstreamClient, writer)
		_ = client.Close()
	}
}
