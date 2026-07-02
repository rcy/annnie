package youtube

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

type Result struct {
	VideoID string `json:"videoId"`
	Title   string `json:"title"`
	URL     string
}

type searchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Search queries the YouTube Data API v3 for the top video matching query.
func Search(query string) (*Result, error) {
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("YOUTUBE_API_KEY not set")
	}

	v := url.Values{}
	v.Set("part", "snippet")
	v.Set("type", "video")
	v.Set("q", query)
	v.Set("maxResults", "1")
	v.Set("key", apiKey)

	u := "https://www.googleapis.com/youtube/v3/search?" + v.Encode()

	resp, err := http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("searching youtube: %w", err)
	}
	defer resp.Body.Close()

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding youtube response: %w", err)
	}

	if sr.Error != nil {
		return nil, fmt.Errorf("youtube api error: %s", sr.Error.Message)
	}

	if len(sr.Items) == 0 {
		return nil, fmt.Errorf("no results for %q", query)
	}

	item := sr.Items[0]
	return &Result{
		VideoID: item.ID.VideoID,
		Title:   item.Snippet.Title,
		URL:     "https://youtu.be/" + item.ID.VideoID,
	}, nil
}
