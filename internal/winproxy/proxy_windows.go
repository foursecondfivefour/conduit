//go:build windows

package winproxy

import (
	"fmt"
	"sync"

	"golang.org/x/sys/windows/registry"
)

const settingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type Manager struct {
	mu      sync.Mutex
	saved   bool
	enable  uint64
	server  string
	enabled bool
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Enable(host string, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k, _, err := registry.CreateKey(registry.CURRENT_USER, settingsKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if !m.saved {
		if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
			m.enable = v
		}
		if s, _, err := k.GetStringValue("ProxyServer"); err == nil {
			m.server = s
		}
		m.saved = true
	}

	server := fmt.Sprintf("%s:%d", host, port)
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	if err := k.SetStringValue("ProxyServer", server); err != nil {
		return err
	}
	m.enabled = true
	return notifyProxyChanged()
}

func (m *Manager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, settingsKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if m.saved {
		_ = k.SetDWordValue("ProxyEnable", uint32(m.enable))
		_ = k.SetStringValue("ProxyServer", m.server)
	} else {
		_ = k.SetDWordValue("ProxyEnable", 0)
	}
	m.enabled = false
	return notifyProxyChanged()
}

func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}
