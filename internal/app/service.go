package app

import (
	"context"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

// ControlService holds proxy controls for the system tray menu.
type ControlService struct {
	proxy    *proxy.Server
	resolver *dns.Resolver
	store    *settingsStore
	prefs    *preferenceStore
}

func NewControlService(proxyServer *proxy.Server, resolver *dns.Resolver, store *settingsStore, prefs *preferenceStore) *ControlService {
	return &ControlService{
		proxy:    proxyServer,
		resolver: resolver,
		store:    store,
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
	st := s.store.Snapshot()
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
	s.store.update(func(st *config.Settings) {
		st.Strategy = strategy
	})
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
	s.store.update(func(st *config.Settings) {
		st.DoHProvider = provider
	})
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
	s.store.update(func(st *config.Settings) {
		st.AllowlistPreset = preset.String()
	})
	if s.prefs != nil {
		s.prefs.Update(func(p *Preferences) {
			p.AllowlistPreset = preset.String()
		})
	}
	return s.GetStatus()
}

func (s *ControlService) ResetCustomDomains() Status {
	s.store.update(func(st *config.Settings) {
		st.CustomDomains = nil
	})
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

func (s *ControlService) Settings() config.Settings {
	return s.store.Snapshot()
}

func (s *ControlService) SetLanguage(lang string) {
	s.store.update(func(st *config.Settings) {
		st.Language = lang
	})
	if s.prefs != nil {
		s.prefs.Update(func(p *Preferences) {
			p.Language = lang
		})
	}
}

func (s *ControlService) Language() string {
	return s.store.Snapshot().Language
}
