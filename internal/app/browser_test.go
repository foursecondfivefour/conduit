package app

import (
	"strings"
	"testing"
)

func TestChromiumArgs_debugAddsRemotePort(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", true)
	if !chromiumArgsContain(args, "--remote-debugging-port=9222") {
		t.Fatalf("expected remote debugging port in debug args: %v", args)
	}
}

func TestChromiumArgs_releaseOmitsRemotePort(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", false)
	if chromiumArgsContain(args, "--remote-debugging-port=9222") {
		t.Fatalf("remote debugging port must not be set without debug: %v", args)
	}
}

func TestChromiumArgs_memoryTuning(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", false)
	for _, want := range []string{
		"--renderer-process-limit=2",
		"--disk-cache-size=8388608",
		"--media-cache-size=4194304",
		"--disable-background-mode",
		`--js-flags=--max-old-space-size=256`,
	} {
		if !chromiumArgsContain(args, want) {
			t.Fatalf("missing %q in args: %v", want, args)
		}
	}
}

func TestChromiumArgs_noDisableQUIC(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", false)
	if chromiumArgsContain(args, "--disable-quic") {
		t.Fatal("--disable-quic should not be present")
	}
}

func TestChromiumArgs_hasUserAgent(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", false)
	found := false
	for _, a := range args {
		if strings.HasPrefix(a, "--user-agent=") {
			found = true
			if !strings.Contains(a, "Chrome/131") {
				t.Fatalf("unexpected user agent: %s", a)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected --user-agent flag")
	}
}

func TestChromiumArgs_hasProxyServer(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", false)
	found := false
	for _, a := range args {
		if a == "--proxy-server=http://127.0.0.1:31284" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected --proxy-server flag")
	}
}

func TestChromiumArgs_hasProxyBypassLoopback(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", false)
	if !chromiumArgsContain(args, "--proxy-bypass-list=<-loopback>") {
		t.Fatal("expected --proxy-bypass-list=<-loopback>")
	}
}

func TestChromiumArgs_backgroundAudioExtraArgs(t *testing.T) {
	BackgroundAudioEnabled = true
	defer func() { BackgroundAudioEnabled = false }()

	args := chromiumArgs("http://127.0.0.1:31284", false)
	for _, want := range []string{
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
	} {
		if !chromiumArgsContain(args, want) {
			t.Fatalf("missing %q when BackgroundAudioEnabled", want)
		}
	}
}

func TestChromiumArgs_backgroundAudioDisabled(t *testing.T) {
	BackgroundAudioEnabled = false
	args := chromiumArgs("http://127.0.0.1:31284", false)
	if chromiumArgsContain(args, "--disable-background-timer-throttling") {
		t.Fatal("background audio args should not be present when disabled")
	}
}

func TestWindowsWebViewOptions_usesProfileDir(t *testing.T) {
	opts := windowsWebViewOptions(`C:\Conduit\webview2`, "http://127.0.0.1:31284", false)
	if opts.WebviewUserDataPath != `C:\Conduit\webview2` {
		t.Fatalf("unexpected profile path: %q", opts.WebviewUserDataPath)
	}
	if !opts.DisableQuitOnLastWindowClosed {
		t.Fatal("expected tray-friendly quit behaviour")
	}
	if len(opts.DisabledFeatures) == 0 {
		t.Fatal("expected disabled WebView2 features")
	}
	foundProxy := false
	for _, a := range opts.AdditionalBrowserArgs {
		if strings.HasPrefix(a, "--proxy-server=") {
			foundProxy = true
		}
	}
	if !foundProxy {
		t.Fatalf("expected proxy arg in browser args: %v", opts.AdditionalBrowserArgs)
	}
}
