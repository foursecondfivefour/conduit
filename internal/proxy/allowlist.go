package proxy

import "github.com/foursecondfivefour/conduit/internal/config"

// AllowedHostForSettings checks the host against runtime allowlist settings.
func AllowedHostForSettings(host string, settings config.Settings) bool {
	preset := AllowlistPreset(settings.AllowlistPreset)
	if !preset.Valid() {
		preset = PresetGoogleMedia
	}
	return AllowedHost(host, preset, settings.CustomDomains)
}