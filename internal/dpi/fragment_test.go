package dpi

import "testing"

func TestClientHelloSize(t *testing.T) {
	// TLS record: handshake (0x16), version 0x0301, length 10
	data := []byte{0x16, 0x03, 0x01, 0x00, 0x0a, 0x01, 0x00, 0x00, 0x06, 0x03, 0x03, 0xaa, 0xbb, 0xcc, 0xdd}
	if !IsTLSClientHello(data) {
		t.Fatal("expected TLS ClientHello")
	}
	if got := ClientHelloSize(data); got != 15 {
		t.Fatalf("ClientHelloSize = %d, want 15", got)
	}
}

func TestStrategyValid(t *testing.T) {
	if !StrategyTCPSegmentation.Valid() || !StrategyTLSRecordFrag.Valid() {
		t.Fatal("expected valid strategies")
	}
	if Strategy("unknown").Valid() {
		t.Fatal("unexpected valid strategy")
	}
}
