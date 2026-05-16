package og

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Data struct {
	Title       sql.NullString
	Description sql.NullString
	Image       sql.NullString
}

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

	data := Data{
		Title:       nullStr(get("og:title")),
		Description: nullStr(get("og:description")),
		Image:       nullStr(get("og:image")),
	}

	if !data.Description.Valid && !data.Title.Valid && !data.Image.Valid {
		return Data{}, fmt.Errorf("no valid og tags found")
	}

	return data, nil
}
