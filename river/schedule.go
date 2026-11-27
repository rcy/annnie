package river

import (
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"github.com/robfig/cron"
)

type cronSchedule struct {
	schedule cron.Schedule
	loc      *time.Location
}

func (s cronSchedule) Next(t time.Time) time.Time {
	return s.schedule.Next(t.In(s.loc))
}

// Cron returns a River schedule for a 6-field cron spec (with seconds),
// evaluated in loc.
func Cron(spec string, loc *time.Location) (river.PeriodicSchedule, error) {
	schedule, err := cron.Parse(spec)
	if err != nil {
		return nil, fmt.Errorf("parse cron %q: %w", spec, err)
	}
	return cronSchedule{schedule: schedule, loc: loc}, nil
}
