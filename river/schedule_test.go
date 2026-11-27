package river

import (
	"testing"
	"time"
)

func TestCronUsesLocation(t *testing.T) {
	vancouver, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Cron("0 15 13 * * 1,2,3,4,5", vancouver)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			// Friday 2026-11-27 12:00 Vancouver, given in UTC.
			name: "later same weekday",
			now:  time.Date(2026, 11, 27, 20, 0, 0, 0, time.UTC),
			want: time.Date(2026, 11, 27, 13, 15, 0, 0, vancouver),
		},
		{
			// Friday 14:00 Vancouver: next run skips the weekend.
			name: "skips weekend",
			now:  time.Date(2026, 11, 27, 22, 0, 0, 0, time.UTC),
			want: time.Date(2026, 11, 30, 13, 15, 0, 0, vancouver),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.Next(tt.now)
			if !got.Equal(tt.want) {
				t.Errorf("Next(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}
