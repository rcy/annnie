package mlb

import (
	"testing"
)

func TestFetchTeams(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external API test")
	}
	teams, err := fetchTeams()
	if err != nil {
		t.Skip("external API unavailable:", err)
	}
	t.Log(teams)
}

func TestFetchLeagueTeams(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external API test")
	}
	for _, league := range []string{"AL", "NL"} {
		t.Run(league, func(t *testing.T) {
			teams, err := fetchLeagueTeams(league)
			if err != nil {
				t.Skip("external API unavailable:", err)
			}
			if len(teams) != 15 {
				t.Errorf("expected 15 teams, got %d", len(teams))
			}
		})
	}
}
