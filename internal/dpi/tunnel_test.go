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
	go func() { _ = Relay(proxySide, upstreamClient, writer) }()

	select {
	case got := <-received:
		if len(got) == 0 {
			t.Fatal("upstream received no data")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for upstream data")
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
		_ = Relay(proxySide, upstreamClient, writer)
		_ = client.Close()
	}
}
