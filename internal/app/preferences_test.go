package app

import (
	"encoding/json"
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
	want.Strategy = string(dpi.StrategyTCPSegmentation8)
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
	if err := writeTestFile(paths.PreferencesPath(), raw); err != nil {
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
	want.SOCKS5 = true
	want.SOCKS5Port = config.DefaultSOCKS5Port
	want.SplitTunnel = true
	want.UpstreamType = "http"
	want.UpstreamHost = "proxy.local"
	want.UpstreamPort = 8080
	want.AutoDNS = true
	want.AutoConnect = true
	want.TUNMode = true
	want.MASQUE = true
	want.AIDPI = true
	want.DomainStrategies = map[string]string{
		"www.youtube.com": string(dpi.StrategyTLSRecordFrag),
	}
	want.ProxyPorts = []config.ProxyPortConfig{
		{Port: 31286, TCPPort: true, Strategy: "tcp_segmentation_8"},
	}

	raw, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(paths.PreferencesPath(), raw); err != nil {
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
	if !got.SOCKS5 {
		t.Fatal("SOCKS5 should be true")
	}
	if got.SOCKS5Port != config.DefaultSOCKS5Port {
		t.Fatalf("SOCKS5Port = %d", got.SOCKS5Port)
	}
	if !got.SplitTunnel {
		t.Fatal("SplitTunnel should be true")
	}
	if got.UpstreamType != "http" || got.UpstreamHost != "proxy.local" || got.UpstreamPort != 8080 {
		t.Fatalf("upstream = %s/%s/%d", got.UpstreamType, got.UpstreamHost, got.UpstreamPort)
	}
	if !got.AutoDNS {
		t.Fatal("AutoDNS should be true")
	}
	if !got.AutoConnect {
		t.Fatal("AutoConnect should be true")
	}
	if !got.TUNMode {
		t.Fatal("TUNMode should be true")
	}
	if !got.MASQUE {
		t.Fatal("MASQUE should be true")
	}
	if !got.AIDPI {
		t.Fatal("AIDPI should be true")
	}
	if len(got.DomainStrategies) != 1 {
		t.Fatalf("DomainStrategies len = %d, want 1", len(got.DomainStrategies))
	}
	if len(got.ProxyPorts) != 1 {
		t.Fatalf("ProxyPorts len = %d, want 1", len(got.ProxyPorts))
	}
}

func TestPreferencesExportImport(t *testing.T) {
	dir := t.TempDir()
	paths := RuntimePaths{ConfigDir: dir, LogDir: filepath.Join(dir, "logs")}
	store, err := newPreferenceStore(paths)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	store.Update(func(p *Preferences) {
		p.KillSwitch = true
		p.Language = "en"
		p.Strategy = string(dpi.StrategyECH)
	})
	time.Sleep(300 * time.Millisecond)

	exportPath := filepath.Join(dir, "export.json")
	if err := store.Export(exportPath); err != nil {
		t.Fatalf("export: %v", err)
	}

	store2, err := newPreferenceStore(RuntimePaths{ConfigDir: filepath.Join(dir, "import"), LogDir: filepath.Join(dir, "logs2")})
	if err != nil {
		t.Fatalf("new store2: %v", err)
	}
	defer store2.Close()

	if err := store2.Import(exportPath); err != nil {
		t.Fatalf("import: %v", err)
	}

	got := store2.Get()
	if got.Language != "en" {
		t.Fatalf("imported language = %s", got.Language)
	}
	if got.Strategy != string(dpi.StrategyECH) {
		t.Fatalf("imported strategy = %s", got.Strategy)
	}
	if !got.KillSwitch {
		t.Fatal("imported killSwitch should be true")
	}
}

func TestPreferencesDefaultFields(t *testing.T) {
	p := DefaultPreferences(true)
	if p.Strategy != string(dpi.StrategyTCPSegmentation) {
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

func writeTestFile(path string, data []byte) error {
	return osWriteFile(path, data)
}

func osWriteFile(path string, data []byte) error {
	return writePreferencesFile(path, data)
}
