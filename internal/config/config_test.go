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

func TestConstants(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{"DefaultProxyPort", DefaultProxyPort, 31284},
		{"DefaultSOCKS5Port", DefaultSOCKS5Port, 31285},
		{"SOCKS5PortOffset", SOCKS5PortOffset, 1},
		{"SplitTunnelPortOffset", SplitTunnelPortOffset, 2},
		{"DefaultMASQUEPort", DefaultMASQUEPort, 4430},
		{"PiPWidth", PiPWidth, 320},
		{"PiPHeight", PiPHeight, 180},
		{"WindowWidth", WindowWidth, 960},
		{"WindowHeight", WindowHeight, 540},
	}
	for _, tt := range tests {
		if tt.value != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.value, tt.want)
		}
	}
}

func TestDefaultAdBlockURL(t *testing.T) {
	if DefaultAdBlockURL == "" {
		t.Fatal("DefaultAdBlockURL must be set")
	}
	if !isHTTPURL(DefaultAdBlockURL) {
		t.Fatalf("DefaultAdBlockURL is not a valid URL: %s", DefaultAdBlockURL)
	}
}

func TestDefaultKillSwitchEnabled(t *testing.T) {
	if DefaultKillSwitchEnabled {
		t.Fatal("DefaultKillSwitchEnabled should be false by default")
	}
}

func TestProxyPortConfig(t *testing.T) {
	cfg := ProxyPortConfig{
		Port:      31286,
		TCPPort:   true,
		Strategy:  "tcp_segmentation_8",
		Allowlist: "youtube",
	}
	if cfg.Port != 31286 {
		t.Errorf("port = %d", cfg.Port)
	}
	if !cfg.TCPPort {
		t.Error("TCPPort should be true")
	}
	if cfg.Strategy != "tcp_segmentation_8" {
		t.Errorf("strategy = %s", cfg.Strategy)
	}
	if cfg.Allowlist != "youtube" {
		t.Errorf("allowlist = %s", cfg.Allowlist)
	}
}

func TestDefaultSettingsFields(t *testing.T) {
	s := DefaultSettings()
	if s.DoHProvider.String() != "cloudflare" {
		t.Fatalf("DoHProvider = %s", s.DoHProvider)
	}
	if s.AllowlistPreset != "youtube" {
		t.Fatalf("AllowlistPreset = %s", s.AllowlistPreset)
	}
	if s.Language != "ru" {
		t.Fatalf("Language = %s", s.Language)
	}
	if s.WindowWidth != WindowWidth {
		t.Fatalf("WindowWidth = %d", s.WindowWidth)
	}
	if s.WindowHeight != WindowHeight {
		t.Fatalf("WindowHeight = %d", s.WindowHeight)
	}
}

func isHTTPURL(s string) bool {
	return len(s) > 8 && (s[:8] == "https://" || s[:7] == "http://")
}
