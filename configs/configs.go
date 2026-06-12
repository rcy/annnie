package configs

import (
	"context"
	"goirc/db/model"
	"goirc/fetch"
	db "goirc/model"
	"net/url"
	"strings"
	"time"
)

// GetLiteral returns the raw config value for key as stored.
func GetLiteral(key string) (string, error) {
	q := model.New(db.DB)
	cfg, err := q.GetConfig(context.TODO(), key)
	if err != nil {
		return "", err
	}
	return cfg.Value, nil
}

// Get returns the config value for key, resolving @url references by fetching
// the URL and returning the response body.
func Get(key string) (string, error) {
	q := model.New(db.DB)
	cfg, err := q.GetConfig(context.TODO(), key)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(cfg.Value, "@") {
		rawURL := cfg.Value[1:]
		status, body, err := fetchURL(rawURL)
		if err != nil {
			return "", err
		}
		_ = status
		return strings.TrimSpace(string(body)), nil
	}
	return cfg.Value, nil
}

func fetchURL(rawURL string) (int, []byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, nil, err
	}
	if u.Host == "api.github.com" {
		return fetch.GetWithHeaders(rawURL, 10*time.Second, map[string]string{
			"Accept": "application/vnd.github.raw+json",
		})
	}
	return fetch.Get(rawURL, 10*time.Second)
}

// Set stores a config key/value pair. Returns the previous value if any.
func Set(key, value, nick string) (string, error) {
	q := model.New(db.DB)
	ctx := context.TODO()

	if strings.HasPrefix(value, "$") {
		refKey := value[1:]
		ref, err := q.GetConfig(ctx, refKey)
		if err != nil {
			return "", err
		}
		value = ref.Value
	}

	prev, err := q.GetConfig(ctx, key)
	var prevValue string
	if err == nil {
		prevValue = prev.Value
	}

	err = q.SetConfig(ctx, model.SetConfigParams{
		Key:   key,
		Value: value,
		Nick:  nick,
	})
	if err != nil {
		return "", err
	}

	return prevValue, nil
}
