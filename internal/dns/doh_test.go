package dns

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
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
	if len(ips) != 1 {
		t.Fatalf("got %d A records, want 1", len(ips))
	}
	if ips[0].String() != "142.250.185.78" {
		t.Fatalf("ip = %s", ips[0])
	}
	if ttl != 120*time.Second {
		t.Fatalf("ttl = %s, want 120s", ttl)
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
	ips, ttl, err := r.queryEndpoint(context.Background(), srv.URL, "example.com")
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
