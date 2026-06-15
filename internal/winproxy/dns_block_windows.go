//go:build windows

package winproxy

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

const (
	dnsBlockRule  = "Conduit DNS Block"
	dnsAllowRule  = "Conduit DNS Allow"
)

// DNSBlocker manages firewall rules that block outbound UDP port 53
// for all applications except Conduit itself.
type DNSBlocker struct {
	mu      sync.Mutex
	blocked bool
}

func NewDNSBlocker() *DNSBlocker {
	return &DNSBlocker{}
}

func (b *DNSBlocker) Block() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("dns blocker: cannot get exe path: %w", err)
	}

	// Block outbound UDP 53 for all
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+dnsBlockRule,
		"dir=out",
		"protocol=udp",
		"remoteport=53",
		"action=block",
		"enable=yes",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dns blocker: block rule: %w: %s", err, string(out))
	}

	// Allow outbound UDP 53 for Conduit
	cmd = exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+dnsAllowRule,
		"dir=out",
		"protocol=udp",
		"remoteport=53",
		"action=allow",
		"program="+exePath,
		"enable=yes",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Clean up the block rule
		if err2 := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
			"name="+dnsBlockRule).Run(); err2 != nil {
			slog.Warn("dns blocker: failed to clean up block rule after allow rule failure",
				"cleanup_err", err2)
		}
		return fmt.Errorf("dns blocker: allow rule: %w: %s", err, string(out))
	}

	b.blocked = true
	return nil
}

func (b *DNSBlocker) Unblock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var lastErr error
	for _, name := range []string{dnsBlockRule, dnsAllowRule} {
		cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
			"name="+name,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("dns blocker: delete %s: %w: %s", name, err, string(out))
		}
	}

	b.blocked = false
	return lastErr
}

func (b *DNSBlocker) IsBlocked() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.blocked
}
