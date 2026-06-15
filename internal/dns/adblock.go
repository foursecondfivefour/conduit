package dns

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AdBlocker blocks DNS resolution for ad/tracker domains.
type AdBlocker struct {
	mu      sync.RWMutex
	blocked map[string]bool
	enabled bool
	client  *http.Client
}

func NewAdBlocker() *AdBlocker {
	return &AdBlocker{
		blocked: make(map[string]bool),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *AdBlocker) IsBlocked(host string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.enabled {
		return false
	}
	if a.blocked[host] {
		return true
	}
	parts := strings.Split(host, ".")
	for i := 0; i < len(parts)-1; i++ {
		if a.blocked[strings.Join(parts[i:], ".")] {
			return true
		}
	}
	return false
}

func (a *AdBlocker) Enabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

func (a *AdBlocker) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
}

// LoadFromURL fetches a hosts-format blocklist and loads it.
func (a *AdBlocker) LoadFromURL(url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return a.loadFromReader(resp.Body)
}

func (a *AdBlocker) loadFromReader(r io.Reader) error {
	newBlocked := make(map[string]bool)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "0.0.0.0" {
			continue
		}
		domain := strings.TrimSpace(fields[1])
		if domain != "" && !strings.Contains(domain, "*") {
			newBlocked[strings.ToLower(domain)] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	a.blocked = newBlocked
	a.mu.Unlock()
	return nil
}

func (a *AdBlocker) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.blocked)
}
