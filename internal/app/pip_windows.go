//go:build windows

package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/foursecondfivefour/conduit/internal/config"
)

// PiPWindow toggles the main YouTube window between full and mini always-on-top mode.
type PiPWindow struct {
	youtube  *application.WebviewWindow
	isActive bool
	normalW  int
	normalH  int
}

func NewPiPWindow(youtube *application.WebviewWindow) *PiPWindow {
	return &PiPWindow{youtube: youtube, normalW: config.WindowWidth, normalH: config.WindowHeight}
}

func (p *PiPWindow) Toggle() {
	if p.isActive {
		p.youtube.SetAlwaysOnTop(false)
		p.youtube.SetSize(p.normalW, p.normalH)
		p.youtube.SetTitle("YouTube")
		p.isActive = false
	} else {
		p.isActive = true
		p.youtube.SetAlwaysOnTop(true)
		p.youtube.SetSize(config.PiPWidth, config.PiPHeight)
		p.youtube.SetTitle("YouTube (PiP)")
	}
}

func (p *PiPWindow) IsActive() bool { return p.isActive }
