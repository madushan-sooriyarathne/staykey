package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const paymentsLKURL = "https://api.payments.lk"

// SignatureTolerance is how far a webhook's timestamp may be from our clock.
const SignatureTolerance = 5 * time.Minute

// ErrBadSignature means a webhook didn't verify: wrong secret, altered body, or too old.
var ErrBadSignature = errors.New("payments.lk: webhook signature doesn't verify")

// PaymentsLK creates hosted checkouts on Payments.lk and verifies its webhooks. Amounts are LKR
// cents, the only currency it takes.
type PaymentsLK struct {
	// SecretKey is sk_test_... in the sandbox and sk_live_... live.
	SecretKey string
	// WebhookSecret is the endpoint's signing secret, whsec_...
	WebhookSecret string

	BaseURL string           // default: Payments.lk's API
	Client  *http.Client     // default: 30 second timeout, no redirects
	Now     func() time.Time // default: time.Now
}

// CheckoutRequest is one payment to take.
type CheckoutRequest struct {
	// Reference is our web checkout id. It also keys the request, so a retry never makes a
	// second checkout.
	Reference   string
	AmountCents int64
	Description string
	SuccessURL  string
	CancelURL   string
}

// Checkout is a hosted checkout to send the owner to.
type Checkout struct {
	ID  string
	URL string
}

// Sandbox reports whether the key is a test key.
func (p PaymentsLK) Sandbox() bool { return strings.HasPrefix(p.SecretKey, "sk_test_") }

// CreateCheckout starts a hosted checkout for one amount.
func (p PaymentsLK) CreateCheckout(ctx context.Context, r CheckoutRequest) (Checkout, error) {
	body := map[string]any{
		"amountCents": r.AmountCents,
		"description": r.Description,
		"reference":   r.Reference,
		"successUrl":  r.SuccessURL,
		"cancelUrl":   r.CancelURL,
	}
	var c struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := request(ctx, p.client(), p.base(), p.SecretKey, http.MethodPost, "/v1/checkouts", r.Reference, body, &c); err != nil {
		return Checkout{}, fmt.Errorf("payments.lk: create checkout: %w", err)
	}
	if c.ID == "" || !strings.HasPrefix(c.URL, "https://") {
		return Checkout{}, errors.New("payments.lk: checkout came back without an id or an https url")
	}
	return Checkout{ID: c.ID, URL: c.URL}, nil
}

// Event is a verified webhook. Payment is set for payment events; ExpiredCheckoutID for
// checkout.expired.
type Event struct {
	ID                string
	Type              string
	Payment           *Payment
	ExpiredCheckoutID string
}

// Payment is the part of a Payments.lk payment the API acts on.
type Payment struct {
	ID          string `json:"id"`
	Mode        string `json:"mode"`
	Status      string `json:"status"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
	Reference   string `json:"reference"`
	CheckoutID  string `json:"checkoutId"`
	// FeeCents is Payments.lk's fee, known once the payment settles.
	FeeCents *int64 `json:"feeCents"`
}

// VerifyEvent checks the Payments-Signature header against the raw body and reads the event.
// The header is t=<unix seconds>,v1=<hex HMAC-SHA256 of "t.body">, and may carry several v1
// values while a secret is being rolled.
func (p PaymentsLK) VerifyEvent(body []byte, header string) (Event, error) {
	if p.WebhookSecret == "" || header == "" || len(header) > 1000 {
		return Event{}, ErrBadSignature
	}
	var stamp int64 = -1
	var candidates [][]byte
	for part := range strings.SplitSeq(header, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch key {
		case "t":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				stamp = n
			}
		case "v1":
			if sig, err := hex.DecodeString(value); err == nil && len(sig) == sha256.Size {
				candidates = append(candidates, sig)
			}
		}
	}
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	if stamp < 0 || len(candidates) == 0 || now().Sub(time.Unix(stamp, 0)).Abs() > SignatureTolerance {
		return Event{}, ErrBadSignature
	}
	mac := hmac.New(sha256.New, []byte(p.WebhookSecret))
	mac.Write([]byte(strconv.FormatInt(stamp, 10) + "."))
	mac.Write(body)
	want := mac.Sum(nil)
	matched := false
	for _, c := range candidates {
		if hmac.Equal(want, c) {
			matched = true
		}
	}
	if !matched {
		return Event{}, ErrBadSignature
	}

	var raw struct {
		Object string          `json:"object"`
		ID     string          `json:"id"`
		Type   string          `json:"type"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.Object != "event" || raw.ID == "" || raw.Type == "" {
		return Event{}, errors.New("payments.lk: webhook body isn't an event")
	}
	ev := Event{ID: raw.ID, Type: raw.Type}
	switch raw.Type {
	case "payment.succeeded", "payment.failed":
		var pay Payment
		if err := json.Unmarshal(raw.Data, &pay); err != nil {
			return Event{}, fmt.Errorf("payments.lk: read payment: %w", err)
		}
		ev.Payment = &pay
	case "checkout.expired":
		var c struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw.Data, &c); err != nil {
			return Event{}, fmt.Errorf("payments.lk: read checkout: %w", err)
		}
		ev.ExpiredCheckoutID = c.ID
	}
	return ev, nil
}

func (p PaymentsLK) base() string {
	if p.BaseURL != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	return paymentsLKURL
}

func (p PaymentsLK) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
