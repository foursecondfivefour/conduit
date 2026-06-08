package app

import (
	"path/filepath"
	"testing"
)

func TestPreferenceStoreCompleteOnboarding(t *testing.T) {
	dir := t.TempDir()
	store := &preferenceStore{path: filepath.Join(dir, "preferences.json")}

	if !store.NeedsOnboarding() {
		t.Fatal("expected onboarding on fresh store")
	}

	if err := store.CompleteOnboarding(); err != nil {
		t.Fatalf("complete onboarding: %v", err)
	}
	if store.NeedsOnboarding() {
		t.Fatal("expected onboarding completed")
	}

	reloaded := &preferenceStore{path: store.path}
	if err := reloaded.load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.NeedsOnboarding() {
		t.Fatal("expected persisted onboarding flag")
	}
}
