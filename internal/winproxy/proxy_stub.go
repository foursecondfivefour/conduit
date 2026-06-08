//go:build !windows

package winproxy

// Manager is a stub on non-Windows platforms.
type Manager struct{}

func NewManager() *Manager { return &Manager{} }

func (m *Manager) Enable(host string, port int) error { return nil }

func (m *Manager) Disable() error { return nil }

func (m *Manager) Enabled() bool { return false }
