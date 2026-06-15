package dpi

import "strings"

// DomainPolicy maps hosts to DPI strategies with a default fallback.
type DomainPolicy struct {
	defaultStrategy Strategy
	domainStrategy  map[string]Strategy
}

// NewDomainPolicy creates a per-domain DPI policy with the given default strategy.
func NewDomainPolicy(defaultStrategy Strategy) *DomainPolicy {
	return &DomainPolicy{
		defaultStrategy: defaultStrategy,
		domainStrategy:  make(map[string]Strategy),
	}
}

// Set assigns a DPI strategy for the given domain.
func (p *DomainPolicy) Set(domain string, strategy Strategy) {
	if p.domainStrategy == nil {
		p.domainStrategy = make(map[string]Strategy)
	}
	p.domainStrategy[domain] = strategy
}

// Get returns the strategy for host, falling back to parent domains and then default.
func (p *DomainPolicy) Get(host string) Strategy {
	if p.domainStrategy == nil {
		return p.defaultStrategy
	}
	if s, ok := p.domainStrategy[host]; ok {
		return s
	}
	parts := strings.Split(host, ".")
	for i := 1; i < len(parts); i++ {
		suffix := strings.Join(parts[i:], ".")
		if s, ok := p.domainStrategy[suffix]; ok {
			return s
		}
	}
	return p.defaultStrategy
}

// SetDefault changes the default strategy for unmatched hosts.
func (p *DomainPolicy) SetDefault(s Strategy) {
	p.defaultStrategy = s
}
