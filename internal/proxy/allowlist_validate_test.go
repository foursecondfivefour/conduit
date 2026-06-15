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

func TestValidateCustomDomainRejectsWildcard(t *testing.T) {
	if err := ValidateCustomDomain("*.example.com"); err == nil {
		t.Fatal("wildcard should be rejected")
	}
}

func TestValidateCustomDomainRejectsSpecialChars(t *testing.T) {
	bad := []string{"test.com/path", "test.com?q=1", "test#com"}
	for _, d := range bad {
		if err := ValidateCustomDomain(d); err == nil {
			t.Fatalf("ValidateCustomDomain(%q) should fail for special chars", d)
		}
	}
}

func TestValidateCustomDomainRejectsTLD(t *testing.T) {
	if err := ValidateCustomDomain("com"); err == nil {
		t.Fatal("single-label TLD should be rejected")
	}
}

func TestFilterCustomDomainsStopsAtMax(t *testing.T) {
	in := make([]string, 100)
	for i := range in {
		in[i] = "host" + string(rune('a'+i%26)) + ".example.com"
	}
	out := FilterCustomDomains(in)
	if len(out) > maxCustomDomains {
		t.Fatalf("FilterCustomDomains returned %d entries, max %d", len(out), maxCustomDomains)
	}
}

func TestValidateCustomDomainRejectsInvalidLabel(t *testing.T) {
	tooLong := ""
	for i := 0; i < 64; i++ {
		tooLong += "a"
	}
	if err := ValidateCustomDomain(tooLong + ".com"); err == nil {
		t.Fatal("label > 63 chars should be rejected")
	}
}

func TestBlockedSuffixesCoverage(t *testing.T) {
	for _, suffix := range []string{".com", ".net", ".org", ".io", ".co", ".ru", ".uk", ".de", ".fr", ".cn", ".google.com", ".youtube.com", ".googlevideo.com"} {
		if _, ok := blockedSuffixes[suffix]; !ok {
			t.Errorf("expected %q in blockedSuffixes", suffix)
		}
	}
}
