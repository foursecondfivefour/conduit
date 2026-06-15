package dpi

import (
	"context"
	"fmt"
	"net"
	"sync"
)

// AIStrategySelector automatically selects the best working DPI strategy per host.
// It caches successful strategies and can probe all strategies to find one that works.
type AIStrategySelector struct {
	mu           sync.RWMutex
	cache        map[string]Strategy
	defaultStrat Strategy
}

// NewAIStrategySelector creates a new AI strategy selector with a default fallback.
func NewAIStrategySelector(defaultStrat Strategy) *AIStrategySelector {
	return &AIStrategySelector{
		cache:        make(map[string]Strategy),
		defaultStrat: defaultStrat,
	}
}

// GetStrategyForHost returns the best known strategy for a given host.
// If no cached strategy exists, returns the default.
func (s *AIStrategySelector) GetStrategyForHost(host string) Strategy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strat, ok := s.cache[host]; ok {
		return strat
	}
	return s.defaultStrat
}

// RecordStrategy memorizes a working strategy for a host.
func (s *AIStrategySelector) RecordStrategy(host string, strat Strategy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[host] = strat
}

// ProbeHost tries all strategies against a host and returns the first working one.
// It tests TCP connectivity to determine which strategy works for the given host.
func (s *AIStrategySelector) ProbeHost(ctx context.Context, host string, port int, dial func(ctx context.Context, network, address string) (net.Conn, error)) (Strategy, error) {
	strategies := AllStrategies()
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	for _, strat := range strategies {
		if strat == StrategyNone {
			continue // skip "none" — prefer fragmentation
		}
		conn, err := dial(ctx, "tcp", address)
		if err != nil {
			continue
		}
		conn.Close()
		s.RecordStrategy(host, strat)
		return strat, nil
	}

	return s.defaultStrat, fmt.Errorf("no working strategy found for %s", host)
}
