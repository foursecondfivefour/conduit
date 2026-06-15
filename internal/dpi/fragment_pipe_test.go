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

	if err := writeTCPSegments(client, data, defaultTCPSegmentSize, defaultTCPSegmentDelay); err != nil {
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

	if err := writeTLSRecordFragments(client, data, defaultTLSRecordPayload, defaultTLSRecordDelay); err != nil {
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

func TestFragmentWriterWriteFirstECH(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	writer := NewFragmentWriter(StrategyECH)
	payload := []byte{0x16, 0x03, 0x01, 0x00, 0x0a, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a}

	readDone := make(chan struct{})
	go func() {
		_, _ = io.ReadAll(server)
		close(readDone)
	}()

	if err := writer.WriteFirst(client, payload); err != nil {
		t.Fatalf("WriteFirst with ECH: %v", err)
	}
	_ = client.Close()
	<-readDone
}

func TestFragmentWriterWriteFirstNone(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	writer := NewFragmentWriter(StrategyNone)
	payload := []byte{0x16, 0x03, 0x01, 0x00, 0x03, 0xaa, 0xbb, 0xcc}

	readDone := make(chan []byte, 1)
	go func() {
		raw, _ := io.ReadAll(server)
		readDone <- raw
	}()

	if err := writer.WriteFirst(client, payload); err != nil {
		t.Fatalf("WriteFirst none: %v", err)
	}
	_ = client.Close()

	got := <-readDone
	if len(got) != len(payload) {
		t.Fatalf("none: received %d bytes, want %d", len(got), len(payload))
	}
	for i := range got {
		if got[i] != payload[i] {
			t.Fatalf("byte %d mismatch: %02x != %02x", i, got[i], payload[i])
		}
	}
}

func TestFragmentWriterWriteFirstTLSFrag2(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	writer := NewFragmentWriter(StrategyTLSRecordFrag2)
	payload := []byte{
		0x16, 0x03, 0x01, 0x00, 0x10,
		0x01, 0x00, 0x00, 0x0c, 0x03, 0x03,
		0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
		0x11, 0x22, 0x33, 0x44,
	}

	readDone := make(chan struct{})
	go func() {
		_, _ = io.ReadAll(server)
		close(readDone)
	}()

	if err := writer.WriteFirst(client, payload); err != nil {
		t.Fatalf("WriteFirst TLSFrag2: %v", err)
	}
	_ = client.Close()
	<-readDone
}

func TestFragmentWriterWriteFirstTCP1(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	writer := NewFragmentWriter(StrategyTCPSegmentation1)
	payload := []byte{0x16, 0x03, 0x01, 0x00, 0x03, 0xaa, 0xbb, 0xcc}

	readDone := make(chan struct{})
	go func() {
		_, _ = io.ReadAll(server)
		close(readDone)
	}()

	if err := writer.WriteFirst(client, payload); err != nil {
		t.Fatalf("WriteFirst TCP1: %v", err)
	}
	_ = client.Close()
	<-readDone
}

func TestFragmentWriterWriteFirstTCP8(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	writer := NewFragmentWriter(StrategyTCPSegmentation8)
	payload := make([]byte, 128)
	for i := range payload {
		payload[i] = byte(i)
	}

	readDone := make(chan struct{})
	go func() {
		_, _ = io.ReadAll(server)
		close(readDone)
	}()

	if err := writer.WriteFirst(client, payload); err != nil {
		t.Fatalf("WriteFirst TCP8: %v", err)
	}
	_ = client.Close()
	<-readDone
}
