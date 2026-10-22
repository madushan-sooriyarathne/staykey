package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/observe"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/store/storetest"
)

const opsSecret = "ops-secret-for-tests-0000000001"

// sentryEvents collects what would go to Sentry. The SDK keeps one global client, so these
// tests take turns and match events by their request id.
type sentryEvents struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (f *sentryEvents) Configure(sentry.ClientOptions)        {}
func (f *sentryEvents) Flush(time.Duration) bool              { return true }
func (f *sentryEvents) FlushWithContext(context.Context) bool { return true }
func (f *sentryEvents) Close()                                {}
func (f *sentryEvents) SendEvent(e *sentry.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

// forRequest returns the events reported for one request.
func (f *sentryEvents) forRequest(id string) []*sentry.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*sentry.Event
	for _, e := range f.events {
		if e.Tags["request_id"] == id {
			out = append(out, e)
		}
	}
	return out
}

var sentryTurn sync.Mutex

func reportingOn(t *testing.T) *sentryEvents {
	t.Helper()
	sentryTurn.Lock()
	events := &sentryEvents{}
	if on, err := observe.Init(observe.Config{DSN: "https://public@example.invalid/1", Environment: observe.Production, Transport: events}); err != nil || !on {
		t.Fatalf("observe.Init = %v, %v", on, err)
	}
	t.Cleanup(func() {
		_, _ = observe.Init(observe.Config{})
		sentryTurn.Unlock()
	})
	return events
}

// brokenStore fails or panics on the routes the tests call.
type brokenStore struct {
	*store.Postgres
}

func (brokenStore) CardState(context.Context, uuid.UUID) (domain.CardState, error) {
	return domain.CardState{}, errors.New("database is down")
}

func (brokenStore) Payouts(context.Context, uuid.UUID, time.Time) (domain.Payouts, error) {
	panic("payouts exploded")
}

func observedEnv(t *testing.T) *env {
	t.Helper()
	st := storetest.New(t)
	return buildEnv(t, st, brokenStore{st}, func(o *Options) { o.OpsSecret = opsSecret })
}

func TestTheTestErrorRouteIsHiddenWithoutTheSecret(t *testing.T) {
	e := observedEnv(t)
	e.expect(e.do(call{method: http.MethodPost, path: "/internal/test-error"}), http.StatusNotFound)
}

func TestTheTestErrorRouteIsHiddenWithAWrongSecret(t *testing.T) {
	e := observedEnv(t)
	e.expect(e.do(call{method: http.MethodPost, path: "/internal/test-error", headers: map[string]string{"X-Ops-Secret": "wrong"}}), http.StatusNotFound)
}

func TestTheTestErrorRouteDoesntExistWithoutAnOpsSecret(t *testing.T) {
	e := newEnv(t)
	e.expect(e.do(call{method: http.MethodPost, path: "/internal/test-error", headers: map[string]string{"X-Ops-Secret": opsSecret}}), http.StatusNotFound)
}

func TestTheTestErrorIsReportedWithItsRequestID(t *testing.T) {
	events := reportingOn(t)
	e := observedEnv(t)
	rec := e.do(call{method: http.MethodPost, path: "/internal/test-error", headers: map[string]string{"X-Ops-Secret": opsSecret}})
	e.expect(rec, http.StatusInternalServerError)
	if got := events.forRequest(rec.Header().Get(observe.HeaderRequestID)); len(got) != 1 {
		t.Errorf("reported %d events for the request, want 1", len(got))
	}
}

func TestAReportCarriesNothingPersonalFromTheRequest(t *testing.T) {
	events := reportingOn(t)
	e := observedEnv(t)
	rec := e.do(call{
		method: http.MethodPost, path: "/internal/test-error?t=guest-token-123&phone=0771234567&email=emma@example.com",
		raw: []byte(`{"guest":{"name":"Emma Larsen","phone":"+94771234567"}}`),
		headers: map[string]string{
			"X-Ops-Secret": opsSecret, "Authorization": "Bearer access-token-xyz", "X-Booking-Token": "guest-token-123",
			"Cookie": "staykey_access=cookie-value",
		},
	})
	got := events.forRequest(rec.Header().Get(observe.HeaderRequestID))
	if len(got) != 1 {
		t.Fatalf("reported %d events, want 1", len(got))
	}
	ev := got[0]
	seen := ev.Message + ev.Exception[0].Value
	if r := ev.Request; r != nil {
		seen += r.URL + r.Data + r.QueryString + r.Cookies
		for k, v := range r.Headers {
			seen += k + v
		}
	}
	for _, leak := range []string{"guest-token-123", "0771234567", "emma@example.com", "Emma Larsen", "+94771234567", "access-token-xyz", "cookie-value", opsSecret} {
		if strings.Contains(seen, leak) {
			t.Errorf("the event leaks %q", leak)
		}
	}
}

func TestAnUnexpectedErrorIsReportedOnce(t *testing.T) {
	events := reportingOn(t)
	e := observedEnv(t)
	owner, account := e.owner("+94773200001", "Alice's villas")
	rec := e.do(call{method: http.MethodGet, path: "/v1/card-payments", as: owner, account: account})
	e.expect(rec, http.StatusInternalServerError)
	if got := events.forRequest(rec.Header().Get(observe.HeaderRequestID)); len(got) != 1 {
		t.Errorf("reported %d events for a 500, want 1", len(got))
	}
}

func TestAnExpectedErrorIsNotReported(t *testing.T) {
	events := reportingOn(t)
	e := observedEnv(t)
	owner, account := e.owner("+94773200002", "Alice's villas")
	rec := e.do(call{method: http.MethodGet, path: "/v1/bookings/" + uuid.NewString(), as: owner, account: account})
	e.expect(rec, http.StatusNotFound)
	if got := events.forRequest(rec.Header().Get(observe.HeaderRequestID)); len(got) != 0 {
		t.Errorf("reported %d events for a 404, want none", len(got))
	}
}

func TestAPanicIsReportedAndAnswers500(t *testing.T) {
	events := reportingOn(t)
	e := observedEnv(t)
	owner, account := e.owner("+94773200003", "Alice's villas")
	rec := e.do(call{method: http.MethodGet, path: "/v1/payouts", as: owner, account: account})
	e.expect(rec, http.StatusInternalServerError)
	if got := events.forRequest(rec.Header().Get(observe.HeaderRequestID)); len(got) != 1 {
		t.Errorf("reported %d events for a panic, want 1", len(got))
	}
}

func TestNothingIsReportedOutsideProduction(t *testing.T) {
	sentryTurn.Lock()
	defer sentryTurn.Unlock()
	if on, _ := observe.Init(observe.Config{DSN: "https://public@example.invalid/1", Environment: "development"}); on {
		t.Error("reporting switched on in development")
	}
}

func TestEveryResponseCarriesARequestID(t *testing.T) {
	e := newEnv(t)
	if got := e.do(call{method: http.MethodGet, path: "/healthz"}).Header().Get(observe.HeaderRequestID); got == "" {
		t.Error("no X-Request-Id on the response")
	}
}
