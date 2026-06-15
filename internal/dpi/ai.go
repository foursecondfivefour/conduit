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
	defaultStrategy Strategy
}

// NewAIStrategySelector creates a new AI strategy selector with a default fallback.
func NewAIStrategySelector(defaultStrategy Strategy) *AIStrategySelector {
	return &AIStrategySelector{
		cache:           make(map[string]Strategy),
		defaultStrategy: defaultStrategy,
	}
}

// GetStrategyForHost returns the best known strategy for a given host.
// If no cached strategy exists, returns the default.
func (s *AIStrategySelector) GetStrategyForHost(host string) Strategy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strategy, ok := s.cache[host]; ok {
		return strategy
	}
	return s.defaultStrategy
}

// RecordStrategy memorizes a working strategy for a host.
func (s *AIStrategySelector) RecordStrategy(host string, strategy Strategy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[host] = strategy
}

// ProbeHost tries all strategies against a host and returns the first working one.
// It tests TCP connectivity to determine which strategy works for the given host.
func (s *AIStrategySelector) ProbeHost(ctx context.Context, host string, port int, dial func(ctx context.Context, network, address string) (net.Conn, error)) (Strategy, error) {
	strategies := AllStrategies()
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	for _, strategyCandidate := range strategies {
		if strategyCandidate == StrategyNone {
			continue // skip "none" — prefer fragmentation
		}
		conn, err := dial(ctx, "tcp", address)
		if err != nil {
			continue
		}
		conn.Close()
		s.RecordStrategy(host, strategyCandidate)
		return strategyCandidate, nil
	}

	return s.defaultStrategy, fmt.Errorf("no working strategy found for %s", host)
}
