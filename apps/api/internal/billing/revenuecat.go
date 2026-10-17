// Package billing talks to the payment providers: RevenueCat for App Store and Google Play
// plans, and Payments.lk for prepaid periods bought on the web. The API decides access from
// what these report; nothing here touches the database.
package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"staykey.direct/api/internal/domain"
)

const revenueCatURL = "https://api.revenuecat.com"

// RevenueCat reads subscribers and records web purchases through RevenueCat's REST API.
type RevenueCat struct {
	// SecretKey reads subscribers (a v1 secret key).
	SecretKey string
	// ExternalKey is the External Source app's secret key. Empty leaves web purchases out of
	// RevenueCat.
	ExternalKey string

	BaseURL string       // default: RevenueCat's API
	Client  *http.Client // default: 15 second timeout
}

type rcSubscriber struct {
	Subscriber struct {
		Subscriptions map[string]rcSubscription `json:"subscriptions"`
	} `json:"subscriber"`
}

type rcSubscription struct {
	Store                   string     `json:"store"`
	ExpiresDate             *time.Time `json:"expires_date"`
	GracePeriodExpiresDate  *time.Time `json:"grace_period_expires_date"`
	UnsubscribeDetectedAt   *time.Time `json:"unsubscribe_detected_at"`
	BillingIssuesDetectedAt *time.Time `json:"billing_issues_detected_at"`
	RefundedAt              *time.Time `json:"refunded_at"`
	StoreTransactionID      string     `json:"store_transaction_id"`
}

// Subscriber returns the App Store and Google Play plans RevenueCat knows for the customer.
// Products that aren't StayKey plans, promotional grants and web stores are left out.
func (rc RevenueCat) Subscriber(ctx context.Context, appUserID string) ([]domain.Subscription, error) {
	var body rcSubscriber
	if err := rc.do(ctx, http.MethodGet, "/v1/subscribers/"+url.PathEscape(appUserID), rc.SecretKey, nil, &body); err != nil {
		return nil, err
	}
	var out []domain.Subscription
	for productID, s := range body.Subscriber.Subscriptions {
		source := domain.SubscriptionSource(s.Store)
		plan, period, ok := domain.ParseProduct(productID)
		if !ok || !source.IsStore() || s.ExpiresDate == nil {
			continue
		}
		end := *s.ExpiresDate
		if s.GracePeriodExpiresDate != nil && s.GracePeriodExpiresDate.After(end) {
			end = *s.GracePeriodExpiresDate
		}
		status := domain.SubActive
		switch {
		case s.RefundedAt != nil:
			status, end = domain.SubExpired, minTime(end, *s.RefundedAt)
		case s.BillingIssuesDetectedAt != nil:
			status = domain.SubBillingIssue
		case s.UnsubscribeDetectedAt != nil:
			status = domain.SubCancelled
		}
		out = append(out, domain.Subscription{
			Source: source, Plan: plan, Period: period, Status: status,
			WillRenew: status == domain.SubActive, PeriodEnd: end,
			ProductID: productID, ExternalID: s.StoreTransactionID,
		})
	}
	return out, nil
}

// ExternalPurchase is one paid web period, as RevenueCat's External Purchases API records it.
type ExternalPurchase struct {
	AccountID   string
	PaymentID   string
	ProductID   string
	PeriodStart time.Time
	PeriodEnd   time.Time
	// Amount in minor units of Currency.
	Amount   int64
	Currency string
	Sandbox  bool
}

// Enabled reports whether web purchases are recorded in RevenueCat.
func (rc RevenueCat) Enabled() bool { return rc.ExternalKey != "" }

// RecordExternal posts a paid web period to RevenueCat (External Purchases API, private beta).
// RevenueCat deduplicates by payment id, so a retry is safe.
func (rc RevenueCat) RecordExternal(ctx context.Context, p ExternalPurchase) error {
	env := "production"
	if p.Sandbox {
		env = "sandbox"
	}
	stamp := func(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05") }
	// Each account's web periods form one subscription that never renews on its own.
	subID := "web-" + p.AccountID
	body := map[string]any{
		"purchase": map[string]any{
			"object":                         "external_subscription",
			"customer_id":                    p.AccountID,
			"source_subscription_identifier": subID,
			"source_product_identifier":      p.ProductID,
			"updated_at":                     stamp(p.PeriodStart),
			"current_period_starts_at":       stamp(p.PeriodStart),
			"current_period_ends_at":         stamp(p.PeriodEnd),
			"gives_access":                   true,
			"status":                         "active",
			"environment":                    env,
			"auto_renewal_status":            "will_not_renew",
		},
		"payment": map[string]any{
			"object":                         "external_subscription_payment",
			"source_subscription_identifier": subID,
			"payment_identifier":             p.PaymentID,
			"processed_at":                   stamp(p.PeriodStart),
			"amount_in_local_currency":       map[string]any{"gross": float64(p.Amount) / 100, "currency": p.Currency},
		},
	}
	return rc.do(ctx, http.MethodPost, "/v1/receipts/external", rc.ExternalKey, body, nil)
}

func (rc RevenueCat) do(ctx context.Context, method, path, key string, in, out any) error {
	base, client := rc.BaseURL, rc.Client
	if base == "" {
		base = revenueCatURL
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if key == "" {
		return errors.New("revenuecat: no API key")
	}
	var reader io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("revenuecat: encode request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return fmt.Errorf("revenuecat: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("revenuecat: %s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("revenuecat: read response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("revenuecat: %s answered %d", method, res.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("revenuecat: decode response: %w", err)
	}
	return nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
