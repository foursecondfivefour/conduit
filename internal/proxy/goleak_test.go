package proxy

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/foursecondfivefour/conduit/internal/config"
	"github.com/foursecondfivefour/conduit/internal/dns"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestProxyStartStopNoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	settings := config.DefaultSettings()
	srv := NewServer(dns.NewResolver(dns.ProviderCloudflare), func() config.Settings { return settings })
	if _, err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}
