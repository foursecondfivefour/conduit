package app

import (
	"sync"

	"github.com/foursecondfivefour/conduit/internal/config"
)

// settingsStore is the single thread-safe source of runtime proxy settings.
type settingsStore struct {
	mu sync.RWMutex
	v  config.Settings
}

// NewSettingsStore builds a thread-safe runtime settings holder from preferences.
func NewSettingsStore(p Preferences) *settingsStore {
	return &settingsStore{v: SettingsFromPreferences(p)}
}

func copySettings(s config.Settings) config.Settings {
	cp := s
	if len(s.CustomDomains) > 0 {
		cp.CustomDomains = append([]string(nil), s.CustomDomains...)
	}
	return cp
}

func (s *settingsStore) Snapshot() config.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySettings(s.v)
}

func (s *settingsStore) update(fn func(*config.Settings)) {
	s.mu.Lock()
	fn(&s.v)
	s.mu.Unlock()
}
