package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// SpeedTestResult holds the outcome of a speed measurement through the proxy.
type SpeedTestResult struct {
	Latency      time.Duration `json:"latency"`
	DownloadMbps float64       `json:"downloadMbps"`
	Error        string        `json:"error,omitempty"`
}

// RunSpeedTest measures latency and download speed through the proxy.
func RunSpeedTest(ctx context.Context, proxyURL string) *SpeedTestResult {
	result := &SpeedTestResult{}

	proxyParsed, err := url.Parse(proxyURL)
	if err != nil {
		result.Error = fmt.Sprintf("invalid proxy URL: %v", err)
		return result
	}

	transport := &http.Transport{
		Proxy:       http.ProxyURL(proxyParsed),
		DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	// Measure latency
	start := time.Now()
	resp, err := client.Get("https://www.google.com/generate_204")
	if err != nil {
		result.Error = fmt.Sprintf("latency check failed: %v", err)
		return result
	}
	resp.Body.Close()
	result.Latency = time.Since(start)

	// Measure download speed (download ~1MB from a test endpoint)
	testURL := "https://speed.cloudflare.com/__down?bytes=1000000"
	dlStart := time.Now()
	resp, err = client.Get(testURL)
	if err != nil {
		result.Error = fmt.Sprintf("speed test failed: %v", err)
		return result
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("download failed: %v", err)
		return result
	}
	elapsed := time.Since(dlStart)
	result.DownloadMbps = float64(n) * 8.0 / elapsed.Seconds() / 1_000_000.0

	return result
}
