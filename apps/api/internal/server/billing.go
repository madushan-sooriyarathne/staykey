package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"staykey.direct/api/internal/billing"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// SubscriberSource reads an account's App Store and Google Play plans. billing.RevenueCat
// implements it.
type SubscriberSource interface {
	Subscriber(ctx context.Context, appUserID string) ([]domain.Subscription, error)
}

// ExternalRecorder records a paid web period in RevenueCat. billing.RevenueCat implements it.
type ExternalRecorder interface {
	Enabled() bool
	RecordExternal(ctx context.Context, p billing.ExternalPurchase) error
}

// WebGateway takes web payments. billing.PaymentsLK implements it.
type WebGateway interface {
	CreateCheckout(ctx context.Context, r billing.CheckoutRequest) (billing.Checkout, error)
	VerifyEvent(body []byte, header string) (billing.Event, error)
	Sandbox() bool
}

var errBillingOff = &apiError{status: http.StatusServiceUnavailable, code: "billing_not_configured", message: "Billing isn't set up on this server yet."}

// GetSubscription returns the account's plan and status. Every role may read it.
func (s *Server) GetSubscription(ctx context.Context, _ oapi.GetSubscriptionRequestObject) (oapi.GetSubscriptionResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	b, err := s.store.Subscription(ctx, t, now)
	if err != nil {
		return nil, err
	}
	return oapi.GetSubscription200JSONResponse(subscriptionToAPI(b, t, now)), nil
}

// RefreshSubscription reads the account's store plans from RevenueCat, for right after a
// purchase or restore in the app.
func (s *Server) RefreshSubscription(ctx context.Context, _ oapi.RefreshSubscriptionRequestObject) (oapi.RefreshSubscriptionResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermBilling) {
		return nil, errForbidden
	}
	if s.opts.Subscribers == nil {
		return nil, errBillingOff
	}
	subs, err := s.opts.Subscribers.Subscriber(ctx, t.AccountID.String())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.store.ApplyStoreSubscriptions(ctx, noEvent, t.AccountID, subs, now, now); err != nil {
		return nil, err
	}
	b, err := s.store.Subscription(ctx, t, now)
	if err != nil {
		return nil, err
	}
	return oapi.RefreshSubscription200JSONResponse(subscriptionToAPI(b, t, now)), nil
}

// CreateCheckout starts a Payments.lk checkout for one prepaid period. The price comes from
// domain.Plans.
func (s *Server) CreateCheckout(ctx context.Context, req oapi.CreateCheckoutRequestObject) (oapi.CreateCheckoutResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermBilling) {
		return nil, errForbidden
	}
	if s.opts.WebPayments == nil || s.opts.BillingURL == "" {
		return nil, errBillingOff
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	plan, period := domain.PlanID(req.Body.Plan), domain.Period(req.Body.Period)
	p, ok := domain.FindPlan(plan)
	if !ok || !period.Valid() {
		return nil, apiErr(&domain.ValidationError{Field: "plan", Message: "choose a plan and monthly or yearly"}, "")
	}
	now := time.Now()
	id, amount, err := s.store.CreateWebCheckout(ctx, t, plan, period, now)
	if errors.Is(err, domain.ErrAlreadySubscribed) {
		return oapi.CreateCheckout409JSONResponse{ConflictJSONResponse: oapi.ConflictJSONResponse{
			Code: oapi.AlreadySubscribed, Message: "Your plan renews through the App Store or Google Play. Manage it there.",
		}}, nil
	}
	if err != nil {
		return nil, err
	}
	length := "1 month"
	if period == domain.PeriodYearly {
		length = "1 year"
	}
	c, err := s.opts.WebPayments.CreateCheckout(ctx, billing.CheckoutRequest{
		Reference: id.String(), AmountCents: amount, Description: "StayKey " + p.Name + " plan, " + length,
		SuccessURL: s.opts.BillingURL + "/done", CancelURL: s.opts.BillingURL,
	})
	if err != nil {
		return nil, err
	}
	if err := s.store.SetWebCheckoutProvider(ctx, t, id, c.ID); err != nil {
		return nil, err
	}
	return oapi.CreateCheckout201JSONResponse{Url: c.URL, Amount: amount, Currency: oapi.Currency(domain.WebCurrency)}, nil
}

func subscriptionToAPI(b domain.AccountBilling, t tenant.Tenant, now time.Time) oapi.Subscription {
	out := oapi.Subscription{
		Status:      oapi.AccountStatus(b.Status),
		Until:       b.Until,
		FreeEndsAt:  b.FreeEndsAt,
		Units:       b.Units,
		Fits:        oapi.PlanId(domain.PlanFor(b.Units)),
		CanPurchase: t.Can(domain.PermBilling) && !b.Sub.GivesAccess(now),
	}
	if sub := b.Sub; sub != nil {
		plan, period, source := oapi.PlanId(sub.Plan), oapi.BillingPeriod(sub.Period), oapi.SubscriptionSource(sub.Source)
		out.Plan, out.Period, out.Source = &plan, &period, &source
		out.WillRenew, out.PeriodEnd = &sub.WillRenew, &sub.PeriodEnd
	}
	return out
}

// ListPlans returns the plans with their web prices, for the marketing site.
func (s *Server) ListPlans(_ context.Context, _ oapi.ListPlansRequestObject) (oapi.ListPlansResponseObject, error) {
	out := oapi.PlanList{Items: make([]oapi.Plan, len(domain.Plans))}
	for i, p := range domain.Plans {
		out.Items[i] = oapi.Plan{
			Id: oapi.PlanId(p.ID), Name: p.Name, Units: p.Units, Currency: oapi.Currency(domain.WebCurrency),
			Monthly: p.Monthly, Yearly: p.Yearly,
		}
	}
	return oapi.ListPlans200JSONResponse(out), nil
}
