package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultTTL = 300 * time.Second
)

var dohEndpoints = []string{
	"https://cloudflare-dns.com/dns-query",
	"https://dns.google/resolve",
}

type cacheEntry struct {
	ips       []net.IP
	expiresAt time.Time
}

// Resolver resolves hostnames via DNS-over-HTTPS with in-memory caching.
type Resolver struct {
	client *http.Client
	mu     sync.RWMutex
	cache  map[string]cacheEntry
}

func NewResolver() *Resolver {
	return &Resolver{
		client: &http.Client{Timeout: 8 * time.Second},
		cache:  make(map[string]cacheEntry),
	}
}

func (r *Resolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")

	r.mu.RLock()
	if entry, ok := r.cache[host]; ok && time.Now().Before(entry.expiresAt) {
		ips := append([]net.IP(nil), entry.ips...)
		r.mu.RUnlock()
		return ips, nil
	}
	r.mu.RUnlock()

	ips, ttl, err := r.queryDoH(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("dns: no addresses for %s", host)
	}

	if ttl <= 0 {
		ttl = defaultTTL
	}
	r.mu.Lock()
	r.cache[host] = cacheEntry{ips: ips, expiresAt: time.Now().Add(ttl)}
	r.mu.Unlock()

	return append([]net.IP(nil), ips...), nil
}

func (r *Resolver) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string]cacheEntry)
}

func (r *Resolver) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	ips, err := r.LookupIP(ctx, host)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("dns: dial failed for %s", address)
}

func (r *Resolver) queryDoH(ctx context.Context, host string) ([]net.IP, time.Duration, error) {
	var lastErr error
	for _, endpoint := range dohEndpoints {
		ips, ttl, err := r.queryEndpoint(ctx, endpoint, host)
		if err == nil {
			return ips, ttl, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, 0, lastErr
	}
	return nil, 0, fmt.Errorf("dns: all DoH endpoints failed")
}

func (r *Resolver) queryEndpoint(ctx context.Context, endpoint, host string) ([]net.IP, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	q := req.URL.Query()
	q.Set("name", host)
	q.Set("type", "A")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Accept", "application/dns-json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("dns: DoH status %d", resp.StatusCode)
	}

	return parseDoHResponse(body)
}

func parseDoHResponse(body []byte) ([]net.IP, time.Duration, error) {
	var payload dohResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, 0, err
	}

	var ips []net.IP
	var minTTL int
	for _, answer := range payload.Answer {
		if answer.Type != 1 {
			continue
		}
		ip := net.ParseIP(answer.Data)
		if ip == nil {
			continue
		}
		ips = append(ips, ip)
		if minTTL == 0 || (answer.TTL > 0 && answer.TTL < minTTL) {
			minTTL = answer.TTL
		}
	}

	ttl := time.Duration(minTTL) * time.Second
	return ips, ttl, nil
}

type dohResponse struct {
	Answer []struct {
		Type int    `json:"type"`
		Data string `json:"data"`
		TTL  int    `json:"TTL"`
	} `json:"Answer"`
}
