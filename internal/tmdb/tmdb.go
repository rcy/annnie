package tmdb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

type Movie struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
	Overview    string  `json:"overview"`
}

type Credit struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

type CrewMember struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

type searchResult struct {
	Results []Movie `json:"results"`
}

type creditsResult struct {
	Cast []Credit     `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

func Search(query string, year string) ([]Movie, error) {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TMDB_API_KEY not set")
	}

	v := url.Values{}
	v.Set("api_key", apiKey)
	v.Set("query", query)
	if year != "" {
		v.Set("year", year)
	}
	v.Set("language", "en-US")

	u := "https://api.themoviedb.org/3/search/movie?" + v.Encode()

	resp, err := http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("searching tmdb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tmdb returned status %d", resp.StatusCode)
	}

	var sr searchResult
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding tmdb response: %w", err)
	}

	return sr.Results, nil
}

func Credits(movieID int) ([]Credit, []CrewMember, error) {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return nil, nil, fmt.Errorf("TMDB_API_KEY not set")
	}

	u := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/credits?api_key=%s&language=en-US", movieID, apiKey)

	resp, err := http.Get(u)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching credits: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("tmdb credits returned status %d", resp.StatusCode)
	}

	var cr creditsResult
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, nil, fmt.Errorf("decoding credits: %w", err)
	}

	return cr.Cast, cr.Crew, nil
}
