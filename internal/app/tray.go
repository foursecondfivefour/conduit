package app

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dpi"
	"github.com/foursecondfivefour/conduit/internal/proxy"
)

type trayUI struct {
	app      *application.App
	proxy    *proxy.Server
	control  *ControlService
	statusItem *application.MenuItem
	tcpItem    *application.MenuItem
	tlsItem    *application.MenuItem
}

func setupTray(app *application.App, proxyServer *proxy.Server, settings *config.Settings) {
	ui := &trayUI{
		app:     app,
		proxy:   proxyServer,
		control: NewControlService(proxyServer, settings),
	}

	ui.statusItem = application.NewMenuItem(ui.statusLabel())
	ui.statusItem.SetEnabled(false)

	ui.tcpItem = application.NewMenuItemRadio(dpi.StrategyTCPSegmentation.String(), settings.Strategy == dpi.StrategyTCPSegmentation)
	ui.tcpItem.OnClick(func(*application.Context) {
		ui.setStrategy(dpi.StrategyTCPSegmentation)
	})
	ui.tlsItem = application.NewMenuItemRadio(dpi.StrategyTLSRecordFrag.String(), settings.Strategy == dpi.StrategyTLSRecordFrag)
	ui.tlsItem.OnClick(func(*application.Context) {
		ui.setStrategy(dpi.StrategyTLSRecordFrag)
	})

	strategyMenu := application.NewMenuFromItems(ui.tcpItem, ui.tlsItem)
	dnsItem := application.NewMenuItem("Сброс DNS кэша").OnClick(func(*application.Context) {
		_, _ = ui.proxy.Restart(context.Background())
		ui.statusItem.SetLabel(ui.statusLabel())
	})
	quitItem := application.NewMenuItem("Выход").OnClick(func(*application.Context) {
		app.Quit()
	})

	menu := application.NewMenuFromItems(
		ui.statusItem,
		application.NewMenuItemSeparator(),
		application.NewSubmenu("Стратегия DPI", strategyMenu),
		dnsItem,
		application.NewMenuItemSeparator(),
		quitItem,
	)

	tray := app.SystemTray.New()
	tray.SetTooltip("Conduit")
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}
	tray.SetMenu(menu)
	tray.OnClick(func() {
		tray.ShowMenu()
	})
	tray.Show()
}

func (ui *trayUI) statusLabel() string {
	st := ui.control.GetStatus()
	state := "остановлен"
	if st.Running {
		state = "работает"
	}
	return fmt.Sprintf("Прокси %s (:%d)", state, st.Port)
}

func (ui *trayUI) setStrategy(strategy dpi.Strategy) {
	ui.control.SetStrategy(strategy.String())
	ui.tcpItem.SetChecked(strategy == dpi.StrategyTCPSegmentation)
	ui.tlsItem.SetChecked(strategy == dpi.StrategyTLSRecordFrag)
	ui.statusItem.SetLabel(ui.statusLabel())
}
