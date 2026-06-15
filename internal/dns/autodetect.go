package dns

import (
	"context"
	"fmt"
	"time"
)

// MeasureLatency tests how long a DoH resolution takes for the given provider.
func (r *Resolver) MeasureLatency(ctx context.Context, provider Provider) (time.Duration, error) {
	// Temporarily switch provider
	original := r.Provider()
	r.SetProvider(provider)
	defer r.SetProvider(original)

	start := time.Now()
	_, err := r.LookupIP(ctx, "dns.google")
	if err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// AutoSelectProvider pings all providers and returns the fastest one.
func AutoSelectProvider(ctx context.Context, resolver *Resolver) (Provider, time.Duration, error) {
	providers := []Provider{ProviderCloudflare, ProviderGoogle, ProviderQuad9}
	var best Provider
	var bestLatency time.Duration

	for _, p := range providers {
		lat, err := resolver.MeasureLatency(ctx, p)
		if err != nil {
			continue
		}
		if bestLatency == 0 || lat < bestLatency {
			bestLatency = lat
			best = p
		}
	}

	if bestLatency == 0 {
		return ProviderCloudflare, 0, fmt.Errorf("all DoH providers failed")
	}
	return best, bestLatency, nil
}
