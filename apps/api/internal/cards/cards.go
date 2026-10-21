// Package cards runs guest card payments through Payments.lk: it opens hosted checkouts,
// checks them when the guest returns, reads each account's events, and sends refunds. The
// store prepares and records each step in short transactions; Payments.lk is only called
// between them, never inside one.
package cards

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/billing"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store"
)

// Service takes and refunds guest card payments.
type Service struct {
	Store *store.Postgres
	// CollectKey is StayKey's own collection key, for accounts on StayKey payouts. Empty when
	// the server has none.
	CollectKey string
	// StayLink is the guest's stay page, which checkouts return to.
	StayLink func(slug string, bookingID uuid.UUID, token string) string
	// Production refuses test keys.
	Production bool

	BaseURL string       // default: Payments.lk's API
	Client  *http.Client // default: 30 second timeout
	Logger  *slog.Logger
	Now     func() time.Time
}

// ErrUnreachable means Payments.lk couldn't be reached or answered with a server error.
var ErrUnreachable = errors.New("payments.lk can't be reached")

// Checkout is where to send a guest to pay.
type Checkout struct {
	URL      string
	Amount   int64
	Currency string
	Charged  int64
	TestMode bool
}

// CheckKey checks a key an owner pasted: its shape, its mode against the server, and that
// Payments.lk accepts it. It returns the key's mode.
func (s *Service) CheckKey(ctx context.Context, key string) (string, error) {
	mode, err := domain.CardKeyMode(key)
	if err != nil {
		return "", err
	}
	if mode == "test" && s.Production {
		return "", domain.ErrTestKeyInProduction
	}
	err = s.merchant(store.CardCollector{Key: key}).Check(ctx)
	switch {
	case billing.Unauthorized(err):
		return "", domain.ErrInvalidCardKey
	case err != nil:
		return "", fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	return mode, nil
}

// StartCheckout opens a hosted checkout for part of a guest's stay, or sends them back to the
// open one for the same amount.
func (s *Service) StartCheckout(ctx context.Context, bookingID uuid.UUID, token string, purpose domain.CardPurpose) (Checkout, error) {
	start, err := s.Store.StartCardCheckout(ctx, bookingID, domain.GuestTokenHash(token), purpose, s.now())
	if err != nil {
		return Checkout{}, err
	}
	out := Checkout{Amount: start.Amount, Currency: start.Currency, Charged: start.Charged, TestMode: start.Collector.Mode == "test"}
	m := s.merchant(start.Collector)
	if start.ProviderID != "" {
		c, err := m.GetCheckout(ctx, start.ProviderID)
		if err == nil && c.Status == "open" && c.URL != "" {
			out.URL = c.URL
			return out, nil
		}
		if err == nil {
			// It closed since; open a new one.
			if err := s.Store.ExpireCardCheckout(ctx, start.Collector.Source(), start.ProviderID); err != nil {
				return Checkout{}, err
			}
			return s.StartCheckout(ctx, bookingID, token, purpose)
		}
		return Checkout{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	link := s.StayLink(start.Slug, bookingID, token)
	c, err := m.CreateCheckout(ctx, billing.CardCheckout{
		Reference: start.ID.String(), AmountCents: start.Charged, Description: start.Description,
		SuccessURL: link + "&paid=1", CancelURL: link, ExpiresInMinutes: domain.CardCheckoutMinutes,
	})
	if err != nil {
		return Checkout{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	if err := s.Store.SetCardCheckoutProvider(ctx, start.ID, c.ID); err != nil {
		return Checkout{}, err
	}
	s.log().Info("card checkout opened", "checkout", start.ID, "provider", c.ID, "booking", bookingID)
	out.URL = c.URL
	return out, nil
}

// Check asks Payments.lk about the stay's open checkouts now, so a guest coming back from
// paying sees their stay confirmed without waiting for the events poll. The poll later finds
// the same payment and leaves it alone.
func (s *Service) Check(ctx context.Context, bookingID uuid.UUID, token string) error {
	open, err := s.Store.OpenCardCheckouts(ctx, bookingID, domain.GuestTokenHash(token))
	if err != nil {
		return err
	}
	for _, o := range open {
		c, err := s.merchant(o.Collector).GetCheckout(ctx, o.ProviderID)
		if err != nil {
			s.log().Warn("card checkout check failed", "error", err, "provider", o.ProviderID)
			continue
		}
		ev := store.CardEvent{Source: o.Collector.Source(), Mode: c.Mode, CheckoutID: c.ID}
		switch {
		case c.Payment != nil && paid(c.Payment.Status):
			ev.Type, ev.PaymentID, ev.AmountCents = "payment.succeeded", c.Payment.ID, c.Payment.AmountCents
		case c.Status == "expired":
			ev.Type = "checkout.expired"
		default:
			continue
		}
		if err := s.apply(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// paid reports whether a Payments.lk payment status means the money was taken.
func paid(status string) bool {
	return status == "succeeded" || status == "partially_refunded" || status == "refunded" || status == "disputed"
}

// SendRefunds sends pending card refunds, of one stay or of all, to Payments.lk. A refund it
// turns down for good is marked failed and the team told; one it couldn't reach waits for the
// next run.
func (s *Service) SendRefunds(ctx context.Context, bookingID *uuid.UUID) (int, error) {
	refunds, err := s.Store.ClaimCardRefunds(ctx, bookingID, 50, s.now())
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, r := range refunds {
		got, err := s.merchant(r.Collector).Refund(ctx, r.PaymentRef, r.Charged, "Stay cancelled", r.ID.String())
		switch {
		case err == nil:
			if err := s.Store.AcceptCardRefund(ctx, r.ID, got.ID, s.now()); err != nil {
				return sent, err
			}
			sent++
			s.log().Info("card refund sent", "refund", r.ID, "provider", got.ID)
		case billing.Refused(err) || billing.Unauthorized(err):
			s.log().Warn("card refund refused", "refund", r.ID, "error", err)
			if err := s.Store.FailCardRefund(ctx, r.ID, "Payments.lk refused the refund", s.now()); err != nil {
				return sent, err
			}
		default:
			s.log().Warn("card refund not sent yet", "refund", r.ID, "error", err)
		}
	}
	return sent, nil
}

// PollEvents reads new card events from StayKey's collection account and every owner
// connection, and applies them. One connection failing doesn't stop the rest; a key
// Payments.lk no longer accepts marks its connection failed.
func (s *Service) PollEvents(ctx context.Context) error {
	var errs []error
	if s.CollectKey != "" {
		since, err := s.Store.CardCursor(ctx, store.StayKeyCardSource, s.now())
		if err == nil {
			err = s.poll(ctx, store.CardCollector{By: domain.CollectedByStayKey}, since, func(ctx context.Context, at time.Time) error {
				return s.Store.AdvanceCardCursor(ctx, store.StayKeyCardSource, at)
			})
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("staykey events: %w", err))
		}
	}
	conns, err := s.Store.LiveCardConnections(ctx)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, c := range conns {
		err := s.poll(ctx, c.Collector, c.EventsSince, func(ctx context.Context, at time.Time) error {
			return s.Store.AdvanceCardEvents(ctx, c.Collector.AccountID, at)
		})
		if billing.Unauthorized(err) {
			s.log().Warn("card key refused, connection marked failed", "account", c.Collector.AccountID)
			err = s.Store.CardKeyRefused(ctx, c.Collector.AccountID, s.now())
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("events for account %s: %w", c.Collector.AccountID, err))
		}
	}
	return errors.Join(errs...)
}

// eventOverlap reads a little before the last event seen, since events created while the
// previous poll ran may not have been listed yet. Repeats are recorded once.
const eventOverlap = 2 * time.Minute

func (s *Service) poll(ctx context.Context, c store.CardCollector, since time.Time, advance func(context.Context, time.Time) error) error {
	m := s.merchant(c)
	events, err := m.Events(ctx, since.Add(-eventOverlap))
	if err != nil {
		return err
	}
	for _, e := range events {
		ev := store.CardEvent{Source: c.Source(), ID: e.ID, Type: e.Type, Mode: e.Mode, CheckoutID: e.CheckoutID}
		switch {
		case e.Payment != nil:
			ev.CheckoutID, ev.PaymentID, ev.AmountCents = e.Payment.CheckoutID, e.Payment.ID, e.Payment.AmountCents
		case e.Refund != nil:
			ev.RefundID, ev.RefundPaymentID, ev.RefundCents, ev.Failure = e.Refund.ID, e.Refund.PaymentID, e.Refund.AmountCents, e.Refund.FailureMessage
		}
		if err := s.apply(ctx, ev); err != nil {
			return err
		}
		if e.Payment != nil && c.By == domain.CollectedByStayKey {
			s.recordFee(ctx, m, e.Payment.ID)
		}
		if err := advance(ctx, e.Created); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) apply(ctx context.Context, ev store.CardEvent) error {
	outcome, err := s.Store.ApplyCardEvent(ctx, ev, s.now())
	if err != nil {
		return err
	}
	switch outcome {
	case store.CardApplied:
		s.log().Info("card event applied", "type", ev.Type, "event", ev.ID, "checkout", ev.CheckoutID, "payment", ev.PaymentID, "refund", ev.RefundID)
	case store.CardAmountMismatch:
		s.log().Error("card payment refused: amount differs from its checkout", "checkout", ev.CheckoutID, "payment", ev.PaymentID)
	}
	return nil
}

// recordFee keeps Payments.lk's fee on a payment StayKey collected, for reconciliation. It is
// best effort: the fee may not be known yet, and nothing depends on it.
func (s *Service) recordFee(ctx context.Context, m billing.Merchant, paymentID string) {
	p, err := m.GetPayment(ctx, paymentID)
	if err != nil || p.FeeCents == nil {
		return
	}
	if err := s.Store.SetCollectedFee(ctx, paymentID, *p.FeeCents); err != nil {
		s.log().Warn("card fee not recorded", "payment", paymentID, "error", err)
	}
}

func (s *Service) merchant(c store.CardCollector) billing.Merchant {
	key := c.Key
	if c.By == domain.CollectedByStayKey {
		key = s.CollectKey
	}
	return billing.Merchant{SecretKey: key, BaseURL: s.BaseURL, Client: s.Client}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}
