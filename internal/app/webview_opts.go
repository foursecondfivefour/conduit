package app

// UIOptions configures WebView debug behaviour from CLI flags.
type UIOptions struct {
	Debug          bool
	DebugInspector bool
}

func (o UIOptions) devToolsEnabled() bool {
	return o.Debug
}

func (o UIOptions) openInspectorOnStartup(forYouTube bool) bool {
	return o.Debug && o.DebugInspector && forYouTube
}
