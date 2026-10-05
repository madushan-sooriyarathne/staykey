package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

var noon = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func (h house) request(ctx context.Context, tb testing.TB, expires time.Time) domain.Booking {
	tb.Helper()
	in := domain.StayInput{UnitID: h.r1, CheckIn: day("2026-09-10"), CheckOut: day("2026-09-12"), Adults: 1,
		Guest: domain.Guest{Name: "Ravi Fernando"}, Source: "page"}
	b, err := h.s.CreateBooking(ctx, h.t, domain.NewBooking{
		PropertyID: h.p.ID, Stay: in, Status: domain.StatusRequested, Currency: "USD", RefPrefix: "TH",
		Price: domain.Pricing{Total: 24000}, LedgerUnits: []uuid.UUID{h.r1}, RequestExpiresAt: &expires,
	}, noon)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

func TestExpiredRequestIsDeclined(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b := h.request(ctx, t, noon.Add(-time.Minute))

	if _, err := h.s.DeclineExpiredRequests(ctx, noon); err != nil {
		t.Fatal(err)
	}

	if got, err := h.s.GetBooking(ctx, h.t, b.ID); err != nil || got.Status != domain.StatusDeclined {
		t.Errorf("status = %s (err %v), want declined", got.Status, err)
	}
}

func TestRequestWithinItsReplyWindowStays(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.request(ctx, t, noon.Add(time.Minute))

	n, err := h.s.DeclineExpiredRequests(ctx, noon)

	if err != nil || n != 0 {
		t.Errorf("declined %d (err %v), want none", n, err)
	}
}

func TestDecliningAnExpiredRequestFreesItsNights(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.request(ctx, t, noon.Add(-time.Minute))
	if _, err := h.s.DeclineExpiredRequests(ctx, noon); err != nil {
		t.Fatal(err)
	}

	if _, err := h.book(ctx, h.r1, "2026-09-10", "2026-09-12"); err != nil {
		t.Errorf("booking the freed nights: %v", err)
	}
}

func TestUnpaidStayIsCancelledAfterItsDeadline(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b := h.request(ctx, t, noon.Add(time.Hour))
	b, err := h.s.TransitionBooking(ctx, h.t, b.ID, b.Version, domain.StatusAwaitingPayment, "", noon)
	if err != nil || b.PaymentDueAt == nil {
		t.Fatalf("approve: due %v, err %v", b.PaymentDueAt, err)
	}

	if _, err := h.s.CancelUnpaidBookings(ctx, b.PaymentDueAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	if got, err := h.s.GetBooking(ctx, h.t, b.ID); err != nil || got.Status != domain.StatusCancelled {
		t.Errorf("status = %s (err %v), want cancelled", got.Status, err)
	}
}

func TestPaidStayIsNotCancelled(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b := h.request(ctx, t, noon.Add(time.Hour))
	b, err := h.s.TransitionBooking(ctx, h.t, b.ID, b.Version, domain.StatusAwaitingPayment, "", noon)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.RecordPayment(ctx, h.t, b.ID, domain.Payment{Method: "bank", Amount: 100, ReceivedAt: noon}); err != nil {
		t.Fatal(err)
	}

	n, err := h.s.CancelUnpaidBookings(ctx, noon.Add(30*24*time.Hour))

	if err != nil || n != 0 {
		t.Errorf("cancelled %d (err %v), want none", n, err)
	}
}
