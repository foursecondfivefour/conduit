package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

func TestPreferenceStoreCompleteOnboarding(t *testing.T) {
	dir := t.TempDir()
	paths := RuntimePaths{ConfigDir: dir, LogDir: filepath.Join(dir, "logs")}
	store, err := newPreferenceStore(paths)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	if !store.NeedsOnboarding() {
		t.Fatal("expected onboarding on fresh store")
	}

	if err := store.CompleteOnboarding(); err != nil {
		t.Fatalf("complete onboarding: %v", err)
	}
	if store.NeedsOnboarding() {
		t.Fatal("expected onboarding completed")
	}
	time.Sleep(300 * time.Millisecond)

	reloaded, err := newPreferenceStore(paths)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	defer reloaded.Close()
	if reloaded.NeedsOnboarding() {
		t.Fatal("expected persisted onboarding flag")
	}
}

func TestPreferencesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	paths := RuntimePaths{ConfigDir: dir, LogDir: filepath.Join(dir, "logs"), Portable: true}

	want := DefaultPreferences(true)
	want.OnboardingCompleted = true
	want.Strategy = string(dpi.StrategyTLSRecordFrag2)
	want.DoHProvider = string(dns.ProviderQuad9)
	want.AllowlistPreset = string(proxy.PresetYouTube)
	want.CustomDomains = []string{"example.com"}
	want.Language = "en"
	want.WindowWidth = 800
	want.WindowHeight = 600
	want.Autostart = true
	want.SystemProxy = true
	want.MinimizeToTray = true
	want.AutoUpdate = false
	want.UpdateCheckInterval = (12 * time.Hour).String()
	want.SkippedVersion = "v1.0.0"

	raw, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.PreferencesPath(), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadPreferences(paths)
	if err != nil {
		t.Fatal(err)
	}
	if got.Strategy != want.Strategy || got.DoHProvider != want.DoHProvider {
		t.Fatalf("strategy/doh mismatch: %+v", got)
	}
	if got.AllowlistPreset != want.AllowlistPreset || got.Language != want.Language {
		t.Fatalf("allowlist/lang mismatch: %+v", got)
	}
	if !got.Autostart || !got.SystemProxy || !got.MinimizeToTray {
		t.Fatalf("flags not loaded: %+v", got)
	}
}

func TestPortablePathResolution(t *testing.T) {
	paths, err := ResolveRuntimePaths(true)
	if err != nil {
		t.Fatal(err)
	}
	if !paths.Portable {
		t.Fatal("expected portable")
	}
	if filepath.Base(paths.ConfigDir) != "Conduit" {
		t.Fatalf("config dir = %s", paths.ConfigDir)
	}
}

func TestPreferencesNewFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	paths := RuntimePaths{ConfigDir: dir, LogDir: filepath.Join(dir, "logs"), Portable: true}

	want := DefaultPreferences(true)
	want.OnboardingCompleted = true
	want.KillSwitch = true
	want.SponsorBlock = true
	want.RYD = true
	want.DNSAdBlock = true
	want.DNSAdBlockURL = config.DefaultAdBlockURL
	want.BackgroundAudio = true
	want.AutoConnect = true

	raw, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.PreferencesPath(), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadPreferences(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !got.KillSwitch {
		t.Fatal("KillSwitch should be true")
	}
	if !got.SponsorBlock {
		t.Fatal("SponsorBlock should be true")
	}
	if !got.RYD {
		t.Fatal("RYD should be true")
	}
	if !got.DNSAdBlock {
		t.Fatal("DNSAdBlock should be true")
	}
	if got.DNSAdBlockURL != config.DefaultAdBlockURL {
		t.Fatalf("DNSAdBlockURL = %s", got.DNSAdBlockURL)
	}
	if !got.BackgroundAudio {
		t.Fatal("BackgroundAudio should be true")
	}
	if !got.AutoConnect {
		t.Fatal("AutoConnect should be true")
	}
}

func TestPreferencesDefaultFields(t *testing.T) {
	p := DefaultPreferences(true)
	if p.Strategy != string(dpi.StrategyTLSRecordFrag) {
		t.Errorf("Strategy = %s", p.Strategy)
	}
	if p.DoHProvider != string(dns.ProviderCloudflare) {
		t.Errorf("DoHProvider = %s", p.DoHProvider)
	}
	if p.AllowlistPreset != proxy.PresetYouTube.String() {
		t.Errorf("AllowlistPreset = %s", p.AllowlistPreset)
	}
	if p.Language != "ru" {
		t.Errorf("Language = %s", p.Language)
	}
	if !p.AutoUpdate {
		t.Fatal("AutoUpdate should default to true")
	}
	if p.KillSwitch {
		t.Fatal("KillSwitch should default to false")
	}
}
