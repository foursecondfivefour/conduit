package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const healthCheckURL = "https://www.youtube.com"

func CheckYouTube(ctx context.Context, proxyURL string) error {
	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxy),
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, healthCheckURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("health: status %d", resp.StatusCode)
	}
	return nil
}
