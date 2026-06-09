package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/foursecondfivefour/conduit/internal/dns"
	"github.com/foursecondfivefour/conduit/internal/proxy"
	"github.com/foursecondfivefour/conduit/internal/update"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestPreferenceStoreCloseNoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	dir := t.TempDir()
	paths := RuntimePaths{ConfigDir: dir, LogDir: filepath.Join(dir, "logs")}
	store, err := newPreferenceStore(paths)
	if err != nil {
		t.Fatal(err)
	}
	store.Update(func(p *Preferences) { p.Strategy = "none" })
	time.Sleep(50 * time.Millisecond)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTrayStopEndsUpdateScheduler(t *testing.T) {
	defer goleak.VerifyNone(t)

	ui := &trayUI{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
		deps: trayDeps{
			prefs: &preferenceStore{
				data: DefaultPreferences(false),
			},
			updater: update.NewService(),
		},
	}
	go ui.statusLoop()
	startUpdateScheduler(ui.deps.prefs, ui.deps.updater, ui)
	ui.stop()
	time.Sleep(20 * time.Millisecond)
}

func TestProxyControlLifecycleNoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	store := NewSettingsStore(Preferences{AllowlistPreset: string(proxy.PresetYouTube)})
	resolver := dns.NewResolver(dns.ProviderCloudflare)
	srv := proxy.NewServer(resolver, store.Snapshot)
	ctrl := NewControlService(srv, resolver, store, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_ = ctrl.SetStrategy(store.Snapshot().Strategy)
	if err := srv.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}
