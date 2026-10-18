package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// Billing methods. Provider events name the account they act on; owner routes go through a
// tenant. Provider calls happen before these run, never inside their transactions.

// BillingEvent is one webhook delivery. Payload holds only the ids and amounts worth keeping,
// never card or customer details.
type BillingEvent struct {
	Source    string // revenuecat or paymentslk
	EventID   string
	Type      string
	AccountID *uuid.UUID
	Payload   map[string]any
}

// ErrDuplicateEvent means the delivery was already applied.
var ErrDuplicateEvent = errors.New("billing event already applied")

// ErrPaymentMismatch means a payment's amount or currency isn't what the checkout asked for.
var ErrPaymentMismatch = errors.New("payment doesn't match its checkout")

// AccountBilling returns the account's plan and status as of now, without changing anything.
func (s *Postgres) AccountBilling(ctx context.Context, accountID uuid.UUID, now time.Time) (domain.AccountBilling, error) {
	q := s.db(ctx)
	r, err := q.GetAccountBilling(ctx, accountID)
	if err != nil {
		return domain.AccountBilling{}, notFound(err)
	}
	units, err := q.BookableUnits(ctx, accountID)
	if err != nil {
		return domain.AccountBilling{}, fmt.Errorf("bookable units: %w", err)
	}
	sub := subscription(r)
	return domain.AccountBilling{
		Billing:    domain.BillingState(now, r.TrialStartedAt, r.FirstDirectBookingAt, sub),
		Sub:        sub,
		Units:      int(units),
		OwnerPhone: r.OwnerPhone,
	}, nil
}

// Subscription is AccountBilling for the tenant's own account.
func (s *Postgres) Subscription(ctx context.Context, t tenant.Tenant, now time.Time) (domain.AccountBilling, error) {
	return s.AccountBilling(ctx, t.AccountID, now)
}

// ApplyStoreSubscriptions records the store plans RevenueCat reports for the account and updates
// its status. ev, when it has an id, is recorded in the same transaction, and a delivery seen
// before gives ErrDuplicateEvent.
func (s *Postgres) ApplyStoreSubscriptions(ctx context.Context, ev BillingEvent, accountID uuid.UUID, subs []domain.Subscription, eventAt, now time.Time) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		if _, err := q.GetAccountBilling(ctx, accountID); err != nil {
			return notFound(err)
		}
		if err := s.recordEvent(ctx, ev); err != nil {
			return err
		}
		var newest *domain.Subscription
		for i := range subs {
			if newest == nil || subs[i].PeriodEnd.After(newest.PeriodEnd) {
				newest = &subs[i]
			}
		}
		if newest != nil {
			if err := s.upsertSubscription(ctx, accountID, *newest, eventAt); err != nil {
				return err
			}
		}
		return s.recomputeBilling(ctx, accountID, now)
	})
}

// RecordBillingEvent keeps a delivery the API doesn't act on, so a retry is recognised.
func (s *Postgres) RecordBillingEvent(ctx context.Context, ev BillingEvent) error {
	return s.recordEvent(ctx, ev)
}

// CreateWebCheckout starts a web purchase of one period for the tenant's account. It refuses
// while a store plan gives access. The amount comes from domain.Plans, never the request.
func (s *Postgres) CreateWebCheckout(ctx context.Context, t tenant.Tenant, plan domain.PlanID, period domain.Period, now time.Time) (uuid.UUID, int64, error) {
	p, ok := domain.FindPlan(plan)
	if !ok || !period.Valid() {
		return uuid.Nil, 0, &domain.ValidationError{Field: "plan", Message: "choose a plan and a period"}
	}
	b, err := s.AccountBilling(ctx, t.AccountID, now)
	if err != nil {
		return uuid.Nil, 0, err
	}
	if b.Sub != nil && b.Sub.Source.IsStore() && b.Sub.GivesAccess(now) {
		return uuid.Nil, 0, domain.ErrAlreadySubscribed
	}
	id := domain.NewID()
	amount := p.Price(period)
	err = s.db(ctx).InsertWebCheckout(ctx, queries.InsertWebCheckoutParams{
		ID: id, AccountID: t.AccountID, Plan: string(plan), Period: string(period),
		Amount: amount, Currency: domain.WebCurrency, CreatedAt: now,
	})
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("insert web checkout: %w", err)
	}
	return id, amount, nil
}

// SetWebCheckoutProvider keeps the provider's checkout id, which its expiry event names.
func (s *Postgres) SetWebCheckoutProvider(ctx context.Context, t tenant.Tenant, id uuid.UUID, providerID string) error {
	if err := s.db(ctx).SetWebCheckoutProvider(ctx, queries.SetWebCheckoutProviderParams{ID: id, ProviderID: providerID}); err != nil {
		return fmt.Errorf("set checkout provider: %w", err)
	}
	return nil
}

// WebPayment is a successful payment Payments.lk reported.
type WebPayment struct {
	CheckoutID uuid.UUID // our reference
	PaymentID  string
	Amount     int64
	Currency   string
}

// PayWebCheckout marks a checkout paid and extends the account's web plan by its period,
// starting when the current web period ends or now, whichever is later. It records ev in the
// same transaction; a repeated delivery, or a second payment event for a paid checkout, gives
// ErrDuplicateEvent and changes nothing.
func (s *Postgres) PayWebCheckout(ctx context.Context, ev BillingEvent, pay WebPayment, now time.Time) (domain.PaidPeriod, error) {
	var out domain.PaidPeriod
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		c, err := q.GetWebCheckoutForUpdate(ctx, pay.CheckoutID)
		if err != nil {
			return notFound(err)
		}
		ev.AccountID = &c.AccountID
		if err := s.recordEvent(ctx, ev); err != nil {
			return err
		}
		if c.Status == "paid" {
			return ErrDuplicateEvent
		}
		if pay.Amount != c.Amount || pay.Currency != c.Currency {
			return fmt.Errorf("%w: checkout %s", ErrPaymentMismatch, c.ID)
		}
		start := now
		current, err := q.GetSubscription(ctx, c.AccountID)
		switch {
		case err == nil && current.Source == string(domain.SourceWeb) && current.CurrentPeriodEnd.After(now):
			start = current.CurrentPeriodEnd
		case err != nil && !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("get subscription: %w", err)
		}
		plan, period := domain.PlanID(c.Plan), domain.Period(c.Period)
		end := period.After(start)
		if err := q.MarkWebCheckoutPaid(ctx, queries.MarkWebCheckoutPaidParams{
			ID: c.ID, ProviderOrderID: pay.PaymentID, PeriodStart: &start, PeriodEnd: &end, PaidAt: &now,
		}); err != nil {
			return fmt.Errorf("mark checkout paid: %w", err)
		}
		sub := domain.Subscription{
			Source: domain.SourceWeb, Plan: plan, Period: period, Status: domain.SubActive, PeriodEnd: end,
			ProductID: domain.ProductID(plan, period), ExternalID: c.ID.String(),
		}
		if err := s.upsertSubscription(ctx, c.AccountID, sub, now); err != nil {
			return err
		}
		out = domain.PaidPeriod{
			AccountID: c.AccountID, PaymentID: pay.PaymentID, Plan: plan, Period: period,
			Start: start, End: end, Amount: c.Amount, Currency: c.Currency,
		}
		if err := s.queueReceipt(ctx, c.ID, out, now); err != nil {
			return err
		}
		return s.recomputeBilling(ctx, c.AccountID, now)
	})
	return out, err
}

// ExpireWebCheckout marks a checkout the owner never paid as failed.
func (s *Postgres) ExpireWebCheckout(ctx context.Context, ev BillingEvent, providerID string) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		if err := s.recordEvent(ctx, ev); err != nil {
			return err
		}
		if _, err := s.db(ctx).MarkWebCheckoutExpired(ctx, providerID); err != nil {
			return fmt.Errorf("expire checkout: %w", err)
		}
		return nil
	})
}

// RecomputeAllBilling brings every account's status up to date with the clock, one account per
// transaction, and reports how many changed.
func (s *Postgres) RecomputeAllBilling(ctx context.Context, now time.Time) (int, error) {
	ids, err := s.db(ctx).ListAccountsForBilling(ctx)
	if err != nil {
		return 0, fmt.Errorf("list accounts: %w", err)
	}
	changed := 0
	for _, id := range ids {
		var moved bool
		err := s.InTx(ctx, func(ctx context.Context) error {
			var err error
			moved, err = s.recompute(ctx, id, now)
			return err
		})
		if err != nil {
			return changed, fmt.Errorf("account %s: %w", id, err)
		}
		if moved {
			changed++
		}
	}
	return changed, nil
}

// directBookingMade ends the free period on the account's first direct booking: a page or
// widget stay that reached awaiting_payment or confirmed. Call it inside the booking's
// transaction.
func (s *Postgres) directBookingMade(ctx context.Context, accountID uuid.UUID, b domain.Booking, now time.Time) error {
	if b.Source != "page" && b.Source != "widget" {
		return nil
	}
	if b.Status != domain.StatusAwaitingPayment && b.Status != domain.StatusConfirmed {
		return nil
	}
	n, err := s.db(ctx).MarkFirstDirectBooking(ctx, queries.MarkFirstDirectBookingParams{ID: accountID, At: &now})
	if err != nil {
		return fmt.Errorf("mark first direct booking: %w", err)
	}
	if n == 0 {
		return nil
	}
	return s.recomputeBilling(ctx, accountID, now)
}

func (s *Postgres) recomputeBilling(ctx context.Context, accountID uuid.UUID, now time.Time) error {
	_, err := s.recompute(ctx, accountID, now)
	return err
}

// recompute sets the account's status from its billing state and, when it changes, adds the
// line the owner sees. The update only applies to a status that differs, so two runs at once
// write one line between them.
func (s *Postgres) recompute(ctx context.Context, accountID uuid.UUID, now time.Time) (bool, error) {
	b, err := s.AccountBilling(ctx, accountID, now)
	if err != nil {
		return false, err
	}
	n, err := s.db(ctx).SetAccountStatus(ctx, queries.SetAccountStatusParams{ID: accountID, Status: string(b.Status)})
	if err != nil {
		return false, fmt.Errorf("set account status: %w", err)
	}
	if n == 0 {
		return false, nil
	}
	if line, ok := domain.BillingChanged(b.Billing, b.Sub); ok {
		if err := s.addActivity(ctx, accountID, line, nil, now); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *Postgres) recordEvent(ctx context.Context, ev BillingEvent) error {
	if ev.EventID == "" {
		return nil
	}
	payload, err := json.Marshal(ev.Payload)
	if err != nil {
		return fmt.Errorf("encode billing event: %w", err)
	}
	if ev.Payload == nil {
		payload = []byte("{}")
	}
	n, err := s.db(ctx).InsertBillingEvent(ctx, queries.InsertBillingEventParams{
		ID: domain.NewID(), AccountID: ev.AccountID, Source: ev.Source, EventID: ev.EventID, Type: ev.Type, Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("insert billing event: %w", err)
	}
	if n == 0 {
		return ErrDuplicateEvent
	}
	return nil
}

func (s *Postgres) upsertSubscription(ctx context.Context, accountID uuid.UUID, sub domain.Subscription, eventAt time.Time) error {
	_, err := s.db(ctx).UpsertSubscription(ctx, queries.UpsertSubscriptionParams{
		AccountID: accountID, Source: string(sub.Source), Plan: string(sub.Plan), Period: string(sub.Period),
		Status: string(sub.Status), WillRenew: sub.WillRenew, CurrentPeriodEnd: sub.PeriodEnd,
		ProductID: sub.ProductID, ExternalID: sub.ExternalID, LastEventAt: eventAt,
	})
	if err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	return nil
}

func subscription(r queries.GetAccountBillingRow) *domain.Subscription {
	if r.SubSource == nil {
		return nil
	}
	return &domain.Subscription{
		Source:    domain.SubscriptionSource(*r.SubSource),
		Plan:      domain.PlanID(*r.SubPlan),
		Period:    domain.Period(*r.SubPeriod),
		Status:    domain.SubscriptionStatus(*r.SubStatus),
		WillRenew: *r.SubWillRenew,
		PeriodEnd: *r.SubPeriodEnd,
	}
}

// queueReceipt queues the receipt for a paid web period to the account's first owner, when
// they have a confirmed email.
func (s *Postgres) queueReceipt(ctx context.Context, checkoutID uuid.UUID, p domain.PaidPeriod, now time.Time) error {
	owner, err := s.db(ctx).OwnerConfirmedEmail(ctx, p.AccountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("owner email: %w", err)
	}
	return s.queueEmail(ctx, queuedEmail{
		accountID: &p.AccountID, userID: &owner.ID, kind: domain.EmailPlanReceipt, to: owner.Email,
		payload: emailPayload{Name: owner.Name, Receipt: &p}, dedupeKey: checkoutID.String() + ":plan_receipt", at: now,
	})
}
