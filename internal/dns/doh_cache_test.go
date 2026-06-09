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
