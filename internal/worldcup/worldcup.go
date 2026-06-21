// Command nextmatch fetches the FIFA World Cup schedule from football-data.org
// and reports when a given country plays next.
//
// Usage:
//
//	export FOOTBALL_DATA_API_KEY=your_key_here   # free key: https://www.football-data.org
//	go run nextmatch.go Brazil
//	go run nextmatch.go "Korea Republic"
//	go run nextmatch.go ARG               # three-letter codes work too
package worldcup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const competitionURL = "https://api.football-data.org/v4/competitions/WC/matches"

// Team holds the bits of a team we need. TLA is the three-letter code, e.g. "BRA".
type Team struct {
	Name string `json:"name"`
	TLA  string `json:"tla"`
}

// Match mirrors the fields of a football-data.org match object that we use.
// utcDate is RFC3339 (e.g. "2026-06-21T19:00:00Z"), which time.Time decodes natively.
type Match struct {
	UTCDate  time.Time `json:"utcDate"`
	Status   string    `json:"status"` // SCHEDULED, TIMED, IN_PLAY, FINISHED, ...
	Stage    string    `json:"stage"`  // GROUP_STAGE, ROUND_OF_16, ...
	Group    string    `json:"group"`  // e.g. "Group A" (may be empty in knockouts)
	HomeTeam Team      `json:"homeTeam"`
	AwayTeam Team      `json:"awayTeam"`
}

type matchesResponse struct {
	Matches []Match `json:"matches"`
}

// fetchMatches downloads every World Cup match.
func fetchMatches(apiKey string) ([]Match, error) {
	req, err := http.NewRequest(http.MethodGet, competitionURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned HTTP %d (check your API key / rate limit)", resp.StatusCode)
	}

	var data matchesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return data.Matches, nil
}

// involves reports whether country plays in this match. It matches the full team
// name, the TLA, or a name substring, all case-insensitively, so "usa", "USA",
// and "United States" all resolve sensibly.
func (m Match) involves(country string) bool {
	c := strings.ToLower(strings.TrimSpace(country))
	if c == "" {
		return false
	}
	for _, t := range []Team{m.HomeTeam, m.AwayTeam} {
		name := strings.ToLower(t.Name)
		if name == c || strings.ToLower(t.TLA) == c || strings.Contains(name, c) {
			return true
		}
	}
	return false
}

// nextMatch returns the soonest match for country that has not yet kicked off.
func nextMatch(matches []Match, country string) (Match, bool) {
	now := time.Now()
	var upcoming []Match
	for _, m := range matches {
		if m.involves(country) && m.UTCDate.After(now) {
			upcoming = append(upcoming, m)
		}
	}
	if len(upcoming) == 0 {
		return Match{}, false
	}
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].UTCDate.Before(upcoming[j].UTCDate)
	})
	return upcoming[0], true
}

func NextMatch(country string) (string, error) {
	matches, err := fetchMatches(os.Getenv("FOOTBALL_DATA_API_KEY"))
	if err != nil {
		return "", fmt.Errorf("fetchMatches: %w", err)
	}

	m, ok := nextMatch(matches, country)
	if !ok {
		return fmt.Sprintf("No upcoming matches found for %q.\n", country), nil
	}

	result := ""

	result += fmt.Sprintf("%s vs %s, ", m.HomeTeam.Name, m.AwayTeam.Name)
	if m.Group != "" {
		result += fmt.Sprintf("%s — %s, ", m.Stage, m.Group)
	} else {
		result += fmt.Sprintf("%s, ", m.Stage)
	}
	result += m.UTCDate.Format("Mon 02 Jan 2006, 15:04 MST")
	//result += fmt.Sprintf("  Kicks off in %s\n", time.Until(m.UTCDate).Round(time.Minute))

	return result, nil
}
