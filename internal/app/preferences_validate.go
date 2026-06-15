package app

import (
	"strings"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

const maxCustomDomainsPref = 64

// SanitizePreferences normalizes persisted preferences and drops unsafe values.
func SanitizePreferences(p *Preferences) {
	if p == nil {
		return
	}

	st := dpi.Strategy(p.Strategy)
	if !st.Valid() {
		p.Strategy = string(dpi.StrategyTCPSegmentation)
	}

	provider := dns.Provider(p.DoHProvider)
	if !provider.Valid() {
		p.DoHProvider = string(dns.ProviderCloudflare)
	}

	preset := proxy.AllowlistPreset(p.AllowlistPreset)
	if !preset.Valid() {
		p.AllowlistPreset = proxy.PresetYouTube.String()
	}

	p.CustomDomains = proxy.FilterCustomDomains(p.CustomDomains)
	if len(p.CustomDomains) > maxCustomDomainsPref {
		p.CustomDomains = p.CustomDomains[:maxCustomDomainsPref]
	}

	lang := strings.ToLower(strings.TrimSpace(p.Language))
	switch lang {
	case "en", "ru", "de", "fr", "es", "tr", "fa", "tt", "ba", "ce", "cv", "sah", "bua", "os", "av", "mhr", "udm", "kv", "myv", "kbd", "tyv", "xal", "uk", "be", "kk", "uz", "hy", "az", "ka", "ro", "ky", "tg", "tk":
	default:
		p.Language = "ru"
	}

	if p.WindowWidth < 320 {
		p.WindowWidth = config.WindowWidth
	}
	if p.WindowHeight < 240 {
		p.WindowHeight = config.WindowHeight
	}

	// Validate per-domain strategies: drop invalid strategy strings.
	if p.DomainStrategies != nil {
		filtered := make(map[string]string, len(p.DomainStrategies))
		for domain, strategy := range p.DomainStrategies {
			if dpi.Strategy(strategy).Valid() {
				filtered[domain] = strategy
			}
		}
		p.DomainStrategies = filtered
	}

	if p.UpdateCheckInterval != "" {
		if d, err := time.ParseDuration(p.UpdateCheckInterval); err != nil || d < time.Hour {
			p.UpdateCheckInterval = config.DefaultUpdateCheckInterval.String()
		}
	} else {
		p.UpdateCheckInterval = config.DefaultUpdateCheckInterval.String()
	}

	// Validate port values: must be between 1 and 65535.
	if p.SOCKS5Port <= 0 || p.SOCKS5Port > 65535 {
		p.SOCKS5Port = config.DefaultSOCKS5Port
	}
	if p.UpstreamPort <= 0 || p.UpstreamPort > 65535 {
		p.UpstreamPort = 0
	}
}
