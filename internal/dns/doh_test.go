package dns

import (
	"context"
	"net"
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
	r := NewResolver()
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
	r := NewResolver()
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
