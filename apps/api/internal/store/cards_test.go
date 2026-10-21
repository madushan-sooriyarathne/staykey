package store_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/tenant"
)

const cardKey = "sk_test_fixture-key-0001"

// cardHouse is the test house taking cards on its own Payments.lk account, priced in USD at
// Rs 300.00 per dollar with a 30% deposit.
func cardHouse(tb testing.TB) house {
	tb.Helper()
	h := newHouse(tb)
	ctx := context.Background()
	deposit := 30
	if _, err := h.s.UpdateProperty(ctx, h.t, h.p.ID, domain.PropertyPatch{
		DepositPercent: &deposit,
		Payments: &domain.PaymentSettings{
			Bank:  domain.BankDetails{PayWithinHours: 24},
			Cards: "on", CardCharge: domain.CardChargeDeposit, CardRate: 30_000,
		},
	}); err != nil {
		tb.Fatal(err)
	}
	if _, err := h.s.ConnectCards(ctx, h.t, cardKey, "test", checkout); err != nil {
		tb.Fatal(err)
	}
	return h
}

// cardBooking books Room 1 for two nights by card, $240.00 in all.
func (h house) cardBooking(ctx context.Context, tb testing.TB, token string) uuid.UUID {
	tb.Helper()
	hash := sha256.Sum256([]byte(token))
	due := checkout.Add(time.Hour)
	id, err := h.s.CreateGuestBooking(ctx, h.t.AccountID, domain.NewBooking{
		PropertyID: h.p.ID,
		Stay: domain.StayInput{UnitID: h.r1, CheckIn: day("2026-10-10"), CheckOut: day("2026-10-12"), Adults: 2,
			Guest: stranger, Source: "page"},
		Status: domain.StatusAwaitingPayment, Currency: "USD", RefPrefix: "TH", Price: domain.Pricing{Total: 24_000},
		LedgerUnits: []uuid.UUID{h.r1}, PaymentMethod: "card", PaymentDueAt: &due, GuestTokenHash: hash[:],
	}, checkout)
	if err != nil {
		tb.Fatal(err)
	}
	return id
}

// openCheckout starts a card checkout and gives it a Payments.lk id, as the cards service does.
func (h house) openCheckout(ctx context.Context, tb testing.TB, bookingID uuid.UUID, token string, purpose domain.CardPurpose, providerID string) store.CheckoutStart {
	tb.Helper()
	hash := sha256.Sum256([]byte(token))
	start, err := h.s.StartCardCheckout(ctx, bookingID, hash[:], purpose, checkout)
	if err != nil {
		tb.Fatal(err)
	}
	if err := h.s.SetCardCheckoutProvider(ctx, start.ID, providerID); err != nil {
		tb.Fatal(err)
	}
	return start
}

func (h house) paid(providerCheckout, payment string, cents int64) store.CardEvent {
	return store.CardEvent{
		Source: store.CardSource(h.t.AccountID), ID: "evt_" + payment, Type: "payment.succeeded", Mode: "test",
		CheckoutID: providerCheckout, PaymentID: payment, AmountCents: cents,
	}
}

func (h house) stay(ctx context.Context, tb testing.TB, id uuid.UUID) domain.Booking {
	tb.Helper()
	b, err := h.s.GetBooking(ctx, h.t, id)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

func TestACardCheckoutChargesTheDepositInRupees(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	start := h.openCheckout(ctx, t, h.cardBooking(ctx, t, "tok"), "tok", domain.PurposeDeposit, "chk_1")
	if start.Amount != 7_200 || start.Charged != 2_160_000 || start.Collector.Key != cardKey {
		t.Errorf("checkout = %d USD cents, Rs %d cents, key %q", start.Amount, start.Charged, start.Collector.Key)
	}
}

func TestAnOpenCheckoutIsReused(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	first := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	hash := sha256.Sum256([]byte("tok"))
	again, err := h.s.StartCardCheckout(ctx, id, hash[:], domain.PurposeDeposit, checkout.Add(time.Minute))
	if err != nil || again.ID != first.ID || again.ProviderID != "chk_1" {
		t.Errorf("second start = %+v, %v; want checkout %s again", again, err, first.ID)
	}
}

func TestAPaymentReportedTwiceIsRecordedAndConfirmsOnce(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	start := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	ev := h.paid("chk_1", "pay_1", start.Charged)
	check := ev
	check.ID = ""
	for _, e := range []store.CardEvent{check, ev, ev} {
		if _, err := h.s.ApplyCardEvent(ctx, e, checkout); err != nil {
			t.Fatal(err)
		}
	}
	b := h.stay(ctx, t, id)
	var confirmations int
	if err := h.s.Pool().QueryRow(ctx, "SELECT count(*) FROM booking_events WHERE booking_id = $1 AND to_status = 'confirmed'", id).Scan(&confirmations); err != nil {
		t.Fatal(err)
	}
	if len(b.Payments) != 1 || b.Status != domain.StatusConfirmed || confirmations != 1 || b.Payments[0].Charged != start.Charged {
		t.Errorf("%d payments, status %s, %d confirmations", len(b.Payments), b.Status, confirmations)
	}
}

func TestAPaymentOfTheWrongAmountChangesNothing(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	start := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	outcome, err := h.s.ApplyCardEvent(ctx, h.paid("chk_1", "pay_1", start.Charged-1), checkout)
	b := h.stay(ctx, t, id)
	if err != nil || outcome != store.CardAmountMismatch || len(b.Payments) != 0 || b.Status != domain.StatusAwaitingPayment {
		t.Errorf("outcome %s, %v; %d payments, status %s", outcome, err, len(b.Payments), b.Status)
	}
}

func TestAPaymentReportedByAnotherKeyIsNotOurs(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	start := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	ev := h.paid("chk_1", "pay_1", start.Charged)
	ev.Source = store.StayKeyCardSource
	outcome, err := h.s.ApplyCardEvent(ctx, ev, checkout)
	if err != nil || outcome != store.CardNotOurs || len(h.stay(ctx, t, id).Payments) != 0 {
		t.Errorf("outcome %s, %v", outcome, err)
	}
}

func TestAnExpiredCheckoutLeavesTheStayToBeCancelledUnpaid(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	if _, err := h.s.ApplyCardEvent(ctx, store.CardEvent{
		Source: store.CardSource(h.t.AccountID), ID: "evt_x", Type: "checkout.expired", Mode: "test", CheckoutID: "chk_1",
	}, checkout); err != nil {
		t.Fatal(err)
	}
	n, err := h.s.CancelUnpaidBookings(ctx, checkout.Add(61*time.Minute))
	if err != nil || n != 1 {
		t.Fatalf("cancelled %d, %v", n, err)
	}
	if _, err := h.book(ctx, h.r1, "2026-10-10", "2026-10-12"); err != nil {
		t.Errorf("booking the freed nights: %v", err)
	}
}

func TestABalanceCheckoutCoversWhatIsLeft(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	start := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	if _, err := h.s.ApplyCardEvent(ctx, h.paid("chk_1", "pay_1", start.Charged), checkout); err != nil {
		t.Fatal(err)
	}
	balance := h.openCheckout(ctx, t, id, "tok", domain.PurposeBalance, "chk_2")
	if balance.Amount != 16_800 || balance.Charged != 5_040_000 {
		t.Errorf("balance checkout = %d USD cents, Rs %d cents; want 16800 and 5040000", balance.Amount, balance.Charged)
	}
}

func TestACardRefundOverTwoPaymentsMakesTwoPendingRefunds(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	deposit := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	if _, err := h.s.ApplyCardEvent(ctx, h.paid("chk_1", "pay_1", deposit.Charged), checkout); err != nil {
		t.Fatal(err)
	}
	balance := h.openCheckout(ctx, t, id, "tok", domain.PurposeBalance, "chk_2")
	if _, err := h.s.ApplyCardEvent(ctx, h.paid("chk_2", "pay_2", balance.Charged), checkout); err != nil {
		t.Fatal(err)
	}
	b := h.stay(ctx, t, id)
	refund := &domain.Payment{Method: "card", Amount: 20_000}
	b, err := h.s.CancelBooking(ctx, h.t, id, b.Version, "Plans changed", refund, checkout)
	if err != nil {
		t.Fatal(err)
	}
	var pending []int64
	for _, p := range b.Payments {
		if p.Kind == "refund" && p.Status == "pending" {
			pending = append(pending, p.Amount)
		}
	}
	if len(pending) != 2 || pending[0]+pending[1] != 20_000 || b.Paid() != 4_000 {
		t.Errorf("pending refunds %v, paid %d", pending, b.Paid())
	}
}

func TestARefundEventSettlesTheRefund(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	start := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	if _, err := h.s.ApplyCardEvent(ctx, h.paid("chk_1", "pay_1", start.Charged), checkout); err != nil {
		t.Fatal(err)
	}
	b := h.stay(ctx, t, id)
	if _, err := h.s.CancelBooking(ctx, h.t, id, b.Version, "", &domain.Payment{Method: "card", Amount: 7_200}, checkout); err != nil {
		t.Fatal(err)
	}
	refunds, err := h.s.ClaimCardRefunds(ctx, &id, 10, checkout)
	if err != nil || len(refunds) != 1 || refunds[0].PaymentRef != "pay_1" || refunds[0].Charged != start.Charged {
		t.Fatalf("claimed %+v, %v", refunds, err)
	}
	if err := h.s.AcceptCardRefund(ctx, refunds[0].ID, "re_1", checkout); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.ApplyCardEvent(ctx, store.CardEvent{
		Source: store.CardSource(h.t.AccountID), ID: "evt_r", Type: "refund.succeeded", Mode: "test", RefundID: "re_1",
	}, checkout); err != nil {
		t.Fatal(err)
	}
	for _, p := range h.stay(ctx, t, id).Payments {
		if p.Kind == "refund" && p.Status != "succeeded" {
			t.Errorf("refund status %s, want succeeded", p.Status)
		}
	}
}

func TestAFailedRefundCountsBackAndTellsTheTeam(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	start := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	if _, err := h.s.ApplyCardEvent(ctx, h.paid("chk_1", "pay_1", start.Charged), checkout); err != nil {
		t.Fatal(err)
	}
	b := h.stay(ctx, t, id)
	if _, err := h.s.CancelBooking(ctx, h.t, id, b.Version, "", &domain.Payment{Method: "card", Amount: 7_200}, checkout); err != nil {
		t.Fatal(err)
	}
	refunds, err := h.s.ClaimCardRefunds(ctx, &id, 10, checkout)
	if err != nil || len(refunds) != 1 {
		t.Fatalf("claimed %d, %v", len(refunds), err)
	}
	if err := h.s.FailCardRefund(ctx, refunds[0].ID, "Payments.lk refused the refund", checkout); err != nil {
		t.Fatal(err)
	}
	var lines int
	if err := h.s.Pool().QueryRow(ctx, "SELECT count(*) FROM activity WHERE booking_id = $1 AND title LIKE 'Card refund to % failed'", id).Scan(&lines); err != nil {
		t.Fatal(err)
	}
	if paid := h.stay(ctx, t, id).Paid(); paid != 7_200 || lines != 1 {
		t.Errorf("paid %d with %d failure lines, want 7200 and 1", paid, lines)
	}
}

func TestARefundIsClaimedOnceUntilItGoesStale(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	id := h.cardBooking(ctx, t, "tok")
	start := h.openCheckout(ctx, t, id, "tok", domain.PurposeDeposit, "chk_1")
	if _, err := h.s.ApplyCardEvent(ctx, h.paid("chk_1", "pay_1", start.Charged), checkout); err != nil {
		t.Fatal(err)
	}
	b := h.stay(ctx, t, id)
	if _, err := h.s.CancelBooking(ctx, h.t, id, b.Version, "", &domain.Payment{Method: "card", Amount: 7_200}, checkout); err != nil {
		t.Fatal(err)
	}
	first, _ := h.s.ClaimCardRefunds(ctx, nil, 10, checkout)
	second, _ := h.s.ClaimCardRefunds(ctx, nil, 10, checkout.Add(time.Minute))
	later, _ := h.s.ClaimCardRefunds(ctx, nil, 10, checkout.Add(6*time.Minute))
	if len(first) != 1 || len(second) != 0 || len(later) != 1 {
		t.Errorf("claims %d, %d, %d; want 1, 0, 1", len(first), len(second), len(later))
	}
}

func TestCardsAreReadyOnlyWithAConnection(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	if _, err := h.s.DisconnectCards(ctx, h.t); err != nil {
		t.Fatal(err)
	}
	p, err := h.s.GetProperty(ctx, h.t, h.p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Payments.CardsReady() || len(p.Payments.CardsMissing) != 1 || p.Payments.CardsMissing[0] != domain.CardsNeedMode {
		t.Errorf("after disconnecting, missing = %v", p.Payments.CardsMissing)
	}
}

func TestPayoutsCantBeChosenWhenTheServerDoesntOfferThem(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	if _, err := h.s.SetCardMode(context.Background(), h.t, domain.CardModePayouts); err != domain.ErrPayoutsUnavailable {
		t.Errorf("SetCardMode(payouts) = %v, want ErrPayoutsUnavailable", err)
	}
}

func TestRowSecurityHidesAnotherAccountsCheckouts(t *testing.T) {
	t.Parallel()
	h := cardHouse(t)
	ctx := context.Background()
	h.openCheckout(ctx, t, h.cardBooking(ctx, t, "tok"), "tok", domain.PurposeDeposit, "chk_1")
	other := tenant.With(ctx, tenant.Tenant{AccountID: uuid.New()})
	var n int
	if err := h.s.Pool().QueryRow(other, "SELECT count(*) FROM card_checkouts").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("another tenant sees %d checkouts, want 0", n)
	}
}
