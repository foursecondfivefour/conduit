//go:build windows

package proxy

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

const (
	killSwitchBlockRule = "Conduit KS BlockAll"
	killSwitchAllowRule = "Conduit KS AllowConduit"
)

// KillSwitch blocks all outgoing traffic (except localhost and Conduit itself)
// when the proxy is not running, preventing IP leaks.
type KillSwitch struct {
	mu      sync.Mutex
	enabled bool
}

func NewKillSwitch() *KillSwitch {
	return &KillSwitch{}
}

func (ks *KillSwitch) Enable() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("killswitch: cannot get exe path: %w", err)
	}

	// Create outbound block-all rule
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+killSwitchBlockRule,
		"dir=out",
		"action=block",
		"enable=yes",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("killswitch: block rule: %w: %s", err, string(out))
	}

	// Create outbound allow rule for Conduit itself
	cmd = exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+killSwitchAllowRule,
		"dir=out",
		"action=allow",
		"program="+exePath,
		"enable=yes",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Clean up the block rule on failure
		if err2 := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
			"name="+killSwitchBlockRule).Run(); err2 != nil {
			slog.Warn("killswitch: failed to clean up block rule after allow rule failure",
				"cleanup_err", err2)
		}
		return fmt.Errorf("killswitch: allow rule: %w: %s", err, string(out))
	}

	ks.enabled = true
	return nil
}

func (ks *KillSwitch) Disable() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	var lastErr error
	for _, name := range []string{killSwitchBlockRule, killSwitchAllowRule} {
		cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
			"name="+name,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("killswitch: delete %s: %w: %s", name, err, string(out))
		}
	}

	ks.enabled = false
	return lastErr
}

func (ks *KillSwitch) IsEnabled() bool {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	return ks.enabled
}
