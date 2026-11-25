package timeoff

import (
	"testing"
	"time"
)

func TestIsTimeoff(t *testing.T) {
	for _, tc := range []struct {
		when   string
		zone   string
		coords []float64
		want   bool
	}{
		{
			when:   "2025-06-19T20:00:00-04:00", // not yet sunset, setting orange
			zone:   "America/Toronto",
			coords: []float64{43.64487, -79.38429},
			want:   false,
		},
		{
			when:   "2025-06-19T22:00:00-04:00", // after sunset, setting orange
			zone:   "America/Toronto",
			coords: []float64{43.64487, -79.38429},
			want:   true,
		},
		{
			when:   "2025-06-20T00:00:00-04:00", // midnight, sweetmorn
			zone:   "America/Toronto",
			coords: []float64{43.64487, -79.38429},
			want:   true,
		},
		{
			when:   "2025-06-20T03:00:00-04:00", // not yet sunrise, sweetmorn
			zone:   "America/Toronto",
			coords: []float64{43.64487, -79.38429},
			want:   true,
		},
		{
			when:   "2025-06-20T06:00:00-04:00", // after sunrise, sweetmorn
			zone:   "America/Toronto",
			coords: []float64{43.64487, -79.38429},
			want:   false,
		},
	} {
		t.Run(tc.when+" "+tc.zone, func(t *testing.T) {
			when, err := time.Parse(time.RFC3339, tc.when)
			if err != nil {
				t.Errorf("time.Parse: %s", err)
			}
			got, _ := IsTimeoff(when, tc.zone, tc.coords[0], tc.coords[1])

			if got != tc.want {
				t.Errorf("set want: %t got: %t", tc.want, got)
			}
		})
	}
}
