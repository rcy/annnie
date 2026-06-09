package xkcd

import (
	"encoding/json"
	"fmt"
	"goirc/internal/responder"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func Handle(params responder.Responder) error {
	msg := params.Msg()
	parts := strings.Fields(msg)

	if len(parts) == 1 {
		latest, err := latestID()
		if err != nil {
			return err
		}
		return fetchAndSend(params, latest)
	}

	if len(parts) == 2 {
		id, err := strconv.Atoi(parts[1])
		if err != nil {
			var latest int
			latest, err = latestID()
			if err != nil {
				return err
			}
			id = rand.Intn(latest-1) + 1
		}
		return fetchAndSend(params, id)
	}

	return fmt.Errorf("usage: !xkcd [num|random]")
}

func fetchAndSend(params responder.Responder, id int) error {
	comic, err := fetchComic(id)
	if err != nil {
		return err
	}

	rootURL := os.Getenv("ROOT_URL")
	params.Privmsgf(params.Target(), "#%d: %s %s/xkcd/%d", comic.Num, comic.Title, rootURL, comic.Num)

	return nil
}

type Comic struct {
	Num   int    `json:"num"`
	Title string `json:"safe_title"`
	Alt   string `json:"alt"`
	Img   string `json:"img"`
	Year  string `json:"year"`
	Month string `json:"month"`
	Day   string `json:"day"`
}

func (c *Comic) DateFormatted() string {
	m, _ := strconv.Atoi(c.Month)
	d, _ := strconv.Atoi(c.Day)
	y, _ := strconv.Atoi(c.Year)
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return t.Format("January 2, 2006")
}

func FetchComic(id int) (*Comic, error) {
	return fetchComic(id)
}

func fetchComic(id int) (*Comic, error) {
	url := fmt.Sprintf("https://xkcd.com/%d/info.0.json", id)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching comic %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("comic %d not found", id)
	}

	var comic Comic
	if err := json.NewDecoder(resp.Body).Decode(&comic); err != nil {
		return nil, fmt.Errorf("decoding comic %d: %w", id, err)
	}

	return &comic, nil
}

func latestID() (int, error) {
	resp, err := http.Get("https://xkcd.com/info.0.json")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var comic Comic
	if err := json.NewDecoder(resp.Body).Decode(&comic); err != nil {
		return 0, err
	}
	return comic.Num, nil
}
