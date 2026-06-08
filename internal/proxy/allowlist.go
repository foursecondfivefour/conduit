package proxy

import "strings"

var allowedSuffixes = []string{
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

// AllowedHost reports whether host is part of the YouTube/Google media ecosystem.
func AllowedHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	for _, suffix := range allowedSuffixes {
		if host == strings.TrimPrefix(suffix, ".") || strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}
