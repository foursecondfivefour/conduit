package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

const saveDebounce = 200 * time.Millisecond

// Preferences stores per-user settings on disk.
type Preferences struct {
	OnboardingCompleted bool     `json:"onboardingCompleted"`
	Strategy            string   `json:"strategy"`
	DoHProvider         string   `json:"dohProvider"`
	AllowlistPreset     string   `json:"allowlistPreset"`
	CustomDomains       []string `json:"customDomains"`
	Language            string   `json:"language"`
	WindowWidth         int      `json:"windowWidth"`
	WindowHeight        int      `json:"windowHeight"`
	Autostart           bool     `json:"autostart"`
	SystemProxy         bool     `json:"systemProxy"`
	MinimizeToTray      bool     `json:"minimizeToTray"`
	KillSwitch          bool     `json:"killSwitch"`
	Portable            bool     `json:"portable"`
	AutoUpdate          bool     `json:"autoUpdate"`
	UpdateCheckInterval string   `json:"updateCheckInterval"`
	LastUpdateCheck     time.Time `json:"lastUpdateCheck"`
	SkippedVersion      string   `json:"skippedVersion"`
	LastHealthOK        bool     `json:"lastHealthOK"`
	LastHealthCheck     time.Time `json:"lastHealthCheck"`
	SponsorBlock        bool     `json:"sponsorBlock"`
	RYD                 bool     `json:"ryd"`
	DNSAdBlock          bool     `json:"dnsAdBlock"`
	DNSAdBlockURL       string   `json:"dnsAdBlockURL"`
	BackgroundAudio     bool     `json:"backgroundAudio"`
	SOCKS5             bool                        `json:"socks5"`
	SOCKS5Port         int                         `json:"socks5Port"`
	DomainStrategies   map[string]string           `json:"domainStrategies"`
	ProxyPorts         []config.ProxyPortConfig    `json:"proxyPorts"`
	SplitTunnel        bool                        `json:"splitTunnel"`
	UpstreamType       string                      `json:"upstreamType"`
	UpstreamHost       string                      `json:"upstreamHost"`
	UpstreamPort       int                         `json:"upstreamPort"`
	UpstreamUser       string                      `json:"upstreamUser"`
	UpstreamPassword   string                      `json:"upstreamPassword"`
	AutoDNS            bool                        `json:"autoDNS"`
	AutoDNSLastCheck   time.Time                   `json:"autoDNSLastCheck"`
	AutoConnect        bool                        `json:"autoConnect"`
	TUNMode            bool                        `json:"tunMode"`
	MASQUE             bool                        `json:"masque"`
	AIDPI              bool                        `json:"aiDPI"`
}

func DefaultPreferences(portable bool) Preferences {
	return Preferences{
		Strategy:            string(dpi.StrategyTCPSegmentation),
		DoHProvider:         string(dns.ProviderCloudflare),
		AllowlistPreset:     proxy.PresetYouTube.String(),
		Language:            "ru",
		WindowWidth:         config.WindowWidth,
		WindowHeight:        config.WindowHeight,
		KillSwitch:          config.DefaultKillSwitchEnabled,
		AutoUpdate:          true,
		UpdateCheckInterval: config.DefaultUpdateCheckInterval.String(),
		Portable:            portable,
	}
}

type preferenceStore struct {
	path      string
	configDir string
	mu        sync.Mutex
	data      Preferences
	saveCh    chan struct{}
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewPreferenceStore loads or creates user preferences on disk.
func NewPreferenceStore(paths RuntimePaths) (*preferenceStore, error) {
	return newPreferenceStore(paths)
}

func newPreferenceStore(paths RuntimePaths) (*preferenceStore, error) {
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		return nil, err
	}

	store := &preferenceStore{
		path:      paths.PreferencesPath(),
		configDir: paths.ConfigDir,
		data:      DefaultPreferences(paths.Portable),
		saveCh:    make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	if err := store.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	store.data.Portable = paths.Portable
	go store.saveLoop()
	return store, nil
}

func (s *preferenceStore) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return err
	}
	SanitizePreferences(&s.data)
	return nil
}

func (s *preferenceStore) saveLoop() {
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-s.stopCh:
			if timer != nil {
				timer.Stop()
			}
			if err := s.saveNow(); err != nil {
				slog.Warn("preferences: save on stop failed", "err", err)
			}
			close(s.doneCh)
			return
		case <-s.saveCh:
			if timer == nil {
				timer = time.NewTimer(saveDebounce)
				timerC = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(saveDebounce)
			}
		case <-timerC:
			timerC = nil
			if err := s.saveNow(); err != nil {
				slog.Warn("preferences: save failed", "err", err)
			}
		}
	}
}

func (s *preferenceStore) scheduleSave() {
	select {
	case s.saveCh <- struct{}{}:
	default:
	}
}

func (s *preferenceStore) saveNow() error {
	s.mu.Lock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

// Export writes preferences as indented JSON to the given path.
func (s *preferenceStore) Export(path string) error {
	s.mu.Lock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// Import reads and applies preferences from a JSON file.
func (s *preferenceStore) Import(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var imported Preferences
	if err := json.Unmarshal(raw, &imported); err != nil {
		return err
	}
	SanitizePreferences(&imported)
	s.Update(func(p *Preferences) {
		*p = imported
	})
	return s.saveNow()
}

func (s *preferenceStore) Close() error {
	select {
	case <-s.doneCh:
		return nil
	default:
	}
	close(s.stopCh)
	<-s.doneCh
	return nil
}

func (s *preferenceStore) Get() Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := s.data
	if len(cp.CustomDomains) > 0 {
		cp.CustomDomains = append([]string(nil), cp.CustomDomains...)
	}
	return cp
}

func (s *preferenceStore) Update(fn func(*Preferences)) {
	s.mu.Lock()
	fn(&s.data)
	s.mu.Unlock()
	s.scheduleSave()
}

func (s *preferenceStore) ConfigDir() string {
	return s.configDir
}

func (s *preferenceStore) PreferencesPath() string {
	return s.path
}

func (s *preferenceStore) NeedsOnboarding() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.data.OnboardingCompleted
}

func (s *preferenceStore) CompleteOnboarding() error {
	s.Update(func(p *Preferences) {
		p.OnboardingCompleted = true
	})
	return s.saveNow()
}

func (s *preferenceStore) UpdateCheckInterval() time.Duration {
	p := s.Get()
	if p.UpdateCheckInterval == "" {
		return config.DefaultUpdateCheckInterval
	}
	d, err := time.ParseDuration(p.UpdateCheckInterval)
	if err != nil || d < time.Hour {
		return config.DefaultUpdateCheckInterval
	}
	return d
}
