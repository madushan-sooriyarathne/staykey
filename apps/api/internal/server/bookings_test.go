package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// stayBody is a new booking on the property's first unit.
func stayBody(p oapi.Property, from, to string) map[string]any {
	return map[string]any{
		"propertyId": p.Id, "unitId": p.Units[0].Id, "checkIn": from, "checkOut": to, "adults": 2,
		"guest": map[string]any{"name": "Nimali Perera"}, "source": "phone",
	}
}

func TestBookingStoresTheQuotedTotal(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94772000001", "Quotes")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	body := stayBody(villa, "2026-10-05", "2026-10-09")

	rec := e.do(call{method: http.MethodPost, path: "/v1/quote", as: owner, account: account, body: body})
	e.expect(rec, http.StatusOK)
	quote := decode[oapi.Quote](t, rec)
	stay := e.createBooking(owner, account, villa, "2026-10-05", "2026-10-09")

	if stay.Total == nil || *stay.Total != quote.Total {
		t.Errorf("booking total = %v, want the quote's %d", stay.Total, quote.Total)
	}
}

func TestOverlappingStayGetsTheClash(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94772000002", "Clashes")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	first := e.createBooking(owner, account, villa, "2026-10-05", "2026-10-09")

	rec := e.do(call{method: http.MethodPost, path: "/v1/bookings", as: owner, account: account, body: stayBody(villa, "2026-10-08", "2026-10-10")})
	e.expect(rec, http.StatusConflict)
	got := decode[oapi.ConflictError](t, rec)

	if got.Code != "dates_taken" || got.Clash == nil || deref(got.Clash.Ref) != first.Ref {
		t.Errorf("conflict = %+v, want dates_taken naming %s", got, first.Ref)
	}
}

func TestIdempotencyKeyReplaysTheBooking(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94772000003", "Retries")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	send := func() *oapi.Booking {
		rec := e.do(call{method: http.MethodPost, path: "/v1/bookings", as: owner, account: account,
			body: stayBody(villa, "2026-10-05", "2026-10-07"), headers: map[string]string{"Idempotency-Key": "form-1"}})
		e.expect(rec, http.StatusCreated)
		b := decode[oapi.Booking](t, rec)
		return &b
	}

	first, retry := send(), send()

	if retry.Id != first.Id {
		t.Errorf("retry made booking %s, want the first one %s", retry.Id, first.Id)
	}
}

func TestIdempotencyKeyReusedForAnotherStay(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94772000004", "Reuse")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	key := map[string]string{"Idempotency-Key": "form-1"}
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/bookings", as: owner, account: account,
		body: stayBody(villa, "2026-10-05", "2026-10-07"), headers: key}), http.StatusCreated)

	rec := e.do(call{method: http.MethodPost, path: "/v1/bookings", as: owner, account: account,
		body: stayBody(villa, "2026-11-05", "2026-11-07"), headers: key})

	e.expect(rec, http.StatusUnprocessableEntity)
}

func TestStaleVersionGetsStale(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94772000005", "Stale")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	stay := e.createBooking(owner, account, villa, "2026-10-05", "2026-10-07")
	path := fmt.Sprintf("/v1/bookings/%s", stay.Id)
	e.expect(e.do(call{method: http.MethodPatch, path: path, as: owner, account: account,
		body: map[string]any{"version": stay.Version, "ownerNote": "Late arrival"}}), http.StatusOK)

	rec := e.do(call{method: http.MethodPatch, path: path, as: owner, account: account,
		body: map[string]any{"version": stay.Version, "ownerNote": "Early arrival"}})

	e.expect(rec, http.StatusConflict)
}

func TestNoteChangeKeepsAnAgreedPrice(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94772000006", "Agreed")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	body := stayBody(villa, "2026-10-05", "2026-10-07")
	body["customTotal"] = 30000
	rec := e.do(call{method: http.MethodPost, path: "/v1/bookings", as: owner, account: account, body: body})
	e.expect(rec, http.StatusCreated)
	stay := decode[oapi.Booking](t, rec)

	rec = e.do(call{method: http.MethodPatch, path: "/v1/bookings/" + stay.Id.String(), as: owner, account: account,
		body: map[string]any{"version": stay.Version, "ownerNote": "Pays cash"}})
	e.expect(rec, http.StatusOK)

	if got := decode[oapi.Booking](t, rec).Total; got == nil || *got != 30000 {
		t.Errorf("total after a note = %v, want the agreed 30000", got)
	}
}

func TestCaretakerSeesStaysWithoutMoney(t *testing.T) {
	t.Parallel()
	e, owner, caretaker, account, villa := caretakerEnv(t, "+94772000007")
	stay := e.createBooking(owner, account, villa, "2026-10-05", "2026-10-07")

	rec := e.do(call{method: http.MethodGet, path: "/v1/bookings/" + stay.Id.String(), as: caretaker, account: account})
	e.expect(rec, http.StatusOK)

	if got := decode[oapi.Booking](t, rec); got.Total != nil || got.Payments != nil {
		t.Errorf("caretaker sees total %v and payments %v, want neither", got.Total, got.Payments)
	}
}

func TestCaretakerChecksGuestsIn(t *testing.T) {
	t.Parallel()
	e, owner, caretaker, account, villa := caretakerEnv(t, "+94772000009")
	stay := e.createBooking(owner, account, villa, "2026-10-05", "2026-10-07")

	rec := e.do(call{method: http.MethodPost, path: "/v1/bookings/" + stay.Id.String() + "/transitions", as: caretaker, account: account,
		body: map[string]any{"to": "checked_in", "version": stay.Version}})

	e.expect(rec, http.StatusOK)
}

func TestCaretakerCantCancel(t *testing.T) {
	t.Parallel()
	e, owner, caretaker, account, villa := caretakerEnv(t, "+94772000011")
	stay := e.createBooking(owner, account, villa, "2026-10-05", "2026-10-07")

	rec := e.do(call{method: http.MethodPost, path: "/v1/bookings/" + stay.Id.String() + "/cancel", as: caretaker, account: account,
		body: map[string]any{"version": stay.Version, "reason": "No show"}})

	e.expect(rec, http.StatusForbidden)
}

func TestListBookingsPages(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94772000013", "Pages")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	e.createBooking(owner, account, villa, "2026-10-05", "2026-10-07")
	second := e.createBooking(owner, account, villa, "2026-10-08", "2026-10-09")
	rec := e.do(call{method: http.MethodGet, path: "/v1/bookings?limit=1", as: owner, account: account})
	e.expect(rec, http.StatusOK)
	page := decode[oapi.BookingList](t, rec)

	rec = e.do(call{method: http.MethodGet, path: "/v1/bookings?limit=1&cursor=" + deref(page.NextCursor), as: owner, account: account})
	e.expect(rec, http.StatusOK)

	if next := decode[oapi.BookingList](t, rec); len(next.Items) != 1 || next.Items[0].Id != second.Id || next.NextCursor != nil {
		t.Errorf("second page = %+v, want only %s and no cursor", next, second.Id)
	}
}

func TestRateOverrideChangesTheQuote(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94772000014", "Overrides")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	e.expect(e.do(call{method: http.MethodPut, path: "/v1/rate-overrides", as: owner, account: account, body: map[string]any{
		"propertyId": villa.Id, "unitIds": []any{villa.Units[0].Id}, "from": "2026-10-05", "to": "2026-10-06", "price": 99900,
	}}), http.StatusNoContent)

	rec := e.do(call{method: http.MethodPost, path: "/v1/quote", as: owner, account: account, body: stayBody(villa, "2026-10-05", "2026-10-06")})
	e.expect(rec, http.StatusOK)

	if got := decode[oapi.Quote](t, rec).Nightly; len(got) != 1 || got[0].Price != 99900 {
		t.Errorf("nightly = %+v, want one night at 99900", got)
	}
}

// caretakerEnv is an owner with a villa and a caretaker who looks after it.
func caretakerEnv(t *testing.T, ownerPhone string) (*env, *session, *session, string, oapi.Property) {
	t.Helper()
	e := newEnv(t)
	owner, account := e.owner(ownerPhone, "Caretaken")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	caretaker, _ := e.signIn(ownerPhone[:len(ownerPhone)-1] + "8")
	ownerTenant := tenant.Tenant{AccountID: uuid.MustParse(account), UserID: owner.userID, Role: domain.RoleOwner}
	if _, err := e.store.AddMember(context.Background(), ownerTenant, caretaker.userID, domain.RoleCaretaker, []uuid.UUID{villa.Id}); err != nil {
		t.Fatal(err)
	}
	return e, owner, caretaker, account, villa
}
