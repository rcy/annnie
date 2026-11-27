package portfoliosnapshot

import (
	"context"
	"time"

	"goirc/handlers/stock"

	"github.com/riverqueue/river"
)

type JobArgs struct {
	// Date is the snapshot date (YYYY-MM-DD), fixed when the job is scheduled
	// so that retries write to the same day.
	Date string `json:"date"`
}

func (JobArgs) Kind() string { return "portfolio_snapshot" }

// InsertOpts allows 12 attempts. River waits about n^4 seconds after the nth
// failure, so retries span roughly 11 hours. The job runs just after the US
// market close (13:00 Vancouver), and the fetched price stays at the close
// until the next open (6:30), so every retry still records the right value.
func (JobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 12}
}

type JobWorker struct {
	river.WorkerDefaults[JobArgs]
}

func (w *JobWorker) Work(ctx context.Context, job *river.Job[JobArgs]) error {
	return stock.SnapshotPortfolios(ctx, job.Args.Date)
}

// PeriodicJob runs the snapshot on the given schedule, dated in loc.
func PeriodicJob(schedule river.PeriodicSchedule, loc *time.Location) *river.PeriodicJob {
	return river.NewPeriodicJob(
		schedule,
		func() (river.JobArgs, *river.InsertOpts) {
			return JobArgs{Date: time.Now().In(loc).Format("2006-01-02")}, nil
		},
		&river.PeriodicJobOpts{ID: "portfolio_snapshot"},
	)
}
