package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceDownload(t *testing.T) {
	payload := make([]byte, 2<<20)
	payload[0] = 'M'
	payload[1] = 'Z'

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	svc := NewService()
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
