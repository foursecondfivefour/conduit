package app

import (
	"context"
	"sync"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

// ControlService holds proxy controls for the system tray menu.
type ControlService struct {
	proxy    *proxy.Server
	resolver *dns.Resolver
	settings *config.Settings
	prefs    *preferenceStore
	mu       sync.RWMutex
}

func NewControlService(proxyServer *proxy.Server, resolver *dns.Resolver, settings *config.Settings, prefs *preferenceStore) *ControlService {
	return &ControlService{
		proxy:    proxyServer,
		resolver: resolver,
		settings: settings,
		prefs:    prefs,
	}
}

type Status struct {
	Running  bool   `json:"running"`
	Port     int    `json:"port"`
	ProxyURL string `json:"proxyURL"`
	Strategy string `json:"strategy"`
	DoH      string `json:"doh"`
}

func (s *ControlService) GetStatus() Status {
	st := s.currentSettings()
	return Status{
		Running:  s.proxy.Running(),
		Port:     s.proxy.Port(),
		ProxyURL: s.proxy.ProxyURL(),
		Strategy: st.Strategy.ShortLabel(),
		DoH:      st.DoHProvider.ShortLabel(),
	}
}

func (s *ControlService) RestartProxy() Status {
	ctx := context.Background()
	_, _ = s.proxy.Restart(ctx)
	return s.GetStatus()
}

func (s *ControlService) SetStrategy(strategy dpi.Strategy) Status {
	if !strategy.Valid() {
		return s.GetStatus()
	}
	s.mu.Lock()
	s.settings.Strategy = strategy
	s.mu.Unlock()
	if s.prefs != nil {
		s.prefs.Update(func(p *Preferences) {
			p.Strategy = strategy.String()
		})
	}
	return s.GetStatus()
}

func (s *ControlService) SetDoHProvider(provider dns.Provider) Status {
	if !provider.Valid() {
		return s.GetStatus()
	}
	if s.resolver != nil {
		s.resolver.SetProvider(provider)
	}
	s.mu.Lock()
	s.settings.DoHProvider = provider
	s.mu.Unlock()
	if s.prefs != nil {
		s.prefs.Update(func(p *Preferences) {
			p.DoHProvider = provider.String()
		})
	}
	return s.GetStatus()
}

func (s *ControlService) SetAllowlistPreset(preset proxy.AllowlistPreset) Status {
	if !preset.Valid() {
		return s.GetStatus()
	}
	s.mu.Lock()
	s.settings.AllowlistPreset = preset.String()
	s.mu.Unlock()
	if s.prefs != nil {
		s.prefs.Update(func(p *Preferences) {
			p.AllowlistPreset = preset.String()
		})
	}
	return s.GetStatus()
}

func (s *ControlService) ResetCustomDomains() Status {
	s.mu.Lock()
	s.settings.CustomDomains = nil
	s.mu.Unlock()
	if s.prefs != nil {
		s.prefs.Update(func(p *Preferences) {
			p.CustomDomains = nil
			p.AllowlistPreset = string(proxy.PresetGoogleMedia)
		})
	}
	return s.GetStatus()
}

func (s *ControlService) ListStrategies() []dpi.Strategy {
	return dpi.AllStrategies()
}

func (s *ControlService) ListDoHProviders() []dns.Provider {
	return []dns.Provider{
		dns.ProviderCloudflare,
		dns.ProviderGoogle,
		dns.ProviderQuad9,
	}
}

func (s *ControlService) ListAllowlistPresets() []proxy.AllowlistPreset {
	return []proxy.AllowlistPreset{
		proxy.PresetYouTube,
		proxy.PresetGoogleMedia,
		proxy.PresetCustom,
	}
}

func (s *ControlService) currentSettings() config.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := *s.settings
	if len(cp.CustomDomains) > 0 {
		cp.CustomDomains = append([]string(nil), cp.CustomDomains...)
	}
	return cp
}

func (s *ControlService) Settings() config.Settings {
	return s.currentSettings()
}

func (s *ControlService) SetLanguage(lang string) {
	s.mu.Lock()
	s.settings.Language = lang
	s.mu.Unlock()
	if s.prefs != nil {
		s.prefs.Update(func(p *Preferences) {
			p.Language = lang
		})
	}
}

func (s *ControlService) Language() string {
	return s.currentSettings().Language
}
