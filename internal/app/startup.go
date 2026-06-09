package app



import (

	"encoding/base64"

	"log/slog"

	"strings"

	"sync"

	"time"



	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/wailsapp/wails/v3/pkg/events"



	"github.com/foursecondfivefour/conduit/internal/app/ui"

	"github.com/foursecondfivefour/conduit/internal/config"

	"github.com/foursecondfivefour/conduit/internal/i18n"

)



const (

	eventOnboardingFinish = "onboarding:finish"

	eventOnboardingSkip   = "onboarding:skip"

)



type startupFlow struct {

	app          *application.App

	youtube      *application.WebviewWindow

	prefs        *preferenceStore

	youtubeReady <-chan struct{}

	ui           UIOptions

}



func newStartupFlow(app *application.App, youtube *application.WebviewWindow, prefs *preferenceStore, youtubeReady <-chan struct{}, ui UIOptions) *startupFlow {

	return &startupFlow{

		app:          app,

		youtube:      youtube,

		prefs:        prefs,

		youtubeReady: youtubeReady,

		ui:           ui,

	}

}



func (f *startupFlow) begin() {

	lang := i18n.ParseLang(f.prefs.Get().Language)

	start := time.Now()



	splash := f.app.Window.NewWithOptions(application.WebviewWindowOptions{

		Name:             "splash",

		Title:            "Conduit",

		Width:            400,

		Height:           360,

		Frameless:        true,

		DisableResize:    true,

		AlwaysOnTop:      true,

		BackgroundColour: application.NewRGB(13, 17, 23),

		HTML:             renderHTML(lang, ui.SplashHTML),

		Windows: application.WindowsWindow{

			HiddenOnTaskbar: true,

		},

		DefaultContextMenuDisabled: true,

		DevToolsEnabled:            f.ui.devToolsEnabled(),

	})



	splash.RegisterHook(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) {

		slog.Debug("splash navigation completed", "elapsed_ms", time.Since(start).Milliseconds())

	})



	splash.Center()

	splash.Show()



	go func() {

		minSplash := time.NewTimer(config.SplashDuration)

		defer minSplash.Stop()



		select {

		case <-minSplash.C:
			slog.Debug("splash minimum duration elapsed", "elapsed_ms", time.Since(start).Milliseconds())

		case <-f.app.Context().Done():

			return

		}



		if !f.prefs.NeedsOnboarding() {

			waitStart := time.Now()

			select {

			case <-f.youtubeReady:
				slog.Debug("youtube ready before show", "wait_ms", time.Since(waitStart).Milliseconds(), "total_ms", time.Since(start).Milliseconds())

			case <-time.After(20 * time.Second):
				slog.Debug("youtube ready timeout", "wait_ms", time.Since(waitStart).Milliseconds())

			case <-f.app.Context().Done():

				return

			}

		}



		slog.Debug("closing splash", "elapsed_ms", time.Since(start).Milliseconds())

		splash.Close()



		if f.prefs.NeedsOnboarding() {

			f.openOnboarding(false)

			return

		}

		f.showYouTube()

	}()

}



func (f *startupFlow) showOnboardingFromTray() {

	f.openOnboarding(true)

}



func (f *startupFlow) openOnboarding(fromTray bool) {

	lang := i18n.ParseLang(f.prefs.Get().Language)



	win := f.app.Window.NewWithOptions(application.WebviewWindowOptions{

		Name:                       "onboarding",

		Title:                      i18n.T(lang, "onboarding.title"),

		Width:                      520,

		Height:                     560,

		MinWidth:                   480,

		MinHeight:                  480,

		Frameless:                  true,

		AlwaysOnTop:                true,

		BackgroundColour:           application.NewRGB(15, 20, 29),

		HTML:                       renderHTML(lang, ui.OnboardingHTML),

		AllowSimpleEventEmit:       true,

		DefaultContextMenuDisabled: true,

		DevToolsEnabled:            f.ui.devToolsEnabled(),

	})



	var once sync.Once

	var removeFinish func()

	var removeSkip func()



	end := func(markComplete bool) {

		once.Do(func() {

			if markComplete {

				_ = f.prefs.CompleteOnboarding()

			}

			win.Close()

			if removeFinish != nil {

				removeFinish()

			}

			if removeSkip != nil {

				removeSkip()

			}

			if !fromTray {

				f.showYouTube()

			}

		})

	}



	removeFinish = f.app.Event.On(eventOnboardingFinish, func(*application.CustomEvent) {

		slog.Debug("onboarding finished")

		end(true)

	})

	removeSkip = f.app.Event.On(eventOnboardingSkip, func(*application.CustomEvent) {

		slog.Debug("onboarding skipped", "from_tray", fromTray)

		end(!fromTray)

	})



	win.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {

		slog.Debug("onboarding window closing", "from_tray", fromTray)

		end(!fromTray)

	})



	win.RegisterHook(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) {

		slog.Debug("onboarding navigation completed")

	})



	win.Center()

	win.Show()

}



func (f *startupFlow) showYouTube() {
	slog.Debug("showing youtube window")

	f.youtube.Center()

	f.youtube.Show()

	// OpenInspectorOnStartup fails on hidden windows (WebView2: parameter incorrect).

	if f.ui.openInspectorOnStartup(true) {

		f.youtube.OpenDevTools()

	}

}



func renderHTML(lang i18n.Lang, template string) string {

	icon := base64.StdEncoding.EncodeToString(appIcon)

	html := strings.ReplaceAll(template, "{{ICON}}", icon)

	replacements := map[string]string{

		"{{I18N_SPLASH_TAGLINE}}":     i18n.T(lang, "splash.tagline"),

		"{{I18N_SPLASH_STATUS}}":      i18n.T(lang, "splash.status"),

		"{{I18N_LOADING_STATUS}}":     i18n.T(lang, "loading.status"),

		"{{I18N_ONBOARDING_TITLE}}":   i18n.T(lang, "onboarding.title"),

		"{{I18N_ONBOARDING_WELCOME}}": i18n.T(lang, "onboarding.welcome"),

	}

	for k, v := range replacements {

		html = strings.ReplaceAll(html, k, v)

	}

	return html

}



func renderLoadingHTML(lang i18n.Lang) string {

	return renderHTML(lang, ui.LoadingHTML)

}

