package jobs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"staykey.direct/api/internal/observe"
)

func sweeperWith(t *testing.T, run func(context.Context, time.Time) error) (*sweeper, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	t.Cleanup(srv.Close)
	return &sweeper{
		sweeps:   map[string]sweep{"send_emails": {time.Minute, run}},
		now:      time.Now,
		beats:    &observe.Heartbeats{URLs: map[string]string{"send_emails": srv.URL}},
		reporter: &observe.JobReporter{},
	}, &hits
}

func TestASweepThatRunsPingsItsHeartbeat(t *testing.T) {
	w, hits := sweeperWith(t, func(context.Context, time.Time) error { return nil })
	if err := w.Work(context.Background(), &river.Job[SweepArgs]{Args: SweepArgs{Name: "send_emails"}}); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("heartbeat pinged %d times, want 1", hits.Load())
	}
}

func TestASweepThatFailsDoesntPingItsHeartbeat(t *testing.T) {
	w, hits := sweeperWith(t, func(context.Context, time.Time) error { return errors.New("claim emails: timeout") })
	_ = w.Work(context.Background(), &river.Job[SweepArgs]{Args: SweepArgs{Name: "send_emails"}})
	if hits.Load() != 0 {
		t.Errorf("heartbeat pinged %d times after a failure, want 0", hits.Load())
	}
}

func TestTheHeartbeatSweepStandsForTheScheduler(t *testing.T) {
	if got := heartbeatName("heartbeat"); got != "jobs" {
		t.Errorf("heartbeatName = %q, want jobs", got)
	}
}
