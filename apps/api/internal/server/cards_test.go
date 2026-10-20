package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/cards"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/store/storetest"
)

const ownerCardKey = "sk_test_owner_fixture-01"

// fakePaymentsLK stands in for Payments.lk at the HTTP boundary: it opens checkouts, lets a
// test pay them, takes refunds, and lists the events a test queues.
type fakePaymentsLK struct {
	mu        sync.Mutex
	checkouts map[string]map[string]any
	refunds   []map[string]any
	events    []map[string]any
	keys      []string
	revoked   bool
}

func newFakePaymentsLK(t *testing.T) (*fakePaymentsLK, string) {
	t.Helper()
	f := &fakePaymentsLK{checkouts: map[string]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv.URL
}

func (f *fakePaymentsLK) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.revoked || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk_test_owner") {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"code":"UNAUTHENTICATED","reason":"unknown_api_key"}`)
		return
	}
	f.keys = append(f.keys, r.Header.Get("Idempotency-Key"))
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/events":
		var data []map[string]any
		for _, ev := range f.events {
			if q := r.URL.Query().Get("type"); q == "" || ev["type"] == q {
				data = append(data, ev)
			}
		}
		writeJSON(w, map[string]any{"object": "list", "data": data, "hasMore": false})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/checkouts":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := fmt.Sprintf("chk_%d", len(f.checkouts)+1)
		c := map[string]any{"object": "checkout", "id": id, "mode": "test", "status": "open",
			"url": "https://pay.payments.lk/c/" + id, "amountCents": body["amountCents"], "successUrl": body["successUrl"]}
		f.checkouts[id] = c
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, c)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/checkouts/"):
		c, ok := f.checkouts[strings.TrimPrefix(r.URL.Path, "/v1/checkouts/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, c)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/refunds":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		refund := map[string]any{"object": "refund", "id": fmt.Sprintf("re_%d", len(f.refunds)+1), "status": "pending",
			"paymentId": body["paymentId"], "amountCents": body["amountCents"]}
		f.refunds = append(f.refunds, refund)
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, refund)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// pay completes a checkout as a guest would.
func (f *fakePaymentsLK) pay(checkoutID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.checkouts[checkoutID]
	c["status"] = "completed"
	c["payment"] = map[string]any{"object": "payment", "id": "pay_" + checkoutID, "mode": "test", "status": "succeeded",
		"amountCents": c["amountCents"], "currency": "LKR", "checkoutId": checkoutID}
}

// queue adds an event the next poll lists.
func (f *fakePaymentsLK) queue(typ string, object map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, map[string]any{"id": fmt.Sprintf("evt_%d", len(f.events)+1), "object": "event", "type": typ,
		"mode": "test", "created": time.Now().UTC().Format(time.RFC3339), "data": map[string]any{"object": object}})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

type cardEnv struct {
	*env
	lk      *fakePaymentsLK
	service *cards.Service
}

func newCardEnv(t *testing.T, production bool) cardEnv {
	t.Helper()
	lk, url := newFakePaymentsLK(t)
	st := storetest.New(t)
	service := &cards.Service{
		Store: st, BaseURL: url, Production: production, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		StayLink: func(slug string, id uuid.UUID, token string) string {
			return "https://" + slug + ".staykey.direct/stay/" + id.String() + "?t=" + token
		},
	}
	e := buildEnv(t, st, st, func(o *Options) { o.Cards = service })
	return cardEnv{env: e, lk: lk, service: service}
}

// cardVilla is a villa taking cards on its owner's own Payments.lk account, priced in USD at
// Rs 300.00 per dollar.
func (e cardEnv) cardVilla(phone string) villa {
	e.t.Helper()
	v := e.villa(phone)
	e.expect(e.do(call{method: http.MethodPut, path: "/v1/card-payments/connection", as: v.owner, account: v.account,
		body: map[string]any{"secretKey": ownerCardKey}}), http.StatusOK)
	rec := e.do(call{method: http.MethodPatch, path: "/v1/properties/" + v.p.Id.String(), as: v.owner, account: v.account,
		body: map[string]any{"payments": map[string]any{
			"bank":       map[string]any{"enabled": false, "bankName": "", "accountName": "", "accountNumber": "", "payWithinHours": 24, "cancelIfUnpaid": true},
			"atProperty": false, "cards": "on", "cardCharge": "full", "cardRate": 30000,
		}}})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.Property](e.t, rec).Payments.Cards; got != oapi.PaymentSettingsCardsOn {
		e.t.Fatalf("cards read back %s, want on", got)
	}
	return v
}

func (e cardEnv) cardCheckout(id uuid.UUID, token string, purpose string) *httptest.ResponseRecorder {
	return e.do(call{method: http.MethodPost, path: "/v1/public/bookings/" + id.String() + "/card-checkout",
		headers: map[string]string{"X-Booking-Token": token}, body: map[string]any{"purpose": purpose}})
}

func TestATestKeyIsRefusedInProduction(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, true)
	owner, account := e.owner("+94776000001", "Card account")
	rec := e.do(call{method: http.MethodPut, path: "/v1/card-payments/connection", as: owner, account: account,
		body: map[string]any{"secretKey": ownerCardKey}})
	e.expect(rec, http.StatusBadRequest)
	if got := decode[oapi.Error](t, rec).Code; got != "test_key_in_production" {
		t.Errorf("code %q, want test_key_in_production", got)
	}
}

func TestAKeyPaymentsLKRefusesIsInvalid(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	owner, account := e.owner("+94776000002", "Card account")
	rec := e.do(call{method: http.MethodPut, path: "/v1/card-payments/connection", as: owner, account: account,
		body: map[string]any{"secretKey": "sk_test_revoked_fixture-01"}})
	e.expect(rec, http.StatusBadRequest)
	if got := decode[oapi.Error](t, rec).Code; got != "invalid_key" {
		t.Errorf("code %q, want invalid_key", got)
	}
}

func TestAConnectedKeyShowsOnlyItsLastFour(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	owner, account := e.owner("+94776000003", "Card account")
	rec := e.do(call{method: http.MethodPut, path: "/v1/card-payments/connection", as: owner, account: account,
		body: map[string]any{"secretKey": ownerCardKey}})
	e.expect(rec, http.StatusOK)
	got := decode[oapi.CardPayments](t, rec)
	if got.Mode != oapi.CardModeOwn || got.Connection == nil || got.Connection.Last4 != ownerCardKey[len(ownerCardKey)-4:] || strings.Contains(rec.Body.String(), ownerCardKey) {
		t.Errorf("card payments %s", rec.Body.String())
	}
}

func TestManagersCantSeeCardPayments(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	owner, account := e.owner("+94776000004", "Card account")
	manager := e.member(account, owner, "+94776000005", domain.RoleManager)
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/card-payments", as: manager, account: account}), http.StatusForbidden)
}

func TestPayoutsCantBeChosenWhenTheyArentOffered(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	owner, account := e.owner("+94776000006", "Card account")
	rec := e.do(call{method: http.MethodPut, path: "/v1/card-payments/mode", as: owner, account: account, body: map[string]any{"mode": "payouts"}})
	e.expect(rec, http.StatusConflict)
}

func TestACardCheckoutWithAnotherStaysTokenIsNotFound(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	v := e.cardVilla("+94776000007")
	first := e.guestBook(v, "k1", map[string]any{"paymentMethod": "card"})
	second := e.guestBook(v, "k2", map[string]any{"paymentMethod": "card", "checkIn": future(40), "checkOut": future(42)})
	e.expect(e.cardCheckout(first.Booking.Id, second.Token, "full"), http.StatusNotFound)
}

func TestACardCheckoutWithCardsOffIsAConflict(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	v := e.villa("+94776000008")
	b := e.guestBook(v, "k1", nil)
	rec := e.cardCheckout(b.Booking.Id, b.Token, "full")
	e.expect(rec, http.StatusConflict)
	if got := decode[oapi.Error](t, rec).Code; got != "cards_off" {
		t.Errorf("code %q, want cards_off", got)
	}
}

func TestBookingByCardNeedsCardsOn(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	v := e.villa("+94776000009")
	rec := e.do(call{method: http.MethodPost, path: "/v1/public/properties/" + v.p.Slug + "/bookings",
		headers: map[string]string{"Idempotency-Key": "k1"}, body: map[string]any{
			"unitId": v.p.Units[0].Id, "checkIn": future(30), "checkOut": future(32), "adults": 2,
			"guest": map[string]any{"name": "Sam Lee", "phone": "+447700900000"}, "paymentMethod": "card", "source": "page",
		}})
	e.expect(rec, http.StatusBadRequest)
}

// The phase 9 flow against a stand-in Payments.lk: book by card, pay, come back, cancel with a
// card refund, and the refund event settles it.
func TestACardStayIsPaidConfirmedAndRefunded(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	v := e.cardVilla("+94776000010")
	page := e.do(call{method: http.MethodGet, path: "/v1/public/properties/" + v.p.Slug})
	e.expect(page, http.StatusOK)
	if card := decode[oapi.PublicProperty](t, page).Payments.Card; card == nil || !card.TestMode || deref(card.Rate) != 30000 {
		t.Fatalf("public card %+v", card)
	}

	b := e.guestBook(v, "k1", map[string]any{"paymentMethod": "card"})
	if b.Booking.Status != oapi.BookingStatusAwaitingPayment || b.Booking.PaymentDueAt == nil || b.Booking.Card == nil {
		t.Fatalf("booked %s, due %v, card %+v", b.Booking.Status, b.Booking.PaymentDueAt, b.Booking.Card)
	}
	rec := e.cardCheckout(b.Booking.Id, b.Token, "full")
	e.expect(rec, http.StatusCreated)
	c := decode[oapi.CardCheckout](t, rec)
	if c.Amount != 36_000 || c.Charged != 10_800_000 || c.ChargedCurrency != "LKR" || !c.TestMode {
		t.Fatalf("checkout %+v", c)
	}

	e.lk.pay("chk_1")
	rec = e.do(call{method: http.MethodPost, path: "/v1/public/bookings/" + b.Booking.Id.String() + "/card-checkout/check",
		headers: map[string]string{"X-Booking-Token": b.Token}})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.GuestBooking](t, rec); got.Status != oapi.BookingStatusConfirmed || got.Paid != 36_000 {
		t.Fatalf("after paying: %s, paid %d", got.Status, got.Paid)
	}

	stay := e.do(call{method: http.MethodGet, path: "/v1/bookings/" + b.Booking.Id.String(), as: v.owner, account: v.account})
	e.expect(stay, http.StatusOK)
	rec = e.do(call{method: http.MethodPost, path: "/v1/bookings/" + b.Booking.Id.String() + "/cancel", as: v.owner, account: v.account,
		headers: map[string]string{"Idempotency-Key": "cancel-1"},
		body:    map[string]any{"version": decode[oapi.Booking](t, stay).Version, "reason": "Guest asked", "refund": map[string]any{"method": "card", "amount": 18_000}}})
	e.expect(rec, http.StatusOK)
	refund := lastRefund(decode[oapi.Booking](t, rec))
	if refund == nil || deref(refund.Status) != oapi.PaymentStatusPending || deref(refund.Charged) != 5_400_000 || len(e.lk.refunds) != 1 {
		t.Fatalf("refund %+v, sent %d", refund, len(e.lk.refunds))
	}

	e.lk.queue("refund.succeeded", map[string]any{"id": "re_1", "paymentId": "pay_chk_1", "status": "succeeded", "amountCents": 5_400_000})
	if err := e.service.PollEvents(t.Context()); err != nil {
		t.Fatal(err)
	}
	stay = e.do(call{method: http.MethodGet, path: "/v1/bookings/" + b.Booking.Id.String(), as: v.owner, account: v.account})
	e.expect(stay, http.StatusOK)
	if refund := lastRefund(decode[oapi.Booking](t, stay)); refund == nil || deref(refund.Status) != oapi.PaymentStatusSucceeded {
		t.Errorf("after the event, refund %+v", refund)
	}
}

func TestARevokedKeyPausesCardsAndTellsTheOwner(t *testing.T) {
	t.Parallel()
	e := newCardEnv(t, false)
	v := e.cardVilla("+94776000011")
	e.lk.mu.Lock()
	e.lk.revoked = true
	e.lk.mu.Unlock()
	if err := e.service.PollEvents(t.Context()); err != nil {
		t.Fatal(err)
	}
	rec := e.do(call{method: http.MethodGet, path: "/v1/properties/" + v.p.Id.String(), as: v.owner, account: v.account})
	e.expect(rec, http.StatusOK)
	pay := decode[oapi.Property](t, rec).Payments
	lines := e.billingLines(v.owner, v.account)
	if pay.Cards != oapi.PaymentSettingsCardsPending || len(deref(pay.CardsMissing)) != 1 || len(lines) != 1 || lines[0] != "Payments.lk stopped accepting your key" {
		t.Errorf("cards %s, missing %v, lines %v", pay.Cards, deref(pay.CardsMissing), lines)
	}
}

func lastRefund(b oapi.Booking) *oapi.Payment {
	var out *oapi.Payment
	for _, p := range deref(b.Payments) {
		if p.Kind == oapi.PaymentKindRefund {
			out = &p
		}
	}
	return out
}
