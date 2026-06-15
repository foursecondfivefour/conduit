package youtube

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const rydAPI = "https://returnyoutubedislikeapi.com/votes"

type RYDResponse struct {
	ID          string  `json:"id"`
	DateCreated string  `json:"dateCreated"`
	Likes       int     `json:"likes"`
	Dislikes    int     `json:"dislikes"`
	Rating      float64 `json:"rating"`
	ViewCount   int     `json:"viewCount"`
	Deleted     bool    `json:"deleted"`
}

func FetchDislikes(videoID string, httpClient *http.Client) (*RYDResponse, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	url := fmt.Sprintf("%s?videoId=%s", rydAPI, videoID)
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
	var result RYDResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RYDInjectJS returns JavaScript to inject that displays the dislike count.
func RYDInjectJS() string {
	return `
(function() {
	var currentVideoID = '';
	var dislikesData = null;

	function getVideoID() {
		var url = new URL(window.location.href);
		return url.searchParams.get('v') || '';
	}

	function updateDislikeDisplay() {
		if (!dislikesData) return;
		// Try to find the like/dislike button container
		var segments = document.querySelectorAll('ytd-segmented-like-dislike-button-renderer');
		if (segments.length === 0) {
			segments = document.querySelectorAll('#top-level-buttons-computed ytd-toggle-button-renderer');
		}
		// Insert dislike count near the buttons
		var likeButton = document.querySelector('#top-level-buttons-computed ytd-like-button-renderer');
		if (likeButton && !document.getElementById('ryd-dislikes')) {
			var span = document.createElement('span');
			span.id = 'ryd-dislikes';
			span.style.cssText = 'color:var(--yt-spec-text-secondary);font-size:12px;margin-left:8px;';
			span.textContent = '\uD83D\uDC4E ' + dislikesData.dislikes.toLocaleString();
			likeButton.parentElement.appendChild(span);
		}
	}

	function onNewVideo(videoID) {
		currentVideoID = videoID;
		dislikesData = null;
		var rydEl = document.getElementById('ryd-dislikes');
		if (rydEl) rydEl.remove();
		if (videoID && window.wails && window.wails.Events) {
			window.wails.Events.Emit("ryd:fetch", videoID);
		}
	}

	window.__ryd_setDislikes = function(data) {
		dislikesData = data;
		updateDislikeDisplay();
	};

	var lastID = '';
	setInterval(function() {
		var vid = getVideoID();
		if (vid !== lastID) {
			lastID = vid;
			onNewVideo(vid);
		}
	}, 2000);

	onNewVideo(getVideoID());
})();
`
}
