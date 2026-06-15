package proxy

import (
	"testing"

	"github.com/foursecondfivefour/conduit/internal/config"
)

func TestAllowedHostGoogleMedia(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = PresetGoogleMedia.String()
	tests := []struct {
		host string
		want bool
	}{
		{"www.youtube.com", true},
		{"r1---sn-abc.googlevideo.com", true},
		{"i.ytimg.com", true},
		{"youtube.googleapis.com", true},
		{"www.google.com", true},
		{"youtu.be", true},
		{"music.youtube.com", true},
		{"example.com", false},
		{"evil-google.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := AllowedHostForSettings(tt.host, settings); got != tt.want {
			t.Errorf("AllowedHostForSettings(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestAllowedHostYouTubeOnly(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = string(PresetYouTube)
	if !AllowedHostForSettings("www.google.com", settings) {
		t.Fatal("google.com should be allowed in youtube preset for player auth")
	}
	if AllowedHostForSettings("googleads.g.doubleclick.net", settings) {
		t.Fatal("doubleclick should be blocked in youtube preset")
	}
	if !AllowedHostForSettings("www.youtube.com", settings) {
		t.Fatal("youtube.com should be allowed")
	}
	if !AllowedHostForSettings("youtubei.googleapis.com", settings) {
		t.Fatal("youtubei.googleapis.com should be allowed for playback")
	}
	if !AllowedHostForSettings("fonts.gstatic.com", settings) {
		t.Fatal("gstatic.com should be allowed for assets")
	}
	if !AllowedHostForSettings("lh3.googleusercontent.com", settings) {
		t.Fatal("googleusercontent.com should be allowed for playback assets")
	}
}

func TestAllowedHostCustom(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = string(PresetCustom)
	settings.CustomDomains = []string{"cdn.example.org"}
	if !AllowedHostForSettings("media.cdn.example.org", settings) {
		t.Fatal("expected custom domain match")
	}
	if AllowedHostForSettings("www.google.com", settings) {
		t.Fatal("google.com should be blocked with custom list")
	}
}

func TestAllowedHostYouTubemusic(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = string(PresetYouTube)

	if !AllowedHostForSettings("music.youtube.com", settings) {
		t.Fatal("music.youtube.com should be allowed")
	}
	if !AllowedHostForSettings("youtube-nocookie.com", settings) {
		t.Fatal("youtube-nocookie.com should be allowed")
	}
}

func TestAllowedHostGoogleAPIs(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = string(PresetYouTube)

	if !AllowedHostForSettings("accounts.google.com", settings) {
		t.Fatal("accounts.google.com should be allowed")
	}
	if !AllowedHostForSettings("youtubei.googleapis.com", settings) {
		t.Fatal("youtubei.googleapis.com should be allowed")
	}
	if !AllowedHostForSettings("youtube.googleapis.com", settings) {
		t.Fatal("youtube.googleapis.com should be allowed")
	}
}

func TestPresetYouTubeDomains(t *testing.T) {
	domains := PresetYouTube.Domains()
	if len(domains) == 0 {
		t.Fatal("Domains() should return non-empty list")
	}
	foundGoogle := false
	for _, d := range domains {
		if d == "google.com" {
			foundGoogle = true
			break
		}
	}
	if !foundGoogle {
		t.Fatal("youtube preset Domains() should include google.com")
	}
}

func TestPresetGoogleMediaDomains(t *testing.T) {
	domains := PresetGoogleMedia.Domains()
	if len(domains) == 0 {
		t.Fatal("Domains() should return non-empty list")
	}
}

func TestPresetDomainsConsistency(t *testing.T) {
	youtubeDomains := PresetYouTube.Domains()
	googleMediaDomains := PresetGoogleMedia.Domains()

	for _, yd := range youtubeDomains {
		found := false
		for _, gd := range googleMediaDomains {
			if yd == gd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("youtube domain %q not found in google_media", yd)
		}
	}
}

func TestAllowlistPresetValid(t *testing.T) {
	if !PresetYouTube.Valid() {
		t.Fatal("youtube preset should be valid")
	}
	if !PresetGoogleMedia.Valid() {
		t.Fatal("google_media preset should be valid")
	}
	if !PresetCustom.Valid() {
		t.Fatal("custom preset should be valid")
	}
	if AllowlistPreset("invalid").Valid() {
		t.Fatal("invalid preset should not be valid")
	}
}

func TestAllowlistPresetString(t *testing.T) {
	if PresetYouTube.String() != "youtube" {
		t.Fatal("wrong string")
	}
	if PresetGoogleMedia.String() != "google_media" {
		t.Fatal("wrong string")
	}
}

func TestAllowedHostInvalidPresetFallsBack(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = "invalid"

	if !AllowedHostForSettings("www.youtube.com", settings) {
		t.Fatal("invalid preset should fall back to google_media")
	}
}

func TestAllowedHostTrimmedSuffix(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = string(PresetYouTube)

	if !AllowedHostForSettings("www.youtube.com.", settings) {
		t.Fatal("trailing dot should be ignored")
	}
}

func TestAllowedHostCustomEmptyDefaults(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = string(PresetCustom)
	settings.CustomDomains = nil

	if !AllowedHostForSettings("www.youtube.com", settings) {
		t.Fatal("custom with no domains should fall back to youtube")
	}
}
