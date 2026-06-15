package proxy

import (
	"sync"
	"sync/atomic"
	"time"
)

// TrafficStats tracks real-time proxy traffic metrics.
type TrafficStats struct {
	BytesIn     atomic.Int64
	BytesOut    atomic.Int64
	Connections atomic.Int64
	ActiveConns atomic.Int64
	StartTime   time.Time

	mu         sync.RWMutex
	domainHits map[string]int64
}

// NewTrafficStats creates a new traffic statistics tracker.
func NewTrafficStats() *TrafficStats {
	return &TrafficStats{
		StartTime:  time.Now(),
		domainHits: make(map[string]int64),
	}
}

// RecordConnection increments counters for a completed connection.
func (s *TrafficStats) RecordConnection(domain string, bytesIn, bytesOut int64) {
	s.Connections.Add(1)
	s.BytesIn.Add(bytesIn)
	s.BytesOut.Add(bytesOut)
	s.mu.Lock()
	s.domainHits[domain]++
	s.mu.Unlock()
}

// Snapshot returns a copy of current statistics.
func (s *TrafficStats) Snapshot() StatsSnapshot {
	s.mu.RLock()
	domains := make(map[string]int64, len(s.domainHits))
	for k, v := range s.domainHits {
		domains[k] = v
	}
	s.mu.RUnlock()
	return StatsSnapshot{
		BytesIn:     s.BytesIn.Load(),
		BytesOut:    s.BytesOut.Load(),
		Connections: s.Connections.Load(),
		ActiveConns: s.ActiveConns.Load(),
		Uptime:      int64(time.Since(s.StartTime).Seconds()),
		DomainHits:  domains,
	}
}

// StatsSnapshot is a read-only snapshot of proxy statistics for JSON serialization.
type StatsSnapshot struct {
	BytesIn     int64            `json:"bytesIn"`
	BytesOut    int64            `json:"bytesOut"`
	Connections int64            `json:"connections"`
	ActiveConns int64            `json:"activeConns"`
	Uptime      int64            `json:"uptime"`
	DomainHits  map[string]int64 `json:"domainHits"`
}
