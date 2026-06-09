package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceDownload(t *testing.T) {
	payload := make([]byte, 2<<20)
	payload[0] = 'M'
	payload[1] = 'Z'
	sum := sha256.Sum256(payload)
	shaLine := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sha" {
			_, _ = w.Write([]byte(shaLine))
			return
		}
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	svc := NewService()
	svc.downloadURLValidator = func(string) error { return nil }
	svc.mu.Lock()
	svc.latest = Release{
		TagName: "v2.0.0",
		Assets: []Asset{
			{
				Name:               "conduit.exe",
				BrowserDownloadURL: srv.URL + "/exe",
				Size:               int64(len(payload)),
			},
			{
				Name:               "conduit.exe.sha256",
				BrowserDownloadURL: srv.URL + "/sha",
			},
		},
	}
	svc.mu.Unlock()

	err := svc.Download(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if svc.State() != StateReady {
		t.Fatalf("state = %s", svc.State())
	}
	if svc.DownloadPath() == "" {
		t.Fatal("expected download path")
	}
}

func TestServiceDownloadRequiresSHA(t *testing.T) {
	payload := make([]byte, 2<<20)
	payload[0] = 'M'
	payload[1] = 'Z'

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	svc := NewService()
	svc.downloadURLValidator = func(string) error { return nil }
	svc.mu.Lock()
	svc.latest = Release{
		TagName: "v2.0.0",
		Assets: []Asset{{
			Name:               "conduit.exe",
			BrowserDownloadURL: srv.URL,
			Size:               int64(len(payload)),
		}},
	}
	svc.mu.Unlock()

	if err := svc.Download(context.Background()); err == nil {
		t.Fatal("expected error without sha asset")
	}
}
