package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/domain/pricing"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// Guest card payments. Payments.lk is never called from here: these methods prepare and record
// what the cards service sends and receives, each in a short transaction.

// StayKeyCardSource names events from StayKey's collection account.
const StayKeyCardSource = "staykey"

// CardSource names events from an owner's own connection.
func CardSource(accountID uuid.UUID) string { return "owner:" + accountID.String() }

// CardConfig is what the server offers for card payments.
type CardConfig struct {
	// CollectMode is test or live for StayKey's collection key, or "" when there is none.
	CollectMode string
	// PayoutsEnabled lets accounts choose StayKey payouts.
	PayoutsEnabled bool
}

// WithCards sets what the server offers for card payments.
func WithCards(c CardConfig) Option { return func(s *Postgres) { s.cards = c } }

// CardCollector is the Payments.lk account that takes or refunds a card payment. Key is the
// owner's secret key; it is empty for StayKey's collection account, whose key the cards
// service holds.
type CardCollector struct {
	By        domain.Collector
	AccountID uuid.UUID
	Key       string
	Mode      string
}

// Source names the collector's events.
func (c CardCollector) Source() string {
	if c.By == domain.CollectedByStayKey {
		return StayKeyCardSource
	}
	return CardSource(c.AccountID)
}

// CardState returns an account's card setup.
func (s *Postgres) CardState(ctx context.Context, accountID uuid.UUID) (domain.CardState, error) {
	r, err := s.db(ctx).GetCardState(ctx, accountID)
	if err != nil {
		return domain.CardState{}, notFound(err)
	}
	st := domain.CardState{Mode: domain.CardMode(r.CardMode), CollectMode: s.cards.CollectMode}
	if r.KeyLast4 != nil {
		st.Connection = &domain.CardConnection{Last4: *r.KeyLast4, Mode: deref(r.Mode), Status: deref(r.Status), CheckedAt: deref(r.CheckedAt)}
	}
	st.PayoutsReady, err = s.payoutsReady(ctx, accountID)
	if err != nil {
		return domain.CardState{}, err
	}
	return st, nil
}

// PayoutsAvailable reports whether this server offers StayKey payouts.
func (s *Postgres) PayoutsAvailable() bool {
	return s.cards.PayoutsEnabled && s.cards.CollectMode != ""
}

// payoutsReady reports whether the account may take card payments through StayKey payouts.
func (s *Postgres) payoutsReady(ctx context.Context, accountID uuid.UUID) (bool, error) {
	if !s.PayoutsAvailable() {
		return false, nil
	}
	return s.payoutProfileComplete(ctx, accountID)
}

// SetCardMode chooses where the account's card payments go. Payouts need the server to offer
// them and the account's payout details.
func (s *Postgres) SetCardMode(ctx context.Context, t tenant.Tenant, mode domain.CardMode) (domain.CardState, error) {
	if !mode.Valid() {
		return domain.CardState{}, &domain.ValidationError{Field: "mode", Message: "must be off, own or payouts"}
	}
	if mode == domain.CardModePayouts {
		ready, err := s.payoutsReady(ctx, t.AccountID)
		if err != nil {
			return domain.CardState{}, err
		}
		if !ready {
			return domain.CardState{}, domain.ErrPayoutsUnavailable
		}
	}
	if err := s.db(ctx).SetCardMode(ctx, queries.SetCardModeParams{AccountID: t.AccountID, CardMode: string(mode)}); err != nil {
		return domain.CardState{}, fmt.Errorf("set card mode: %w", err)
	}
	return s.CardState(ctx, t.AccountID)
}

// ConnectCards keeps the owner's Payments.lk secret key, which the caller has checked with
// Payments.lk, and starts reading its events from now. An account with no card mode yet
// switches to its own account.
func (s *Postgres) ConnectCards(ctx context.Context, t tenant.Tenant, key, mode string, now time.Time) (domain.CardState, error) {
	if s.secrets == nil {
		return domain.CardState{}, errNoSecrets
	}
	sealed, err := s.secrets.Seal([]byte(key), cardKeyContext(t.AccountID))
	if err != nil {
		return domain.CardState{}, fmt.Errorf("seal card key: %w", err)
	}
	err = s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		if err := q.UpsertCardConnection(ctx, queries.UpsertCardConnectionParams{
			AccountID: t.AccountID, SecretKeySealed: sealed, KeyLast4: key[len(key)-4:], Mode: mode,
			CheckedAt: now, EventsSince: now,
		}); err != nil {
			return fmt.Errorf("save card connection: %w", err)
		}
		st, err := q.GetCardState(ctx, t.AccountID)
		if err != nil {
			return fmt.Errorf("card state: %w", err)
		}
		if domain.CardMode(st.CardMode) == domain.CardModeOff {
			return q.SetCardMode(ctx, queries.SetCardModeParams{AccountID: t.AccountID, CardMode: string(domain.CardModeOwn)})
		}
		return nil
	})
	if err != nil {
		return domain.CardState{}, err
	}
	return s.CardState(ctx, t.AccountID)
}

// DisconnectCards forgets the owner's key. An account using its own account goes back to no
// card mode, so its properties stop offering cards.
func (s *Postgres) DisconnectCards(ctx context.Context, t tenant.Tenant) (domain.CardState, error) {
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		if err := q.DeleteCardConnection(ctx, t.AccountID); err != nil {
			return fmt.Errorf("delete card connection: %w", err)
		}
		st, err := q.GetCardState(ctx, t.AccountID)
		if err != nil {
			return fmt.Errorf("card state: %w", err)
		}
		if domain.CardMode(st.CardMode) == domain.CardModeOwn {
			return q.SetCardMode(ctx, queries.SetCardModeParams{AccountID: t.AccountID, CardMode: string(domain.CardModeOff)})
		}
		return nil
	})
	if err != nil {
		return domain.CardState{}, err
	}
	return s.CardState(ctx, t.AccountID)
}

// cardsMissing fills in what keeps cards from working on each property that has them on.
func (s *Postgres) cardsMissing(ctx context.Context, accountID uuid.UUID, details []domain.PropertyDetail) error {
	var st *domain.CardState
	for i := range details {
		pay := &details[i].Payments
		if pay.Cards != "on" {
			continue
		}
		if st == nil {
			got, err := s.CardState(ctx, accountID)
			if err != nil {
				return err
			}
			st = &got
		}
		pay.CardsMissing = domain.CardsMissing(*st, *pay, details[i].Currency)
		pay.CardsTest = st.KeyMode() == "test"
	}
	return nil
}

// cardCollector is the account that takes card payments for collector by, with its key.
func (s *Postgres) cardCollector(ctx context.Context, accountID uuid.UUID, by domain.Collector) (CardCollector, error) {
	c := CardCollector{By: by, AccountID: accountID, Mode: s.cards.CollectMode}
	if by == domain.CollectedByStayKey {
		if c.Mode == "" {
			return CardCollector{}, domain.ErrCardsOff
		}
		return c, nil
	}
	conn, err := s.db(ctx).GetCardConnection(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return CardCollector{}, domain.ErrCardsOff
	}
	if err != nil {
		return CardCollector{}, fmt.Errorf("card connection: %w", err)
	}
	if s.secrets == nil {
		return CardCollector{}, errNoSecrets
	}
	key, err := s.secrets.Open(conn.SecretKeySealed, cardKeyContext(accountID))
	if err != nil {
		return CardCollector{}, fmt.Errorf("open card key: %w", err)
	}
	c.Key, c.Mode = string(key), conn.Mode
	return c, nil
}

func cardKeyContext(accountID uuid.UUID) []byte { return []byte("card-key:" + accountID.String()) }

// CheckoutStart is a card checkout to open, or to send the guest to again when ProviderID is set.
type CheckoutStart struct {
	ID          uuid.UUID
	ProviderID  string
	Collector   CardCollector
	Amount      int64
	Currency    string
	Charged     int64
	Description string
	Slug        string
	ExpiresAt   time.Time
}

// StartCardCheckout prepares a card payment of purpose for a guest's stay: it works out the
// amount, picks who collects it from the account's card mode, and reuses an open checkout for
// the same amount that still has ten minutes to run. Otherwise it records a new checkout, which
// the caller opens with Payments.lk and passes to SetCardCheckoutProvider.
func (s *Postgres) StartCardCheckout(ctx context.Context, bookingID uuid.UUID, tokenHash []byte, purpose domain.CardPurpose, now time.Time) (CheckoutStart, error) {
	var out CheckoutStart
	err := s.InTx(ctx, func(ctx context.Context) error {
		gb, err := s.GuestBooking(ctx, bookingID, tokenHash)
		if err != nil {
			return err
		}
		q := s.db(ctx)
		if _, err := q.GetBookingForUpdate(ctx, queries.GetBookingForUpdateParams{ID: bookingID, AccountID: gb.AccountID}); err != nil {
			return notFound(err)
		}
		if gb.Source != "page" && gb.Source != "widget" {
			return domain.ErrCardsOff
		}
		p, err := s.PublicProperty(ctx, gb.PropertySlug)
		if err != nil {
			return err
		}
		if !p.Payments.CardsReady() {
			return domain.ErrCardsOff
		}
		amount, err := domain.CardAmount(purpose, gb.Status, gb.Total, pricing.Deposit(p, gb.Total), gb.Paid(), p.Payments.CardCharge)
		if err != nil {
			return err
		}
		rate := int64(0)
		if gb.Currency != domain.CardCurrency {
			rate = p.Payments.CardRate
		}
		charged := domain.ToLKR(amount, rate)
		if charged < domain.MinCardCharge || charged > domain.MaxCardCharge {
			return &domain.ValidationError{Field: "purpose", Message: "is outside what a card payment can charge"}
		}
		st, err := s.CardState(ctx, gb.AccountID)
		if err != nil {
			return err
		}
		collector, err := s.cardCollector(ctx, gb.AccountID, st.Mode.Collector())
		if err != nil {
			return err
		}
		out = CheckoutStart{
			Collector: collector, Amount: amount, Currency: gb.Currency, Charged: charged,
			Description: gb.PropertyName + ", " + gb.Ref, Slug: gb.PropertySlug,
		}
		reuse, err := q.ReusableCardCheckout(ctx, queries.ReusableCardCheckoutParams{
			BookingID: bookingID, AccountID: gb.AccountID, Purpose: string(purpose), ChargedCents: charged,
			CollectedBy: string(collector.By), Mode: collector.Mode, FreshUntil: now.Add(10 * time.Minute),
		})
		switch {
		case err == nil:
			out.ID, out.ProviderID = reuse.ID, deref(reuse.ProviderCheckoutID)
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("find open checkout: %w", err)
		}
		out.ID = domain.NewID()
		out.ExpiresAt = now.Add(domain.CardCheckoutMinutes * time.Minute)
		var ratePtr *int64
		if rate > 0 {
			ratePtr = &rate
		}
		return q.InsertCardCheckout(ctx, queries.InsertCardCheckoutParams{
			ID: out.ID, AccountID: gb.AccountID, BookingID: bookingID, CollectedBy: string(collector.By),
			Purpose: string(purpose), Amount: amount, Currency: gb.Currency, ChargedCents: charged, Rate: ratePtr,
			Mode: collector.Mode, ExpiresAt: out.ExpiresAt, CreatedAt: now,
		})
	})
	return out, err
}

// SetCardCheckoutProvider records the Payments.lk id of a checkout we opened.
func (s *Postgres) SetCardCheckoutProvider(ctx context.Context, id uuid.UUID, providerID string) error {
	if err := s.db(ctx).SetCardCheckoutProvider(ctx, queries.SetCardCheckoutProviderParams{ID: id, ProviderCheckoutID: &providerID}); err != nil {
		return fmt.Errorf("set checkout provider: %w", err)
	}
	return nil
}

// OpenCheckout is an open card checkout of a stay, with who collects it.
type OpenCheckout struct {
	ProviderID string
	Collector  CardCollector
}

// OpenCardCheckouts lists a guest's open card checkouts, newest first, for the stay page's
// check when the guest returns.
func (s *Postgres) OpenCardCheckouts(ctx context.Context, bookingID uuid.UUID, tokenHash []byte) ([]OpenCheckout, error) {
	gb, err := s.GuestBooking(ctx, bookingID, tokenHash)
	if err != nil {
		return nil, err
	}
	rows, err := s.db(ctx).OpenCardCheckouts(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("open checkouts: %w", err)
	}
	out := make([]OpenCheckout, 0, len(rows))
	for _, r := range rows {
		c, err := s.cardCollector(ctx, gb.AccountID, domain.Collector(r.CollectedBy))
		if errors.Is(err, domain.ErrCardsOff) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, OpenCheckout{ProviderID: deref(r.ProviderCheckoutID), Collector: c})
	}
	return out, nil
}

// CardEvent is a Payments.lk event, or what the stay page's check read, in the terms the store
// acts on. ID is empty for a check, which is matched on the payment instead.
type CardEvent struct {
	Source string
	ID     string
	Type   string
	Mode   string
	// For payment.succeeded and checkout.expired.
	CheckoutID  string
	PaymentID   string
	AmountCents int64
	// For refund.succeeded and refund.failed.
	RefundID        string
	RefundPaymentID string
	RefundCents     int64
	Failure         string
}

// ownerAccount is the account of an owner source, or nil for StayKey's.
func (ev CardEvent) ownerAccount() *uuid.UUID {
	if ev.Source == StayKeyCardSource || len(ev.Source) <= len("owner:") {
		return nil
	}
	id, err := uuid.Parse(ev.Source[len("owner:"):])
	if err != nil {
		return nil
	}
	return &id
}

// CardOutcome says what an event did, for the log.
type CardOutcome string

const (
	CardApplied        CardOutcome = "applied"
	CardRepeat         CardOutcome = "repeat"
	CardNotOurs        CardOutcome = "not_ours"
	CardAmountMismatch CardOutcome = "amount_mismatch"
)

// ApplyCardEvent acts on a card event once per source and event id, and on a payment once
// however often it is reported. A payment adds a card payment to the stay and confirms a stay
// waiting for one; an expiry closes the checkout; a refund event settles the refund.
// Payments.lk may send events about payments StayKey didn't start, which are left alone.
func (s *Postgres) ApplyCardEvent(ctx context.Context, ev CardEvent, now time.Time) (CardOutcome, error) {
	outcome := CardApplied
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		if ev.ID != "" {
			n, err := q.InsertCardEvent(ctx, queries.InsertCardEventParams{
				Source: ev.Source, EventID: ev.ID, AccountID: ev.ownerAccount(), Type: ev.Type, ReceivedAt: now,
			})
			if err != nil {
				return fmt.Errorf("record card event: %w", err)
			}
			if n == 0 {
				outcome = CardRepeat
				return nil
			}
		}
		var err error
		switch ev.Type {
		case "payment.succeeded":
			outcome, err = s.applyCardPayment(ctx, ev, now)
		case "checkout.expired":
			outcome, err = s.expireCardCheckout(ctx, ev)
		case "refund.succeeded", "refund.failed":
			outcome, err = s.settleCardRefund(ctx, ev, now)
		default:
			outcome = CardNotOurs
		}
		return err
	})
	return outcome, err
}

// ourCheckout finds the checkout an event names and makes sure it came from the account that
// collected it, in the same mode.
func (s *Postgres) ourCheckout(ctx context.Context, ev CardEvent) (queries.CardCheckout, bool, error) {
	c, err := s.db(ctx).GetCardCheckoutByProviderForUpdate(ctx, &ev.CheckoutID)
	if errors.Is(err, pgx.ErrNoRows) {
		return queries.CardCheckout{}, false, nil
	}
	if err != nil {
		return queries.CardCheckout{}, false, fmt.Errorf("get checkout: %w", err)
	}
	from := CardCollector{By: domain.Collector(c.CollectedBy), AccountID: c.AccountID}.Source()
	if from != ev.Source || (ev.Mode != "" && ev.Mode != c.Mode) {
		return queries.CardCheckout{}, false, nil
	}
	return c, true, nil
}

func (s *Postgres) applyCardPayment(ctx context.Context, ev CardEvent, now time.Time) (CardOutcome, error) {
	c, ok, err := s.ourCheckout(ctx, ev)
	if err != nil || !ok {
		return CardNotOurs, err
	}
	if ev.AmountCents != c.ChargedCents {
		return CardAmountMismatch, nil
	}
	q := s.db(ctx)
	seen, err := q.CardPaymentExists(ctx, queries.CardPaymentExistsParams{AccountID: c.AccountID, ProviderRef: &ev.PaymentID})
	if err != nil {
		return "", fmt.Errorf("find card payment: %w", err)
	}
	if seen {
		return CardRepeat, nil
	}
	r, err := q.GetBookingForUpdate(ctx, queries.GetBookingForUpdateParams{ID: c.BookingID, AccountID: c.AccountID})
	if err != nil {
		return "", fmt.Errorf("get booking: %w", err)
	}
	bookings := []domain.Booking{booking(queries.GetBookingRow(r))}
	if err := s.loadBookingDetails(ctx, c.AccountID, bookings); err != nil {
		return "", err
	}
	b := &bookings[0]

	note := ""
	switch {
	case c.Status == "paid":
		note = "Paid twice by card; refund one of the payments"
	case !b.Status.Holds():
		note = "Paid after the stay was cancelled; refund it"
	case b.Paid()+c.Amount > b.Total:
		note = "More than the stay's total; refund the difference"
	}
	paymentID := domain.NewID()
	if err := q.InsertCardPayment(ctx, queries.InsertCardPaymentParams{
		ID: paymentID, AccountID: c.AccountID, BookingID: c.BookingID, Kind: "payment", Amount: c.Amount,
		Currency: c.Currency, Note: note, ReceivedAt: now, ProviderRef: &ev.PaymentID, Status: "succeeded",
		ChargedCents: &c.ChargedCents, Rate: c.Rate, CardCheckoutID: &c.ID, CollectedBy: &c.CollectedBy,
	}); err != nil {
		return "", fmt.Errorf("insert card payment: %w", err)
	}
	if c.Status != "paid" {
		if err := q.MarkCardCheckoutPaid(ctx, queries.MarkCardCheckoutPaidParams{ID: c.ID, ProviderPaymentID: &ev.PaymentID, PaidAt: &now}); err != nil {
			return "", fmt.Errorf("mark checkout paid: %w", err)
		}
	}
	if err := s.addActivity(ctx, c.AccountID, domain.CardPaid(*b, c.Amount), nil, now); err != nil {
		return "", err
	}
	if err := s.cardPaymentCollected(ctx, c, paymentID, *b, now); err != nil {
		return "", err
	}
	if b.Status != domain.StatusAwaitingPayment {
		return CardApplied, nil
	}
	b.PaymentDueAt = nil
	if err := s.moveTo(ctx, c.AccountID, b, domain.StatusConfirmed, nil, "Paid by card", now); err != nil {
		return "", err
	}
	return CardApplied, s.saveBooking(ctx, c.AccountID, *b)
}

func (s *Postgres) expireCardCheckout(ctx context.Context, ev CardEvent) (CardOutcome, error) {
	c, ok, err := s.ourCheckout(ctx, ev)
	if err != nil || !ok {
		return CardNotOurs, err
	}
	if err := s.db(ctx).MarkCardCheckoutExpired(ctx, c.ID); err != nil {
		return "", fmt.Errorf("expire checkout: %w", err)
	}
	return CardApplied, nil
}

// ExpireCardCheckout closes a checkout the stay page found had ended.
func (s *Postgres) ExpireCardCheckout(ctx context.Context, source, providerID string) error {
	_, err := s.ApplyCardEvent(ctx, CardEvent{Source: source, Type: "checkout.expired", CheckoutID: providerID}, time.Now())
	return err
}

func (s *Postgres) settleCardRefund(ctx context.Context, ev CardEvent, now time.Time) (CardOutcome, error) {
	q := s.db(ctx)
	status, failure := "succeeded", (*string)(nil)
	if ev.Type == "refund.failed" {
		status = "failed"
		f := truncate(ev.Failure, 300)
		failure = &f
	}
	owner := ev.ownerAccount()
	r, err := q.SettleCardRefund(ctx, queries.SettleCardRefundParams{
		Status: status, Failure: failure, ProviderRef: &ev.RefundID, AccountID: owner,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// The event can beat the refund's id being saved after the send. Match the pending
		// refund of that payment and amount instead.
		id, err := q.PendingRefundOfPayment(ctx, queries.PendingRefundOfPaymentParams{
			PaymentRef: &ev.RefundPaymentID, ChargedCents: &ev.RefundCents, AccountID: owner,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return CardNotOurs, nil
		}
		if err != nil {
			return "", fmt.Errorf("find pending refund: %w", err)
		}
		if err := q.AcceptCardRefund(ctx, queries.AcceptCardRefundParams{ID: id, ProviderRef: &ev.RefundID}); err != nil {
			return "", fmt.Errorf("accept refund: %w", err)
		}
		r, err = q.SettleCardRefund(ctx, queries.SettleCardRefundParams{
			Status: status, Failure: failure, ProviderRef: &ev.RefundID, AccountID: owner,
		})
		if err != nil {
			return "", fmt.Errorf("settle refund: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("settle refund: %w", err)
	}
	if status == "failed" {
		if err := s.refundFailed(ctx, r.AccountID, r.BookingID, r.ID, now); err != nil {
			return "", err
		}
	}
	return CardApplied, nil
}

// refundFailed tells the team a card refund didn't go through.
func (s *Postgres) refundFailed(ctx context.Context, accountID, bookingID, refundID uuid.UUID, now time.Time) error {
	r, err := s.db(ctx).GetBooking(ctx, queries.GetBookingParams{ID: bookingID, AccountID: accountID})
	if err != nil {
		return fmt.Errorf("get booking: %w", err)
	}
	amount, err := s.db(ctx).PaymentAmount(ctx, refundID)
	if err != nil {
		return fmt.Errorf("refund amount: %w", err)
	}
	if err := s.cardRefundReturned(ctx, accountID, refundID, now); err != nil {
		return err
	}
	return s.addActivity(ctx, accountID, domain.CardRefundFailed(booking(r), amount), nil, now)
}

// SetCollectedFee records Payments.lk's fee on a card payment StayKey collected, for StayKey's
// reconciliation.
func (s *Postgres) SetCollectedFee(ctx context.Context, paymentID string, fee int64) error {
	if err := s.db(ctx).SetCollectedFee(ctx, queries.SetCollectedFeeParams{ProviderRef: &paymentID, ProviderFeeCents: &fee}); err != nil {
		return fmt.Errorf("set collected fee: %w", err)
	}
	return nil
}

// CardCursor is where the next events poll of a source starts. A source read for the first
// time starts from now.
func (s *Postgres) CardCursor(ctx context.Context, source string, now time.Time) (time.Time, error) {
	since, err := s.db(ctx).GetCardCursor(ctx, source)
	if errors.Is(err, pgx.ErrNoRows) {
		return now, s.AdvanceCardCursor(ctx, source, now)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("card cursor: %w", err)
	}
	return since, nil
}

// AdvanceCardCursor moves a source's events cursor forward to at.
func (s *Postgres) AdvanceCardCursor(ctx context.Context, source string, at time.Time) error {
	if err := s.db(ctx).SetCardCursor(ctx, queries.SetCardCursorParams{Source: source, EventsSince: at}); err != nil {
		return fmt.Errorf("advance card cursor: %w", err)
	}
	return nil
}

// LiveConnection is an owner connection the events job reads.
type LiveConnection struct {
	Collector   CardCollector
	EventsSince time.Time
}

// LiveCardConnections lists the working owner connections of open accounts using them.
func (s *Postgres) LiveCardConnections(ctx context.Context) ([]LiveConnection, error) {
	rows, err := s.db(ctx).ListLiveCardConnections(ctx)
	if err != nil {
		return nil, fmt.Errorf("live card connections: %w", err)
	}
	if s.secrets == nil {
		return nil, errNoSecrets
	}
	out := make([]LiveConnection, 0, len(rows))
	for _, r := range rows {
		key, err := s.secrets.Open(r.SecretKeySealed, cardKeyContext(r.AccountID))
		if err != nil {
			return nil, fmt.Errorf("open card key of account %s: %w", r.AccountID, err)
		}
		out = append(out, LiveConnection{
			Collector:   CardCollector{By: domain.CollectedByOwner, AccountID: r.AccountID, Key: string(key), Mode: r.Mode},
			EventsSince: r.EventsSince,
		})
	}
	return out, nil
}

// AdvanceCardEvents moves an owner connection's events cursor forward to at.
func (s *Postgres) AdvanceCardEvents(ctx context.Context, accountID uuid.UUID, at time.Time) error {
	if err := s.db(ctx).AdvanceCardEvents(ctx, queries.AdvanceCardEventsParams{AccountID: accountID, EventsSince: at}); err != nil {
		return fmt.Errorf("advance card events: %w", err)
	}
	return nil
}

// CardKeyRefused marks an owner's connection failed, which pauses card payments on the
// account's properties, and tells the owners once.
func (s *Postgres) CardKeyRefused(ctx context.Context, accountID uuid.UUID, now time.Time) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		n, err := s.db(ctx).MarkCardConnectionFailed(ctx, queries.MarkCardConnectionFailedParams{AccountID: accountID, CheckedAt: now})
		if err != nil {
			return fmt.Errorf("mark card connection failed: %w", err)
		}
		if n == 0 {
			return nil
		}
		return s.addActivity(ctx, accountID, domain.CardKeyRefused(), nil, now)
	})
}

// addCardRefunds spreads a card refund over the stay's card payments, newest first, as pending
// refunds the cards service sends after the caller commits. It reports false when the stay
// has no card payments taken through Payments.lk, so the refund is only recorded.
func (s *Postgres) addCardRefunds(ctx context.Context, t tenant.Tenant, b domain.Booking, refund domain.Payment) (bool, error) {
	q := s.db(ctx)
	rows, err := q.RefundableCardPayments(ctx, queries.RefundableCardPaymentsParams{BookingID: b.ID, AccountID: t.AccountID})
	if err != nil {
		return false, fmt.Errorf("refundable card payments: %w", err)
	}
	if len(rows) == 0 {
		return false, nil
	}
	payments := make([]domain.RefundablePayment, len(rows))
	byID := make(map[string]queries.RefundableCardPaymentsRow, len(rows))
	for i, r := range rows {
		payments[i] = domain.RefundablePayment{ID: r.ID.String(), Rate: deref(r.Rate), AmountLeft: r.AmountLeft, ChargedLeft: r.ChargedLeft}
		byID[r.ID.String()] = r
	}
	splits, err := domain.SplitCardRefund(refund.Amount, payments)
	if err != nil {
		return false, err
	}
	for _, sp := range splits {
		of := byID[sp.PaymentID]
		charged := sp.Charged
		if err := q.InsertCardPayment(ctx, queries.InsertCardPaymentParams{
			ID: domain.NewID(), AccountID: t.AccountID, BookingID: b.ID, Kind: "refund", Amount: sp.Amount,
			Currency: b.Currency, Note: refund.Note, ReceivedAt: refund.ReceivedAt, RecordedBy: &t.UserID,
			Status: "pending", ChargedCents: &charged, Rate: of.Rate, CollectedBy: of.CollectedBy, RefundOf: &of.ID,
		}); err != nil {
			return false, fmt.Errorf("insert card refund: %w", err)
		}
	}
	return true, nil
}

// CardRefund is a pending card refund claimed for one send.
type CardRefund struct {
	ID         uuid.UUID
	AccountID  uuid.UUID
	BookingID  uuid.UUID
	PaymentRef string
	Charged    int64
	Collector  CardCollector
}

// cardRefundStale is how long a claimed refund waits before another send may take it.
const cardRefundStale = 5 * time.Minute

// ClaimCardRefunds claims up to limit card refunds Payments.lk hasn't accepted yet, of one
// stay or of all. A refund whose collector has no key any more fails at once.
func (s *Postgres) ClaimCardRefunds(ctx context.Context, bookingID *uuid.UUID, limit int, now time.Time) ([]CardRefund, error) {
	var out []CardRefund
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		rows, err := q.ClaimCardRefunds(ctx, queries.ClaimCardRefundsParams{
			StaleBefore: now.Add(-cardRefundStale), BookingID: bookingID, RowLimit: int32(limit),
		})
		if err != nil {
			return fmt.Errorf("claim card refunds: %w", err)
		}
		ids := make([]uuid.UUID, 0, len(rows))
		for _, r := range rows {
			c, err := s.cardCollector(ctx, r.AccountID, domain.Collector(deref(r.CollectedBy)))
			if errors.Is(err, domain.ErrCardsOff) {
				if err := s.failCardRefund(ctx, r.ID, "The Payments.lk account is no longer connected", now); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			ids = append(ids, r.ID)
			out = append(out, CardRefund{
				ID: r.ID, AccountID: r.AccountID, BookingID: r.BookingID, PaymentRef: deref(r.PaymentRef),
				Charged: deref(r.ChargedCents), Collector: c,
			})
		}
		return q.StampCardRefunds(ctx, queries.StampCardRefundsParams{AttemptedAt: &now, Ids: ids})
	})
	return out, err
}

// AcceptCardRefund records the Payments.lk id of a refund it accepted. The refund stays
// pending until its event.
func (s *Postgres) AcceptCardRefund(ctx context.Context, id uuid.UUID, providerRef string, now time.Time) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		if err := s.db(ctx).AcceptCardRefund(ctx, queries.AcceptCardRefundParams{ID: id, ProviderRef: &providerRef}); err != nil {
			return fmt.Errorf("accept card refund: %w", err)
		}
		return s.cardRefundSent(ctx, id, now)
	})
}

// FailCardRefund marks a refund Payments.lk turned down and tells the team.
func (s *Postgres) FailCardRefund(ctx context.Context, id uuid.UUID, reason string, now time.Time) error {
	return s.InTx(ctx, func(ctx context.Context) error { return s.failCardRefund(ctx, id, reason, now) })
}

func (s *Postgres) failCardRefund(ctx context.Context, id uuid.UUID, reason string, now time.Time) error {
	r, err := s.db(ctx).FailCardRefund(ctx, queries.FailCardRefundParams{ID: id, Failure: truncate(reason, 300)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("fail card refund: %w", err)
	}
	return s.refundFailed(ctx, r.AccountID, r.BookingID, id, now)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
