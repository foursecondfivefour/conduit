package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Preferences stores per-user UI state on disk.
type Preferences struct {
	OnboardingCompleted bool `json:"onboardingCompleted"`
}

type preferenceStore struct {
	path string
	mu   sync.Mutex
	data Preferences
}

func newPreferenceStore() (*preferenceStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(dir, "Conduit")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, err
	}

	store := &preferenceStore{path: filepath.Join(appDir, "preferences.json")}
	if err := store.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return store, nil
}

func (s *preferenceStore) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(raw, &s.data)
}

func (s *preferenceStore) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func (s *preferenceStore) NeedsOnboarding() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.data.OnboardingCompleted
}

func (s *preferenceStore) CompleteOnboarding() error {
	s.mu.Lock()
	s.data.OnboardingCompleted = true
	s.mu.Unlock()
	return s.save()
}
