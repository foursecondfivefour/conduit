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
	if AllowedHostForSettings("www.google.com", settings) {
		t.Fatal("google.com should be blocked in youtube preset")
	}
	if !AllowedHostForSettings("www.youtube.com", settings) {
		t.Fatal("youtube.com should be allowed")
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
