package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/tenant"
)

func countBookings(ctx context.Context, tb testing.TB, h house) int {
	tb.Helper()
	var n int
	if err := h.s.Pool().QueryRow(ctx, "SELECT count(*) FROM bookings").Scan(&n); err != nil {
		tb.Fatal(err)
	}
	return n
}

func TestRowSecurityHidesAnotherAccountsRows(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	if _, err := h.book(context.Background(), h.r1, "2026-11-01", "2026-11-03"); err != nil {
		t.Fatal(err)
	}
	other := tenant.With(context.Background(), tenant.Tenant{AccountID: uuid.New()})
	if n := countBookings(other, t, h); n != 0 {
		t.Errorf("another tenant sees %d bookings, want 0", n)
	}
}

func TestRowSecurityLetsRequestsWithoutATenantThrough(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.book(ctx, h.r1, "2026-11-01", "2026-11-03"); err != nil {
		t.Fatal(err)
	}
	other := tenant.With(ctx, tenant.Tenant{AccountID: uuid.New()})
	countBookings(other, t, h) // leaves the other account's id on a pooled connection
	if n := countBookings(ctx, t, h); n != 1 {
		t.Errorf("without a tenant %d bookings, want 1", n)
	}
}

func TestRowSecurityRefusesWritesIntoAnotherAccount(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	other := tenant.With(context.Background(), tenant.Tenant{AccountID: uuid.New()})
	_, err := h.s.Pool().Exec(other, `INSERT INTO activity (id, account_id, kind, title) VALUES ($1, $2, 'booking', 'x')`,
		domain.NewID(), h.t.AccountID)
	if err == nil {
		t.Error("insert into another account succeeded")
	}
}

func TestRowSecurityLetsATenantSeeItsOwnRows(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := tenant.With(context.Background(), h.t)
	b, err := h.book(ctx, h.r1, "2026-11-01", "2026-11-03")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.GetBooking(ctx, h.t, b.ID); err != nil {
		t.Errorf("own booking: %v", err)
	}
}
