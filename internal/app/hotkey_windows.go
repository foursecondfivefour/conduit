//go:build windows

package app

import (
	"syscall"

	"github.com/foursecondfivefour/conduit/internal/config"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
)

const hotkeyID = 1

// HotkeyToggle registers Ctrl+Shift+Y to toggle YouTube window visibility.
type HotkeyToggle struct {
	hwnd uintptr
	onToggle func()
}

func NewHotkeyToggle(onToggle func()) *HotkeyToggle {
	return &HotkeyToggle{onToggle: onToggle}
}

func (h *HotkeyToggle) Register(hwnd uintptr) error {
	h.hwnd = hwnd
	ret, _, err := procRegisterHotKey.Call(
		hwnd,
		uintptr(hotkeyID),
		uintptr(config.HotkeyModifiers),
		uintptr(config.HotkeyVK),
	)
	if ret == 0 {
		return err
	}
	return nil
}

func (h *HotkeyToggle) Unregister() {
	if h.hwnd != 0 {
		_, _, _ = procUnregisterHotKey.Call(h.hwnd, uintptr(hotkeyID))
	}
}

func (h *HotkeyToggle) HandleMessage(msg uint32) bool {
	const wmHotkey = 0x0312
	if msg == wmHotkey && h.onToggle != nil {
		h.onToggle()
		return true
	}
	return false
}

