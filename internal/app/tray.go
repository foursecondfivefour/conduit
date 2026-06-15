package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	appui "github.com/foursecondfivefour/conduit/internal/app/ui"
	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/i18n"
	filelog "github.com/foursecondfivefour/conduit/internal/log"
	"github.com/foursecondfivefour/conduit/internal/proxy"
	"github.com/foursecondfivefour/conduit/internal/tun"
	"github.com/foursecondfivefour/conduit/internal/update"
	"github.com/foursecondfivefour/conduit/internal/winproxy"
	yt "github.com/foursecondfivefour/conduit/internal/youtube"
)

type trayDeps struct {
	app          *application.App
	youtube      *application.WebviewWindow
	proxy        *proxy.Server
	socks5       *proxy.SOCKS5Server
	stats        *proxy.TrafficStats
	control      *ControlService
	prefs        *preferenceStore
	flow         *startupFlow
	updater      *update.Service
	winProxy     *winproxy.Manager
	paths        RuntimePaths
	ui           UIOptions
	pip          *PiPWindow
	adBlocker    *dns.AdBlocker
	tunMode      *tun.TUNMode
	masque       *proxy.MASQUEServer
	splitTunnelPort int
}

type trayUI struct {
	deps trayDeps

	stopCh   chan struct{}
	stopOnce sync.Once
	doneCh   chan struct{}

	mu              sync.Mutex
	checkMu         sync.Mutex
	speedTestMu     sync.Mutex
	rebuildScheduled atomic.Bool
	tray            *application.SystemTray
	menu            *application.Menu
	statusItem      *application.MenuItem
	updateStatus    *application.MenuItem
	installItem     *application.MenuItem
	healthOK        bool
	healthKnown     bool
	previousRunning bool
	strategyItems     map[dpi.Strategy]*application.MenuItem
	dnsItems          map[dns.Provider]*application.MenuItem
	domainItems       map[proxy.AllowlistPreset]*application.MenuItem
	promptedUpdateVer string
	statsWindow       *application.WebviewWindow
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
	raw := ui.deps.control.Language()
	return i18n.ParseLang(raw)
}

func (ui *trayUI) rebuildMenu() {
	lang := ui.lang()
	st := ui.deps.control.GetStatus()
	p := ui.deps.prefs.Get()

	ui.statusItem = application.NewMenuItem(ui.statusLabel(st))
	ui.statusItem.SetEnabled(false)

	// DPI strategies
	strategyItems := make([]*application.MenuItem, 0, len(ui.deps.control.ListStrategies()))
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
	dnsItems := make([]*application.MenuItem, 0, len(ui.deps.control.ListDoHProviders()))
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
		if _, err := ui.deps.proxy.Restart(context.Background()); err != nil {
			slog.Warn("tray: dns flush restart failed", "err", err)
		}
		ui.refreshStatus()
	})
	dnsLeakItem := application.NewMenuItem(i18n.T(lang, "tray.dns_leak_check")).OnClick(func(*application.Context) {
		go ui.runDNSLeakCheck()
	})
	dnsBlockItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.dns_block"), ui.deps.control.IsSystemDNSBlocked())
	dnsBlockItem.OnClick(func(*application.Context) {
		ui.toggleDNSBlock()
	})
	dnsSubItems := append(dnsItems, application.NewMenuItemSeparator(), flushDNS, dnsLeakItem, dnsBlockItem)

	// Feature 4.5: Auto DNS Selection
	autoDNSItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.dns_auto"), p.AutoDNS)
	autoDNSItem.OnClick(func(*application.Context) {
		ui.deps.prefs.Update(func(pref *Preferences) { pref.AutoDNS = !pref.AutoDNS })
		ui.refreshMenu()
	})
	autoDNSSelectItem := application.NewMenuItem(i18n.T(lang, "tray.dns_auto.selecting")).OnClick(func(*application.Context) {
		go ui.runAutoDNSSelect()
	})
	dnsSubItems = append(dnsSubItems, application.NewMenuItemSeparator(), autoDNSItem, autoDNSSelectItem)

	dnsAdBlockItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.dns_adblock"), p.DNSAdBlock)
	dnsAdBlockItem.OnClick(func(*application.Context) {
		ui.toggleDNSAdBlock()
	})
	dnsSubItems = append(dnsSubItems, dnsAdBlockItem)
	dnsMenu := menuFromItems(dnsSubItems)

	// Domains
	domainItems := make([]*application.MenuItem, 0, len(ui.deps.control.ListAllowlistPresets()))
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
	killSwitchItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.kill_switch"), p.KillSwitch)
	killSwitchItem.SetTooltip(i18n.T(lang, "tray.kill_switch.tooltip"))
	killSwitchItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().KillSwitch
		if err := ui.deps.control.ToggleKillSwitch(enabled); err != nil {
			slog.Warn("kill switch toggle failed", "err", err)
			return
		}
		ui.deps.prefs.Update(func(pref *Preferences) { pref.KillSwitch = enabled })
		ui.refreshMenu()
	})
	socks5Item := application.NewMenuItemCheckbox(i18n.T(lang, "tray.socks5"), p.SOCKS5)
	socks5Item.OnClick(func(*application.Context) {
		ui.deps.prefs.Update(func(pref *Preferences) { pref.SOCKS5 = !pref.SOCKS5 })
		ui.refreshMenu()
	})

	// Feature 4.1: Split Tunnel
	splitTunnelItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.split_tunnel"), p.SplitTunnel)
	splitTunnelItem.OnClick(func(*application.Context) {
		ui.deps.prefs.Update(func(pref *Preferences) { pref.SplitTunnel = !pref.SplitTunnel })
		ui.refreshMenu()
	})

	// Feature 4.6: Auto-connect
	autoConnectItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.auto_connect"), p.AutoConnect)
	autoConnectItem.OnClick(func(*application.Context) {
		ui.deps.prefs.Update(func(pref *Preferences) { pref.AutoConnect = !pref.AutoConnect })
		ui.refreshMenu()
	})

	// Feature 4.7: TUN Mode
	tunModeItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.tun_mode"), p.TUNMode)
	tunModeItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().TUNMode
		if enabled {
			showTUNWarning(ui.lang())
		}
		if ui.deps.tunMode != nil {
			if enabled {
				exePath, _ := os.Executable()
				if err := ui.deps.tunMode.Enable(exePath); err != nil {
					slog.Warn("tray: tun mode enable failed", "err", err)
				}
			} else {
				if err := ui.deps.tunMode.Disable(); err != nil {
					slog.Warn("tray: tun mode disable failed", "err", err)
				}
			}
		}
		ui.deps.prefs.Update(func(pref *Preferences) { pref.TUNMode = enabled })
		ui.refreshMenu()
	})

	// Feature 4.2: Upstream Proxy submenu
	upstreamNone := application.NewMenuItemRadio(i18n.T(lang, "tray.upstream.none"), p.UpstreamHost == "" || p.UpstreamPort == 0)
	upstreamNone.OnClick(func(*application.Context) {
		ui.deps.prefs.Update(func(pref *Preferences) {
			pref.UpstreamType = ""
			pref.UpstreamHost = ""
			pref.UpstreamPort = 0
			pref.UpstreamUser = ""
			pref.UpstreamPassword = ""
		})
		ui.refreshMenu()
	})
	upstreamConfigured := application.NewMenuItemRadio(
		i18n.Tf(lang, "tray.upstream.configured", p.UpstreamHost, p.UpstreamPort),
		p.UpstreamHost != "" && p.UpstreamPort > 0,
	)
	upstreamConfigured.SetEnabled(false)
	upstreamMenu := menuFromItems([]*application.MenuItem{upstreamNone, upstreamConfigured})

	// Feature 5.2: MASQUE (HTTP/3)
	masqueItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.masque"), p.MASQUE)
	masqueItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().MASQUE
		if enabled {
			if ui.deps.masque != nil {
				if err := ui.deps.masque.Start(context.Background(), config.ListenHost, config.DefaultMASQUEPort); err != nil {
					slog.Warn("tray: masque start failed", "err", err)
				}
			}
		} else {
			if ui.deps.masque != nil {
				if err := ui.deps.masque.Stop(); err != nil {
					slog.Warn("tray: masque stop failed", "err", err)
				}
			}
		}
		ui.deps.prefs.Update(func(pref *Preferences) { pref.MASQUE = enabled })
		ui.refreshMenu()
	})

	// Feature 5.3: AI DPI
	aiDPIItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.ai_dpi"), p.AIDPI)
	aiDPIItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().AIDPI
		ui.deps.prefs.Update(func(pref *Preferences) { pref.AIDPI = enabled })
		if ui.deps.proxy != nil {
			st := ui.deps.control.Settings()
			if enabled {
				aiSelector := dpi.NewAIStrategySelector(st.Strategy)
				ui.deps.proxy.SetAISelector(aiSelector)
			} else {
				ui.deps.proxy.SetAISelector(nil)
			}
		}
		ui.refreshMenu()
	})

	optionsMenu := application.NewMenuFromItems(
		autostartItem, sysProxyItem, minTrayItem, killSwitchItem, socks5Item,
		splitTunnelItem, autoConnectItem, tunModeItem, masqueItem, aiDPIItem,
	)

	// YouTube features
	sponsorBlockItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.sponsorblock"), p.SponsorBlock)
	sponsorBlockItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().SponsorBlock
		ui.deps.prefs.Update(func(pref *Preferences) { pref.SponsorBlock = enabled })
		if enabled {
			ui.deps.youtube.ExecJS(yt.SponsorBlockJS())
		}
		ui.refreshMenu()
	})

	pipItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.pip"), ui.deps.pip.IsActive())
	pipItem.OnClick(func(*application.Context) {
		ui.deps.pip.Toggle()
		ui.refreshMenu()
	})

	rydItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.ryd"), p.RYD)
	rydItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().RYD
		ui.deps.prefs.Update(func(pref *Preferences) { pref.RYD = enabled })
		if enabled {
			ui.deps.youtube.ExecJS(yt.RYDInjectJS())
		}
		ui.refreshMenu()
	})

	bgAudioItem := application.NewMenuItemCheckbox(i18n.T(lang, "tray.background_audio"), p.BackgroundAudio)
	bgAudioItem.OnClick(func(*application.Context) {
		enabled := !ui.deps.prefs.Get().BackgroundAudio
		ui.deps.prefs.Update(func(pref *Preferences) { pref.BackgroundAudio = enabled })
		ui.refreshMenu()
	})

	youtubeFeaturesMenu := application.NewMenuFromItems(sponsorBlockItem, pipItem, rydItem, bgAudioItem)

	// Statistics
	statsItem := application.NewMenuItem(i18n.T(lang, "tray.stats")).OnClick(func(*application.Context) {
		ui.openStatsWindow()
	})

	// Feature 4.4: Speed Test
	speedTestItem := application.NewMenuItem(i18n.T(lang, "tray.speedtest")).OnClick(func(*application.Context) {
		go ui.runSpeedTest()
	})

	// Feature 4.3: Copy PAC URL
	pacURLItem := application.NewMenuItem(i18n.T(lang, "tray.pac_url")).OnClick(func(*application.Context) {
		port := ui.deps.proxy.Port()
		pacURL := fmt.Sprintf("http://%s:%d/proxy.pac", config.ListenHost, port)
		copyToClipboard(pacURL)
		if ui.tray != nil {
			ui.tray.SetTooltip(i18n.T(lang, "tray.pac_copied"))
		}
	})

	// Export / Import settings
	desktop := desktopDir()
	exportPath := filepath.Join(desktop, "conduit-settings.json")
	exportItem := application.NewMenuItem(i18n.T(lang, "tray.export_settings")).OnClick(func(*application.Context) {
		timestamp := time.Now().Format("20060102")
		exportFile := filepath.Join(desktop, fmt.Sprintf("conduit-settings-%s.json", timestamp))
		if err := ui.deps.prefs.Export(exportFile); err != nil {
			slog.Warn("export settings failed", "err", err)
		} else {
			if ui.tray != nil {
				ui.tray.SetTooltip(i18n.T(lang, "tray.export.done"))
			}
		}
	})
	importItem := application.NewMenuItem(i18n.T(lang, "tray.import_settings")).OnClick(func(*application.Context) {
		if err := ui.deps.prefs.Import(exportPath); err != nil {
			slog.Warn("import settings failed", "err", err)
			if ui.tray != nil {
				ui.tray.SetTooltip(i18n.T(lang, "tray.import.error"))
			}
		} else {
			if ui.tray != nil {
				ui.tray.SetTooltip(i18n.T(lang, "tray.import.done"))
			}
		}
		ui.refreshMenu()
	})
	settingsMenu := application.NewMenuFromItems(exportItem, importItem)

	// Per-domain DPI
	perDomainItem := application.NewMenuItem(i18n.T(lang, "tray.per_domain_dpi.managed"))
	perDomainItem.SetEnabled(false)
	perDomainMenu := application.NewMenuFromItems(perDomainItem)

	// Multi-port
	portsItem := application.NewMenuItem(i18n.T(lang, "tray.ports.configure"))
	portsItem.SetEnabled(false)
	extraPorts := ui.deps.proxy.ExtraPorts()
	if len(extraPorts) > 0 {
		portStrs := make([]string, len(extraPorts))
		for i, p := range extraPorts {
			portStrs[i] = fmt.Sprintf(":%d", p)
		}
		portsItem.SetLabel(fmt.Sprintf("%s: %s", i18n.T(lang, "tray.ports"), strings.Join(portStrs, ", ")))
	}
	portsMenu := application.NewMenuFromItems(portsItem)

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
			if err := openURL(rel.HTMLURL); err != nil {
				slog.Warn("tray: open release URL failed", "err", err, "url", rel.HTMLURL)
			}
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
	langDE := application.NewMenuItemRadio(i18n.T(lang, "tray.language.de"), lang == i18n.LangDE)
	langDE.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangDE) })
	langFR := application.NewMenuItemRadio(i18n.T(lang, "tray.language.fr"), lang == i18n.LangFR)
	langFR.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangFR) })
	langES := application.NewMenuItemRadio(i18n.T(lang, "tray.language.es"), lang == i18n.LangES)
	langES.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangES) })
	langTR := application.NewMenuItemRadio(i18n.T(lang, "tray.language.tr"), lang == i18n.LangTR)
	langTR.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangTR) })
	langFA := application.NewMenuItemRadio(i18n.T(lang, "tray.language.fa"), lang == i18n.LangFA)
	langFA.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangFA) })
	langTT := application.NewMenuItemRadio(i18n.T(lang, "tray.language.tt"), lang == i18n.LangTT)
	langTT.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangTT) })
	langBA := application.NewMenuItemRadio(i18n.T(lang, "tray.language.ba"), lang == i18n.LangBA)
	langBA.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangBA) })
	langCE := application.NewMenuItemRadio(i18n.T(lang, "tray.language.ce"), lang == i18n.LangCE)
	langCE.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangCE) })
	langCV := application.NewMenuItemRadio(i18n.T(lang, "tray.language.cv"), lang == i18n.LangCV)
	langCV.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangCV) })
	langSAH := application.NewMenuItemRadio(i18n.T(lang, "tray.language.sah"), lang == i18n.LangSAH)
	langSAH.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangSAH) })
	langBUA := application.NewMenuItemRadio(i18n.T(lang, "tray.language.bua"), lang == i18n.LangBUA)
	langBUA.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangBUA) })
	langOS := application.NewMenuItemRadio(i18n.T(lang, "tray.language.os"), lang == i18n.LangOS)
	langOS.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangOS) })
	langAV := application.NewMenuItemRadio(i18n.T(lang, "tray.language.av"), lang == i18n.LangAV)
	langAV.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangAV) })
	langMHR := application.NewMenuItemRadio(i18n.T(lang, "tray.language.mhr"), lang == i18n.LangMHR)
	langMHR.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangMHR) })
	langUDM := application.NewMenuItemRadio(i18n.T(lang, "tray.language.udm"), lang == i18n.LangUDM)
	langUDM.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangUDM) })
	langKV := application.NewMenuItemRadio(i18n.T(lang, "tray.language.kv"), lang == i18n.LangKV)
	langKV.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangKV) })
	langMYV := application.NewMenuItemRadio(i18n.T(lang, "tray.language.myv"), lang == i18n.LangMYV)
	langMYV.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangMYV) })
	langKBD := application.NewMenuItemRadio(i18n.T(lang, "tray.language.kbd"), lang == i18n.LangKBD)
	langKBD.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangKBD) })
	langTYV := application.NewMenuItemRadio(i18n.T(lang, "tray.language.tyv"), lang == i18n.LangTYV)
	langTYV.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangTYV) })
	langXAL := application.NewMenuItemRadio(i18n.T(lang, "tray.language.xal"), lang == i18n.LangXAL)
	langXAL.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangXAL) })
	langUK := application.NewMenuItemRadio(i18n.T(lang, "tray.language.uk"), lang == i18n.LangUK)
	langUK.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangUK) })
	langBE := application.NewMenuItemRadio(i18n.T(lang, "tray.language.be"), lang == i18n.LangBE)
	langBE.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangBE) })
	langKK := application.NewMenuItemRadio(i18n.T(lang, "tray.language.kk"), lang == i18n.LangKK)
	langKK.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangKK) })
	langUZ := application.NewMenuItemRadio(i18n.T(lang, "tray.language.uz"), lang == i18n.LangUZ)
	langUZ.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangUZ) })
	langHY := application.NewMenuItemRadio(i18n.T(lang, "tray.language.hy"), lang == i18n.LangHY)
	langHY.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangHY) })
	langAZ := application.NewMenuItemRadio(i18n.T(lang, "tray.language.az"), lang == i18n.LangAZ)
	langAZ.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangAZ) })
	langKA := application.NewMenuItemRadio(i18n.T(lang, "tray.language.ka"), lang == i18n.LangKA)
	langKA.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangKA) })
	langRO := application.NewMenuItemRadio(i18n.T(lang, "tray.language.ro"), lang == i18n.LangRO)
	langRO.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangRO) })
	langKY := application.NewMenuItemRadio(i18n.T(lang, "tray.language.ky"), lang == i18n.LangKY)
	langKY.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangKY) })
	langTG := application.NewMenuItemRadio(i18n.T(lang, "tray.language.tg"), lang == i18n.LangTG)
	langTG.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangTG) })
	langTK := application.NewMenuItemRadio(i18n.T(lang, "tray.language.tk"), lang == i18n.LangTK)
	langTK.OnClick(func(*application.Context) { ui.setLanguage(i18n.LangTK) })
	langMenu := application.NewMenuFromItems(langRU, langEN, langDE, langFR, langES, langTR, langFA,
		langTT, langBA, langCE, langCV, langSAH, langBUA, langOS, langAV, langMHR, langUDM, langKV, langMYV, langKBD, langTYV, langXAL,
		langUK, langBE, langKK, langUZ, langHY, langAZ, langKA, langRO, langKY, langTG, langTK)

	testConn := application.NewMenuItem(i18n.T(lang, "tray.connection_test")).OnClick(func(*application.Context) {
		go ui.runHealthCheck()
	})
	openSettings := application.NewMenuItem(i18n.T(lang, "tray.open_settings")).OnClick(func(*application.Context) {
		dir := ui.deps.paths.ConfigDir
		if err := os.MkdirAll(dir, 0o700); err != nil {
			slog.Warn("open settings: mkdir failed", "err", err)
		}
		if err := openFolder(dir); err != nil {
			slog.Warn("open settings: explorer failed", "err", err, "dir", dir)
		}
	})
	openLog := application.NewMenuItem(i18n.T(lang, "tray.open_log")).OnClick(func(*application.Context) {
		logDir := ui.deps.paths.LogDir
		logFile := filelog.LogPath(logDir)
		if err := os.MkdirAll(logDir, 0o700); err != nil {
			slog.Warn("open log: mkdir failed", "err", err)
		}
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			if f, err := os.Create(logFile); err == nil {
				if err := f.Close(); err != nil {
					slog.Warn("tray: close log file failed", "err", err)
				}
			}
		}
		if err := filelog.OpenLog(logFile); err != nil {
			slog.Warn("open log: shell failed", "err", err, "path", logFile)
		}
	})
	helpItem := application.NewMenuItem(i18n.T(lang, "tray.help")).OnClick(func(*application.Context) {
		if ui.deps.flow != nil {
			ui.deps.flow.showOnboardingFromTray()
		}
	})
	quitItem := application.NewMenuItem(i18n.T(lang, "tray.quit")).OnClick(func(*application.Context) {
		ui.stop()
		if err := ui.deps.winProxy.Disable(); err != nil {
			slog.Warn("tray: winproxy disable on quit failed", "err", err)
		}
		ui.deps.app.Quit()
	})

	items := []*application.MenuItem{
		ui.statusItem,
		application.NewMenuItemSeparator(),
		application.NewSubmenu(i18n.T(lang, "tray.strategy"), strategyMenu),
		application.NewSubmenu(i18n.T(lang, "tray.dns"), dnsMenu),
		application.NewSubmenu(i18n.T(lang, "tray.domains"), domainMenu),
		application.NewSubmenu(i18n.T(lang, "tray.options"), optionsMenu),
		application.NewSubmenu(i18n.T(lang, "tray.upstream"), upstreamMenu),
		application.NewSubmenu(i18n.T(lang, "tray.youtube_features"), youtubeFeaturesMenu),
		statsItem,
		speedTestItem,
		pacURLItem,
		application.NewSubmenu(i18n.T(lang, "tray.settings_io"), settingsMenu),
		application.NewSubmenu(i18n.T(lang, "tray.per_domain_dpi"), perDomainMenu),
		application.NewSubmenu(i18n.T(lang, "tray.ports"), portsMenu),
		testConn,
		application.NewSubmenu(i18n.T(lang, "tray.updates"), updatesMenu),
		application.NewSubmenu(i18n.T(lang, "tray.language"), langMenu),
		application.NewMenuItemSeparator(),
		openSettings,
		openLog,
		helpItem,
	}
	if ui.deps.ui.Debug {
		openDevTools := application.NewMenuItem(i18n.T(lang, "tray.devtools")).OnClick(func(*application.Context) {
			if ui.deps.youtube != nil {
				ui.deps.youtube.OpenDevTools()
			}
		})
		reloadYouTube := application.NewMenuItem(i18n.T(lang, "tray.reload_youtube")).OnClick(func(*application.Context) {
			if ui.deps.youtube != nil {
				ui.deps.youtube.Reload()
			}
		})
		items = append(items,
			application.NewMenuItemSeparator(),
			openDevTools,
			reloadYouTube,
		)
	}
	items = append(items,
		application.NewMenuItemSeparator(),
		quitItem,
	)
	ui.menu = application.NewMenuFromItems(items[0], items[1:]...)
	if ui.tray != nil {
		ui.tray.SetMenu(ui.menu)
	}
}

func (ui *trayUI) refreshMenu() {
	if ui.rebuildScheduled.Load() {
		return
	}
	ui.rebuildScheduled.Store(true)
	go func() {
		time.Sleep(50 * time.Millisecond)
		ui.rebuildScheduled.Store(false)
		ui.rebuildMenu()
	}()
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
			ui.syncTrayIcon()
			ui.maybeNotifyProxyChange()
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
	ui.mu.Lock()
	hk := ui.healthKnown
	hok := ui.healthOK
	ui.mu.Unlock()
	if hk {
		if hok {
			healthKey = "tray.status.health.ok"
		} else {
			healthKey = "tray.status.health.fail"
		}
	}
	label := i18n.Tf(lang, "tray.status",
		i18n.T(lang, stateKey),
		st.Port,
		st.Strategy,
		st.DoH,
		i18n.T(lang, healthKey),
	)
	if ui.deps.socks5 != nil && ui.deps.prefs.Get().SOCKS5 {
		label += " | " + i18n.Tf(lang, "tray.socks5.port", ui.deps.socks5.Port())
	}
	if ui.deps.prefs.Get().SplitTunnel && ui.deps.splitTunnelPort > 0 {
		label += " | " + i18n.Tf(lang, "tray.split_tunnel.port", ui.deps.splitTunnelPort)
	}
	return label
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

func (ui *trayUI) checkUpdates(manual bool) bool {
	if !ui.checkMu.TryLock() {
		return false
	}
	defer ui.checkMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	skipped := ui.deps.prefs.Get().SkippedVersion
	available, err := ui.deps.updater.Check(ctx, skipped)
	if err != nil {
		ui.refreshMenu()
		ui.syncUpdateTooltip()
		return false
	}
	ui.deps.prefs.Update(func(p *Preferences) { p.LastUpdateCheck = time.Now() })
	if available && ui.deps.prefs.Get().AutoUpdate {
		if err := ui.deps.updater.Download(ctx); err != nil {
			slog.Warn("tray: auto update download failed", "err", err)
		}
	}
	ui.refreshMenu()
	ui.syncUpdateTooltip()
	ui.maybePromptInstall(manual)
	return true
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
	if err := ui.deps.winProxy.Disable(); err != nil {
		slog.Warn("tray: winproxy disable for install failed", "err", err)
	}
	if err := ui.deps.updater.Apply(target, source, os.Getpid()); err != nil {
		return
	}
	ui.deps.app.Quit()
}

func (ui *trayUI) onUpdateChange() {
	ui.refreshMenu()
	ui.syncUpdateTooltip()
}

func (ui *trayUI) syncUpdateTooltip() {
	if ui.tray == nil {
		return
	}
	ui.tray.SetTooltip(ui.updateStatusLabel())
}

func (ui *trayUI) maybePromptInstall(manual bool) {
	if manual || ui.deps.updater.State() != update.StateReady {
		return
	}
	ver := ui.deps.updater.Latest().TagName
	if ver == "" || ver == ui.promptedUpdateVer {
		return
	}
	ui.promptedUpdateVer = ver
	if showUpdateReadyPrompt(ui.lang(), ver) {
		ui.installUpdate()
	}
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
		dpi.StrategyECH:              "strategy.ech",
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

func (ui *trayUI) syncTrayIcon() {
	if ui.tray == nil {
		return
	}
	ui.mu.Lock()
	running := ui.deps.proxy.Running()
	healthOK := ui.healthOK
	healthKnown := ui.healthKnown
	ui.mu.Unlock()

	icon := TrayStatusIcon(running, healthOK, healthKnown)
	if len(icon) > 0 {
		ui.tray.SetIcon(icon)
	}
}

func (ui *trayUI) maybeNotifyProxyChange() {
	running := ui.deps.proxy.Running()
	ui.mu.Lock()
	prev := ui.previousRunning
	ui.previousRunning = running
	ui.mu.Unlock()

	if prev && !running {
		// Proxy went down
		lang := ui.lang()
		showStatusNotification(ui.tray, i18n.T(lang, "notify.proxy_down"), func() {
			if ui.tray != nil {
				ui.syncUpdateTooltip()
			}
		})
	} else if !prev && running {
		// Proxy came up
		lang := ui.lang()
		showStatusNotification(ui.tray, i18n.T(lang, "notify.proxy_up"), func() {
			if ui.tray != nil {
				ui.syncUpdateTooltip()
			}
		})
	}
}

func (ui *trayUI) runDNSLeakCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	leaked, err := ui.deps.control.CheckDNSLeak(ctx)
	lang := ui.lang()
	if err != nil {
		slog.Warn("DNS leak check failed", "err", err)
		if ui.tray != nil {
			ui.tray.SetTooltip(i18n.T(lang, "tray.dns_leak.fail"))
		}
		return
	}
	if leaked {
		if ui.tray != nil {
			ui.tray.SetTooltip(i18n.T(lang, "tray.dns_leak.fail"))
		}
		showStatusNotification(ui.tray, i18n.T(lang, "notify.dns_leak"), func() {
			if ui.tray != nil {
				ui.syncUpdateTooltip()
			}
		})
	} else {
		if ui.tray != nil {
			ui.tray.SetTooltip(i18n.T(lang, "tray.dns_leak.ok"))
		}
	}
}

func (ui *trayUI) toggleDNSBlock() {
	if ui.deps.control.IsSystemDNSBlocked() {
		if err := ui.deps.control.UnblockSystemDNS(); err != nil {
			slog.Warn("unblock system DNS failed", "err", err)
		}
	} else {
		if err := ui.deps.control.BlockSystemDNS(); err != nil {
			slog.Warn("block system DNS failed", "err", err)
		}
	}
	ui.refreshMenu()
}

func (ui *trayUI) toggleDNSAdBlock() {
	enabled := !ui.deps.prefs.Get().DNSAdBlock
	ui.deps.prefs.Update(func(pref *Preferences) { pref.DNSAdBlock = enabled })
	if ui.deps.adBlocker != nil {
		ui.deps.adBlocker.SetEnabled(enabled)
		if enabled {
			blockURL := ui.deps.prefs.Get().DNSAdBlockURL
			if blockURL == "" {
				blockURL = config.DefaultAdBlockURL
			}
			go func() {
				if err := ui.deps.adBlocker.LoadFromURL(blockURL); err != nil {
					slog.Warn("adblock load failed", "err", err)
				}
			}()
		}
	}
	ui.refreshMenu()
}

func (ui *trayUI) openStatsWindow() {
	ui.mu.Lock()
	existing := ui.statsWindow
	ui.mu.Unlock()
	if existing != nil {
		existing.Show()
		return
	}
	w := ui.deps.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "stats",
		Title:            "Conduit Statistics",
		HTML:             appui.StatsHTML,
		Width:            500,
		Height:           600,
		BackgroundColour: application.NewRGB(26, 26, 46),
	})
	ui.mu.Lock()
	ui.statsWindow = w
	ui.mu.Unlock()
	w.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {
		ui.mu.Lock()
		ui.statsWindow = nil
		ui.mu.Unlock()
	})
	ui.deps.app.Event.On("stats:fetch", func(e *application.CustomEvent) {
		if ui.deps.stats == nil {
			return
		}
		snapshot := ui.deps.stats.Snapshot()
		b, err := json.Marshal(snapshot)
		if err != nil {
			return
		}
		w.ExecJS("window.__stats_update && window.__stats_update(" + string(b) + ")")
	})
	w.Show()
}

func desktopDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Desktop")
}

func menuFromItems(items []*application.MenuItem) *application.Menu {
	if len(items) == 0 {
		return application.NewMenu()
	}
	return application.NewMenuFromItems(items[0], items[1:]...)
}

func (ui *trayUI) runSpeedTest() {
	if !ui.speedTestMu.TryLock() {
		return
	}
	defer ui.speedTestMu.Unlock()

	lang := ui.lang()
	if ui.tray != nil {
		ui.tray.SetTooltip(i18n.T(lang, "tray.speedtest.running"))
	}
	result := ui.deps.control.RunSpeedTest()
	if result.Error != "" {
		if ui.tray != nil {
			ui.tray.SetTooltip(i18n.Tf(lang, "tray.connection.fail", result.Error))
		}
		return
	}
	if ui.tray != nil {
		ui.tray.SetTooltip(i18n.Tf(lang, "tray.speedtest.result",
			result.Latency.Milliseconds(), result.DownloadMbps))
	}
}

func (ui *trayUI) runAutoDNSSelect() {
	lang := ui.lang()
	if ui.tray != nil {
		ui.tray.SetTooltip(i18n.T(lang, "tray.dns_auto.selecting"))
	}
	provider, latency, err := ui.deps.control.AutoSelectDNS()
	if err != nil {
		if ui.tray != nil {
			ui.tray.SetTooltip(i18n.T(lang, "tray.dns_leak.fail"))
		}
		return
	}
	ui.deps.control.SetDoHProvider(provider)
	if ui.tray != nil {
		ui.tray.SetTooltip(i18n.Tf(lang, "tray.dns_auto.selected",
			provider.ShortLabel(), latency.Milliseconds()))
	}
	ui.refreshMenu()
}

func startUpdateScheduler(prefs *preferenceStore, svc *update.Service, ui *trayUI) {
	go func() {
		retryWaits := []time.Duration{
			config.UpdateCheckDelay,
			time.Minute,
			5 * time.Minute,
		}
		var elapsed time.Duration
		for _, wait := range retryWaits {
			wait -= elapsed
			if wait < 0 {
				wait = 0
			}
			elapsed += wait
			select {
			case <-time.After(wait):
				if ui.checkUpdates(false) {
					return
				}
			case <-ui.stopCh:
				return
			}
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
