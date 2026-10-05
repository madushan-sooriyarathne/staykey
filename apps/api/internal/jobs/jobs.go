// Package jobs runs StayKey's background work on river, the Postgres job queue: requests that
// expire, stays that go unpaid, and sweeps of expired holds, idempotency keys and uploads that
// were never attached to anything.
//
// Each sweep is a periodic river job. River elects one leader to schedule them, so running
// several API machines doesn't run a sweep several times over.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/store"
)

// UploadGrace is how long an upload may wait to be attached to a property or slip before the
// sweep removes it. The app attaches files within minutes; a day leaves room for retries.
const UploadGrace = 24 * time.Hour

// SweepArgs names the sweep a job runs.
type SweepArgs struct {
	Name string `json:"name"`
}

// Kind is river's name for sweep jobs.
func (SweepArgs) Kind() string { return "sweep" }

type sweep struct {
	every time.Duration
	run   func(ctx context.Context, now time.Time) error
}

// Start runs the sweeps until ctx is done. Stop the returned client to wait for running jobs.
func Start(ctx context.Context, pool *pgxpool.Pool, st *store.Postgres, fs files.Store, log *slog.Logger) (*river.Client[pgx.Tx], error) {
	all := sweeps(st, fs, log)
	workers := river.NewWorkers()
	river.AddWorker(workers, &sweeper{sweeps: all, now: time.Now})
	periodic := make([]*river.PeriodicJob, 0, len(all))
	for name, sw := range all {
		periodic = append(periodic, river.NewPeriodicJob(
			river.PeriodicInterval(sw.every),
			func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{Name: name}, nil },
			&river.PeriodicJobOpts{ID: name, RunOnStart: true},
		))
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 2}},
		Workers:      workers,
		PeriodicJobs: periodic,
		Logger:       log,
	})
	if err != nil {
		return nil, fmt.Errorf("create job client: %w", err)
	}
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("start jobs: %w", err)
	}
	return client, nil
}

// sweeps are the periodic jobs by name.
func sweeps(st *store.Postgres, fs files.Store, log *slog.Logger) map[string]sweep {
	return map[string]sweep{
		"expire_requests": {time.Minute, func(ctx context.Context, now time.Time) error {
			n, err := st.DeclineExpiredRequests(ctx, now)
			if n > 0 {
				log.Info("declined expired requests", "count", n)
			}
			return err
		}},
		"cancel_unpaid": {time.Minute, func(ctx context.Context, now time.Time) error {
			n, err := st.CancelUnpaidBookings(ctx, now)
			if n > 0 {
				log.Info("cancelled unpaid stays", "count", n)
			}
			return err
		}},
		"sweep_expired": {time.Minute, st.SweepExpired},
		"sweep_uploads": {time.Hour, func(ctx context.Context, now time.Time) error {
			return SweepUploads(ctx, st, fs, now)
		}},
	}
}

// SweepUploads deletes files older than UploadGrace that no property or slip points at.
func SweepUploads(ctx context.Context, st *store.Postgres, fs files.Store, now time.Time) error {
	keys, err := fs.WrittenBefore(ctx, now.Add(-UploadGrace))
	if err != nil {
		return err
	}
	for _, key := range keys {
		used, err := st.FileInUse(ctx, key)
		if err != nil {
			return err
		}
		if used {
			continue
		}
		if err := fs.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

type sweeper struct {
	river.WorkerDefaults[SweepArgs]
	sweeps map[string]sweep
	now    func() time.Time
}

func (w *sweeper) Work(ctx context.Context, job *river.Job[SweepArgs]) error {
	sw, ok := w.sweeps[job.Args.Name]
	if !ok {
		return river.JobCancel(fmt.Errorf("unknown sweep %q", job.Args.Name))
	}
	return sw.run(ctx, w.now())
}
