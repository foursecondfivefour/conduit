package app

import (
	"testing"
	"time"
)

func TestTrayUIStopEndsStatusLoop(t *testing.T) {
	ui := &trayUI{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go ui.statusLoop()

	done := make(chan struct{})
	go func() {
		ui.stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stop timed out")
	}
}
