//go:build windows

package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// setupWindowCloseToTray intercepts window close when minimize-to-tray is enabled.
func setupWindowCloseToTray(win *application.WebviewWindow, minimizeToTray func() bool) {
	if win == nil {
		return
	}
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if minimizeToTray != nil && minimizeToTray() {
			e.Cancel()
			win.Hide()
		}
	})
}
