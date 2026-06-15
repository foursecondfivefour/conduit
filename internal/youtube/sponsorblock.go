package youtube

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const sponsorBlockAPI = "https://sponsor.ajay.app/api/skipSegments"

type Segment struct {
	Category string    `json:"category"`
	Segment  [2]float64 `json:"segment"`
	UUID     string    `json:"UUID"`
}

// FetchSegments fetches SponsorBlock segments for a video ID.
func FetchSegments(videoID string, categories []string, httpClient *http.Client) ([]Segment, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	url := fmt.Sprintf("%s?videoID=%s", sponsorBlockAPI, videoID)
	for _, c := range categories {
		url += "&category=" + c
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var segments []Segment
	if err := json.NewDecoder(resp.Body).Decode(&segments); err != nil {
		return nil, err
	}
	return segments, nil
}

// DefaultCategories returns the most common SponsorBlock categories to skip.
func DefaultCategories() []string {
	return []string{"sponsor", "intro", "outro", "interaction", "selfpromo", "music_offtopic"}
}

// SponsorBlockJS returns JavaScript to inject into the YouTube WebView.
// This script listens for video changes, fetches segments via the Go backend bridge,
// and skips them using player.seekTo().
func SponsorBlockJS() string {
	return `
(function() {
	var currentVideoID = '';
	var segments = [];
	var skipCheckInterval = null;

	function getVideoID() {
		var url = new URL(window.location.href);
		return url.searchParams.get('v') || '';
	}

	function checkAndSkip() {
		if (!window.player || !window.player.getCurrentTime) return;
		var ct = window.player.getCurrentTime();
		for (var i = 0; i < segments.length; i++) {
			var seg = segments[i];
			if (ct >= seg.segment[0] && ct < seg.segment[1]) {
				window.player.seekTo(seg.segment[1]);
				break;
			}
		}
	}

	function onNewVideo(videoID) {
		currentVideoID = videoID;
		if (skipCheckInterval) clearInterval(skipCheckInterval);
		segments = [];
		if (videoID && window.wails && window.wails.Events) {
			window.wails.Events.Emit("sponsorblock:fetch", videoID);
		}
		skipCheckInterval = setInterval(checkAndSkip, 300);
	}

	// Expose API
	window.__sponsorblock_setSegments = function(data) {
		segments = data || [];
	};

	// Watch for video changes
	var lastID = '';
	setInterval(function() {
		var vid = getVideoID();
		if (vid !== lastID) {
			lastID = vid;
			onNewVideo(vid);
		}
	}, 1000);

	// Initial check
	onNewVideo(getVideoID());
})();
`
}
