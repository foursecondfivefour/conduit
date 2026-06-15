package app

import (
	"context"
	"runtime"
	"testing"
)

func TestCheckYouTubeHeapStable(t *testing.T) {
	ctx := context.Background()
	proxyURL := "http://127.0.0.1:1"

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < 50; i++ {
		_ = CheckYouTube(ctx, proxyURL)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	growth := int64(after.HeapInuse) - int64(before.HeapInuse)
	const maxGrowth = 4 << 20
	if growth > maxGrowth {
		t.Fatalf("heap grew too much after 50 health checks: %d bytes (limit %d)", growth, maxGrowth)
	}
}
