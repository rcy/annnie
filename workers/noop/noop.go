package noop

import (
	"context"

	"github.com/riverqueue/river"
)

type JobArgs struct{}

func (JobArgs) Kind() string { return "noop" }

type JobWorker struct {
	river.WorkerDefaults[JobArgs]
}

func (w *JobWorker) Work(ctx context.Context, job *river.Job[JobArgs]) error {
	return nil
}
