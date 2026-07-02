package track

import (
	"fmt"
	"goirc/internal/responder"
	"goirc/internal/youtube"
	"strings"
)

func Handle(params responder.Responder) error {
	query := strings.TrimSpace(params.Match(1))
	if query == "" {
		return fmt.Errorf("usage: !track <query>")
	}

	result, err := youtube.Search(query)
	if err != nil {
		return err
	}

	// Truncate title if too long for IRC
	title := result.Title
	if len(title) > 300 {
		title = title[:300] + "…"
	}

	params.Privmsgf(params.Target(), "%s — %s", title, result.URL)
	return nil
}
