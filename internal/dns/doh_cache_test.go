package dns

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestDNSCacheBounded(t *testing.T) {
	r := NewResolver(ProviderCloudflare)
	for i := 0; i < maxCacheHosts+50; i++ {
		host := fmt.Sprintf("host%d.example.com", i)
		r.mu.Lock()
		r.cache[host] = cacheEntry{
			ips:       []net.IP{net.ParseIP("1.1.1.1")},
			expiresAt: time.Now().Add(time.Hour),
		}
		r.enforceCacheCapLocked()
		r.mu.Unlock()
	}
	if size := r.CacheSize(); size > maxCacheHosts {
		t.Fatalf("cache size %d exceeds cap %d", size, maxCacheHosts)
	}
}

func TestDNSCacheExpiryEviction(t *testing.T) {
	r := NewResolver(ProviderCloudflare)
	r.mu.Lock()
	r.cache["fresh.com"] = cacheEntry{
		ips:       []net.IP{net.ParseIP("1.1.1.1")},
		expiresAt: time.Now().Add(time.Hour),
	}
	r.cache["expired.com"] = cacheEntry{
		ips:       []net.IP{net.ParseIP("2.2.2.2")},
		expiresAt: time.Now().Add(-time.Hour),
	}
	r.mu.Unlock()

	r.mu.Lock()
	r.evictExpiredLocked(time.Now())
	r.mu.Unlock()

	r.mu.RLock()
	_, freshExists := r.cache["fresh.com"]
	_, expiredExists := r.cache["expired.com"]
	r.mu.RUnlock()

	if !freshExists {
		t.Fatal("fresh entry was incorrectly evicted")
	}
	if expiredExists {
		t.Fatal("expired entry should have been evicted")
	}
}
