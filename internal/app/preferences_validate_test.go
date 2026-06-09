package app

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/foursecondfivefour/conduit/internal/proxy"
)

func TestPreferencesValidateCustomDomains(t *testing.T) {
	dir := t.TempDir()
	paths := RuntimePaths{ConfigDir: dir, LogDir: filepath.Join(dir, "logs")}
	raw, _ := json.Marshal(Preferences{
		OnboardingCompleted: true,
		AllowlistPreset:     proxy.PresetCustom.String(),
		CustomDomains:       []string{"com", "cdn.example.org"},
	})
	if err := writeTestFile(paths.PreferencesPath(), raw); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPreferences(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CustomDomains) != 1 || got.CustomDomains[0] != "cdn.example.org" {
		t.Fatalf("custom domains = %v", got.CustomDomains)
	}
}
