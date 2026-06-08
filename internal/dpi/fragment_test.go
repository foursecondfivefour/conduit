package dpi

import "testing"

func TestClientHelloSize(t *testing.T) {
	data := []byte{0x16, 0x03, 0x01, 0x00, 0x0a, 0x01, 0x00, 0x00, 0x06, 0x03, 0x03, 0xaa, 0xbb, 0xcc, 0xdd}
	if !IsTLSClientHello(data) {
		t.Fatal("expected TLS ClientHello")
	}
	if got := ClientHelloSize(data); got != 15 {
		t.Fatalf("ClientHelloSize = %d, want 15", got)
	}
}

func TestStrategyValid(t *testing.T) {
	for _, st := range AllStrategies() {
		if !st.Valid() {
			t.Fatalf("expected valid strategy %s", st)
		}
	}
	if Strategy("unknown").Valid() {
		t.Fatal("unexpected valid strategy")
	}
}

func TestNewFragmentWriterDefaults(t *testing.T) {
	w := NewFragmentWriter(Strategy("bad"))
	if w.strategy != StrategyTCPSegmentation {
		t.Fatalf("got %s", w.strategy)
	}
}
