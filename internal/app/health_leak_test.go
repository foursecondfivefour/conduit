package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

// Successful health checks through a local CONNECT proxy must not leak idle transports.
func TestCheckYouTubeSuccessHeapStable(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxyLn.Close()

	go func() {
		for {
			conn, err := proxyLn.Accept()
			if err != nil {
				return
			}
			go handleTestCONNECT(conn, target.URL)
		}
	}()

	proxyURL := "http://" + proxyLn.Addr().String()
	ctx := context.Background()

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < 100; i++ {
		_ = CheckYouTube(ctx, proxyURL)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	growth := int64(after.HeapInuse) - int64(before.HeapInuse)
	if growth > 6<<20 {
		t.Fatalf("heap grew %d bytes after 100 successful health checks", growth)
	}
}

func handleTestCONNECT(client net.Conn, targetURL string) {
	defer client.Close()
	buf := make([]byte, 4096)
	n, err := client.Read(buf)
	if err != nil {
		return
	}
	if n == 0 {
		return
	}
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	host := targetURL[len("http://"):]
	up, err := net.Dial("tcp", host)
	if err != nil {
		return
	}
	defer up.Close()

	// TLS-ish stub: forward first client bytes then bidirectional copy
	if n > 0 {
		_, _ = up.Write(buf[:n])
	}
	go func() { _, _ = io.Copy(up, client) }()
	_, _ = io.Copy(client, up)
}
