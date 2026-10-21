package observe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

// fakeTransport collects the events the SDK would send.
type fakeTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (f *fakeTransport) Configure(sentry.ClientOptions)        {}
func (f *fakeTransport) Flush(time.Duration) bool              { return true }
func (f *fakeTransport) FlushWithContext(context.Context) bool { return true }
func (f *fakeTransport) Close()                                {}
func (f *fakeTransport) SendEvent(e *sentry.Event) {
	f.mu.Lock()
	f.events = append(f.events, e)
	f.mu.Unlock()
}
func (f *fakeTransport) sent() []*sentry.Event { f.mu.Lock(); defer f.mu.Unlock(); return f.events }
func (f *fakeTransport) only(t *testing.T) *sentry.Event {
	t.Helper()
	got := f.sent()
	if len(got) != 1 {
		t.Fatalf("sent %d events, want 1", len(got))
	}
	return got[0]
}

// The SDK keeps one global client, so tests that send events run one at a time.
var global sync.Mutex

func production(t *testing.T) *fakeTransport {
	t.Helper()
	global.Lock()
	t.Cleanup(func() {
		enabled.Store(false)
		global.Unlock()
	})
	tr := &fakeTransport{}
	if on, err := Init(Config{DSN: "https://public@example.invalid/1", Environment: Production, Release: "abc123", Transport: tr}); err != nil || !on {
		t.Fatalf("Init = %v, %v", on, err)
	}
	return tr
}

func TestReportingIsOffOutsideProduction(t *testing.T) {
	for _, env := range []string{"development", "staging", "test", ""} {
		t.Run(env, func(t *testing.T) {
			global.Lock()
			defer global.Unlock()
			if on, _ := Init(Config{DSN: "https://public@example.invalid/1", Environment: env}); on {
				t.Errorf("Init(%q) switched reporting on", env)
			}
		})
	}
}

func TestReportingIsOffWithoutADSN(t *testing.T) {
	global.Lock()
	defer global.Unlock()
	if on, _ := Init(Config{Environment: Production}); on {
		t.Error("Init without a DSN switched reporting on")
	}
}

func TestScrub(t *testing.T) {
	tests := []struct {
		name, in string
	}{
		{"a Sri Lankan mobile number", "guest +94 77 123 4567 booked"},
		{"a local mobile number", "call 0771234567 now"},
		{"an international number", "phone +46-70-123-4567"},
		{"an email address", "sent to nadeesha.perera@example.com"},
		{"a live key", "key sk_live_abc123XYZ was refused"},
		{"a test key", "key sk_test_owner_fixture-01 was refused"},
		{"a bearer token", "Authorization: Bearer abc.def-ghi"},
		{"an old NIC", "nic 851234567V on file"},
		{"a new NIC", "nic 198512345678 on file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Scrub(tt.in); !strings.Contains(got, removed) {
				t.Errorf("Scrub(%q) = %q, nothing removed", tt.in, got)
			}
		})
	}
}

func TestScrubKeepsIdsAndAmounts(t *testing.T) {
	in := "booking 01a15d07-203b-76d3-9add-9a39bf9ee8db charged 5418000 at 2026-10-21T11:25:10+05:30"
	if got := Scrub(in); got != in {
		t.Errorf("Scrub changed %q to %q", in, got)
	}
}

func TestCleanURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://kingfisher.staykey.direct/stay/01a1?t=secret-token&paid=1", "https://kingfisher.staykey.direct/stay/01a1"},
		{"https://staykey.direct/join/W3_NICS5-ctAUXD4", "https://staykey.direct/join/:token"},
		{"https://staykey.direct/verify-email/abcdef123", "https://staykey.direct/verify-email/:token"},
		{"https://api.staykey.direct/ical/kingfisher-3081d4a7.ics", "https://api.staykey.direct/ical/:file"},
		{"/v1/bookings/123#frag", "/v1/bookings/123"},
	}
	for _, tt := range tests {
		if got := CleanURL(tt.in); got != tt.want {
			t.Errorf("CleanURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAReportCarriesNoPersonalData(t *testing.T) {
	tr := production(t)
	req := httptest.NewRequest(http.MethodPost, "https://api.staykey.direct/v1/public/bookings/1?t=guest-token&phone=0771234567", strings.NewReader(`{"guest":{"name":"Emma Larsen"}}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("X-Booking-Token", "guest-token")
	req.Header.Set("Cookie", "staykey_access=abc")
	req.Header.Set("User-Agent", "StayKey/1.0")
	hub := sentry.CurrentHub().Clone()
	hub.Scope().SetRequest(req)
	hub.CaptureException(errors.New(`duplicate key value (phone)=(+94771234567) for emma@example.com`))
	e := tr.only(t)
	body := e.Request.URL + e.Request.Data + e.Request.QueryString + e.Request.Cookies + e.Exception[0].Value
	for k, v := range e.Request.Headers {
		body += k + v
	}
	for _, leak := range []string{"guest-token", "secret-token", "0771234567", "+94771234567", "emma@example.com", "Emma Larsen", "staykey_access"} {
		if strings.Contains(body, leak) {
			t.Errorf("the event leaks %q: %s", leak, body)
		}
	}
}

func TestAReportCarriesTheRequestID(t *testing.T) {
	tr := production(t)
	Report(WithRequestID(context.Background(), "req-0001-abcd"), errors.New("boom"), nil)
	if got := tr.only(t).Tags["request_id"]; got != "req-0001-abcd" {
		t.Errorf("request_id tag = %q", got)
	}
}

func TestAJobErrorIsReportedOnceAnHour(t *testing.T) {
	tr := production(t)
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	j := &JobReporter{Now: func() time.Time { return now }}
	j.Report(context.Background(), "send_emails", errors.New("claim emails: timeout after 5s"))
	now = now.Add(30 * time.Minute)
	j.Report(context.Background(), "send_emails", errors.New("claim emails: timeout after 7s"))
	if got := len(tr.sent()); got != 1 {
		t.Errorf("sent %d events in half an hour, want 1", got)
	}
}

func TestAJobErrorIsReportedAgainAfterAnHour(t *testing.T) {
	tr := production(t)
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	j := &JobReporter{Now: func() time.Time { return now }}
	j.Report(context.Background(), "send_emails", errors.New("claim emails: timeout"))
	now = now.Add(61 * time.Minute)
	j.Report(context.Background(), "send_emails", errors.New("claim emails: timeout"))
	if got := len(tr.sent()); got != 2 {
		t.Errorf("sent %d events over an hour, want 2", got)
	}
}

func TestHeartbeatsPingOnceAMinute(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	h := &Heartbeats{URLs: map[string]string{"jobs": srv.URL}, Now: func() time.Time { return now }}
	h.Ping(context.Background(), "jobs")
	h.Ping(context.Background(), "jobs")
	now = now.Add(61 * time.Second)
	h.Ping(context.Background(), "jobs")
	if got := hits.Load(); got != 2 {
		t.Errorf("pinged %d times, want 2", got)
	}
}

func TestAHeartbeatNeverWaitsPastItsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	h := &Heartbeats{URLs: map[string]string{"jobs": srv.URL}, Client: &http.Client{Timeout: 100 * time.Millisecond}}
	start := time.Now()
	h.Ping(context.Background(), "jobs")
	if took := time.Since(start); took > time.Second {
		t.Errorf("ping took %s", took)
	}
}

func TestParseHeartbeats(t *testing.T) {
	got, err := ParseHeartbeats("jobs=https://uptime.example/a, send_emails=https://uptime.example/b")
	if err != nil || len(got) != 2 || got["send_emails"] != "https://uptime.example/b" {
		t.Errorf("ParseHeartbeats = %v, %v", got, err)
	}
}

func TestAnIncomingRequestIDIsKeptOnlyWhenTrusted(t *testing.T) {
	var seen string
	h := RequestIDs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = RequestID(r.Context()) }),
		func(*http.Request) bool { return false })
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(HeaderRequestID, "chosen-by-outsider")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen == "chosen-by-outsider" || seen == "" {
		t.Errorf("request id = %q", seen)
	}
}
