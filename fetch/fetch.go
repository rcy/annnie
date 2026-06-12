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
	return GetWithHeaders(url, maxAge, nil)
}

// GetWithHeaders fetches url with custom headers, returning cached data if not
// older than maxAge. The cache key includes url only (headers are not part of
// the key).
func GetWithHeaders(url string, maxAge time.Duration, headers map[string]string) (int, []byte, error) {
	rec, ok := cache[url]
	if ok && time.Since(rec.Time) < maxAge {
		log.Printf("cache hit")
		return rec.StatusCode, rec.Body, nil
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	cache[url] = record{resp.StatusCode, body, time.Now()}

	return resp.StatusCode, body, nil
}
