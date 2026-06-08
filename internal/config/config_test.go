package config

import (
	"testing"

	"github.com/foursecondfivefour/conduit/internal/dpi"
)

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if s.Strategy != dpi.StrategyTCPSegmentation {
		t.Fatalf("strategy = %s, want %s", s.Strategy, dpi.StrategyTCPSegmentation)
	}
	if ListenHost != "127.0.0.1" {
		t.Fatalf("ListenHost = %s", ListenHost)
	}
	if DefaultProxyPort == 0 {
		t.Fatal("DefaultProxyPort must be set")
	}
}
