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

	YouTubeURL = "https://m.youtube.com"

	WindowWidth  = 960
	WindowHeight = 540

	SplashDuration = 2 * time.Second

	HotkeyModifiers = 0x0002 | 0x0004 // Ctrl+Shift
	HotkeyVK        = 0x59             // Y

	DefaultUpdateCheckInterval = 24 * time.Hour
	UpdateCheckDelay           = 15 * time.Second
)

// Settings holds runtime configuration shared across components.
type Settings struct {
	Strategy        dpi.Strategy
	DoHProvider     dns.Provider
	AllowlistPreset string
	CustomDomains   []string
	Language        string
	WindowWidth     int
	WindowHeight    int
}

func DefaultSettings() Settings {
	return Settings{
		Strategy:        dpi.StrategyTCPSegmentation,
		DoHProvider:     dns.ProviderCloudflare,
		AllowlistPreset: "google_media",
		CustomDomains:   nil,
		Language:        "ru",
		WindowWidth:     WindowWidth,
		WindowHeight:    WindowHeight,
	}
}
