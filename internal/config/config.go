package config

import (
	"time"

	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
)

const (
	ListenHost        = "127.0.0.1"
	DefaultProxyPort  = 31284
	DialTimeout       = 10 * time.Second
	IdleTimeout       = 120 * time.Second
	ReadHeaderTimeout = 5 * time.Second

	YouTubeURL = "https://www.youtube.com"

	WindowWidth  = 960
	WindowHeight = 540

	SplashDuration = 1 * time.Second

	HotkeyModifiers = 0x0002 | 0x0004 // Ctrl+Shift
	HotkeyVK        = 0x59             // Y

	DefaultUpdateCheckInterval = 24 * time.Hour
	UpdateCheckDelay           = 15 * time.Second

	DefaultKillSwitchEnabled = false

	DefaultSOCKS5Port    = 31285
	SOCKS5PortOffset      = 1
	SplitTunnelPortOffset = 2

	PiPWidth  = 320
	PiPHeight = 180

	DefaultAdBlockURL = "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"

	DefaultMASQUEPort = 4430
)

// ProxyPortConfig defines an additional proxy port with its own settings.
type ProxyPortConfig struct {
	Port      int    `json:"port"`
	TCPPort   bool   `json:"tcpPort"`
	Strategy  string `json:"strategy"`
	Allowlist string `json:"allowlist"`
}

// Settings holds runtime configuration shared across components.
type Settings struct {
	Strategy         dpi.Strategy
	DoHProvider      dns.Provider
	AllowlistPreset  string
	CustomDomains    []string
	DomainStrategies map[string]string
	Language         string
	WindowWidth      int
	WindowHeight     int
}

func DefaultSettings() Settings {
	return Settings{
		Strategy:        dpi.StrategyTCPSegmentation,
		DoHProvider:     dns.ProviderCloudflare,
		AllowlistPreset: "youtube",
		CustomDomains:   nil,
		Language:        "ru",
		WindowWidth:     WindowWidth,
		WindowHeight:    WindowHeight,
	}
}
