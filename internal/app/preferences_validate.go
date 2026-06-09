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
	if lang != "en" && lang != "ru" {
		p.Language = "ru"
	}

	if p.WindowWidth < 320 {
		p.WindowWidth = config.WindowWidth
	}
	if p.WindowHeight < 240 {
		p.WindowHeight = config.WindowHeight
	}

	if p.UpdateCheckInterval != "" {
		if d, err := time.ParseDuration(p.UpdateCheckInterval); err != nil || d < time.Hour {
			p.UpdateCheckInterval = config.DefaultUpdateCheckInterval.String()
		}
	} else {
		p.UpdateCheckInterval = config.DefaultUpdateCheckInterval.String()
	}
}
