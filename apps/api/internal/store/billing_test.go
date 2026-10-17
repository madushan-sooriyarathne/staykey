package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store"
)

func (h house) status(ctx context.Context, tb testing.TB) domain.AccountStatus {
	tb.Helper()
	accounts, err := h.s.ListAccounts(ctx, h.t.UserID)
	if err != nil || len(accounts) != 1 {
		tb.Fatalf("list accounts: %v %v", accounts, err)
	}
	return accounts[0].Status
}

func (h house) billingTitles(ctx context.Context, tb testing.TB) string {
	tb.Helper()
	items, err := h.s.ListActivity(ctx, h.t, nil, 50)
	if err != nil {
		tb.Fatal(err)
	}
	var out []string
	for _, a := range items {
		if a.Kind == domain.ActivityBilling {
			out = append(out, a.Title)
		}
	}
	return strings.Join(out, " | ")
}

func storePlan(end time.Time) domain.Subscription {
	return domain.Subscription{Source: domain.SourceAppStore, Plan: domain.PlanGuesthouse, Period: domain.PeriodMonthly,
		Status: domain.SubActive, WillRenew: true, PeriodEnd: end, ProductID: "staykey_guesthouse_monthly"}
}

func rcEvent(id string) store.BillingEvent {
	return store.BillingEvent{Source: "revenuecat", EventID: id, Type: "INITIAL_PURCHASE"}
}

func lkEvent(id string) store.BillingEvent {
	return store.BillingEvent{Source: "paymentslk", EventID: id, Type: "payment.succeeded"}
}

func TestAGuestBookingEndsTheFreePeriod(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.guestBooking(ctx, nil, stranger, "token", domain.Pricing{Total: 24000}); err != nil {
		t.Fatal(err)
	}
	if got := h.billingTitles(ctx, t); got != "Your first direct booking is in" {
		t.Errorf("billing activity = %q", got)
	}
}

func TestADeclinedRequestLeavesTheFreePeriod(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b := h.request(ctx, t, time.Now().Add(time.Hour))
	if _, err := h.s.TransitionBooking(ctx, h.t, b.ID, b.Version, domain.StatusDeclined, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := h.status(ctx, t); got != domain.StatusTrial {
		t.Errorf("status = %s, want trial", got)
	}
}

func TestAStorePlanMakesTheAccountActiveOnce(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	for _, id := range []string{"evt-1", "evt-2"} {
		if err := h.s.ApplyStoreSubscriptions(ctx, rcEvent(id), h.t.AccountID, []domain.Subscription{storePlan(now.AddDate(0, 1, 0))}, now, now); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.billingTitles(ctx, t); got != "Guesthouse plan active" {
		t.Errorf("billing activity = %q", got)
	}
}

func TestARepeatedDeliveryIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	plan := []domain.Subscription{storePlan(now.AddDate(0, 1, 0))}
	if err := h.s.ApplyStoreSubscriptions(ctx, rcEvent("evt-1"), h.t.AccountID, plan, now, now); err != nil {
		t.Fatal(err)
	}
	err := h.s.ApplyStoreSubscriptions(ctx, rcEvent("evt-1"), h.t.AccountID, plan, now, now)
	if !errors.Is(err, store.ErrDuplicateEvent) {
		t.Errorf("second delivery = %v, want ErrDuplicateEvent", err)
	}
}

func TestAnOlderEventDoesntUndoANewerOne(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	newer := now.AddDate(0, 2, 0)
	if err := h.s.ApplyStoreSubscriptions(ctx, rcEvent("new"), h.t.AccountID, []domain.Subscription{storePlan(newer)}, now, now); err != nil {
		t.Fatal(err)
	}
	if err := h.s.ApplyStoreSubscriptions(ctx, rcEvent("old"), h.t.AccountID, []domain.Subscription{storePlan(now.AddDate(0, 1, 0))}, now.Add(-time.Hour), now); err != nil {
		t.Fatal(err)
	}
	b, err := h.s.AccountBilling(ctx, h.t.AccountID, now)
	if err != nil || !b.Sub.PeriodEnd.Equal(newer.Truncate(time.Microsecond)) {
		t.Errorf("period end = %v (err %v), want %v", b.Sub.PeriodEnd, err, newer)
	}
}

func (h house) payWeb(ctx context.Context, tb testing.TB, eventID string, now time.Time) domain.PaidPeriod {
	tb.Helper()
	id, amount, err := h.s.CreateWebCheckout(ctx, h.t, domain.PlanVilla, domain.PeriodMonthly, now)
	if err != nil {
		tb.Fatal(err)
	}
	paid, err := h.s.PayWebCheckout(ctx, lkEvent(eventID), store.WebPayment{CheckoutID: id, PaymentID: "pay_" + eventID, Amount: amount, Currency: "LKR"}, now)
	if err != nil {
		tb.Fatal(err)
	}
	return paid
}

func TestASecondWebPeriodStartsWhenTheFirstEnds(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)
	first := h.payWeb(ctx, t, "evt-1", now)
	second := h.payWeb(ctx, t, "evt-2", now)
	if !second.Start.Equal(first.End) || !second.End.Equal(first.End.AddDate(0, 1, 0)) {
		t.Errorf("second period %v to %v, want it to follow %v", second.Start, second.End, first.End)
	}
}

func TestASecondPaymentEventForACheckoutChangesNothing(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	id, amount, err := h.s.CreateWebCheckout(ctx, h.t, domain.PlanVilla, domain.PeriodYearly, now)
	if err != nil {
		t.Fatal(err)
	}
	pay := store.WebPayment{CheckoutID: id, PaymentID: "pay_1", Amount: amount, Currency: "LKR"}
	if _, err := h.s.PayWebCheckout(ctx, lkEvent("evt-1"), pay, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.PayWebCheckout(ctx, lkEvent("evt-2"), pay, now); !errors.Is(err, store.ErrDuplicateEvent) {
		t.Errorf("second payment = %v, want ErrDuplicateEvent", err)
	}
}

func TestAPaymentForTheWrongAmountIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	id, amount, err := h.s.CreateWebCheckout(ctx, h.t, domain.PlanVilla, domain.PeriodMonthly, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.s.PayWebCheckout(ctx, lkEvent("evt-1"), store.WebPayment{CheckoutID: id, PaymentID: "pay_1", Amount: amount - 1, Currency: "LKR"}, now)
	if !errors.Is(err, store.ErrPaymentMismatch) {
		t.Errorf("payment = %v, want ErrPaymentMismatch", err)
	}
}

func TestAWebCheckoutIsRefusedWhileAStorePlanRuns(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	if err := h.s.ApplyStoreSubscriptions(ctx, rcEvent("evt-1"), h.t.AccountID, []domain.Subscription{storePlan(now.AddDate(0, 1, 0))}, now, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.s.CreateWebCheckout(ctx, h.t, domain.PlanVilla, domain.PeriodMonthly, now); !errors.Is(err, domain.ErrAlreadySubscribed) {
		t.Errorf("checkout = %v, want ErrAlreadySubscribed", err)
	}
}

func TestTheBillingJobLapsesAnAccountPastGrace(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.s.RecomputeAllBilling(ctx, time.Now().AddDate(0, 0, domain.FreeDays+domain.GraceDays+1)); err != nil {
		t.Fatal(err)
	}
	if got := h.status(ctx, t); got != domain.StatusLapsed {
		t.Errorf("status = %s, want lapsed", got)
	}
}

func TestBookableUnitsCountRoomsNotTheWholeHouse(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	b, err := h.s.AccountBilling(context.Background(), h.t.AccountID, time.Now())
	if err != nil || b.Units != 2 {
		t.Errorf("units = %d (err %v), want 2", b.Units, err)
	}
}
