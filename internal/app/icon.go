package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	_ "embed"
)

// App icon (256×256) for the Wails application shell.
//
//go:embed assets/icon.png
var appIcon []byte

// Tray icon (32×32) for the Windows notification area.
//
//go:embed assets/icon-tray.png
var trayIcon []byte

var (
	trayIconGreen  = genTrayColorIcon(46, 204, 113)
	trayIconYellow = genTrayColorIcon(241, 196, 15)
	trayIconRed    = genTrayColorIcon(231, 76, 60)
	trayIconGray   = genTrayColorIcon(149, 165, 166)
)

func genTrayColorIcon(r, g, b uint8) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	c := color.RGBA{R: r, G: g, B: b, A: 255}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		slog.Warn("icon: png encode failed", "err", err)
	}
	return buf.Bytes()
}

// TrayStatusIcon returns a colored tray icon based on proxy health.
// unknown → gray, running+OK → green, running+fail → yellow, stopped → red.
func TrayStatusIcon(running bool, healthOK bool, healthKnown bool) []byte {
	switch {
	case running && healthKnown && healthOK:
		return trayIconGreen
	case running && healthKnown && !healthOK:
		return trayIconYellow
	case !running:
		return trayIconRed
	default:
		return trayIconGray
	}
}
