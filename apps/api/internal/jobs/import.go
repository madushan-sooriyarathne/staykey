package jobs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"staykey.direct/api/internal/ical"
	"staykey.direct/api/internal/store"
)

const (
	// importBatch is how many due feeds one run reads, importParallel at a time.
	importBatch    = 20
	importParallel = 5
	// importLease keeps a claimed feed from a second run until this run reports back. It outlasts
	// the job's timeout.
	importLease = 5 * time.Minute
)

// ImportFeeds reads the OTA calendars that are due and brings their stays into StayKey. Feeds
// are fetched outside any transaction. A feed that can't be read is recorded with a message
// for the owner and tried again later; only database errors fail the run. Logs name the feed,
// never its link, which carries the channel's secret.
func ImportFeeds(ctx context.Context, st *store.Postgres, fetcher *ical.Fetcher, log *slog.Logger, now time.Time) error {
	due, err := st.ClaimDueFeeds(ctx, now, importLease, importBatch)
	if err != nil {
		return err
	}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
		sem  = make(chan struct{}, importParallel)
	)
	for _, f := range due {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := importFeed(ctx, st, fetcher, log, f, now); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

func importFeed(ctx context.Context, st *store.Postgres, fetcher *ical.Fetcher, log *slog.Logger, f store.DueFeed, now time.Time) error {
	loc, err := time.LoadLocation(f.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	body, etag, notModified, err := fetcher.Fetch(ctx, f.URL, f.ETag)
	if notModified {
		return st.FeedNotModified(ctx, f, now)
	}
	var events []ical.Event
	if err == nil {
		events, err = ical.Parse(bytes.NewReader(body), loc, today)
	}
	if err != nil {
		return failFeed(ctx, st, log, f, ical.Problem(err), now)
	}
	if err := st.ApplyFeed(ctx, f, events, etag, today, now); err != nil {
		log.Error("import feed", "feed", f.ID, "error", err)
		return failFeed(ctx, st, log, f, "We couldn't import that calendar. We'll keep trying.", now)
	}
	return nil
}

func failFeed(ctx context.Context, st *store.Postgres, log *slog.Logger, f store.DueFeed, message string, now time.Time) error {
	failures, err := st.RecordFeedFailure(ctx, f, message, now)
	if err != nil {
		return err
	}
	log.Warn("feed not read", "feed", f.ID, "failures", failures, "problem", message)
	return nil
}
