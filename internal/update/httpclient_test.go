package update

import (
	"net/http"
	"testing"
)

func TestDirectHTTPClientBypassesProxy(t *testing.T) {
	c := directHTTPClient(0)
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type %T", c.Transport)
	}
	url, err := tr.Proxy(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if url != nil {
		t.Fatalf("proxy = %v, want nil", url)
	}
}
