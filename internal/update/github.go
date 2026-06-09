package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
)

const repoLatest = "https://api.github.com/repos/foursecondfivefour/conduit/releases/latest"

type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type Client struct {
	HTTP    *http.Client
	RepoURL string
}

func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		RepoURL: repoLatest,
	}
}

func (c *Client) Latest(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.RepoURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Conduit/"+config.Version)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Release{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github: status %d", resp.StatusCode)
	}

	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return Release{}, err
	}
	if rel.TagName == "" {
		return Release{}, fmt.Errorf("github: empty tag")
	}
	return rel, nil
}

func (r Release) AssetURL(name string) (string, bool) {
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, name) {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

func (r Release) Version() string {
	v, err := SanitizeVersion(r.TagName)
	if err != nil {
		return strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
	}
	return v
}
