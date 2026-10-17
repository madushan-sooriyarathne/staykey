package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/tenant"
)

// otherAccount is a second owner's account in the same database, with the house's owner added
// to its team as a manager.
func (h house) otherAccount(t *testing.T) tenant.Tenant {
	t.Helper()
	ctx := context.Background()
	owner := h.user(t, "+94779990001")
	acc, err := h.s.CreateAccount(ctx, owner.ID, "Someone else")
	if err != nil {
		t.Fatal(err)
	}
	other := tenant.Tenant{AccountID: acc.ID, UserID: owner.ID, MembershipID: acc.MembershipID, Role: domain.RoleOwner}
	if _, err := h.s.AddMember(ctx, other, h.t.UserID, domain.RoleManager, nil); err != nil {
		t.Fatal(err)
	}
	return other
}

func (h house) countRows(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := h.s.Pool().QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE account_id = $1", h.t.AccountID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDeletingAnOwnerTakesTheirBookingPageOffline(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.PublicProperty(ctx, h.p.Slug); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("public property after deletion = %v, want not found", err)
	}
}

func TestDeletionKeepsTheDataUntilThePurge(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.book(ctx, h.r1, "2026-11-01", "2026-11-03"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if n := h.countRows(t, "bookings"); n != 1 {
		t.Errorf("bookings during the 30 days = %d, want 1", n)
	}
}

func TestThePurgeDeletesEverythingAfter30Days(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.book(ctx, h.r1, "2026-11-01", "2026-11-03"); err != nil {
		t.Fatal(err)
	}
	deleted := time.Now()
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, deleted); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.PurgeDeletedUsers(ctx, deleted.AddDate(0, 0, domain.DeletionDays+1)); err != nil {
		t.Fatal(err)
	}
	if n := h.countRows(t, "bookings") + h.countRows(t, "guests") + h.countRows(t, "properties"); n != 0 {
		t.Errorf("%d rows left after the purge", n)
	}
}

func TestThePurgeWaitsFor30Days(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	deleted := time.Now()
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, deleted); err != nil {
		t.Fatal(err)
	}
	if n, err := h.s.PurgeDeletedUsers(ctx, deleted.AddDate(0, 0, domain.DeletionDays-1)); err != nil || n != 0 {
		t.Errorf("purged %d (err %v) before 30 days, want 0", n, err)
	}
}

func TestSigningInAgainRestoresTheAccount(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.RestoreUser(ctx, h.t.UserID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := h.status(ctx, t); got != domain.StatusTrial {
		t.Errorf("status after restoring = %s, want trial", got)
	}
}

func TestDeletingAManagerLeavesTheOtherAccount(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	other := h.otherAccount(t)
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, time.Now()); err != nil {
		t.Fatal(err)
	}
	members, _, err := h.s.ListTeam(ctx, other, time.Now())
	if err != nil || len(members) != 1 || members[0].UserID != other.UserID {
		t.Errorf("other account's team = %+v (err %v), want only its owner", members, err)
	}
}

func TestARenewingStorePlanBlocksDeletion(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	if err := h.s.ApplyStoreSubscriptions(ctx, rcEvent("evt-1"), h.t.AccountID, []domain.Subscription{storePlan(now.AddDate(0, 1, 0))}, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, now); !errors.Is(err, domain.ErrStorePlanRenews) {
		t.Errorf("deletion = %v, want ErrStorePlanRenews", err)
	}
}

func TestErasingAGuestClearsEveryStayAndActivity(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	first, err := h.guestBooking(ctx, nil, stranger, "token", domain.Pricing{Total: 24000})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.s.EraseGuest(ctx, h.t, first); err != nil {
		t.Fatal(err)
	}
	b, err := h.s.GetBooking(ctx, h.t, first)
	if err != nil || b.Guest.Name != domain.GuestRemovedName || b.Guest.Phone != "" {
		t.Errorf("guest after erasure = %+v (err %v)", b.Guest, err)
	}
	items, err := h.s.ListActivity(ctx, h.t, nil, 50)
	if err != nil || strings.Contains(strings.Join(titles(items), " | "), stranger.Name) {
		t.Errorf("activity still names the guest: %q (err %v)", titles(items), err)
	}
}

func TestErasingAGuestKeepsTheirPayments(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.book(ctx, h.r1, "2026-11-01", "2026-11-03")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.RecordPayment(ctx, h.t, b.ID, domain.Payment{Method: "cash", Amount: 5000, ReceivedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := h.s.EraseGuest(ctx, h.t, b.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := h.s.GetBooking(ctx, h.t, b.ID); err != nil || got.Paid() != 5000 {
		t.Errorf("paid after erasure = %d (err %v), want 5000", got.Paid(), err)
	}
}

func TestOnlyOwnersEraseGuests(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.book(ctx, h.r1, "2026-11-01", "2026-11-03")
	if err != nil {
		t.Fatal(err)
	}
	manager := h.t
	manager.Role = domain.RoleManager
	if err := h.s.EraseGuest(ctx, manager, b.ID); err == nil {
		t.Error("a manager erased a guest")
	}
}
