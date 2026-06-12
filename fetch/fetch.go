package fetch

import (
	"io"
	"log"
	"net/http"
	"time"
)

type record struct {
	StatusCode int
	Body       []byte
	Time       time.Time
}

var cache = make(map[string]record)

// Fetch url returning cached data if it exists and is not older than maxAge
func Get(url string, maxAge time.Duration) (int, []byte, error) {
	rec, ok := cache[url]
	if ok && time.Since(rec.Time) < maxAge {
		log.Printf("cache hit")
		return rec.StatusCode, rec.Body, nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return 0, nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	cache[url] = record{resp.StatusCode, body, time.Now()}

	return resp.StatusCode, body, nil
}
