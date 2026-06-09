//go:build windows



package app



import (

	"log/slog"

	"sync"

	"sync/atomic"

	"time"



	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/wailsapp/wails/v3/pkg/events"



	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/i18n"

)



// StartYouTubePreload shows a dark placeholder instantly, then loads YouTube while hidden.

// The returned channel closes when YouTube's first navigation completes.

func StartYouTubePreload(win *application.WebviewWindow, lang i18n.Lang) <-chan struct{} {

	ready := make(chan struct{})

	var once sync.Once

	var phase atomic.Uint32

	start := time.Now()

	win.RegisterHook(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) {

		switch phase.Add(1) {

		case 1:
			elapsed := time.Since(start).Milliseconds()
			slog.Debug("youtube preload: placeholder navigation completed", "elapsed_ms", elapsed)
			win.SetURL(config.YouTubeURL)

		case 2:
			elapsed := time.Since(start).Milliseconds()
			slog.Debug("youtube preload: youtube navigation completed", "elapsed_ms", elapsed)
			once.Do(func() { close(ready) })

		default:

		}

	})



	return ready

}

