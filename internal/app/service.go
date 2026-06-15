package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
	"github.com/foursecondfivefour/conduit/internal/winproxy"
)

// ControlService holds proxy controls for the system tray menu.
type ControlService struct {
	proxy      *proxy.Server
	resolver   *dns.Resolver
	store      *settingsStore
	prefs      *preferenceStore
	killSwitch *proxy.KillSwitch
	dnsLeak    *dns.LeakDetector
	dnsBlocker *winproxy.DNSBlocker
}

func NewControlService(proxyServer *proxy.Server, resolver *dns.Resolver, store *settingsStore, prefs *preferenceStore) *ControlService {
	return &ControlService{
		proxy:    proxyServer,
		resolver: resolver,
		store:    store,
		prefs:    prefs,
	}
}

func (s *ControlService) SetKillSwitch(ks *proxy.KillSwitch) {
	s.killSwitch = ks
}

func (s *ControlService) SetDNSBlocker(db *winproxy.DNSBlocker) {
	s.dnsBlocker = db
}

func (s *ControlService) SetLeakDetector(ld *dns.LeakDetector) {
	s.dnsLeak = ld
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
	if _, err := s.proxy.Restart(ctx); err != nil {
		slog.Warn("proxy restart failed", "err", err)
	}
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

// KillSwitchEnabled returns whether the kill switch is currently active.
func (s *ControlService) KillSwitchEnabled() bool {
	if s.killSwitch == nil {
		return false
	}
	return s.killSwitch.IsEnabled()
}

// ToggleKillSwitch enables or disables the kill switch.
func (s *ControlService) ToggleKillSwitch(enabled bool) error {
	if s.killSwitch == nil {
		return nil
	}
	if enabled {
		return s.killSwitch.Enable()
	}
	return s.killSwitch.Disable()
}

// CheckDNSLeak performs a DNS leak test using the configured DoH resolver.
func (s *ControlService) CheckDNSLeak(ctx context.Context) (bool, error) {
	if s.dnsLeak == nil || s.resolver == nil {
		return false, nil
	}
	return s.dnsLeak.TestLeak(ctx, s.resolver)
}

// BlockSystemDNS blocks outbound UDP port 53 for all programs except Conduit.
func (s *ControlService) BlockSystemDNS() error {
	if s.dnsBlocker == nil {
		return nil
	}
	return s.dnsBlocker.Block()
}

// UnblockSystemDNS removes the UDP port 53 block rule.
func (s *ControlService) UnblockSystemDNS() error {
	if s.dnsBlocker == nil {
		return nil
	}
	return s.dnsBlocker.Unblock()
}

// IsSystemDNSBlocked returns whether DNS blocking is active.
func (s *ControlService) IsSystemDNSBlocked() bool {
	if s.dnsBlocker == nil {
		return false
	}
	return s.dnsBlocker.IsBlocked()
}

// RunSpeedTest runs a speed test through the proxy.
func (s *ControlService) RunSpeedTest() *proxy.SpeedTestResult {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return proxy.RunSpeedTest(ctx, s.proxy.ProxyURL())
}

// AutoSelectDNS automatically picks the fastest DoH provider.
func (s *ControlService) AutoSelectDNS() (dns.Provider, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return dns.AutoSelectProvider(ctx, s.resolver)
}
