// Package jobs runs StayKey's background work on river, the Postgres job queue: requests that
// expire, stays that go unpaid, sweeps of expired holds, idempotency keys and uploads that were
// never attached to anything, the team's alerts and their morning summary, and OTA calendars.
//
// Each sweep is a periodic river job. River elects one leader to schedule them, so running
// several API machines doesn't run a sweep several times over.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/ical"
	"staykey.direct/api/internal/notify"
	"staykey.direct/api/internal/store"
)

// UploadGrace is how long an upload may wait to be attached to a property or slip before the
// sweep removes it. The app attaches files within minutes; a day leaves room for retries.
const UploadGrace = 24 * time.Hour

// Alerts sends the team's push notifications and WhatsApp messages.
type Alerts struct {
	Push     notify.Expo
	WhatsApp notify.WhatsApp
	Logger   *slog.Logger
}

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
func Start(ctx context.Context, pool *pgxpool.Pool, st *store.Postgres, fs files.Store, alerts Alerts, feeds *ical.Fetcher, log *slog.Logger) (*river.Client[pgx.Tx], error) {
	all := sweeps(st, fs, alerts, feeds, log)
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
func sweeps(st *store.Postgres, fs files.Store, alerts Alerts, feeds *ical.Fetcher, log *slog.Logger) map[string]sweep {
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
		"notify": {15 * time.Second, func(ctx context.Context, now time.Time) error {
			return SendAlerts(ctx, st, alerts, now)
		}},
		"morning_summary": {15 * time.Minute, func(ctx context.Context, now time.Time) error {
			return SendSummaries(ctx, st, alerts, now)
		}},
		"ical_import": {time.Minute, func(ctx context.Context, now time.Time) error {
			return ImportFeeds(ctx, st, feeds, log, now)
		}},
		// Moves accounts into grace and lapsed as their free or paid access runs out.
		"billing": {15 * time.Minute, func(ctx context.Context, now time.Time) error {
			n, err := st.RecomputeAllBilling(ctx, now)
			if n > 0 {
				log.Info("account statuses changed", "count", n)
			}
			return err
		}},
	}
}

// SendAlerts sends activity nobody has been alerted about yet to the people who asked for it.
// Push goes first; if Expo can't be reached the batch is left for the next run. A WhatsApp
// message that fails is logged and not retried, since the push already went out.
func SendAlerts(ctx context.Context, st *store.Postgres, a Alerts, now time.Time) error {
	_, err := st.SendAlerts(ctx, 100, now, func(ctx context.Context, batch []store.Alert) error {
		var pushes []notify.Push
		for _, alert := range batch {
			data := map[string]string{"activityId": alert.Activity.ID.String(), "accountId": alert.AccountID.String()}
			if alert.Activity.BookingID != nil {
				data["bookingId"] = alert.Activity.BookingID.String()
			}
			for _, r := range alert.Recipients {
				for _, tok := range r.PushTokens {
					pushes = append(pushes, notify.Push{Token: tok, Title: alert.Activity.Title, Body: alert.Activity.Subtitle, Data: data})
				}
			}
		}
		dead, err := a.Push.Send(ctx, pushes)
		if err != nil {
			return err
		}
		if err := st.DeletePushTokens(ctx, dead); err != nil {
			return err
		}
		for _, alert := range batch {
			for _, r := range alert.Recipients {
				if !r.WhatsApp {
					continue
				}
				if err := a.WhatsApp.SendWhatsApp(ctx, r.Phone, alert.Activity.Title, alert.Activity.Subtitle); err != nil {
					a.Logger.Warn("whatsapp alert failed", "error", err, "activity", alert.Activity.ID)
				}
			}
		}
		return nil
	})
	return err
}

// SendSummaries sends each person who wants one their morning summary, once a day.
func SendSummaries(ctx context.Context, st *store.Postgres, a Alerts, now time.Time) error {
	due, err := st.DueSummaries(ctx, now)
	if err != nil {
		return err
	}
	for _, sum := range due {
		title, body := sum.AccountName+" today", SummaryText(sum)
		pushes := make([]notify.Push, len(sum.PushTokens))
		for i, tok := range sum.PushTokens {
			pushes[i] = notify.Push{Token: tok, Title: title, Body: body, Data: map[string]string{"accountId": sum.AccountID.String()}}
		}
		dead, err := a.Push.Send(ctx, pushes)
		if err != nil {
			return err
		}
		if err := st.DeletePushTokens(ctx, dead); err != nil {
			return err
		}
		if sum.WhatsApp {
			if err := a.WhatsApp.SendWhatsApp(ctx, sum.Phone, title, body); err != nil {
				a.Logger.Warn("whatsapp summary failed", "error", err, "account", sum.AccountID)
			}
		}
		if err := st.MarkSummarySent(ctx, sum.AccountID, sum.UserID, sum.Day); err != nil {
			return err
		}
	}
	return nil
}

// SummaryText is the line a morning summary carries, for example "2 arrivals, 1 departure and
// 3 requests waiting".
func SummaryText(s store.Summary) string {
	var parts []string
	if s.Arrivals > 0 {
		parts = append(parts, plural(s.Arrivals, "arrival"))
	}
	if s.Departures > 0 {
		parts = append(parts, plural(s.Departures, "departure"))
	}
	if s.Requests > 0 {
		parts = append(parts, plural(s.Requests, "request")+" waiting")
	}
	switch len(parts) {
	case 0:
		return "No arrivals, departures or requests today."
	case 1:
		return parts[0] + "."
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1] + "."
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
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

// Timeout gives the calendar import longer than river's default minute: it reads up to
// importBatch feeds, importParallel at a time, each allowed the fetcher's 20 seconds.
func (w *sweeper) Timeout(job *river.Job[SweepArgs]) time.Duration {
	if job.Args.Name == "ical_import" {
		return 3 * time.Minute
	}
	return 0
}

func (w *sweeper) Work(ctx context.Context, job *river.Job[SweepArgs]) error {
	sw, ok := w.sweeps[job.Args.Name]
	if !ok {
		return river.JobCancel(fmt.Errorf("unknown sweep %q", job.Args.Name))
	}
	return sw.run(ctx, w.now())
}
