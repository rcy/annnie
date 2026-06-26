package movie

import (
	"fmt"
	"goirc/internal/responder"
	"goirc/internal/tmdb"
	"strconv"
	"strings"
	"time"
)

func Handle(params responder.Responder) error {
	query := params.Match(1)
	if query == "" {
		return fmt.Errorf("usage: !movie <query>")
	}

	var year string
	parts := strings.Fields(query)
	if len(parts) > 1 {
		if last := parts[len(parts)-1]; len(last) == 4 {
			if _, err := strconv.Atoi(last); err == nil {
				year = last
				query = strings.Join(parts[:len(parts)-1], " ")
			}
		}
	}

	results, err := tmdb.Search(query, year)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		params.Privmsgf(params.Target(), "no results for %q", query)
		return nil
	}

	m := results[0]

	date, _ := time.Parse("2006-01-02", m.ReleaseDate)
	yearStr := strconv.Itoa(date.Year())
	if date.Year() == 1 {
		yearStr = "?"
	}

	overview := strings.TrimSpace(m.Overview)
	if len(overview) > 200 {
		overview = overview[:200] + "…"
	}

	msg := fmt.Sprintf("%s (%s) ★ %.1f — %s", m.Title, yearStr, m.VoteAverage, overview)

	cast, crew, err := tmdb.Credits(m.ID)
	if err == nil {
		topCast := take(cast, 5)
		if len(topCast) > 0 {
			var parts []string
			for _, c := range topCast {
				parts = append(parts, fmt.Sprintf("%s as %s", c.Name, c.Character))
			}
			msg += " | Cast: " + strings.Join(parts, ", ")
		}

		keyJobs := []string{"Director", "Screenplay", "Producer", "Writer", "Story"}
		var shown []string
		for _, kj := range keyJobs {
			for _, cm := range crew {
				if cm.Job == kj {
					shown = append(shown, fmt.Sprintf("%s (%s)", cm.Name, cm.Job))
					if len(shown) >= 3 {
						break
					}
				}
			}
			if len(shown) >= 3 {
				break
			}
		}
		if len(shown) > 0 {
			msg += " | " + strings.Join(shown, ", ")
		}
	}

	params.Privmsgf(params.Target(), "%s", msg)

	return nil
}

func take[T any](s []T, n int) []T {
	if len(s) < n {
		return s
	}
	return s[:n]
}
