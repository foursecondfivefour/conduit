package proxy

import (
	"testing"

	"github.com/foursecondfivefour/conduit/internal/config"
)

func TestValidateCustomDomainRejectsBroadSuffix(t *testing.T) {
	bad := []string{"com", "org", ".com", "google.com", "*.youtube.com", ""}
	for _, d := range bad {
		if err := ValidateCustomDomain(d); err == nil {
			t.Fatalf("ValidateCustomDomain(%q) should fail", d)
		}
	}
	if err := ValidateCustomDomain("cdn.example.org"); err != nil {
		t.Fatalf("expected valid domain: %v", err)
	}
}

func TestAllowedHostRejectsBroadSuffix(t *testing.T) {
	settings := config.DefaultSettings()
	settings.AllowlistPreset = PresetCustom.String()
	settings.CustomDomains = []string{"com"}
	if AllowedHostForSettings("evil.example.com", settings) {
		t.Fatal("broad custom suffix must not match")
	}
}

func TestFilterCustomDomainsCap(t *testing.T) {
	in := make([]string, 100)
	for i := range in {
		in[i] = "host.example.com"
	}
	out := FilterCustomDomains(in)
	if len(out) != 1 {
		t.Fatalf("expected dedupe to 1 entry, got %d", len(out))
	}
}
