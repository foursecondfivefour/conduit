package app

import (
	"encoding/json"
	"os"
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
	if err := os.WriteFile(paths.PreferencesPath(), raw, 0o600); err != nil {
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

func TestPreferencesValidateLanguageSupported(t *testing.T) {
	validLangs := []string{"ru", "en"}

	for _, lang := range validLangs {
		p := &Preferences{Language: lang}
		SanitizePreferences(p)
		if p.Language != lang {
			t.Errorf("valid language %q was sanitized to %q", lang, p.Language)
		}
	}
}

func TestPreferencesValidateLanguageInvalidFallsBack(t *testing.T) {
	invalidLangs := []string{"", "jp", "it", "pt", "ch", "unknown", "123", "de", "fr"}
	for _, lang := range invalidLangs {
		p := &Preferences{Language: lang}
		SanitizePreferences(p)
		if p.Language != "ru" {
			t.Errorf("invalid language %q should fall back to 'ru', got %q", lang, p.Language)
		}
	}
}

func TestPreferencesValidateInvalidStrategyFallsBack(t *testing.T) {
	p := &Preferences{Strategy: "invalid_strategy"}
	SanitizePreferences(p)
	if p.Strategy != "tls_record_frag" {
		t.Fatalf("invalid strategy = %s", p.Strategy)
	}
}

func TestPreferencesValidateInvalidDoHProviderFallsBack(t *testing.T) {
	p := &Preferences{DoHProvider: "invalid_provider"}
	SanitizePreferences(p)
	if p.DoHProvider != "cloudflare" {
		t.Fatalf("invalid DoH Provider = %s", p.DoHProvider)
	}
}

func TestPreferencesValidateInvalidAllowlistPresetFallsBack(t *testing.T) {
	p := &Preferences{AllowlistPreset: "invalid_preset"}
	SanitizePreferences(p)
	if p.AllowlistPreset != "youtube" {
		t.Fatalf("invalid AllowlistPreset = %s", p.AllowlistPreset)
	}
}

func TestPreferencesValidateWindowSizeMinimums(t *testing.T) {
	p := &Preferences{WindowWidth: 100, WindowHeight: 100}
	SanitizePreferences(p)
	if p.WindowWidth < 320 {
		t.Fatalf("WindowWidth = %d, want >= 320", p.WindowWidth)
	}
	if p.WindowHeight < 240 {
		t.Fatalf("WindowHeight = %d, want >= 240", p.WindowHeight)
	}
}

func TestPreferencesValidateUpdateCheckInterval(t *testing.T) {
	p := &Preferences{UpdateCheckInterval: "30m"}
	SanitizePreferences(p)
	if p.UpdateCheckInterval == "30m" {
		t.Fatal("too-short interval should be reset to default")
	}

	p = &Preferences{UpdateCheckInterval: ""}
	SanitizePreferences(p)
	if p.UpdateCheckInterval == "" {
		t.Fatal("empty interval should be set to default")
	}

	p = &Preferences{UpdateCheckInterval: "24h0m0s"}
	SanitizePreferences(p)
	if p.UpdateCheckInterval == "" {
		t.Fatal("valid interval was dropped")
	}
}

func TestPreferencesValidateNilPreferences(t *testing.T) {
	SanitizePreferences(nil)
}

func TestPreferencesValidateCustomDomainsCap(t *testing.T) {
	domains := make([]string, 100)
	for i := range domains {
		domains[i] = "host" + string(rune('a'+i%26)) + ".example.com"
	}
	p := &Preferences{CustomDomains: domains}
	SanitizePreferences(p)
	if len(p.CustomDomains) > maxCustomDomainsPref {
		t.Fatalf("CustomDomains len = %d, max %d", len(p.CustomDomains), maxCustomDomainsPref)
	}
}
