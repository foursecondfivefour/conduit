//go:build !windows

package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/foursecondfivefour/conduit/internal/i18n"
)

func StartYouTubePreload(_ *application.WebviewWindow, _ i18n.Lang) <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}
