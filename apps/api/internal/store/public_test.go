package store_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

var checkout = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func (h house) hold(ctx context.Context, unit uuid.UUID, from, to string, expires time.Time) (uuid.UUID, error) {
	return h.s.CreateHold(ctx, h.t.AccountID, h.p.ID, domain.LedgerUnits(h.p.Units, unit), day(from), day(to), expires, checkout, nil)
}

func (h house) guestBooking(ctx context.Context, holdID *uuid.UUID, guest domain.Guest, token string, price domain.Pricing) (uuid.UUID, error) {
	hash := sha256.Sum256([]byte(token))
	return h.s.CreateGuestBooking(ctx, h.t.AccountID, domain.NewBooking{
		PropertyID: h.p.ID,
		Stay: domain.StayInput{UnitID: h.r1, CheckIn: day("2026-10-10"), CheckOut: day("2026-10-12"), Adults: 2,
			Guest: guest, Source: "page"},
		Status: domain.StatusAwaitingPayment, Currency: "USD", RefPrefix: "TH", Price: price,
		LedgerUnits: []uuid.UUID{h.r1}, HoldID: holdID, PaymentMethod: "bank", GuestTokenHash: hash[:],
	}, checkout)
}

var stranger = domain.Guest{Name: "Sam Lee", Phone: "+447700900000"}

func TestAHoldBlocksASecondHold(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.hold(ctx, h.r1, "2026-10-10", "2026-10-12", checkout.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, err := h.hold(ctx, h.whole, "2026-10-11", "2026-10-13", checkout.Add(15*time.Minute))
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) || conflict.BookingID != nil || conflict.GuestName != "" {
		t.Errorf("second hold = %v, want a conflict naming nobody", err)
	}
}

func TestALapsedHoldDoesntBlockABooking(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.hold(ctx, h.r1, "2026-10-10", "2026-10-12", checkout.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.guestBooking(ctx, nil, stranger, "token", domain.Pricing{Total: 24000}); err != nil {
		t.Errorf("booking over a lapsed hold: %v", err)
	}
}

func TestBookingWithItsHoldTakesTheNights(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	holdID, err := h.hold(ctx, h.r1, "2026-10-10", "2026-10-12", checkout.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.guestBooking(ctx, &holdID, stranger, "token", domain.Pricing{Total: 24000}); err != nil {
		t.Fatal(err)
	}
	_, err = h.book(ctx, h.r1, "2026-10-11", "2026-10-12")
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) || conflict.BookingID == nil {
		t.Errorf("owner booking over the guest's stay = %v, want a clash with it", err)
	}
}

func TestAUsedUpPromoIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	p, err := h.s.UpdateProperty(ctx, h.t, h.p.ID, domain.PropertyPatch{Promos: &[]domain.PromoInput{
		{Ref: "new", Promo: domain.Promo{Code: "ONCE", Kind: "percent", Amount: 10, Limit: 1}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	price := domain.Pricing{Total: 21600, PromoID: &p.Promos[0].ID}
	if _, err := h.guestBooking(ctx, nil, stranger, "first", price); err != nil {
		t.Fatal(err)
	}
	h2 := domain.Guest{Name: "Ama Silva", Phone: "+94771112222"}
	hash := sha256.Sum256([]byte("second"))
	_, err = h.s.CreateGuestBooking(ctx, h.t.AccountID, domain.NewBooking{
		PropertyID: h.p.ID,
		Stay: domain.StayInput{UnitID: h.r2, CheckIn: day("2026-10-10"), CheckOut: day("2026-10-12"), Adults: 2,
			Guest: h2, Source: "page"},
		Status: domain.StatusAwaitingPayment, Currency: "USD", RefPrefix: "TH", Price: price,
		LedgerUnits: []uuid.UUID{h.r2}, PaymentMethod: "bank", GuestTokenHash: hash[:],
	}, checkout)
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Field != "promo" {
		t.Errorf("second use = %v, want a promo error", err)
	}
}

func TestAWrongTokenIsNotFound(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	id, err := h.guestBooking(ctx, nil, stranger, "right", domain.Pricing{Total: 24000})
	if err != nil {
		t.Fatal(err)
	}
	wrong := sha256.Sum256([]byte("wrong"))
	if _, err := h.s.GuestBooking(ctx, id, wrong[:]); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GuestBooking with a wrong token = %v, want not found", err)
	}
}

func TestOwnerBookingsArentOpenToGuests(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.book(ctx, h.r1, "2026-10-10", "2026-10-12")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.GuestBooking(ctx, b.ID, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GuestBooking of an owner's stay = %v, want not found", err)
	}
}

func TestAGuestBookingLeavesAnExistingGuestAsItWas(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	owners, err := h.book(ctx, h.r2, "2026-09-10", "2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	impostor := domain.Guest{Name: "Someone Else", Phone: owners.Guest.Phone}
	if _, err := h.guestBooking(ctx, nil, impostor, "token", domain.Pricing{Total: 24000}); err != nil {
		t.Fatal(err)
	}
	again, err := h.s.GetBooking(ctx, h.t, owners.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Guest.Name != "Nimali Perera" {
		t.Errorf("guest name = %q, want it unchanged", again.Guest.Name)
	}
}

func TestAGuestSlipTellsTheTeam(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	id, err := h.guestBooking(ctx, nil, stranger, "token", domain.Pricing{Total: 24000})
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("token"))
	key := "accounts/" + h.t.AccountID.String() + "/slips/" + uuid.New().String() + ".jpg"
	if err := h.s.AddGuestSlip(ctx, id, hash[:], key, 12000, checkout); err != nil {
		t.Fatal(err)
	}
	items, err := h.s.ListActivity(ctx, h.t, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Title != "Bank slip from Sam Lee" {
		t.Errorf("latest activity = %q", titles(items))
	}
}

func TestAGuestSlipFromAnotherAccountIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	id, err := h.guestBooking(ctx, nil, stranger, "token", domain.Pricing{Total: 24000})
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("token"))
	key := "accounts/" + uuid.New().String() + "/slips/" + uuid.New().String() + ".jpg"
	var invalid *domain.ValidationError
	if err := h.s.AddGuestSlip(ctx, id, hash[:], key, 12000, checkout); !errors.As(err, &invalid) {
		t.Errorf("AddGuestSlip with another account's key = %v, want a validation error", err)
	}
}

func TestAvailabilityLeavesOutLapsedHolds(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.hold(ctx, h.r1, "2026-10-10", "2026-10-12", checkout.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	taken, err := h.s.Availability(ctx, h.t.AccountID, h.p.ID, day("2026-10-01"), day("2026-11-01"), checkout)
	if err != nil {
		t.Fatal(err)
	}
	if len(taken) != 0 {
		t.Errorf("taken = %v, want nothing", taken)
	}
}
