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

func TestPreferencesValidateLanguageAll33(t *testing.T) {
	validLangs := []string{
		"ru", "en", "de", "fr", "es", "tr", "fa",
		"tt", "ba", "ce", "cv", "sah", "bua", "os", "av",
		"mhr", "udm", "kv", "myv", "kbd", "tyv", "xal",
		"uk", "be", "kk", "uz", "hy", "az", "ka", "ro",
		"ky", "tg", "tk",
	}

	if len(validLangs) != 33 {
		t.Fatalf("expected 33 languages, got %d", len(validLangs))
	}

	for _, lang := range validLangs {
		p := &Preferences{Language: lang}
		SanitizePreferences(p)
		if p.Language != lang {
			t.Errorf("valid language %q was sanitized to %q", lang, p.Language)
		}
	}
}

func TestPreferencesValidateLanguageInvalidFallsBack(t *testing.T) {
	invalidLangs := []string{"", "jp", "it", "pt", "ch", "unknown", "123"}
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
	if p.Strategy != "tcp_segmentation" {
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

func TestPreferencesValidateDomainStrategies(t *testing.T) {
	p := &Preferences{
		DomainStrategies: map[string]string{
			"youtube.com":        "tcp_segmentation_8",
			"googlevideo.com":    "tls_record_frag",
			"invalid.com":        "bad_strategy",
			"also_invalid.com":   "",
		},
	}
	SanitizePreferences(p)
	if len(p.DomainStrategies) != 2 {
		t.Fatalf("DomainStrategies len = %d, want 2", len(p.DomainStrategies))
	}
	if p.DomainStrategies["youtube.com"] != "tcp_segmentation_8" {
		t.Fatal("valid strategy was dropped")
	}
	if _, exists := p.DomainStrategies["invalid.com"]; exists {
		t.Fatal("invalid strategy was not dropped")
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
