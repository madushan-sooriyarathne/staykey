package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// memStore is an in-memory Store for handler tests.
type memStore struct {
	mu      sync.Mutex
	bySlug  map[string]domain.Property
	pingErr error
}

func newMemStore() *memStore { return &memStore{bySlug: map[string]domain.Property{}} }

func (m *memStore) Ping(context.Context) error { return m.pingErr }

func (m *memStore) ListProperties(context.Context) ([]domain.Property, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Property, 0, len(m.bySlug))
	for _, p := range m.bySlug {
		out = append(out, p)
	}
	return out, nil
}

func (m *memStore) CreateProperty(_ context.Context, in domain.NewProperty) (domain.Property, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.bySlug[in.Slug]; ok {
		return domain.Property{}, domain.ErrSlugTaken
	}
	p := domain.Property{
		ID: uuid.New(), Slug: in.Slug, Name: in.Name, BookingType: in.BookingType,
		Location: in.Location, Currency: in.Currency, BaseRateMinor: in.BaseRateMinor, CreatedAt: time.Now(),
	}
	m.bySlug[p.Slug] = p
	return p, nil
}

func (m *memStore) GetPropertyBySlug(_ context.Context, slug string) (domain.Property, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.bySlug[slug]
	if !ok {
		return domain.Property{}, domain.ErrNotFound
	}
	return p, nil
}

func newTestServer(store Store) http.Handler {
	return New(store, Options{
		BookingDomain: "staykey.direct",
		BookingScheme: "https",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}).Handler()
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v (body %q)", err, rec.Body.String())
	}
	return v
}

func kingfisher() map[string]any {
	return map[string]any{"name": "Kingfisher Villa", "bookingType": "entire", "currency": "USD", "baseRate": 18000}
}

func TestCreateThenFetchPublicProperty(t *testing.T) {
	h := newTestServer(newMemStore())

	rec := do(t, h, http.MethodPost, "/v1/properties", kingfisher())
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body)
	}
	created := decode[oapi.Property](t, rec)
	if created.Slug != "kingfisher-villa" {
		t.Errorf("slug = %q, want kingfisher-villa", created.Slug)
	}
	if created.BookingPageUrl != "https://kingfisher-villa.staykey.direct" {
		t.Errorf("bookingPageUrl = %q", created.BookingPageUrl)
	}

	rec = do(t, h, http.MethodGet, "/v1/public/properties/kingfisher-villa", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public get: status %d, body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("public CORS header = %q, want *", got)
	}
	public := decode[oapi.PublicProperty](t, rec)
	if public.Name != "Kingfisher Villa" || public.BaseRate != 18000 {
		t.Errorf("unexpected public property: %+v", public)
	}

	rec = do(t, h, http.MethodGet, "/v1/properties", nil)
	list := decode[oapi.PropertyList](t, rec)
	if len(list.Items) != 1 {
		t.Errorf("list returned %d items, want 1", len(list.Items))
	}
}

func TestCreatePropertyRejectsDuplicateSlug(t *testing.T) {
	h := newTestServer(newMemStore())
	do(t, h, http.MethodPost, "/v1/properties", kingfisher())

	rec := do(t, h, http.MethodPost, "/v1/properties", kingfisher())
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
	if e := decode[oapi.Error](t, rec); e.Code != "slug_taken" {
		t.Errorf("code = %q, want slug_taken", e.Code)
	}
}

func TestCreatePropertyValidation(t *testing.T) {
	cases := map[string]map[string]any{
		"zero rate":     {"name": "Villa One", "bookingType": "entire", "currency": "USD", "baseRate": 0},
		"bad currency":  {"name": "Villa One", "bookingType": "entire", "currency": "JPY", "baseRate": 100},
		"bad type":      {"name": "Villa One", "bookingType": "hostel", "currency": "USD", "baseRate": 100},
		"short name":    {"name": "V", "bookingType": "entire", "currency": "USD", "baseRate": 100},
		"reserved slug": {"name": "Villa One", "slug": "api", "bookingType": "entire", "currency": "USD", "baseRate": 100},
		"malformed":     nil,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h := newTestServer(newMemStore())
			rec := do(t, h, http.MethodPost, "/v1/properties", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400 (body %s)", rec.Code, rec.Body)
			}
		})
	}
}

func TestGetPublicPropertyNotFound(t *testing.T) {
	h := newTestServer(newMemStore())
	rec := do(t, h, http.MethodGet, "/v1/public/properties/nowhere", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
}

func TestHealthReportsDatabaseDown(t *testing.T) {
	store := newMemStore()
	store.pingErr = errors.New("connection refused")
	h := newTestServer(store)

	health := decode[oapi.Health](t, do(t, h, http.MethodGet, "/healthz", nil))
	if health.Status != oapi.Degraded || health.Database != oapi.Down {
		t.Errorf("health = %+v, want degraded/down", health)
	}
}
