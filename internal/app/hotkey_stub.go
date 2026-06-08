//go:build !windows

package app

// HotkeyToggle is a stub on non-Windows platforms.
type HotkeyToggle struct{}

func NewHotkeyToggle(onToggle func()) *HotkeyToggle { return &HotkeyToggle{} }

func (h *HotkeyToggle) Register(hwnd uintptr) error { return nil }

func (h *HotkeyToggle) Unregister() {}

func (h *HotkeyToggle) HandleMessage(msg uint32) bool { return false }
