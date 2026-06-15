package app

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// disabledChromiumFeatures are turned off via WebView2 --disable-features (Wails WindowsOptions.DisabledFeatures).
// See https://v3.wails.io/features/windows/options and Microsoft WebView2 browser flags docs.
var disabledChromiumFeatures = []string{
	"AutofillServerCommunication",
	"BackForwardCache",
	"HeavyAdIntervention",
	"InterestFeedContentSuggestions",
	"MediaRouter",
	"NotificationTriggers",
	"OptimizationHints",
	"PushMessaging",
	"Translate",
	"WebOTP",
	"WebPayments",
}

// BackgroundAudioEnabled controls whether Chromium keeps rendering/audio
// when the window is minimized or occluded. Set before application.New().
var BackgroundAudioEnabled bool

// windowsWebViewOptions configures the shared WebView2 environment for all windows.
// Flags apply globally (one browser process); keep ephemeral HTML shells out of extra WebViews when possible.
func windowsWebViewOptions(webDataDir, proxyURL string, debug bool) application.WindowsOptions {
	return application.WindowsOptions{
		WebviewUserDataPath:           webDataDir,
		DisableQuitOnLastWindowClosed: true,
		DisabledFeatures:              disabledChromiumFeatures,
		AdditionalBrowserArgs:         chromiumArgs(proxyURL, debug),
	}
}

// applyLightweightWebviewDefaults trims per-window WebView2 overhead for local HTML shells.
func applyLightweightWebviewDefaults(opts *application.WebviewWindowOptions) {
	if opts == nil {
		return
	}
	opts.EnableFileDrop = false
	opts.ZoomControlEnabled = false
	opts.DefaultContextMenuDisabled = true
}

// chromiumArgs returns Chromium flags tuned for lower RAM use while keeping YouTube playback working.
// Video needs a separate renderer/GPU pipeline — do not set renderer-process-limit below 2.
func chromiumArgs(proxyURL string, debug bool) []string {
	args := []string{
		"--proxy-server=" + proxyURL,
		"--proxy-bypass-list=<-loopback>",
		"--disable-extensions",
		"--disable-sync",
		"--disable-background-mode",
		"--disable-default-apps",
		"--disable-component-update",
		"--disable-domain-reliability",
		"--disable-client-side-phishing-detection",
		"--disable-breakpad",
		"--disable-notifications",
		"--disable-speech-api",
		"--disable-print-preview",
		"--disable-session-crashed-bubble",
		"--disable-translate",
		"--no-first-run",
		"--no-default-browser-check",
		"--metrics-recording-only",
		"--disk-cache-size=8388608",
		"--media-cache-size=4194304",
		"--renderer-process-limit=2",
		`--js-flags=--max-old-space-size=256`,
		"--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0",
	}
	if debug {
		args = append(args, "--remote-debugging-port=9222")
	}
	if BackgroundAudioEnabled {
		args = append(args,
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
		)
	}
	return args
}

func chromiumArgsContain(args []string, prefix string) bool {
	for _, a := range args {
		if a == prefix || strings.HasPrefix(a, prefix+"=") {
			return true
		}
	}
	return false
}
