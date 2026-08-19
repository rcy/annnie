package bsky

import (
	"context"
	"encoding/json"
	"fmt"
	"goirc/internal/ai"
	"goirc/internal/responder"
	"io"
	"net/http"
	"sort"
	"strings"
)

type Trend struct {
	Topic       string `json:"topic"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Link        string `json:"link"`
	StartedAt   string `json:"startedAt"`
	PostCount   int    `json:"postCount"`
	Status      string `json:"status"`
	Category    string `json:"category"`
}

type TrendsResponse struct {
	RecIdStr string  `json:"recIdStr"`
	Trends   []Trend `json:"trends"`
}

func Handle(params responder.Responder) error {
	trends, err := fetchTrends()
	if err != nil {
		return err
	}
	if len(trends) == 0 {
		return fmt.Errorf("no trends found")
	}

	sort.Slice(trends, func(i, j int) bool {
		return trends[i].PostCount > trends[j].PostCount
	})
	if len(trends) > 8 {
		trends = trends[:8]
	}

	summary, err := summarize(params.Context(), trends)
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s", summary)
	return nil
}

func summarize(ctx context.Context, trends []Trend) (string, error) {
	var items []string
	for _, t := range trends {
		items = append(items, fmt.Sprintf("%s: %s (%d posts, %s)", t.DisplayName, t.Description, t.PostCount, t.Category))
	}

	completion, err := ai.Complete(ctx, ai.Params{
		SystemPrompt: "Briefly summarize what's going on based on the trending topics below. Be terse, write like a human in a chat, not a machine. Use minimal punctuation and lowercase. Don't mention Bluesky or that these are trends. It's OK to omit things.",
		UserPrompt:   strings.Join(items, "\n"),
	})
	if err != nil {
		return "", err
	}
	return completion, nil
}

func fetchTrends() ([]Trend, error) {
	req, err := http.NewRequest("GET", "https://api.bsky.app/xrpc/app.bsky.unspecced.getTrends", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "github.com/rcy/annnie")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getTrends: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response TrendsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	return response.Trends, nil
}
