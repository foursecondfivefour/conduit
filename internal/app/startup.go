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



// The onboarding window shares the same WebView2 environment as the YouTube window.
// Security note: this shared WebView2 context means cookies, localStorage, and other
// site data are shared between the two. The onboarding window loads ephemeral local HTML
// only (no external URLs), which mitigates cross-contamination risk. If the onboarding
// window were ever pointed to external content, it would share the YouTube session and
// bypass the proxy allowlist — keep it strictly local HTML.
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
	start := time.Now()
	needsOnboarding := f.prefs.NeedsOnboarding()

	go func() {
		minSplash := time.NewTimer(config.SplashDuration)
		defer minSplash.Stop()

		// Reuse the YouTube WebView loading page instead of a separate splash window (saves one WebView + renderer).
		if !needsOnboarding {
			f.youtube.Center()
			f.youtube.Show()
		}

		select {
		case <-minSplash.C:
			slog.Debug("startup minimum wait elapsed", "elapsed_ms", time.Since(start).Milliseconds())
		case <-f.app.Context().Done():
			return
		}

		if needsOnboarding {
			f.openOnboarding(false)
			return
		}

		waitStart := time.Now()
		select {
		case <-f.youtubeReady:
			slog.Debug("youtube ready before show", "wait_ms", time.Since(waitStart).Milliseconds(), "total_ms", time.Since(start).Milliseconds())
		case <-time.After(20 * time.Second):
			slog.Debug("youtube ready timeout", "wait_ms", time.Since(waitStart).Milliseconds())
		case <-f.app.Context().Done():
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



	onboardingOpts := application.WebviewWindowOptions{
		Name:                 "onboarding",
		Title:                i18n.T(lang, "onboarding.title"),
		Width:                520,
		Height:               560,
		MinWidth:             480,
		MinHeight:            480,
		Frameless:            true,
		AlwaysOnTop:          true,
		BackgroundColour:     application.NewRGB(15, 20, 29),
		HTML:                 renderHTML(lang, ui.OnboardingHTML),
		AllowSimpleEventEmit: true,
		DevToolsEnabled:      f.ui.devToolsEnabled(),
	}
	applyLightweightWebviewDefaults(&onboardingOpts)
	win := f.app.Window.NewWithOptions(onboardingOpts)



	var once sync.Once

	var removeFinish func()

	var removeSkip func()



	end := func(markComplete bool) {

		once.Do(func() {

			if markComplete {

				if err := f.prefs.CompleteOnboarding(); err != nil {
					slog.Warn("startup: complete onboarding failed", "err", err)
				}

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

		"{{I18N_ONBOARDING_SUBTITLE}}":     i18n.T(lang, "onboarding.subtitle"),

		"{{I18N_ONBOARDING_STEP1_TITLE}}":  i18n.T(lang, "onboarding.step1.title"),

		"{{I18N_ONBOARDING_STEP1_TEXT}}":   i18n.T(lang, "onboarding.step1.text"),

		"{{I18N_ONBOARDING_STEP2_TITLE}}":  i18n.T(lang, "onboarding.step2.title"),

		"{{I18N_ONBOARDING_STEP2_TEXT}}":   i18n.T(lang, "onboarding.step2.text"),

		"{{I18N_ONBOARDING_STEP3_TITLE}}":  i18n.T(lang, "onboarding.step3.title"),

		"{{I18N_ONBOARDING_STEP3_TEXT}}":   i18n.T(lang, "onboarding.step3.text"),

		"{{I18N_ONBOARDING_STEP4_TITLE}}":  i18n.T(lang, "onboarding.step4.title"),

		"{{I18N_ONBOARDING_STEP4_TEXT}}":   i18n.T(lang, "onboarding.step4.text"),

		"{{I18N_ONBOARDING_STEP5_TITLE}}":  i18n.T(lang, "onboarding.step5.title"),

		"{{I18N_ONBOARDING_STEP5_TEXT}}":   i18n.T(lang, "onboarding.step5.text"),

		"{{I18N_ONBOARDING_STEP6_TITLE}}":  i18n.T(lang, "onboarding.step6.title"),

		"{{I18N_ONBOARDING_STEP6_TEXT}}":   i18n.T(lang, "onboarding.step6.text"),

		"{{I18N_ONBOARDING_STEP7_TITLE}}":  i18n.T(lang, "onboarding.step7.title"),

		"{{I18N_ONBOARDING_STEP7_TEXT}}":   i18n.T(lang, "onboarding.step7.text"),

		"{{I18N_ONBOARDING_STEP8_TITLE}}":  i18n.T(lang, "onboarding.step8.title"),

		"{{I18N_ONBOARDING_STEP8_TEXT}}":   i18n.T(lang, "onboarding.step8.text"),

		"{{I18N_ONBOARDING_SKIP}}":         i18n.T(lang, "onboarding.skip"),

		"{{I18N_ONBOARDING_BACK}}":         i18n.T(lang, "onboarding.back"),

		"{{I18N_ONBOARDING_NEXT}}":         i18n.T(lang, "onboarding.next"),

		"{{I18N_ONBOARDING_FINISH}}":       i18n.T(lang, "onboarding.finish"),

	}

	for k, v := range replacements {

		html = strings.ReplaceAll(html, k, v)

	}

	return html

}



func renderLoadingHTML(lang i18n.Lang) string {

	return renderHTML(lang, ui.LoadingHTML)

}

