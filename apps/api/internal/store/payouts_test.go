package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/tenant"
)

// The deposit of the card house's two-night stay: $72.00 at Rs 300.00, less StayKey's 3%.
const (
	depositCharged int64 = 2_160_000
	depositFee     int64 = 64_800
	depositNet           = depositCharged - depositFee
)

var (
	// firstPayout is 08:00 in Colombo on the first payout Monday, 2 Nov 2026.
	firstPayout  = time.Date(2026, 11, 2, 2, 30, 0, 0, time.UTC)
	secondPayout = firstPayout.AddDate(0, 0, domain.PayoutEveryDays)
)

func payoutDetails(accountNumber string) domain.PayoutProfileInput {
	return domain.PayoutProfileInput{
		LegalName: "Nadeesha Perera", NIC: "851234567V", BankName: "Commercial Bank", Branch: "Galle",
		AccountName: "N Perera", AccountNumber: accountNumber, AcceptTerms: domain.PayoutTermsVersion,
	}
}

// payoutHouse is the card house on StayKey payouts, its details given at checkout time.
func payoutHouse(tb testing.TB) house {
	tb.Helper()
	h := newHouseWith(tb, store.WithCards(store.CardConfig{CollectMode: "test", PayoutsEnabled: true}))
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
	if _, err := h.s.SetPayoutProfile(ctx, h.t, payoutDetails("8001234567"), checkout); err != nil {
		tb.Fatal(err)
	}
	if _, err := h.s.SetCardMode(ctx, h.t, domain.CardModePayouts); err != nil {
		tb.Fatal(err)
	}
	return h
}

// payDeposit books Room 1 by card for in to out and pays its deposit through StayKey.
func (h house) payDeposit(ctx context.Context, tb testing.TB, token, in, out string) uuid.UUID {
	tb.Helper()
	hash := sha256.Sum256([]byte(token))
	due := checkout.Add(time.Hour)
	id, err := h.s.CreateGuestBooking(ctx, h.t.AccountID, domain.NewBooking{
		PropertyID: h.p.ID,
		Stay: domain.StayInput{UnitID: h.r1, CheckIn: day(in), CheckOut: day(out), Adults: 2,
			Guest: stranger, Source: "page"},
		Status: domain.StatusAwaitingPayment, Currency: "USD", RefPrefix: "TH", Price: domain.Pricing{Total: 24_000},
		LedgerUnits: []uuid.UUID{h.r1}, PaymentMethod: "card", PaymentDueAt: &due, GuestTokenHash: hash[:],
	}, checkout)
	if err != nil {
		tb.Fatal(err)
	}
	start := h.openCheckout(ctx, tb, id, token, domain.PurposeDeposit, "chk_"+token)
	if _, err := h.s.ApplyCardEvent(ctx, store.CardEvent{
		Source: store.StayKeyCardSource, ID: "evt_" + token, Type: "payment.succeeded", Mode: "test",
		CheckoutID: "chk_" + token, PaymentID: "pay_" + token, AmountCents: start.Charged,
	}, checkout); err != nil {
		tb.Fatal(err)
	}
	return id
}

// refundAll cancels a stay with its whole deposit back to the card, sent at now.
func (h house) refundAll(ctx context.Context, tb testing.TB, id uuid.UUID, now time.Time) {
	tb.Helper()
	b := h.stay(ctx, tb, id)
	if _, err := h.s.CancelBooking(ctx, h.t, id, b.Version, "Plans changed", &domain.Payment{Method: "card", Amount: 7_200}, now); err != nil {
		tb.Fatal(err)
	}
	refunds, err := h.s.ClaimCardRefunds(ctx, &id, 10, now)
	if err != nil || len(refunds) != 1 {
		tb.Fatalf("claimed %d refunds, %v", len(refunds), err)
	}
	if err := h.s.AcceptCardRefund(ctx, refunds[0].ID, "re_"+id.String()[:8], now); err != nil {
		tb.Fatal(err)
	}
}

func (h house) payouts(ctx context.Context, tb testing.TB, now time.Time) domain.Payouts {
	tb.Helper()
	p, err := h.s.Payouts(ctx, h.t.AccountID, now)
	if err != nil {
		tb.Fatal(err)
	}
	return p
}

func prepare(ctx context.Context, tb testing.TB, s *store.Postgres, now time.Time) store.PreparedBatch {
	tb.Helper()
	b, _, err := s.PreparePayouts(ctx, now)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

func TestAStayKeyPaymentIsCreditedLessThreePercentOnce(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	if _, err := h.s.ApplyCardEvent(ctx, store.CardEvent{
		Source: store.StayKeyCardSource, ID: "evt_a_again", Type: "payment.succeeded", Mode: "test",
		CheckoutID: "chk_a", PaymentID: "pay_a", AmountCents: depositCharged,
	}, checkout); err != nil {
		t.Fatal(err)
	}
	if got := h.payouts(ctx, t, checkout).Available; got != depositNet {
		t.Errorf("available = %d, want %d", got, depositNet)
	}
}

func TestAPayoutCoversAStayOnceItHasStarted(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	if b := prepare(ctx, t, h.s, firstPayout); b.Payouts != 1 || b.Total != depositNet {
		t.Errorf("batch = %d payouts, %d in all; want 1 of %d", b.Payouts, b.Total, depositNet)
	}
}

func TestNothingIsPaidOutBeforeCheckIn(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-11-01", "2026-11-03")
	if b := prepare(ctx, t, h.s, firstPayout); b.Payouts != 0 {
		t.Errorf("batch has %d payouts for a stay checking in on its Sunday, want 0", b.Payouts)
	}
}

func TestAStayThatMovesEarlierIsPaidOutWithItsNewDates(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	id := h.payDeposit(ctx, t, "a", "2026-11-20", "2026-11-22")
	if _, err := h.s.Pool().Exec(ctx, "UPDATE bookings SET check_in = '2026-10-20', check_out = '2026-10-22' WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	if b := prepare(ctx, t, h.s, firstPayout); b.Payouts != 1 {
		t.Errorf("batch has %d payouts after the stay moved before its period end, want 1", b.Payouts)
	}
}

func TestABatchIsPreparedOncePerMonday(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	prepare(ctx, t, h.s, firstPayout)
	if _, again, err := h.s.PreparePayouts(ctx, firstPayout.Add(5*time.Hour)); err != nil || again {
		t.Errorf("second run prepared = %v, %v; want false", again, err)
	}
}

func TestAnEntryIsNeverPaidTwice(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	prepare(ctx, t, h.s, firstPayout)
	if b := prepare(ctx, t, h.s, secondPayout); b.Payouts != 0 {
		t.Errorf("the next batch has %d payouts, want 0", b.Payouts)
	}
}

func TestACancellationBeforeCheckInLeavesOnlyTheProcessingCharge(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	id := h.payDeposit(ctx, t, "a", "2026-11-20", "2026-11-22")
	h.refundAll(ctx, t, id, checkout.Add(time.Hour))
	p := h.payouts(ctx, t, checkout.Add(time.Hour))
	if p.Pending != -depositFee || p.Available != 0 {
		t.Errorf("pending %d, available %d; want %d pending and nothing available", p.Pending, p.Available, -depositFee)
	}
}

func TestARefundAfterAPayoutIsTakenFromTheNext(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	first := h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	prepare(ctx, t, h.s, firstPayout)
	h.refundAll(ctx, t, first, firstPayout.Add(24*time.Hour))
	h.payDeposit(ctx, t, "b", "2026-11-05", "2026-11-07")
	h.payDeposit(ctx, t, "c", "2026-11-08", "2026-11-10")
	if b := prepare(ctx, t, h.s, secondPayout); b.Total != 2*depositNet-depositCharged {
		t.Errorf("second payout = %d, want %d", b.Total, 2*depositNet-depositCharged)
	}
}

func TestABalanceBelowTheMinimumRollsOver(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	if err := h.s.AdjustPayouts(ctx, h.t.AccountID, -(depositNet - domain.PayoutMinimum + 1), "Chargeback", checkout); err != nil {
		t.Fatal(err)
	}
	if b := prepare(ctx, t, h.s, firstPayout); b.Payouts != 0 {
		t.Errorf("batch has %d payouts for a balance under the minimum, want 0", b.Payouts)
	}
}

func TestABankChangeHoldsTheNextPayout(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	if _, err := h.s.SetPayoutProfile(ctx, h.t, payoutDetails("9001234567"), firstPayout.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if b := prepare(ctx, t, h.s, firstPayout); b.Payouts != 0 {
		t.Errorf("batch has %d payouts two days after a bank change, want 0", b.Payouts)
	}
}

func TestChangingOnlyTheNameKeepsPayoutsGoing(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	in := payoutDetails("8001234567")
	in.LegalName = "Nadeesha K Perera"
	p, err := h.s.SetPayoutProfile(ctx, h.t, in, firstPayout)
	if err != nil {
		t.Fatal(err)
	}
	if !p.BankChangedAt.Equal(checkout) {
		t.Errorf("bank changed at %s, want %s", p.BankChangedAt, checkout)
	}
}

func TestAFailedPayoutReturnsTheMoney(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	prepare(ctx, t, h.s, firstPayout)
	history := h.payouts(ctx, t, firstPayout).History
	if len(history) != 1 {
		t.Fatalf("%d payouts, want 1", len(history))
	}
	if _, err := h.s.FailPayout(ctx, history[0].ID, "Account closed", firstPayout.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := h.payouts(ctx, t, firstPayout.Add(24*time.Hour)).Available; got != depositNet {
		t.Errorf("available after the failure = %d, want %d", got, depositNet)
	}
}

func TestMarkingABatchPaidPaysItsPayouts(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	b := prepare(ctx, t, h.s, firstPayout)
	if _, err := h.s.MarkBatchPaid(ctx, b.ID, "BOC-0001", firstPayout); err != nil {
		t.Fatal(err)
	}
	if got := h.payouts(ctx, t, firstPayout).History[0].Status; got != domain.PayoutPaid {
		t.Errorf("payout status %s, want paid", got)
	}
}

func TestABatchIsMarkedPaidOnce(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	b := prepare(ctx, t, h.s, firstPayout)
	if _, err := h.s.MarkBatchPaid(ctx, b.ID, "BOC-0001", firstPayout); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.MarkBatchPaid(ctx, b.ID, "BOC-0002", firstPayout); !errors.Is(err, store.ErrBatchPaid) {
		t.Errorf("second mark = %v, want ErrBatchPaid", err)
	}
}

func TestAStatementListsTheStayAndItsCharge(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	prepare(ctx, t, h.s, firstPayout)
	st, err := h.s.PayoutStatement(ctx, h.t.AccountID, h.payouts(ctx, t, firstPayout).History[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []domain.PayoutEntryKind
	for _, l := range st.Lines {
		kinds = append(kinds, l.Kind)
	}
	if len(kinds) != 2 || kinds[0] != domain.EntryPayment || kinds[1] != domain.EntryProcessing {
		t.Errorf("statement kinds %v, want payment and processing", kinds)
	}
}

func TestPayoutDetailsAreStoredSealed(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	var nic, number []byte
	if err := h.s.Pool().QueryRow(context.Background(),
		"SELECT nic_sealed, account_number_sealed FROM payout_profiles WHERE account_id = $1", h.t.AccountID).Scan(&nic, &number); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(nic, []byte("851234567")) || bytes.Contains(number, []byte("8001234567")) {
		t.Error("payout details are stored in the clear")
	}
}

func TestRowSecurityHidesAnotherAccountsPayouts(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	other := tenant.With(ctx, tenant.Tenant{AccountID: uuid.New()})
	var n int
	if err := h.s.Pool().QueryRow(other, "SELECT (SELECT count(*) FROM payout_entries) + (SELECT count(*) FROM payout_profiles)").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("another tenant sees %d payout rows, want 0", n)
	}
}

func TestABatchWithAHeldPayoutCantBeMarkedPaid(t *testing.T) {
	t.Parallel()
	h := payoutHouse(t)
	ctx := context.Background()
	h.payDeposit(ctx, t, "a", "2026-10-10", "2026-10-12")
	b := prepare(ctx, t, h.s, firstPayout)
	if _, err := h.s.SetPayoutProfile(ctx, h.t, payoutDetails("9001234567"), firstPayout.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.MarkBatchPaid(ctx, b.ID, "BOC-0001", firstPayout.Add(2*time.Hour)); !errors.Is(err, store.ErrHeldPayouts) {
		t.Errorf("MarkBatchPaid = %v, want ErrHeldPayouts", err)
	}
}
