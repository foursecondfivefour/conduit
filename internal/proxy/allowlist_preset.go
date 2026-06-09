package proxy

import "strings"

// AllowlistPreset selects which host suffixes are permitted through the proxy.
type AllowlistPreset string

const (
	PresetYouTube     AllowlistPreset = "youtube"
	PresetGoogleMedia AllowlistPreset = "google_media"
	PresetCustom      AllowlistPreset = "custom"
)

func (p AllowlistPreset) Valid() bool {
	switch p {
	case PresetYouTube, PresetGoogleMedia, PresetCustom:
		return true
	default:
		return false
	}
}

func (p AllowlistPreset) String() string {
	return string(p)
}

// YouTube playback needs Innertube/API and static assets; ads (doubleclick, pagead) stay blocked.
var youtubeOnlySuffixes = []string{
	".youtube.com",
	".googlevideo.com",
	".ytimg.com",
	".googleapis.com",
	".gstatic.com",
	".ggpht.com",
	".youtu.be",
	".youtube-nocookie.com",
}

var googleMediaSuffixes = []string{
	".youtube.com",
	".googlevideo.com",
	".ytimg.com",
	".googleapis.com",
	".ggpht.com",
	".gstatic.com",
	".google.com",
	".youtube-nocookie.com",
	".youtu.be",
}

// AllowedHost reports whether host is permitted for the given settings.
func AllowedHost(host string, preset AllowlistPreset, custom []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	suffixes := suffixesForPreset(preset, custom)
	for _, suffix := range suffixes {
		if host == strings.TrimPrefix(suffix, ".") || strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func suffixesForPreset(preset AllowlistPreset, custom []string) []string {
	switch preset {
	case PresetYouTube:
		return youtubeOnlySuffixes
	case PresetCustom:
		if len(custom) == 0 {
			return youtubeOnlySuffixes
		}
		return normalizeCustomDomains(custom)
	default:
		return googleMediaSuffixes
	}
}

func normalizeCustomDomains(domains []string) []string {
	filtered := FilterCustomDomains(domains)
	out := make([]string, 0, len(filtered))
	for _, d := range filtered {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimSuffix(d, ".")
		if !strings.HasPrefix(d, ".") {
			d = "." + d
		}
		out = append(out, d)
	}
	return out
}
