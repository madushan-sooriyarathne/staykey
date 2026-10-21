// Package observe reports unexpected errors to Sentry, pings uptime heartbeats and tags every
// request with an id. Sentry is switched on only in production with a DSN, so development,
// staging, CI and tests never spend the plan's quota. Nothing here ever slows or fails a
// request: reports are queued by the SDK, and heartbeats time out quickly.
package observe

import (
	"sync/atomic"
	"time"

	"github.com/getsentry/sentry-go"
)

// Production is the only environment that sends events to Sentry.
const Production = "production"

// Config sets up error reporting. Transport is for tests; the SDK's HTTP transport is used
// otherwise.
type Config struct {
	DSN         string
	Environment string
	Release     string
	Transport   sentry.Transport
}

var enabled atomic.Bool

// Init starts error reporting when the environment is production and a DSN is set, and
// reports whether it did. Anything else leaves reporting off.
func Init(c Config) (bool, error) {
	if c.Environment != Production || c.DSN == "" {
		enabled.Store(false)
		return false, nil
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              c.DSN,
		Environment:      c.Environment,
		Release:          c.Release,
		SendDefaultPII:   false,
		AttachStacktrace: true,
		MaxBreadcrumbs:   20,
		BeforeSend:       func(e *sentry.Event, _ *sentry.EventHint) *sentry.Event { return Clean(e) },
		BeforeBreadcrumb: func(b *sentry.Breadcrumb, _ *sentry.BreadcrumbHint) *sentry.Breadcrumb { return cleanBreadcrumb(b) },
		Transport:        c.Transport,
	})
	if err != nil {
		return false, err
	}
	enabled.Store(true)
	return true, nil
}

// Enabled reports whether errors go to Sentry.
func Enabled() bool { return enabled.Load() }

// Flush waits up to timeout for queued events to be sent, at shutdown.
func Flush(timeout time.Duration) {
	if Enabled() {
		sentry.Flush(timeout)
	}
}
