package app

import (
	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/i18n"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

// ApplyPreferences copies persisted preferences into runtime settings.
func ApplyPreferences(p Preferences, settings *config.Settings) {
	if settings == nil {
		return
	}

	st := dpi.Strategy(p.Strategy)
	if st.Valid() {
		settings.Strategy = st
	}

	provider := dns.Provider(p.DoHProvider)
	if provider.Valid() {
		settings.DoHProvider = provider
	}

	preset := proxy.AllowlistPreset(p.AllowlistPreset)
	if preset.Valid() {
		settings.AllowlistPreset = preset.String()
	}

	if len(p.CustomDomains) > 0 {
		settings.CustomDomains = append([]string(nil), p.CustomDomains...)
	}

	settings.Language = i18n.ParseLang(p.Language).String()

	if p.WindowWidth > 0 {
		settings.WindowWidth = p.WindowWidth
	}
	if p.WindowHeight > 0 {
		settings.WindowHeight = p.WindowHeight
	}
}

// SettingsFromPreferences builds a config.Settings snapshot from preferences.
func SettingsFromPreferences(p Preferences) config.Settings {
	settings := config.DefaultSettings()
	ApplyPreferences(p, &settings)
	return settings
}

// SyncPreferencesFromSettings updates preference fields from runtime settings.
func SyncPreferencesFromSettings(p *Preferences, settings config.Settings) {
	if p == nil {
		return
	}
	p.Strategy = settings.Strategy.String()
	p.DoHProvider = settings.DoHProvider.String()
	p.AllowlistPreset = settings.AllowlistPreset
	p.CustomDomains = append([]string(nil), settings.CustomDomains...)
	p.Language = settings.Language
}
