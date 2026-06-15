//go:build windows

package app

import "time"

const notificationDuration = 5 * time.Second

type trayNotifier interface {
	SetTooltip(text string)
}

// showStatusNotification sets a temporary tray tooltip and restores it after
// a short delay. It is used to surface status changes without creating native
// toast notifications.
func showStatusNotification(tray trayNotifier, message string, onRestore func()) {
	tray.SetTooltip(message)
	time.AfterFunc(notificationDuration, func() {
		if onRestore != nil {
			onRestore()
		}
	})
}
