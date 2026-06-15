//go:build !windows

package proxy

// KillSwitch is a no-op on non-Windows platforms.
type KillSwitch struct{}

func NewKillSwitch() *KillSwitch { return &KillSwitch{} }

func (ks *KillSwitch) Enable() error  { return nil }
func (ks *KillSwitch) Disable() error { return nil }
func (ks *KillSwitch) IsEnabled() bool { return false }
