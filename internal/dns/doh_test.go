package dns

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseDoHResponse(t *testing.T) {
	body := []byte(`{
		"Answer": [
			{"type": 1, "data": "142.250.185.78", "TTL": 120},
			{"type": 28, "data": "2607:f8b0:4004:c1c::93", "TTL": 120}
		]
	}`)

	ips, ttl, err := parseDoHResponse(body)
	if err != nil {
		t.Fatalf("parseDoHResponse: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("got %d records, want 2", len(ips))
	}
	if ips[0].String() != "142.250.185.78" {
		t.Fatalf("ip = %s", ips[0])
	}
	if ttl != 120*time.Second {
		t.Fatalf("ttl = %s, want 120s", ttl)
	}
}

func TestParseDoHResponseAAAAOnly(t *testing.T) {
	body := []byte(`{"Answer":[{"type":28,"data":"2607:f8b0:4004:c1c::93","TTL":60}]}`)
	ips, ttl, err := parseDoHResponse(body)
	if err != nil {
		t.Fatalf("parseDoHResponse: %v", err)
	}
	if len(ips) != 1 || ips[0].String() != "2607:f8b0:4004:c1c::93" {
		t.Fatalf("ips = %v", ips)
	}
	if ttl != 60*time.Second {
		t.Fatalf("ttl = %s", ttl)
	}
}

func TestClearCache(t *testing.T) {
	r := NewResolver(ProviderCloudflare)
	r.mu.Lock()
	r.cache["youtube.com"] = cacheEntry{
		ips:       []net.IP{net.ParseIP("1.2.3.4")},
		expiresAt: time.Now().Add(time.Hour),
	}
	r.mu.Unlock()

	r.ClearCache()

	r.mu.RLock()
	n := len(r.cache)
	r.mu.RUnlock()
	if n != 0 {
		t.Fatalf("cache len = %d, want 0", n)
	}
}

func TestLookupIPUsesCache(t *testing.T) {
	r := NewResolver(ProviderCloudflare)
	ip := net.ParseIP("93.184.216.34")
	r.mu.Lock()
	r.cache["example.com"] = cacheEntry{
		ips:       []net.IP{ip},
		expiresAt: time.Now().Add(time.Hour),
	}
	r.mu.Unlock()

	ips, err := r.LookupIP(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("LookupIP: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(ip) {
		t.Fatalf("unexpected ips: %v", ips)
	}
}

func TestProviderEndpoints(t *testing.T) {
	tests := []struct {
		p    Provider
		want string
	}{
		{ProviderCloudflare, "https://cloudflare-dns.com/dns-query"},
		{ProviderGoogle, "https://dns.google/resolve"},
		{ProviderQuad9, "https://dns.quad9.net:5053/dns-query"},
	}
	for _, tt := range tests {
		if got := tt.p.Endpoint(); got != tt.want {
			t.Fatalf("%s endpoint = %s", tt.p, got)
		}
	}
}

func TestSetProvider(t *testing.T) {
	r := NewResolver(ProviderCloudflare)
	r.SetProvider(ProviderGoogle)
	if r.Provider() != ProviderGoogle {
		t.Fatalf("provider = %s", r.Provider())
	}
}

func TestResolverQueryMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-json")
		_, _ = w.Write([]byte(`{"Answer":[{"type":1,"data":"1.1.1.1","TTL":60}]}`))
	}))
	defer srv.Close()

	r := NewResolver(ProviderCloudflare)
	ips, ttl, err := r.queryEndpoint(context.Background(), srv.URL, "example.com", "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 || ips[0].String() != "1.1.1.1" {
		t.Fatalf("ips = %v", ips)
	}
	if ttl != 60*time.Second {
		t.Fatalf("ttl = %s", ttl)
	}
}

func TestProviderFallback(t *testing.T) {
	tests := []struct {
		p    Provider
		want Provider
	}{
		{ProviderCloudflare, ProviderGoogle},
		{ProviderGoogle, ProviderCloudflare},
		{ProviderQuad9, ProviderCloudflare},
	}
	for _, tt := range tests {
		if got := tt.p.Fallback(); got != tt.want {
			t.Errorf("%s.Fallback() = %s, want %s", tt.p, got, tt.want)
		}
	}
}

func TestProviderShortLabel(t *testing.T) {
	tests := []struct {
		p    Provider
		want string
	}{
		{ProviderCloudflare, "CF"},
		{ProviderGoogle, "Google"},
		{ProviderQuad9, "Quad9"},
	}
	for _, tt := range tests {
		if got := tt.p.ShortLabel(); got != tt.want {
			t.Errorf("%s.ShortLabel() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestInvalidProvider(t *testing.T) {
	if Provider("invalid").Valid() {
		t.Fatal("invalid provider should not be valid")
	}
	r := NewResolver(Provider("bad"))
	if r.Provider() != ProviderCloudflare {
		t.Fatalf("default provider = %s", r.Provider())
	}
}

func TestSetProviderInvalid(t *testing.T) {
	r := NewResolver(ProviderCloudflare)
	r.SetProvider(Provider("bad"))
	if r.Provider() != ProviderCloudflare {
		t.Fatalf("provider should not change for invalid: %s", r.Provider())
	}
}

func TestAdBlockerNew(t *testing.T) {
	ab := NewAdBlocker()
	if ab == nil {
		t.Fatal("NewAdBlocker returned nil")
	}
	if ab.Enabled() {
		t.Fatal("ad blocker should be disabled by default")
	}
	if ab.Count() != 0 {
		t.Fatalf("count = %d, want 0", ab.Count())
	}
}

func TestAdBlockerEnableDisable(t *testing.T) {
	ab := NewAdBlocker()
	ab.SetEnabled(true)
	if !ab.Enabled() {
		t.Fatal("ad blocker should be enabled")
	}
	ab.SetEnabled(false)
	if ab.Enabled() {
		t.Fatal("ad blocker should be disabled")
	}
}

func TestAdBlockerIsBlockedDisabled(t *testing.T) {
	ab := NewAdBlocker()
	ab.loadFromReader(strings.NewReader("0.0.0.0 ads.example.com\n"))
	if ab.IsBlocked("ads.example.com") {
		t.Fatal("blocked should return false when disabled")
	}
}

func TestAdBlockerIsBlockedExact(t *testing.T) {
	ab := NewAdBlocker()
	ab.SetEnabled(true)
	ab.loadFromReader(strings.NewReader("0.0.0.0 doubleclick.net\n0.0.0.0 ads.example.com\n"))

	if !ab.IsBlocked("doubleclick.net") {
		t.Fatal("doubleclick.net should be blocked")
	}
	if !ab.IsBlocked("ads.example.com") {
		t.Fatal("ads.example.com should be blocked")
	}
	if ab.IsBlocked("youtube.com") {
		t.Fatal("youtube.com should not be blocked")
	}
}

func TestAdBlockerIsBlockedSubdomain(t *testing.T) {
	ab := NewAdBlocker()
	ab.SetEnabled(true)
	ab.loadFromReader(strings.NewReader("0.0.0.0 doubleclick.net\n"))

	if !ab.IsBlocked("ad.doubleclick.net") {
		t.Fatal("ad.doubleclick.net should be blocked via parent domain")
	}
	if !ab.IsBlocked("a.b.doubleclick.net") {
		t.Fatal("deep subdomain should be blocked")
	}
}

func TestAdBlockerLoadFromReader(t *testing.T) {
	ab := NewAdBlocker()
	ab.SetEnabled(true)

	blocklist := `# Blocklist
0.0.0.0 tracker.example.com
0.0.0.0 ad.server.com
127.0.0.1 malware.test.org
# comment
`
	ab.loadFromReader(strings.NewReader(blocklist))
	if ab.Count() != 2 {
		t.Fatalf("count = %d, want 2", ab.Count())
	}
	if !ab.IsBlocked("tracker.example.com") {
		t.Fatal("tracker.example.com should be blocked")
	}
	if !ab.IsBlocked("ad.server.com") {
		t.Fatal("ad.server.com should be blocked")
	}
	if ab.IsBlocked("malware.test.org") {
		t.Fatal("127.0.0.1 entries should be ignored")
	}
}

func TestAdBlockerCount(t *testing.T) {
	ab := NewAdBlocker()
	ab.loadFromReader(strings.NewReader("0.0.0.0 a.com\n0.0.0.0 b.com\n0.0.0.0 c.com\n"))
	if ab.Count() != 3 {
		t.Fatalf("count = %d, want 3", ab.Count())
	}
}

func TestAdBlockerIntegrationWithResolver(t *testing.T) {
	ab := NewAdBlocker()
	ab.SetEnabled(true)
	ab.loadFromReader(strings.NewReader("0.0.0.0 blocked.test.domain\n"))

	r := NewResolver(ProviderCloudflare)
	r.SetAdBlocker(ab)

	ips, err := r.LookupIP(context.Background(), "blocked.test.domain")
	if err != nil {
		t.Fatalf("LookupIP unexpected error: %v", err)
	}
	if len(ips) != 1 || ips[0].String() != "0.0.0.0" {
		t.Fatalf("blocked domain got ips = %v, want [0.0.0.0]", ips)
	}
}

func TestAdBlockerNotBlockedWhenNoBlocker(t *testing.T) {
	r := NewResolver(ProviderCloudflare)
	ip := net.ParseIP("1.2.3.4")
	r.mu.Lock()
	r.cache["normal.domain.com"] = cacheEntry{
		ips:       []net.IP{ip},
		expiresAt: time.Now().Add(time.Hour),
	}
	r.mu.Unlock()

	ips, err := r.LookupIP(context.Background(), "normal.domain.com")
	if err != nil {
		t.Fatalf("LookupIP: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(ip) {
		t.Fatalf("non-blocked domain got ips = %v", ips)
	}
}

func TestLeakDetectorStructure(t *testing.T) {
	d := &LeakDetector{}
	_ = d // ensure d is used; it can never be nil after allocation
}
