package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	// speedTestLatencyURL answers 204 with no body, so the measurement is
	// pure round-trip time.
	speedTestLatencyURL = "https://www.google.com/generate_204"
	// speedTestDownloadURL serves an exact byte count; 2 MiB keeps the test
	// under ~2s on a 10 Mbit link while still measuring past TCP slow start.
	speedTestDownloadURL = "https://speed.cloudflare.com/__down?bytes=2097152"

	speedTestConnectTimeout  = 5 * time.Second
	speedTestRequestTimeout  = 15 * time.Second
	speedTestOverallDeadline = 25 * time.Second
)

// SpeedTestResult holds the outcome of a speed measurement, both as measured
// directly and as measured through the local proxy.
type SpeedTestResult struct {
	DirectLatency      time.Duration `json:"directLatency"`
	DirectDownloadMbps float64       `json:"directDownloadMbps"`
	ProxyLatency       time.Duration `json:"proxyLatency"`
	ProxyDownloadMbps  float64       `json:"proxyDownloadMbps"`
	Error              string        `json:"error,omitempty"`
}

// RunSpeedTest measures latency and download throughput twice: once straight to
// the network and once tunneled through the local proxy. That makes the proxy
// overhead visible instead of hiding it in a single number.
//
// Both measurements reuse one connection pool per mode, and the whole run is
// bounded by speedTestOverallDeadline so a stuck request cannot pin a tray
// goroutine.
func RunSpeedTest(ctx context.Context, proxyURL string) *SpeedTestResult {
	ctx, cancel := context.WithTimeout(ctx, speedTestOverallDeadline)
	defer cancel()

	result := &SpeedTestResult{}

	if err := measure(ctx, nil, result); err != nil {
		result.Error = err.Error()
		return result
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Host == "" {
		result.Error = "proxy is not running"
		return result
	}
	if err := measure(ctx, parsed, result); err != nil {
		result.Error = err.Error()
	}
	return result
}

// measure runs the latency and download probes with an optional HTTP proxy and
// stores them in the matching fields of result.
func measure(ctx context.Context, proxy *url.URL, result *SpeedTestResult) error {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: speedTestConnectTimeout,
		}).DialContext,
		// Measure the tunnel, not TLS session reuse across probes.
		DisableKeepAlives:     false,
		MaxIdleConns:          2,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: speedTestConnectTimeout,
	}
	if proxy != nil {
		transport.Proxy = http.ProxyURL(proxy)
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   speedTestRequestTimeout,
	}

	latency, err := measureLatency(ctx, client)
	if err != nil {
		return fmt.Errorf("latency: %w", err)
	}
	mbps, err := measureDownload(ctx, client)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	if proxy == nil {
		result.DirectLatency = latency
		result.DirectDownloadMbps = mbps
	} else {
		result.ProxyLatency = latency
		result.ProxyDownloadMbps = mbps
	}
	return nil
}

// measureLatency returns the round-trip time of a bodyless 204 response.
func measureLatency(ctx context.Context, client *http.Client) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, speedTestLatencyURL, nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusNoContent {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return time.Since(start), nil
}

// measureDownload streams a fixed-size payload into io.Discard and converts the
// observed byte rate to Mbit/s.
func measureDownload(ctx context.Context, client *http.Client) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, speedTestDownloadURL, nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return 0, err
	}
	elapsed := time.Since(start)
	if elapsed <= 0 || n <= 0 {
		return 0, errors.New("no data received")
	}
	return float64(n) * 8 / elapsed.Seconds() / 1_000_000, nil
}
