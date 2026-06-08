package app

import _ "embed"

// App icon (256×256) for the Wails application shell.
//
//go:embed assets/icon.png
var appIcon []byte

// Tray icon (32×32) for the Windows notification area.
//
//go:embed assets/icon-tray.png
var trayIcon []byte
