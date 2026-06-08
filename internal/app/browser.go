package app

// chromiumArgs returns WebView2 flags tuned for lower memory use.
func chromiumArgs(proxyURL string) []string {
	return []string{
		"--proxy-server=" + proxyURL,
		"--proxy-bypass-list=<-loopback>",
		"--disable-quic",
		"--disable-extensions",
		"--disable-sync",
		"--disable-background-networking",
		"--disable-default-apps",
		"--disable-component-update",
		"--disable-domain-reliability",
		"--disable-client-side-phishing-detection",
		"--disable-features=Translate,MediaRouter,OptimizationHints,AutofillServerCommunication,InterestFeedContentSuggestions",
		"--disk-cache-size=16777216",
		"--media-cache-size=1048576",
		"--renderer-process-limit=2",
	}
}
