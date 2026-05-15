package handlers

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

type ogData struct {
	Title       sql.NullString
	Description sql.NullString
	Image       sql.NullString
}

func fetchOG(ctx context.Context, url string) ogData {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ogData{}
	}
	req.Header.Set("User-Agent", "annnie-bot/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ogData{}
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ogData{}
	}

	nullStr := func(s string) sql.NullString {
		return sql.NullString{String: s, Valid: s != ""}
	}

	get := func(prop string) string {
		v, _ := doc.Find(`meta[property="` + prop + `"]`).Attr("content")
		return v
	}

	return ogData{
		Title:       nullStr(get("og:title")),
		Description: nullStr(get("og:description")),
		Image:       nullStr(get("og:image")),
	}
}
