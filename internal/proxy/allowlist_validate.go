package proxy

import (
	"fmt"
	"strings"
	"unicode"
)

const maxCustomDomains = 64

// blockedSuffixes are over-broad suffixes that must never be used in custom allowlists.
var blockedSuffixes = map[string]struct{}{
	".com": {}, ".net": {}, ".org": {}, ".io": {}, ".co": {},
	".ru": {}, ".uk": {}, ".de": {}, ".fr": {}, ".cn": {},
	".google.com": {}, ".youtube.com": {}, ".googlevideo.com": {},
}

// ValidateCustomDomain rejects TLD-only or public-suffix entries that would bypass the allowlist.
func ValidateCustomDomain(domain string) error {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.TrimSuffix(d, ".")
	if d == "" {
		return fmt.Errorf("empty domain")
	}
	if strings.Contains(d, "*") {
		return fmt.Errorf("wildcards not allowed")
	}
	if strings.ContainsAny(d, `/\?#%`) {
		return fmt.Errorf("invalid characters")
	}

	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return fmt.Errorf("domain must have at least two labels")
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return fmt.Errorf("invalid label")
		}
		for _, r := range label {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-') {
				return fmt.Errorf("invalid label characters")
			}
		}
	}

	suffix := d
	if !strings.HasPrefix(suffix, ".") {
		suffix = "." + suffix
	}
	if _, blocked := blockedSuffixes[suffix]; blocked {
		return fmt.Errorf("suffix %q is too broad", suffix)
	}
	// Reject single-label TLD-style entries after normalization (e.g. "com" -> ".com").
	if len(labels) == 1 {
		return fmt.Errorf("top-level suffix not allowed")
	}
	return nil
}

// FilterCustomDomains returns only domains that pass validation (max maxCustomDomains).
func FilterCustomDomains(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}
	out := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		if len(out) >= maxCustomDomains {
			break
		}
		if err := ValidateCustomDomain(d); err != nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(d))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}
