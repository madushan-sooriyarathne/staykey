package observe

import (
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"

	"staykey.direct/api/internal/tenant"
)

// Report sends an unexpected error to Sentry with the request id and the account and person it
// happened for, by id only. It does nothing when reporting is off.
func Report(ctx context.Context, err error, tags map[string]string) {
	if !Enabled() || err == nil {
		return
	}
	hub := scoped(ctx, tags)
	hub.CaptureException(err)
}

// ReportPanic sends a recovered panic to Sentry.
func ReportPanic(ctx context.Context, v any) {
	if !Enabled() {
		return
	}
	scoped(ctx, nil).RecoverWithContext(ctx, v)
}

// Warn sends a message about something that failed for good but didn't fail a request, such
// as an email that ran out of attempts.
func Warn(ctx context.Context, message string, tags map[string]string) {
	if !Enabled() {
		return
	}
	hub := scoped(ctx, tags)
	hub.WithScope(func(s *sentry.Scope) {
		s.SetLevel(sentry.LevelWarning)
		hub.CaptureMessage(message)
	})
}

func scoped(ctx context.Context, tags map[string]string) *sentry.Hub {
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(s *sentry.Scope) {
		if id := RequestID(ctx); id != "" {
			s.SetTag("request_id", id)
		}
		if t, ok := tenant.From(ctx); ok {
			s.SetTag("account_id", t.AccountID.String())
			s.SetUser(sentry.User{ID: t.UserID.String()})
		}
		for k, v := range tags {
			s.SetTag(k, v)
		}
	})
	return hub
}

// JobReporter reports a periodic job's errors at most once per hour for each job and kind of
// error, since a job failing every minute would use the month's quota in days.
type JobReporter struct {
	Every time.Duration // default an hour
	Now   func() time.Time

	mu   sync.Mutex
	last map[string]time.Time
}

// Numbers and ids vary between runs of the same failure; they don't make it a new one.
var variable = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F-]{27}|\d+`)

// Report sends err for job unless the same job reported the same kind of error within the
// last hour. It reports whether it sent.
func (j *JobReporter) Report(ctx context.Context, job string, err error) bool {
	if err == nil || !j.due(job+"|"+variable.ReplaceAllString(err.Error(), "#")) {
		return false
	}
	Report(ctx, err, map[string]string{"job": job})
	return true
}

func (j *JobReporter) due(key string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	if j.Now != nil {
		now = j.Now()
	}
	every := j.Every
	if every == 0 {
		every = time.Hour
	}
	if j.last == nil {
		j.last = map[string]time.Time{}
	}
	if last, ok := j.last[key]; ok && now.Sub(last) < every {
		return false
	}
	j.last[key] = now
	return true
}
