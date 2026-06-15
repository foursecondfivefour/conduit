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

func TestDirectHTTPClientHasTimeout(t *testing.T) {
	c := directHTTPClient(30)
	if c.Timeout == 0 {
		t.Fatal("client should have a timeout")
	}
}

func TestDirectHTTPClientZeroTimeout(t *testing.T) {
	c := directHTTPClient(0)
	if c.Timeout != 0 {
		t.Fatal("client with zero timeout should have no timeout set")
	}
}
