package dpi

import (
	"io"
	"net"
	"testing"
)

func TestWriteTCPSegmentsSplitsPayload(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	data := []byte{0x16, 0x03, 0x01, 0x00, 0x05, 0x01, 0x02, 0x03, 0x04, 0x05}
	readDone := make(chan []byte, 1)
	go func() {
		raw, _ := io.ReadAll(server)
		readDone <- raw
	}()

	if err := writeTCPSegments(client, data); err != nil {
		t.Fatalf("writeTCPSegments: %v", err)
	}
	_ = client.Close()

	got := <-readDone
	if len(got) != len(data) {
		t.Fatalf("received %d bytes, want %d", len(got), len(data))
	}
}

func TestWriteTLSRecordFragmentsReassembles(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	data := []byte{
		0x16, 0x03, 0x01, 0x00, 0x08,
		0x01, 0x00, 0x00, 0x04, 0x03, 0x03, 0xaa, 0xbb,
	}

	readDone := make(chan []byte, 1)
	go func() {
		raw, _ := io.ReadAll(server)
		readDone <- raw
	}()

	if err := writeTLSRecordFragments(client, data); err != nil {
		t.Fatalf("writeTLSRecordFragments: %v", err)
	}
	_ = client.Close()

	got := <-readDone
	if len(got) < 5 {
		t.Fatalf("received %d bytes, want TLS records", len(got))
	}
}

func TestFragmentWriterWriteFirstTCP(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	writer := NewFragmentWriter(StrategyTCPSegmentation)
	payload := []byte{0x16, 0x03, 0x01, 0x00, 0x03, 0xaa, 0xbb, 0xcc}

	readDone := make(chan struct{})
	go func() {
		_, _ = io.ReadAll(server)
		close(readDone)
	}()

	if err := writer.WriteFirst(client, payload); err != nil {
		t.Fatalf("WriteFirst: %v", err)
	}
	_ = client.Close()
	<-readDone
}
