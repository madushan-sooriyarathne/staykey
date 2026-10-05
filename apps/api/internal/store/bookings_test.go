package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/store/storetest"
	"staykey.direct/api/internal/tenant"
)

// house is a property with a whole-house unit that books both its rooms.
type house struct {
	s             *store.Postgres
	t             tenant.Tenant
	p             domain.PropertyDetail
	whole, r1, r2 uuid.UUID
}

func newHouse(tb testing.TB) house {
	tb.Helper()
	ctx := context.Background()
	s := storetest.New(tb)
	u, _, err := s.UpsertUserByPhone(ctx, "+94771234567")
	if err != nil {
		tb.Fatal(err)
	}
	acc, err := s.CreateAccount(ctx, u.ID, "Test stays")
	if err != nil {
		tb.Fatal(err)
	}
	t := tenant.Tenant{AccountID: acc.ID, UserID: u.ID, MembershipID: acc.MembershipID, Role: domain.RoleOwner}
	p, err := s.CreateProperty(ctx, t, domain.NewProperty{
		Slug: "test-house", Name: "Test House", BookingType: domain.BookingRooms, Currency: "USD",
		Setup: domain.PropertyPatch{Units: &[]domain.UnitInput{
			{Ref: "whole", Name: "Whole house", Sleeps: 6, Rate: 30000, LinkedUnitRef: []string{"r1", "r2"}},
			{Ref: "r1", Name: "Room 1", Sleeps: 2, Rate: 12000},
			{Ref: "r2", Name: "Room 2", Sleeps: 2, Rate: 12000},
		}},
	})
	if err != nil {
		tb.Fatal(err)
	}
	h := house{s: s, t: t, p: p}
	for _, u := range p.Units {
		switch u.Name {
		case "Whole house":
			h.whole = u.ID
		case "Room 1":
			h.r1 = u.ID
		case "Room 2":
			h.r2 = u.ID
		}
	}
	return h
}

func (h house) book(ctx context.Context, unit uuid.UUID, from, to string) (domain.Booking, error) {
	in := domain.StayInput{UnitID: unit, CheckIn: day(from), CheckOut: day(to), Adults: 2,
		Guest: domain.Guest{Name: "Nimali Perera", Phone: "+94770000000"}, Source: "whatsapp"}
	return h.s.CreateBooking(ctx, h.t, domain.NewBooking{
		PropertyID: h.p.ID, Stay: in, Status: domain.StatusConfirmed, Currency: "USD", RefPrefix: "TH",
		Price:       domain.Pricing{Lines: []domain.PriceLine{{Label: "2 nights", Amount: 24000}}, Total: 24000},
		LedgerUnits: domain.LedgerUnits(h.p.Units, unit),
	}, time.Now())
}

// The phase 3 gate: of 50 bookings racing for overlapping nights on a room and the whole house
// that contains it, exactly one gets through.
func TestOneWinnerOutOfFiftyParallelBookings(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	var wins, clashes atomic.Int32
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unit, from := h.r1, fmt.Sprintf("2026-09-%02d", 10+i%3)
			if i%2 == 0 {
				unit = h.whole
			}
			_, err := h.book(context.Background(), unit, from, "2026-09-14")
			var conflict *domain.ConflictError
			switch {
			case err == nil:
				wins.Add(1)
			case errors.As(err, &conflict):
				clashes.Add(1)
			default:
				t.Errorf("booking %d: %v", i, err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 || clashes.Load() != 49 {
		t.Errorf("wins = %d, clashes = %d, want 1 and 49", wins.Load(), clashes.Load())
	}
}

func TestTwoRoomsShareANight(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.book(ctx, h.r1, "2026-09-10", "2026-09-12"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.book(ctx, h.r2, "2026-09-10", "2026-09-12"); err != nil {
		t.Errorf("second room: %v", err)
	}
}

func TestClashNamesTheStayHoldingTheNight(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	first, err := h.book(ctx, h.r2, "2026-09-11", "2026-09-13")
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.book(ctx, h.whole, "2026-09-10", "2026-09-12")
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) || conflict.Ref != first.Ref {
		t.Errorf("err = %v, want a clash with %s", err, first.Ref)
	}
}

func TestCheckOutDayIsFreeForTheNextArrival(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.book(ctx, h.whole, "2026-09-10", "2026-09-12"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.book(ctx, h.r1, "2026-09-12", "2026-09-13"); err != nil {
		t.Errorf("arrival on check-out day: %v", err)
	}
}

func TestCancellingFreesTheNights(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.book(ctx, h.whole, "2026-09-10", "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.CancelBooking(ctx, h.t, b.ID, b.Version, "Plans changed", nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.book(ctx, h.r1, "2026-09-10", "2026-09-12"); err != nil {
		t.Errorf("rebooking after cancel: %v", err)
	}
}

func TestStaleVersionIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.book(ctx, h.whole, "2026-09-10", "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.TransitionBooking(ctx, h.t, b.ID, b.Version, domain.StatusCheckedIn, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	_, err = h.s.TransitionBooking(ctx, h.t, b.ID, b.Version, domain.StatusCheckedOut, "", time.Now())
	if !errors.Is(err, domain.ErrStale) {
		t.Errorf("err = %v, want ErrStale", err)
	}
}

func TestEditMovesTheNights(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.book(ctx, h.r1, "2026-09-10", "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	stay := domain.StayInput{UnitID: h.r1, CheckIn: day("2026-09-20"), CheckOut: day("2026-09-22"), Adults: 2,
		Guest: b.Guest, Source: b.Source}
	if _, err := h.s.UpdateBooking(ctx, h.t, b.ID, domain.BookingChange{
		Version: b.Version, Stay: stay, Price: domain.Pricing{Total: 1}, LedgerUnits: []uuid.UUID{h.r1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.book(ctx, h.whole, "2026-09-10", "2026-09-12"); err != nil {
		t.Errorf("old nights still held: %v", err)
	}
}

func TestRefundCantExceedWhatWasPaid(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.book(ctx, h.whole, "2026-09-10", "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	if b, err = h.s.RecordPayment(ctx, h.t, b.ID, domain.Payment{Method: "cash", Amount: 5000, ReceivedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_, err = h.s.CancelBooking(ctx, h.t, b.ID, b.Version, "No show", &domain.Payment{Method: "cash", Amount: 5001}, time.Now())
	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Errorf("err = %v, want a validation error", err)
	}
}

func TestPaymentConfirmsAStayAwaitingIt(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	in := domain.StayInput{UnitID: h.r1, CheckIn: day("2026-09-10"), CheckOut: day("2026-09-12"), Adults: 1,
		Guest: domain.Guest{Name: "Ravi Fernando"}, Source: "page"}
	b, err := h.s.CreateBooking(ctx, h.t, domain.NewBooking{
		PropertyID: h.p.ID, Stay: in, Status: domain.StatusRequested, Currency: "USD", RefPrefix: "TH",
		Price: domain.Pricing{Total: 24000}, LedgerUnits: []uuid.UUID{h.r1},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if b, err = h.s.TransitionBooking(ctx, h.t, b.ID, b.Version, domain.StatusAwaitingPayment, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	b, err = h.s.RecordPayment(ctx, h.t, b.ID, domain.Payment{Method: "bank", Amount: 7200, ReceivedAt: time.Now()})
	if err != nil || b.Status != domain.StatusConfirmed {
		t.Errorf("status = %s (err %v), want confirmed", b.Status, err)
	}
}

func TestIdempotentReplaysTheFirstReply(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	runs := 0
	run := func(context.Context) ([]byte, error) { runs++; return fmt.Appendf(nil, `{"run":%d}`, runs), nil }
	if _, err := h.s.Idempotent(ctx, h.t, "key-1", []byte("hash"), 201, time.Now(), run); err != nil {
		t.Fatal(err)
	}
	reply, err := h.s.Idempotent(ctx, h.t, "key-1", []byte("hash"), 201, time.Now(), run)
	if err != nil || string(reply) != `{"run": 1}` {
		t.Errorf("reply = %s (err %v), want the first run's", reply, err)
	}
}

func TestIdempotencyKeyWithAnotherRequestIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	run := func(context.Context) ([]byte, error) { return []byte(`{}`), nil }
	if _, err := h.s.Idempotent(ctx, h.t, "key-1", []byte("hash"), 201, time.Now(), run); err != nil {
		t.Fatal(err)
	}
	_, err := h.s.Idempotent(ctx, h.t, "key-1", []byte("other"), 201, time.Now(), run)
	if !errors.Is(err, domain.ErrKeyReused) {
		t.Errorf("err = %v, want ErrKeyReused", err)
	}
}

func TestBlockHoldsTheRoomsOfAWholeHouse(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b := domain.Block{PropertyID: h.p.ID, UnitIDs: []uuid.UUID{h.whole}, From: day("2026-09-10"), To: day("2026-09-12"), Reason: "maintenance"}
	if _, err := h.s.CreateBlock(ctx, h.t, b, domain.LedgerUnits(h.p.Units, h.whole)); err != nil {
		t.Fatal(err)
	}
	_, err := h.book(ctx, h.r2, "2026-09-11", "2026-09-13")
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) || conflict.BlockID == nil {
		t.Errorf("err = %v, want a clash with the block", err)
	}
}

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}
