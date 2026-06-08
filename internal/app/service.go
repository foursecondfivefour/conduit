package app

import (
	"context"
	"sync"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

// ControlService holds proxy controls for the system tray menu.
type ControlService struct {
	proxy    *proxy.Server
	settings *config.Settings
	mu       sync.RWMutex
}

func NewControlService(proxyServer *proxy.Server, settings *config.Settings) *ControlService {
	return &ControlService{
		proxy:    proxyServer,
		settings: settings,
	}
}

type Status struct {
	Running  bool   `json:"running"`
	Port     int    `json:"port"`
	ProxyURL string `json:"proxyURL"`
	Strategy string `json:"strategy"`
}

func (s *ControlService) GetStatus() Status {
	return Status{
		Running:  s.proxy.Running(),
		Port:     s.proxy.Port(),
		ProxyURL: s.proxy.ProxyURL(),
		Strategy: s.currentStrategy().String(),
	}
}

func (s *ControlService) RestartProxy() Status {
	ctx := context.Background()
	_, _ = s.proxy.Restart(ctx)
	return s.GetStatus()
}

func (s *ControlService) SetStrategy(strategy string) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := dpi.Strategy(strategy)
	if st.Valid() {
		s.settings.Strategy = st
	}
	return s.GetStatus()
}

func (s *ControlService) ListStrategies() []string {
	return []string{
		dpi.StrategyTCPSegmentation.String(),
		dpi.StrategyTLSRecordFrag.String(),
	}
}

func (s *ControlService) currentStrategy() dpi.Strategy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.Strategy
}

func (s *ControlService) Settings() config.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.settings
}
