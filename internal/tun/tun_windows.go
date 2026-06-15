//go:build windows

package tun

import (
	"os/exec"
)

// TUNMode blocks non-Conduit outbound traffic via Windows Firewall.
// It acts as a system-wide VPN-like mode by enforcing all outbound
// connections to go through the Conduit proxy.
type TUNMode struct {
	enabled bool
}

// NewTUNMode creates a new TUN mode controller.
func NewTUNMode() *TUNMode {
	return &TUNMode{}
}

// Enable creates firewall rules that block all outbound traffic except Conduit.
func (t *TUNMode) Enable(exePath string) error {
	// Block all outbound
	exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=Conduit TUN BlockAll", "dir=out", "action=block", "enable=yes").Run()
	// Allow Conduit executable
	exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=Conduit TUN Allow", "dir=out", "action=allow",
		"program="+exePath, "enable=yes").Run()
	t.enabled = true
	return nil
}

// Disable removes the TUN firewall rules.
func (t *TUNMode) Disable() error {
	exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name=Conduit TUN BlockAll").Run()
	exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name=Conduit TUN Allow").Run()
	t.enabled = false
	return nil
}

// IsEnabled reports whether TUN mode is active.
func (t *TUNMode) IsEnabled() bool { return t.enabled }
