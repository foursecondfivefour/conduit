package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientLatestParsesAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.3.0",
			"html_url": "https://github.com/foursecondfivefour/conduit/releases/tag/v1.3.0",
			"assets": [
				{"name": "conduit.exe", "browser_download_url": "https://example.com/conduit.exe"}
			]
		}`))
	}))
	defer srv.Close()

	client := &Client{HTTP: srv.Client(), RepoURL: srv.URL}
	rel, err := client.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version() != "1.3.0" {
		t.Fatalf("version = %q", rel.Version())
	}
	url, ok := rel.AssetURL("conduit.exe")
	if !ok || url == "" {
		t.Fatal("expected conduit.exe asset")
	}
}
