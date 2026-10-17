package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"staykey.direct/api/internal/oapi"
)

// future is a date days from today, in the format routes take.
func future(days int) string { return time.Now().UTC().AddDate(0, 0, days).Format(time.DateOnly) }

// villa is an owner's property with its public slug, for booking page tests.
type villa struct {
	owner   *session
	account string
	p       oapi.Property
}

func (e *env) villa(phone string) villa {
	e.t.Helper()
	s, account := e.owner(phone, "Villa account")
	return villa{owner: s, account: account, p: e.createProperty(s, account, "Lagoon Villa")}
}

func (e *env) guestBook(v villa, key string, body map[string]any) oapi.GuestBookingCreated {
	e.t.Helper()
	b := map[string]any{
		"unitId": v.p.Units[0].Id, "checkIn": future(30), "checkOut": future(32), "adults": 2,
		"guest": map[string]any{"name": "Sam Lee", "phone": "+447700900000"}, "paymentMethod": "property", "source": "page",
	}
	for k, val := range body {
		b[k] = val
	}
	rec := e.do(call{method: http.MethodPost, path: "/v1/public/properties/" + v.p.Slug + "/bookings", body: b,
		headers: map[string]string{"Idempotency-Key": key}})
	e.expect(rec, http.StatusCreated)
	return decode[oapi.GuestBookingCreated](e.t, rec)
}

func TestPublicQuoteRefusesMoreGuestsThanTheUnitSleeps(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94774000001")
	rec := e.do(call{method: http.MethodPost, path: "/v1/public/properties/" + v.p.Slug + "/quote", body: map[string]any{
		"unitId": v.p.Units[0].Id, "checkIn": future(30), "checkOut": future(32), "adults": 12,
	}})
	e.expect(rec, http.StatusBadRequest)
	if got := deref(decode[oapi.Error](t, rec).Field); got != "adults" {
		t.Errorf("field = %q, want adults", got)
	}
}

func TestAHeldStayBooksAndTheTokenOpensIt(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94774000002")
	rec := e.do(call{method: http.MethodPost, path: "/v1/public/properties/" + v.p.Slug + "/holds", body: map[string]any{
		"unitId": v.p.Units[0].Id, "checkIn": future(30), "checkOut": future(32),
	}})
	e.expect(rec, http.StatusCreated)
	hold := decode[oapi.Hold](t, rec)
	created := e.guestBook(v, "form-1", map[string]any{"holdId": hold.HoldId})

	rec = e.do(call{method: http.MethodGet, path: "/v1/public/bookings/" + created.Booking.Id.String(),
		headers: map[string]string{"X-Booking-Token": created.Token}})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.GuestBooking](t, rec).Ref; got != created.Booking.Ref {
		t.Errorf("ref = %q, want %q", got, created.Booking.Ref)
	}
}

func TestARetriedBookingGetsTheSameToken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94774000003")
	first := e.guestBook(v, "form-1", nil)
	if again := e.guestBook(v, "form-1", nil); again.Token != first.Token {
		t.Error("a retry with the same key made a second booking")
	}
}

func TestTakenDatesSayNothingAboutTheOtherStay(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94774000004")
	stay := e.createBooking(v.owner, v.account, v.p, future(30), future(33))
	rec := e.do(call{method: http.MethodPost, path: "/v1/public/properties/" + v.p.Slug + "/holds", body: map[string]any{
		"unitId": v.p.Units[0].Id, "checkIn": future(31), "checkOut": future(32),
	}})
	e.expect(rec, http.StatusConflict)
	if body := rec.Body.String(); strings.Contains(body, "Nimali") || strings.Contains(body, stay.Ref) || !strings.Contains(body, "dates_taken") {
		t.Errorf("409 body = %s", body)
	}
}

func TestAWrongBookingTokenIsNotFound(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94774000005")
	created := e.guestBook(v, "form-1", nil)
	rec := e.do(call{method: http.MethodGet, path: "/v1/public/bookings/" + created.Booking.Id.String(),
		headers: map[string]string{"X-Booking-Token": "guess"}})
	e.expect(rec, http.StatusNotFound)
}

func TestAPropertyWithNoWayToPayTakesNoBookings(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94774000006")
	rec := e.do(call{method: http.MethodPatch, path: "/v1/properties/" + v.p.Id.String(), as: v.owner, account: v.account, body: map[string]any{
		"payments": map[string]any{"bank": v.p.Payments.Bank, "atProperty": false, "cards": "off"},
	}})
	e.expect(rec, http.StatusOK)
	rec = e.do(call{method: http.MethodPost, path: "/v1/public/properties/" + v.p.Slug + "/holds", body: map[string]any{
		"unitId": v.p.Units[0].Id, "checkIn": future(30), "checkOut": future(32),
	}})
	e.expect(rec, http.StatusConflict)
	if got := decode[oapi.Error](t, rec).Code; got != "not_taking_bookings" {
		t.Errorf("code = %q, want not_taking_bookings", got)
	}
}

func TestAGuestSlipShowsInActivity(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94774000007")
	created := e.guestBook(v, "form-1", nil)
	token := map[string]string{"X-Booking-Token": created.Token}
	path := "/v1/public/bookings/" + created.Booking.Id.String()
	rec := e.do(call{method: http.MethodPost, path: path + "/uploads", headers: token,
		body: map[string]any{"contentType": "image/jpeg", "size": 2048}})
	e.expect(rec, http.StatusCreated)
	ticket := decode[oapi.UploadTicket](t, rec)
	rec = e.do(call{method: http.MethodPost, path: path + "/slips", headers: token,
		body: map[string]any{"key": ticket.Key, "amount": 10000}})
	e.expect(rec, http.StatusCreated)

	rec = e.do(call{method: http.MethodGet, path: "/v1/activity", as: v.owner, account: v.account})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.ActivityPage](t, rec).Items[0].Title; got != "Bank slip from Sam Lee" {
		t.Errorf("latest activity = %q", got)
	}
}

func TestAvailabilityShowsTakenNights(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94774000008")
	e.createBooking(v.owner, v.account, v.p, future(30), future(32))
	rec := e.do(call{method: http.MethodGet, path: "/v1/public/properties/" + v.p.Slug + "/availability?from=" + future(0) + "&to=" + future(60)})
	e.expect(rec, http.StatusOK)
	if taken := decode[oapi.Availability](t, rec).Units[0].Taken; len(taken) != 2 || taken[0].Format(time.DateOnly) != future(30) {
		t.Errorf("taken = %v, want the two booked nights", taken)
	}
}
