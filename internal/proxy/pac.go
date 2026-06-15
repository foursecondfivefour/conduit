package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

// GeneratePAC creates a PAC file that routes allowed domains through the proxy.
func GeneratePAC(proxyHost string, proxyPort int, domains []string) string {
	var conditions []string
	for _, d := range domains {
		d = strings.TrimPrefix(d, ".")
		conditions = append(conditions, fmt.Sprintf(`dnsDomainIs(host, "%s")`, d))
		conditions = append(conditions, fmt.Sprintf(`shExpMatch(host, "*.%s")`, d))
	}
	proxy := fmt.Sprintf("PROXY %s:%d", proxyHost, proxyPort)
	if len(conditions) == 0 {
		return fmt.Sprintf("function FindProxyForURL(url, host) { return \"%s\"; }", proxy)
	}
	return fmt.Sprintf(`function FindProxyForURL(url, host) {
    if (%s) { return "%s"; }
    return "DIRECT";
}`, strings.Join(conditions, " || "), proxy)
}

// PACHandler returns an HTTP handler that serves the PAC file.
func PACHandler(proxyHost string, proxyPort int, domains []string) http.HandlerFunc {
	pac := GeneratePAC(proxyHost, proxyPort, domains)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		_, _ = w.Write([]byte(pac))
	}
}
