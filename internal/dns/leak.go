package dns

import (
	"context"
	"fmt"
	"net"
)

const leakTestDomain = "whoami.akamai.net"

// LeakDetector checks whether DNS queries are leaking outside the DoH resolver.
type LeakDetector struct{}

// TestLeak checks for DNS leaks by comparing DoH resolution with system DNS.
// It returns (leaked bool, error). A non-nil error means the check could not
// be completed. A true leak value means system DNS returned results,
// indicating a potential leak.
func (d *LeakDetector) TestLeak(ctx context.Context, resolver *Resolver) (bool, error) {
	// Resolve via DoH
	dohIPs, err := resolver.LookupIP(ctx, leakTestDomain)
	if err != nil {
		return false, fmt.Errorf("dns leak check: DoH lookup failed: %w", err)
	}

	// Resolve via system DNS
	sysIPs, err := net.DefaultResolver.LookupHost(ctx, leakTestDomain)
	if err != nil {
		// System DNS failed — no leak (or can't determine via system DNS)
		return false, nil
	}

	// If system DNS returned results, there is a potential leak.
	if len(sysIPs) > 0 && len(dohIPs) > 0 {
		return true, nil
	}
	return false, nil
}
