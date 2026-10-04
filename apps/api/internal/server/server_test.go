package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/store/storetest"
	"staykey.direct/api/internal/tenant"
)

func kingfisher() map[string]any {
	return map[string]any{"name": "Kingfisher Villa", "bookingType": "entire", "currency": "USD", "baseRate": 18000}
}

func TestCreateThenFetchPublicProperty(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94771000001", "Kingfisher Villa")

	rec := e.do(call{method: http.MethodPost, path: "/v1/properties", body: kingfisher(), as: s, account: account})
	e.expect(rec, http.StatusCreated)
	created := decode[oapi.Property](t, rec)
	if created.Slug != "kingfisher-villa" {
		t.Errorf("slug = %q, want kingfisher-villa", created.Slug)
	}
	if created.BookingPageUrl != "https://kingfisher-villa.staykey.direct" {
		t.Errorf("bookingPageUrl = %q", created.BookingPageUrl)
	}
	if created.Id.Version() != 7 {
		t.Errorf("id version = %d, want UUIDv7", created.Id.Version())
	}

	rec = e.do(call{method: http.MethodGet, path: "/v1/public/properties/kingfisher-villa"})
	e.expect(rec, http.StatusOK)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("public CORS header = %q, want *", got)
	}
	public := decode[oapi.PublicProperty](t, rec)
	if public.Name != "Kingfisher Villa" || public.BaseRate != 18000 {
		t.Errorf("unexpected public property: %+v", public)
	}

	rec = e.do(call{method: http.MethodGet, path: "/v1/properties", as: s, account: account})
	e.expect(rec, http.StatusOK)
	if list := decode[oapi.PropertyList](t, rec); len(list.Items) != 1 {
		t.Errorf("list returned %d items, want 1", len(list.Items))
	}
}

func TestCreatePropertyRejectsDuplicateSlug(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94771000002", "Kingfisher Villa")
	e.do(call{method: http.MethodPost, path: "/v1/properties", body: kingfisher(), as: s, account: account})

	// Slugs are unique across accounts, since they are subdomains.
	other, otherAccount := e.owner("+94771000003", "Someone else")
	rec := e.do(call{method: http.MethodPost, path: "/v1/properties", body: kingfisher(), as: other, account: otherAccount})
	e.expect(rec, http.StatusConflict)
	if body := decode[oapi.Error](t, rec); body.Code != "slug_taken" {
		t.Errorf("code = %q, want slug_taken", body.Code)
	}
}

func TestCreatePropertyValidation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94771000004", "Villa One")
	cases := map[string]map[string]any{
		"zero rate":     {"name": "Villa One", "bookingType": "entire", "currency": "USD", "baseRate": 0},
		"bad currency":  {"name": "Villa One", "bookingType": "entire", "currency": "JPY", "baseRate": 100},
		"bad type":      {"name": "Villa One", "bookingType": "hostel", "currency": "USD", "baseRate": 100},
		"short name":    {"name": "V", "bookingType": "entire", "currency": "USD", "baseRate": 100},
		"reserved slug": {"name": "Villa One", "slug": "api", "bookingType": "entire", "currency": "USD", "baseRate": 100},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := e.do(call{method: http.MethodPost, path: "/v1/properties", body: body, as: s, account: account})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400 (body %s)", rec.Code, rec.Body)
			}
		})
	}
	t.Run("malformed", func(t *testing.T) {
		rec := e.do(call{method: http.MethodPost, path: "/v1/properties", raw: []byte("{"), as: s, account: account})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400 (body %s)", rec.Code, rec.Body)
		}
	})
}

func TestOnlyOwnersCreateProperties(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94771000005", "Coral Bay House")
	accountID := uuid.MustParse(account)

	for _, role := range []domain.Role{domain.RoleManager, domain.RoleCaretaker} {
		member, _ := e.signIn(map[domain.Role]string{domain.RoleManager: "+94771000006", domain.RoleCaretaker: "+94771000007"}[role])
		ownerTenant := tenant.Tenant{AccountID: accountID, UserID: owner.userID, Role: domain.RoleOwner}
		if _, err := e.store.AddMember(context.Background(), ownerTenant, member.userID, role, nil); err != nil {
			t.Fatal(err)
		}
		rec := e.do(call{method: http.MethodPost, path: "/v1/properties", body: kingfisher(), as: member, account: account})
		e.expect(rec, http.StatusForbidden)
	}
}

func TestCaretakerSeesOnlyAssignedProperties(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94771000008", "Two villas")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	e.createProperty(owner, account, "Coral Bay House")

	caretaker, _ := e.signIn("+94771000009")
	ownerTenant := tenant.Tenant{AccountID: uuid.MustParse(account), UserID: owner.userID, Role: domain.RoleOwner}
	if _, err := e.store.AddMember(context.Background(), ownerTenant, caretaker.userID, domain.RoleCaretaker, []uuid.UUID{villa.Id}); err != nil {
		t.Fatal(err)
	}

	rec := e.do(call{method: http.MethodGet, path: "/v1/properties", as: caretaker, account: account})
	e.expect(rec, http.StatusOK)
	list := decode[oapi.PropertyList](t, rec)
	if len(list.Items) != 1 || list.Items[0].Id != villa.Id {
		t.Fatalf("caretaker sees %+v, want only %s", list.Items, villa.Id)
	}

	rec = e.do(call{method: http.MethodGet, path: "/v1/accounts", as: caretaker})
	e.expect(rec, http.StatusOK)
	accounts := decode[oapi.AccountList](t, rec)
	if len(accounts.Items) != 1 || accounts.Items[0].Role != oapi.Caretaker || len(accounts.Items[0].PropertyIds) != 1 {
		t.Fatalf("caretaker accounts = %+v", accounts.Items)
	}
}

func TestGetPublicPropertyNotFound(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rec := e.do(call{method: http.MethodGet, path: "/v1/public/properties/nowhere"})
	e.expect(rec, http.StatusNotFound)
}

type downStore struct{ Store }

func (downStore) Ping(context.Context) error { return errors.New("connection refused") }

func TestHealthReportsDatabaseDown(t *testing.T) {
	t.Parallel()
	st := storetest.New(t)
	e := newEnvWithStore(t, st, downStore{st})

	health := decode[oapi.Health](t, e.do(call{method: http.MethodGet, path: "/healthz"}))
	if health.Status != oapi.HealthStatusDegraded || health.Database != oapi.Down {
		t.Errorf("health = %+v, want degraded/down", health)
	}
}
