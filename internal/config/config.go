package config

import (
	"time"

	"github.com/foursecondfivefour/conduit/internal/dpi"
)

const (
	ListenHost        = "127.0.0.1"
	DefaultProxyPort  = 31284
	DialTimeout       = 10 * time.Second
	IdleTimeout       = 120 * time.Second
	ReadHeaderTimeout = 5 * time.Second

	// Mobile YouTube loads less JS/UI than the desktop site.
	YouTubeURL = "https://m.youtube.com"

	WindowWidth  = 960
	WindowHeight = 540
)

// Settings holds runtime configuration shared across components.
type Settings struct {
	Strategy dpi.Strategy
}

func DefaultSettings() Settings {
	return Settings{
		Strategy: dpi.StrategyTCPSegmentation,
	}
}
