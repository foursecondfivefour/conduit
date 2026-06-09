package app

import "testing"

func TestChromiumArgs_debugAddsRemotePort(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", true)
	found := false
	for _, a := range args {
		if a == "--remote-debugging-port=9222" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected remote debugging port in debug args: %v", args)
	}
}

func TestChromiumArgs_releaseOmitsRemotePort(t *testing.T) {
	args := chromiumArgs("http://127.0.0.1:31284", false)
	for _, a := range args {
		if a == "--remote-debugging-port=9222" {
			t.Fatalf("remote debugging port must not be set without debug: %v", args)
		}
	}
}
