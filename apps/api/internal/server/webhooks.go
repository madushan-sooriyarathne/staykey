package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/billing"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/email"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/store"
)

// Webhooks from the billing and email providers. They have no signed-in user: each is checked
// against its provider's secret or signature, and the account comes from the event. Bodies are
// never logged.

// maxWebhookBody is far above any event either provider sends.
const maxWebhookBody = 1 << 20

// noEvent applies a change that no webhook delivery stands behind, such as a refresh.
var noEvent = store.BillingEvent{}

type rcWebhook struct {
	Event struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		AppUserID string `json:"app_user_id"`
	} `json:"event"`
}

// RevenueCatWebhook acts on a RevenueCat event by reading the customer back from RevenueCat,
// since the event body alone could be replayed or out of date.
func (s *Server) RevenueCatWebhook(ctx context.Context, req oapi.RevenueCatWebhookRequestObject) (oapi.RevenueCatWebhookResponseObject, error) {
	want := s.opts.RevenueCatWebhookAuth
	if s.opts.Subscribers == nil || want == "" {
		return oapi.RevenueCatWebhook503JSONResponse{ServiceUnavailableJSONResponse: oapi.ServiceUnavailableJSONResponse(errBody(errBillingOff.code, errBillingOff.message))}, nil
	}
	got := ""
	if req.Params.Authorization != nil {
		got = *req.Params.Authorization
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return oapi.RevenueCatWebhook401JSONResponse{UnauthorizedJSONResponse: oapi.UnauthorizedJSONResponse(errBody("unauthorized", "Wrong webhook authorization."))}, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxWebhookBody))
	if err != nil {
		return nil, fmt.Errorf("read revenuecat webhook: %w", err)
	}
	var hook rcWebhook
	if err := json.Unmarshal(body, &hook); err != nil || hook.Event.ID == "" {
		s.log.Warn("revenuecat webhook without an event")
		return oapi.RevenueCatWebhook200Response{}, nil
	}
	ev := store.BillingEvent{Source: "revenuecat", EventID: hook.Event.ID, Type: hook.Event.Type}
	accountID, err := uuid.Parse(hook.Event.AppUserID)
	if hook.Event.Type == "TEST" || err != nil {
		// Test pings and customers that never signed in (anonymous ids) name no account.
		return oapi.RevenueCatWebhook200Response{}, s.recordIgnored(ctx, ev)
	}
	subs, err := s.opts.Subscribers.Subscriber(ctx, accountID.String())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	ev.AccountID = &accountID
	err = s.store.ApplyStoreSubscriptions(ctx, ev, accountID, subs, now, now)
	switch {
	case errors.Is(err, store.ErrDuplicateEvent):
	case errors.Is(err, domain.ErrNotFound):
		ev.AccountID = nil
		return oapi.RevenueCatWebhook200Response{}, s.recordIgnored(ctx, ev)
	case err != nil:
		return nil, err
	default:
		s.log.Info("store plan updated", "event_id", ev.EventID, "type", ev.Type, "account_id", accountID)
	}
	return oapi.RevenueCatWebhook200Response{}, nil
}

// PaymentsLkWebhook acts on a signed Payments.lk event: a paid checkout extends the account's web
// plan, an expired one is closed. Anything else is recorded and left alone.
func (s *Server) PaymentsLkWebhook(ctx context.Context, req oapi.PaymentsLkWebhookRequestObject) (oapi.PaymentsLkWebhookResponseObject, error) {
	gw := s.opts.WebPayments
	if gw == nil {
		return oapi.PaymentsLkWebhook503JSONResponse{ServiceUnavailableJSONResponse: oapi.ServiceUnavailableJSONResponse(errBody(errBillingOff.code, errBillingOff.message))}, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxWebhookBody))
	if err != nil {
		return nil, fmt.Errorf("read payments.lk webhook: %w", err)
	}
	header := ""
	if req.Params.PaymentsSignature != nil {
		header = *req.Params.PaymentsSignature
	}
	event, err := gw.VerifyEvent(body, header)
	if err != nil {
		s.log.Warn("payments.lk webhook refused", "error", err)
		return oapi.PaymentsLkWebhook400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(errBody("bad_signature", "The webhook didn't verify."))}, nil
	}
	ev := store.BillingEvent{Source: "paymentslk", EventID: event.ID, Type: event.Type}

	switch {
	case event.Type == "payment.succeeded" && event.Payment != nil && event.Payment.Status == "succeeded":
		return oapi.PaymentsLkWebhook200Response{}, s.webPaymentSucceeded(ctx, ev, *event.Payment)
	case event.Type == "checkout.expired" && event.ExpiredCheckoutID != "":
		ev.Payload = map[string]any{"checkoutId": event.ExpiredCheckoutID}
		err := s.store.ExpireWebCheckout(ctx, ev, event.ExpiredCheckoutID)
		if err != nil && !errors.Is(err, store.ErrDuplicateEvent) {
			return nil, err
		}
		return oapi.PaymentsLkWebhook200Response{}, nil
	default:
		return oapi.PaymentsLkWebhook200Response{}, s.recordIgnored(ctx, ev)
	}
}

func (s *Server) webPaymentSucceeded(ctx context.Context, ev store.BillingEvent, pay billing.Payment) error {
	ev.Payload = map[string]any{
		"paymentId": pay.ID, "checkoutId": pay.CheckoutID, "reference": pay.Reference,
		"amount": pay.AmountCents, "currency": pay.Currency, "mode": pay.Mode,
	}
	checkoutID, err := uuid.Parse(pay.Reference)
	if err != nil {
		// A payment link or checkout made outside StayKey.
		return s.recordIgnored(ctx, ev)
	}
	now := time.Now()
	paid, err := s.store.PayWebCheckout(ctx, ev, store.WebPayment{
		CheckoutID: checkoutID, PaymentID: pay.ID, Amount: pay.AmountCents, Currency: pay.Currency,
	}, now)
	switch {
	case errors.Is(err, store.ErrDuplicateEvent):
		return nil
	case errors.Is(err, domain.ErrNotFound):
		return s.recordIgnored(ctx, ev)
	case errors.Is(err, store.ErrPaymentMismatch):
		// Retrying won't change the amount, so the event is kept for a person to look at.
		s.log.Error("web payment doesn't match its checkout", "event_id", ev.EventID, "checkout_id", checkoutID)
		return s.recordIgnored(ctx, ev)
	case err != nil:
		return err
	}
	s.log.Info("web plan paid", "event_id", ev.EventID, "account_id", paid.AccountID, "until", paid.End)

	if rc := s.opts.ExternalPurchases; rc != nil && rc.Enabled() {
		err := rc.RecordExternal(ctx, billing.ExternalPurchase{
			AccountID: paid.AccountID.String(), PaymentID: paid.PaymentID, ProductID: domain.ProductID(paid.Plan, paid.Period),
			PeriodStart: paid.Start, PeriodEnd: paid.End, Amount: paid.Amount, Currency: paid.Currency,
			Sandbox: s.opts.WebPayments.Sandbox(),
		})
		if err != nil {
			// The plan already counts; RevenueCat only mirrors it.
			s.log.Warn("record web purchase in revenuecat", "account_id", paid.AccountID, "error", err)
		}
	}
	return nil
}

// recordIgnored keeps a delivery the API doesn't act on, so a retry is recognised.
func (s *Server) recordIgnored(ctx context.Context, ev store.BillingEvent) error {
	err := s.store.RecordBillingEvent(ctx, ev)
	if err != nil && !errors.Is(err, store.ErrDuplicateEvent) {
		return err
	}
	return nil
}

// SNSChecker verifies SNS messages and confirms subscriptions. email.SNS implements it.
type SNSChecker interface {
	Verify(ctx context.Context, body []byte) (email.SNSMessage, error)
	ConfirmSubscription(ctx context.Context, m email.SNSMessage) error
}

// SesWebhook records the permanent bounces and complaints SES reports through SNS, so those
// addresses are never emailed again. A subscription request is confirmed once.
func (s *Server) SesWebhook(ctx context.Context, req oapi.SesWebhookRequestObject) (oapi.SesWebhookResponseObject, error) {
	if s.opts.SNS == nil {
		return oapi.SesWebhook503JSONResponse{ServiceUnavailableJSONResponse: oapi.ServiceUnavailableJSONResponse(
			errBody("email_not_configured", "Email isn't set up on this server yet."))}, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxWebhookBody))
	if err != nil {
		return nil, fmt.Errorf("read ses webhook: %w", err)
	}
	m, err := s.opts.SNS.Verify(ctx, body)
	if err != nil {
		s.log.Warn("ses webhook refused", "error", err)
		return oapi.SesWebhook400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(errBody("bad_signature", "The notification didn't verify."))}, nil
	}
	switch m.Type {
	case email.SNSSubscriptionConfirm:
		if err := s.opts.SNS.ConfirmSubscription(ctx, m); err != nil {
			return nil, err
		}
		s.log.Info("ses notifications subscribed", "topic", m.TopicArn)
	case email.SNSNotification:
		found, err := email.Suppressions(m.Message)
		if err != nil {
			s.log.Warn("ses notification unreadable", "message_id", m.MessageID)
			return oapi.SesWebhook200Response{}, nil
		}
		for _, sup := range found {
			if err := s.store.RecordSuppression(ctx, sup.Address, sup.Reason); err != nil {
				return nil, err
			}
			s.log.Info("email address suppressed", "to", email.MaskAddress(sup.Address), "reason", sup.Reason)
		}
	}
	return oapi.SesWebhook200Response{}, nil
}
