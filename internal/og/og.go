package og

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Data struct {
	Title       sql.NullString
	Description sql.NullString
	Image       sql.NullString
}

var ErrNoTags = fmt.Errorf("no valid og tags found")

func Fetch(ctx context.Context, rawURL string) (Data, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Data{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Data{}, err
	}
	defer resp.Body.Close()

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		return Data{Image: sql.NullString{String: rawURL, Valid: true}}, nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Data{}, err
	}

	nullStr := func(s string) sql.NullString {
		return sql.NullString{String: s, Valid: s != ""}
	}

	get := func(prop string) string {
		v, _ := doc.Find(`meta[property="` + prop + `"]`).Attr("content")
		return v
	}

	title := get("og:title")
	if title == "" {
		title = doc.Find("title").First().Text()
	}

	data := Data{
		Title:       nullStr(title),
		Description: nullStr(get("og:description")),
		Image:       nullStr(get("og:image")),
	}

	if !data.Description.Valid && !data.Title.Valid && !data.Image.Valid {
		return Data{}, ErrNoTags
	}

	return data, nil
}
