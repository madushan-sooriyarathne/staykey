package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/store/storetest"
)

// clock is a settable time source shared by the auth service and the tests.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// env is an API wired to its own database schema, file folder and a fixed clock.
type env struct {
	t     *testing.T
	h     http.Handler
	store *store.Postgres
	files *files.Local
	clock *clock
}

func newEnv(t *testing.T, configure ...func(*auth.Config)) *env {
	t.Helper()
	st := storetest.New(t)
	return newEnvWithStore(t, st, st, configure...)
}

func newEnvWithStore(t *testing.T, st *store.Postgres, serverStore Store, configure ...func(*auth.Config)) *env {
	t.Helper()
	clk := &clock{now: time.Now().UTC().Truncate(time.Second)}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := auth.Config{
		Secret:         []byte("test-secret-that-is-at-least-32-bytes-long"),
		ExposeDevCodes: true,
		Now:            clk.Now,
		Logger:         log,
	}
	for _, f := range configure {
		f(&cfg)
	}
	authService, err := auth.New(st, auth.LogSender{Logger: log}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	local, err := files.NewLocal(t.TempDir(), []byte("test uploads"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(serverStore, authService, local, Options{
		BookingDomain: "staykey.direct",
		BookingScheme: "https",
		Media:         local.Handler(),
		Logger:        log,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, h: srv.Handler(), store: st, files: local, clock: clk}
}

// session is a signed-in caller.
type session struct {
	access  string
	refresh string
	userID  uuid.UUID
}

// call describes one request.
type call struct {
	method  string
	path    string
	body    any
	as      *session
	account string // X-Account-Id
	raw     []byte // sent as is instead of body
	headers map[string]string
}

func (e *env) do(c call) *httptest.ResponseRecorder {
	e.t.Helper()
	var r io.Reader = http.NoBody
	switch {
	case c.raw != nil:
		r = bytes.NewReader(c.raw)
	case c.body != nil:
		b, err := json.Marshal(c.body)
		if err != nil {
			e.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(c.method, c.path, r)
	req.Header.Set("Content-Type", "application/json")
	if c.as != nil {
		req.Header.Set("Authorization", "Bearer "+c.as.access)
	}
	if c.account != "" {
		req.Header.Set("X-Account-Id", c.account)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func (e *env) expect(rec *httptest.ResponseRecorder, status int) {
	e.t.Helper()
	if rec.Code != status {
		e.t.Fatalf("status %d, want %d (body %s)", rec.Code, status, rec.Body)
	}
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v (body %q)", err, rec.Body.String())
	}
	return v
}

// requestCode asks for a code and returns it (development servers echo it back).
func (e *env) requestCode(phone string) string {
	e.t.Helper()
	rec := e.do(call{method: http.MethodPost, path: "/v1/auth/otp", body: oapi.OtpRequest{Phone: phone}})
	e.expect(rec, http.StatusOK)
	sent := decode[oapi.OtpSent](e.t, rec)
	if sent.DevCode == nil {
		e.t.Fatal("no devCode in response")
	}
	return *sent.DevCode
}

// signIn requests and verifies a code for phone.
func (e *env) signIn(phone string) (*session, oapi.SignIn) {
	e.t.Helper()
	code := e.requestCode(phone)
	rec := e.do(call{method: http.MethodPost, path: "/v1/auth/verify", body: oapi.VerifyRequest{Phone: phone, Code: code}})
	e.expect(rec, http.StatusOK)
	in := decode[oapi.SignIn](e.t, rec)
	return &session{access: in.Tokens.AccessToken, refresh: in.Tokens.RefreshToken, userID: in.User.Id}, in
}

// owner signs a new person in and gives them an account.
func (e *env) owner(phone, accountName string) (*session, string) {
	e.t.Helper()
	s, _ := e.signIn(phone)
	rec := e.do(call{method: http.MethodPost, path: "/v1/accounts", body: oapi.NewAccount{Name: accountName}, as: s})
	e.expect(rec, http.StatusCreated)
	return s, decode[oapi.Account](e.t, rec).Id.String()
}

// createProperty adds a property to an account through the API.
func (e *env) createProperty(s *session, account, name string) oapi.Property {
	e.t.Helper()
	rec := e.do(call{method: http.MethodPost, path: "/v1/properties", as: s, account: account,
		body: map[string]any{"name": name, "bookingType": "entire", "currency": "USD", "baseRate": 18000}})
	e.expect(rec, http.StatusCreated)
	return decode[oapi.Property](e.t, rec)
}

// createBooking adds a stay on a property's first unit through the API.
func (e *env) createBooking(s *session, account string, p oapi.Property, from, to string) oapi.Booking {
	e.t.Helper()
	rec := e.do(call{method: http.MethodPost, path: "/v1/bookings", as: s, account: account, body: map[string]any{
		"propertyId": p.Id, "unitId": p.Units[0].Id, "checkIn": from, "checkOut": to, "adults": 2,
		"guest": map[string]any{"name": "Nimali Perera", "phone": "+94770000001"}, "source": "whatsapp",
	}})
	e.expect(rec, http.StatusCreated)
	return decode[oapi.Booking](e.t, rec)
}

func (e *env) refresh(token string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(call{method: http.MethodPost, path: "/v1/auth/refresh", body: oapi.RefreshRequest{RefreshToken: token}})
}
