package app

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/i18n"
	filelog "github.com/foursecondfivefour/conduit/internal/log"
	"github.com/foursecondfivefour/conduit/internal/proxy"
	"github.com/foursecondfivefour/conduit/internal/update"
	"github.com/foursecondfivefour/conduit/internal/winproxy"
)

type trayDeps struct {
	app      *application.App
	youtube  *application.WebviewWindow
	proxy    *proxy.Server
	control  *ControlService
	prefs    *preferenceStore
	flow     *startupFlow
	updater  *update.Service
	winProxy *winproxy.Manager
	paths    RuntimePaths
}

type trayUI struct {
	deps trayDeps

	stopCh   chan struct{}
	stopOnce sync.Once
	doneCh   chan struct{}

	mu           sync.Mutex
	checkMu      sync.Mutex
	tray         *application.SystemTray
	menu         *application.Menu
	statusItem   *application.MenuItem
	updateStatus *application.MenuItem
	installItem  *application.MenuItem
	healthOK     bool
	healthKnown  bool
	strategyItems map[dpi.Strategy]*application.MenuItem
	dnsItems      map[dns.Provider]*application.MenuItem
	domainItems   map[proxy.AllowlistPreset]*application.MenuItem
}

func setupTray(deps trayDeps) *trayUI {
	ui := &trayUI{
		deps:          deps,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		strategyItems: make(map[dpi.Strategy]*application.MenuItem),
		dnsItems:      make(map[dns.Provider]*application.MenuItem),
		domainItems:   make(map[proxy.AllowlistPreset]*application.MenuItem),
	}
	ui.rebuildMenu()
	ui.deps.updater.OnChange(ui.onUpdateChange)

	tray := deps.app.SystemTray.New()
	ui.tray = tray
	tray.SetTooltip("Conduit")
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}
	tray.SetMenu(ui.menu)
	tray.OnClick(func() { tray.ShowMenu() })
	tray.Show()

	go ui.statusLoop()
	startUpdateScheduler(deps.prefs, deps.updater, ui)
	return ui
}

// stop ends background tray goroutines.
func (ui *trayUI) stop() {
	if ui == nil {
		return
	}
	ui.stopOnce.Do(func() {
		close(ui.stopCh)
		<-ui.doneCh
	})
}

func (ui *trayUI) lang() i18n.Lang {
	return i18n.ParseLang(ui.deps.control.Language())
}

func (ui *trayUI) rebuildMenu() {
	lang := ui.lang()
	st := ui.deps.control.GetStatus()
	p := ui.deps.prefs.Get()

	ui.statusItem = application.NewMenuItem(ui.statusLabel(st))
	ui.statusItem.SetEnabled(false)

	// DPI strategies
	var strategyItems []*application.MenuItem
	for _, strategy := range ui.deps.control.ListStrategies() {
		s := strategy
		item := application.NewMenuItemRadio(strategyMenuLabel(lang, s), ui.deps.control.Settings().Strategy == s)
		item.OnClick(func(*application.Context) {
			ui.deps.control.SetStrategy(s)
			ui.refreshMenu()
		})
		ui.strategyItems[s] = item
		strategyItems = append(strategyItems, item)
	}
	strategyMenu := menuFromItems(strategyItems)

	// DNS providers
	var dnsItems []*application.MenuItem
	for _, provider := range ui.deps.control.ListDoHProviders() {
		pr := provider
		item := application.NewMenuItemRadio(provider.ShortLabel(), ui.deps.control.Settings().DoHProvider == pr)
		item.OnClick(func(*application.Context) {
			ui.deps.control.SetDoHProvider(pr)
			ui.refreshMenu()
		})
		ui.dnsItems[pr] = item
		dnsItems = append(dnsItems, item)
	}
	flushDNS := application.NewMenuItem(i18n.T(lang, "tray.dns_flush")).OnClick(func(*application.Context) {
		_, _ = ui.deps.proxy.Restart(context.Background())
		ui.refreshStatus()
	})
	dnsSubItems := append(dnsItems, application.NewMenuItemSeparator(), flushDNS)
	dnsMenu := menuFromItems(dnsSubItems)

	// Domains
	var domainItems []*application.MenuItem
	for _, preset := range ui.deps.control.ListAllowlistPresets() {
		pr := preset
		label := domainPresetLabel(lang, pr)
		item := application.NewMenuItemRadio(label, proxy.AllowlistPreset(ui.deps.control.Settings().AllowlistPreset) == pr)
		item.OnClick(func(*application.Context) {
			ui.deps.control.SetAllowlistPreset(pr)
			ui.refreshMenu()
		})
		ui.domainItems[pr] = item
		domainItems = append(domainItems, item)
	}
	resetCustom := application.NewMenuItem(i18n.T(lang, "tray.domains.reset")).OnClick(func(*application.Context) {
		ui.deps.control.ResetCustomDomains()
		ui.refreshMenu()
	})
	domainMenu := menuFromItems(append(domainItems, application.NewMenuItemSeparator(), resetCustom))

	// Options
	autostartItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.autostart"), p.Autostart)
	autostartItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().Autostart
		if err := setAutostart(enabled); err == nil {
			ui.deps.prefs.Update(func(pref *Preferences) { pref.Autostart = enabled })
		}
		ui.refreshMenu()
	})
	sysProxyItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.system_proxy"), p.SystemProxy)
	sysProxyItem.OnClick(func(*application.Context) {
		ui.toggleSystemProxy()
	})
	minTrayItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.minimize_tray"), p.MinimizeToTray)
	minTrayItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().MinimizeToTray
		ui.deps.prefs.Update(func(pref *Preferences) { pref.MinimizeToTray = enabled })
		ui.refreshMenu()
	})
	optionsMenu := application.NewMenuFromItems(autostartItem, sysProxyItem, minTrayItem)

	// Updates
	ui.updateStatus = application.NewMenuItem(ui.updateStatusLabel())
	ui.updateStatus.SetEnabled(false)
	checkUpdate := application.NewMenuItem(i18n.T(lang, "tray.updates.check")).OnClick(func(*application.Context) {
		go ui.checkUpdates(true)
	})
	ui.installItem = application.NewMenuItem(i18n.T(lang, "tray.updates.install"))
	ui.installItem.SetEnabled(ui.deps.updater.State() == update.StateReady)
	ui.installItem.OnClick(func(*application.Context) { ui.installUpdate() })
	remindLater := application.NewMenuItem(i18n.T(lang, "tray.updates.later")).OnClick(func(*application.Context) {
		rel := ui.deps.updater.Latest()
		if rel.TagName != "" {
			ui.deps.prefs.Update(func(pref *Preferences) { pref.SkippedVersion = rel.TagName })
		}
		ui.refreshMenu()
	})
	autoUpdateItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.updates.auto"), p.AutoUpdate)
	autoUpdateItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().AutoUpdate
		ui.deps.prefs.Update(func(pref *Preferences) { pref.AutoUpdate = enabled })
		ui.refreshMenu()
	})
	openBrowser := application.NewMenuItem(i18n.T(lang, "tray.updates.browser")).OnClick(func(*application.Context) {
		rel := ui.deps.updater.Latest()
		if rel.HTMLURL != "" && update.ValidateReleaseURL(rel.HTMLURL) == nil {
			_ = openURL(rel.HTMLURL)
		}
	})
	updatesMenu := application.NewMenuFromItems(
		ui.updateStatus,
		application.NewMenuItemSeparator(),
		checkUpdate,
		ui.installItem,
		remindLater,
		autoUpdateItem,
		openBrowser,
	)

	// Language
	langRU := application.NewMenuItemRadio(i18n.T(lang, "tray.language.ru"), lang == i18n.LangRU)
	langRU.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangRU) })
	langEN := application.NewMenuItemRadio(i18n.T(lang, "tray.language.en"), lang == i18n.LangEN)
	langEN.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangEN) })
	langMenu := application.NewMenuFromItems(langRU, langEN)

	testConn := application.NewMenuItem(i18n.T(lang, "tray.connection_test")).OnClick(func(*application.Context) {
		go ui.runHealthCheck()
	})
	openSettings := application.NewMenuItem(i18n.T(lang, "tray.open_settings")).OnClick(func(*application.Context) {
		_ = openFolder(ui.deps.paths.ConfigDir)
	})
	openLog := application.NewMenuItem(i18n.T(lang, "tray.open_log")).OnClick(func(*application.Context) {
		_ = filelog.OpenLog(filelog.LogPath(ui.deps.paths.LogDir))
	})
	helpItem := application.NewMenuItem(i18n.T(lang, "tray.help")).OnClick(func(*application.Context) {
		if ui.deps.flow != nil {
			ui.deps.flow.showOnboardingFromTray()
		}
	})
	quitItem := application.NewMenuItem(i18n.T(lang, "tray.quit")).OnClick(func(*application.Context) {
		ui.stop()
		_ = ui.deps.winProxy.Disable()
		ui.deps.app.Quit()
	})

	ui.menu = application.NewMenuFromItems(
		ui.statusItem,
		application.NewMenuItemSeparator(),
		application.NewSubmenu(i18n.T(lang, "tray.strategy"), strategyMenu),
		application.NewSubmenu(i18n.T(lang, "tray.dns"), dnsMenu),
		application.NewSubmenu(i18n.T(lang, "tray.domains"), domainMenu),
		application.NewSubmenu(i18n.T(lang, "tray.options"), optionsMenu),
		testConn,
		application.NewSubmenu(i18n.T(lang, "tray.updates"), updatesMenu),
		application.NewSubmenu(i18n.T(lang, "tray.language"), langMenu),
		application.NewMenuItemSeparator(),
		openSettings,
		openLog,
		helpItem,
		application.NewMenuItemSeparator(),
		quitItem,
	)
	if ui.tray != nil {
		ui.tray.SetMenu(ui.menu)
	}
}

func (ui *trayUI) refreshMenu() {
	ui.rebuildMenu()
}

func (ui *trayUI) refreshStatus() {
	if ui.statusItem == nil || ui.deps.control == nil {
		return
	}
	ui.statusItem.SetLabel(ui.statusLabel(ui.deps.control.GetStatus()))
}

func (ui *trayUI) statusLoop() {
	defer close(ui.doneCh)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ui.stopCh:
			return
		case <-ticker.C:
			ui.refreshStatus()
		}
	}
}

func (ui *trayUI) statusLabel(st Status) string {
	lang := ui.lang()
	stateKey := "tray.status.stopped"
	if st.Running {
		stateKey = "tray.status.running"
	}
	healthKey := "tray.status.health.unknown"
	if ui.healthKnown {
		if ui.healthOK {
			healthKey = "tray.status.health.ok"
		} else {
			healthKey = "tray.status.health.fail"
		}
	}
	return i18n.Tf(lang, "tray.status",
		i18n.T(lang, stateKey),
		st.Port,
		st.Strategy,
		st.DoH,
		i18n.T(lang, healthKey),
	)
}

func (ui *trayUI) setLanguage(lang i18n.Lang) {
	ui.deps.control.SetLanguage(lang.String())
	ui.deps.prefs.Update(func(p *Preferences) { p.Language = lang.String() })
	ui.refreshMenu()
}

func (ui *trayUI) toggleSystemProxy() {
	enabled := !ui.deps.prefs.Get().SystemProxy
	if enabled {
		showSystemProxyWarning(ui.lang())
		port := ui.deps.proxy.Port()
		if err := ui.deps.winProxy.Enable(config.ListenHost, port); err != nil {
			return
		}
	} else {
		if err := ui.deps.winProxy.Disable(); err != nil {
			return
		}
	}
	ui.deps.prefs.Update(func(p *Preferences) { p.SystemProxy = enabled })
	ui.refreshMenu()
}

func (ui *trayUI) runHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	proxyURL := ui.deps.proxy.ProxyURL()
	err := CheckYouTube(ctx, proxyURL)
	ui.mu.Lock()
	ui.healthKnown = true
	ui.healthOK = err == nil
	ui.mu.Unlock()
	ui.deps.prefs.Update(func(p *Preferences) {
		p.LastHealthOK = err == nil
		p.LastHealthCheck = time.Now()
	})
	ui.refreshStatus()
	lang := ui.lang()
	if err != nil {
		slog.Warn("connection test failed", "err", err)
		if ui.tray != nil {
			ui.tray.SetTooltip(i18n.Tf(lang, "tray.connection.fail", err.Error()))
		}
	} else if ui.tray != nil {
		ui.tray.SetTooltip(i18n.T(lang, "tray.connection.ok"))
	}
}

func (ui *trayUI) checkUpdates(manual bool) {
	if !ui.checkMu.TryLock() {
		return
	}
	defer ui.checkMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	skipped := ui.deps.prefs.Get().SkippedVersion
	available, err := ui.deps.updater.Check(ctx, skipped)
	if err != nil {
		ui.refreshMenu()
		return
	}
	ui.deps.prefs.Update(func(p *Preferences) { p.LastUpdateCheck = time.Now() })
	if available && ui.deps.prefs.Get().AutoUpdate {
		_ = ui.deps.updater.Download(ctx)
	}
	if manual {
		ui.refreshMenu()
	}
}

func (ui *trayUI) installUpdate() {
	target, err := os.Executable()
	if err != nil {
		return
	}
	source := ui.deps.updater.DownloadPath()
	if source == "" {
		return
	}
	ui.stop()
	_ = ui.deps.winProxy.Disable()
	if err := ui.deps.updater.Apply(target, source, os.Getpid()); err != nil {
		return
	}
	ui.deps.app.Quit()
}

func (ui *trayUI) onUpdateChange() {
	ui.refreshMenu()
}

func (ui *trayUI) updateStatusLabel() string {
	lang := ui.lang()
	state := ui.deps.updater.State()
	switch state {
	case update.StateDownloading:
		return i18n.Tf(lang, "tray.updates.downloading", ui.deps.updater.Progress())
	case update.StateReady:
		return i18n.Tf(lang, "tray.updates.available", ui.deps.updater.Latest().Version())
	case update.StateAvailable:
		return i18n.Tf(lang, "tray.updates.available", ui.deps.updater.Latest().Version())
	case update.StateError:
		return i18n.T(lang, "tray.updates.error")
	default:
		return i18n.Tf(lang, "tray.updates.current", config.Version)
	}
}

func strategyMenuLabel(lang i18n.Lang, s dpi.Strategy) string {
	key := map[dpi.Strategy]string{
		dpi.StrategyNone:             "strategy.none",
		dpi.StrategyTCPSegmentation:  "strategy.tcp_seg",
		dpi.StrategyTCPSegmentation1: "strategy.tcp_1",
		dpi.StrategyTCPSegmentation8: "strategy.tcp_8",
		dpi.StrategyTLSRecordFrag:    "strategy.tls_frag",
		dpi.StrategyTLSRecordFrag2:   "strategy.tls_frag_2",
	}[s]
	if key == "" {
		return s.String()
	}
	return i18n.T(lang, key)
}

func domainPresetLabel(lang i18n.Lang, p proxy.AllowlistPreset) string {
	switch p {
	case proxy.PresetYouTube:
		return i18n.T(lang, "tray.domains.youtube")
	case proxy.PresetCustom:
		return i18n.T(lang, "tray.domains.custom")
	default:
		return i18n.T(lang, "tray.domains.google")
	}
}

func menuFromItems(items []*application.MenuItem) *application.Menu {
	if len(items) == 0 {
		return application.NewMenu()
	}
	return application.NewMenuFromItems(items[0], items[1:]...)
}

func startUpdateScheduler(prefs *preferenceStore, svc *update.Service, ui *trayUI) {
	go func() {
		select {
		case <-time.After(config.UpdateCheckDelay):
			ui.checkUpdates(false)
		case <-ui.stopCh:
		}
	}()
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ui.stopCh:
				return
			case <-ticker.C:
				interval := prefs.UpdateCheckInterval()
				last := prefs.Get().LastUpdateCheck
				if time.Since(last) >= interval {
					ui.checkUpdates(false)
				}
			}
		}
	}()
}
