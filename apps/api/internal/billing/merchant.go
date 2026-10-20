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
	"slices"
	"strings"
	"time"
)

// Merchant takes guest card payments on one Payments.lk account: an owner's own, or StayKey's
// collection account. Amounts are LKR cents.
type Merchant struct {
	// SecretKey is sk_test_... in the sandbox and sk_live_... live.
	SecretKey string

	BaseURL string       // default: Payments.lk's API
	Client  *http.Client // default: 30 second timeout, no redirects
}

// APIError is an answer from Payments.lk other than success. Code and Reason are its error
// code (VALIDATION_FAILED, CONFLICT) and the more precise reason (not_refundable).
type APIError struct {
	Status  int
	Code    string
	Reason  string
	TraceID string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("payments.lk answered %d %s %s (trace %s)", e.Status, e.Code, e.Reason, e.TraceID)
}

// Unauthorized reports whether Payments.lk refused the key itself.
func Unauthorized(err error) bool {
	var e *APIError
	return errors.As(err, &e) && (e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden)
}

// Refused reports whether Payments.lk turned the request down for good: a validation error, a
// conflict other than a request still running, or not found. Anything else may succeed later.
func Refused(err error) bool {
	var e *APIError
	if !errors.As(err, &e) {
		return false
	}
	switch e.Status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusRequestEntityTooLarge:
		return true
	case http.StatusConflict:
		return e.Reason != "idempotency_key_in_progress"
	}
	return false
}

// Mode is test for a sandbox key and live otherwise.
func (m Merchant) Mode() string {
	if strings.HasPrefix(m.SecretKey, "sk_test_") {
		return "test"
	}
	return "live"
}

// Check makes sure Payments.lk accepts the key by reading one event.
func (m Merchant) Check(ctx context.Context) error {
	return m.call(ctx, http.MethodGet, "/v1/events?limit=1", "", nil, nil)
}

// CardCheckout is what a guest pays through a hosted checkout.
type CardCheckout struct {
	// Reference is our checkout id. It also keys the request, so a retry never makes a second
	// checkout.
	Reference        string
	AmountCents      int64
	Description      string
	SuccessURL       string
	CancelURL        string
	ExpiresInMinutes int
}

// CheckoutState is a hosted checkout as Payments.lk sees it. Payment is set once one was taken.
type CheckoutState struct {
	ID        string
	URL       string
	Mode      string
	Status    string
	ExpiresAt time.Time
	Payment   *Payment
}

// CreateCheckout starts a hosted checkout for a guest.
func (m Merchant) CreateCheckout(ctx context.Context, r CardCheckout) (CheckoutState, error) {
	body := map[string]any{
		"amountCents":      r.AmountCents,
		"description":      r.Description,
		"reference":        r.Reference,
		"successUrl":       r.SuccessURL,
		"cancelUrl":        r.CancelURL,
		"expiresInMinutes": r.ExpiresInMinutes,
	}
	var c checkoutJSON
	if err := m.call(ctx, http.MethodPost, "/v1/checkouts", r.Reference, body, &c); err != nil {
		return CheckoutState{}, fmt.Errorf("payments.lk: create checkout: %w", err)
	}
	if c.ID == "" || !strings.HasPrefix(c.URL, "https://") {
		return CheckoutState{}, errors.New("payments.lk: checkout came back without an id or an https url")
	}
	return c.state(), nil
}

// GetCheckout reads a checkout and the payment taken through it, if any.
func (m Merchant) GetCheckout(ctx context.Context, id string) (CheckoutState, error) {
	var c checkoutJSON
	if err := m.call(ctx, http.MethodGet, "/v1/checkouts/"+url.PathEscape(id), "", nil, &c); err != nil {
		return CheckoutState{}, fmt.Errorf("payments.lk: get checkout: %w", err)
	}
	return c.state(), nil
}

// GetPayment reads a payment, with the fee once Payments.lk has worked it out.
func (m Merchant) GetPayment(ctx context.Context, id string) (Payment, error) {
	var p Payment
	if err := m.call(ctx, http.MethodGet, "/v1/payments/"+url.PathEscape(id), "", nil, &p); err != nil {
		return Payment{}, fmt.Errorf("payments.lk: get payment: %w", err)
	}
	return p, nil
}

// Refund is money sent back to a guest's card.
type Refund struct {
	ID             string `json:"id"`
	PaymentID      string `json:"paymentId"`
	Status         string `json:"status"`
	AmountCents    int64  `json:"amountCents"`
	FailureMessage string `json:"failureMessage"`
}

// Refund sends part of a payment back to the card. key is our refund's id, so sending it again
// never refunds twice.
func (m Merchant) Refund(ctx context.Context, paymentID string, amountCents int64, reason, key string) (Refund, error) {
	body := map[string]any{"paymentId": paymentID, "amountCents": amountCents}
	if reason != "" {
		body["reason"] = reason
	}
	var r Refund
	if err := m.call(ctx, http.MethodPost, "/v1/refunds", key, body, &r); err != nil {
		return Refund{}, fmt.Errorf("payments.lk: refund: %w", err)
	}
	if r.ID == "" {
		return Refund{}, errors.New("payments.lk: refund came back without an id")
	}
	return r, nil
}

// CardEventTypes are the events card payments act on.
var CardEventTypes = []string{"payment.succeeded", "checkout.expired", "refund.succeeded", "refund.failed"}

// CardEvent is one Payments.lk event about a card payment. Payment, CheckoutID or Refund is set
// by type.
type CardEvent struct {
	ID         string
	Type       string
	Mode       string
	Created    time.Time
	Payment    *Payment
	CheckoutID string
	Refund     *Refund
}

// maxEventPages caps how many pages of one event type a poll reads.
const maxEventPages = 20

// Events lists the card events created at or after since, oldest first.
func (m Merchant) Events(ctx context.Context, since time.Time) ([]CardEvent, error) {
	var all []CardEvent
	for _, typ := range CardEventTypes {
		cursor := ""
		for page := 0; ; page++ {
			if page == maxEventPages {
				return nil, fmt.Errorf("payments.lk: more than %d pages of %s events", maxEventPages, typ)
			}
			q := url.Values{"limit": {"100"}, "type": {typ}, "since": {since.UTC().Format(time.RFC3339)}}
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			var list struct {
				Data       []eventJSON `json:"data"`
				HasMore    bool        `json:"hasMore"`
				NextCursor *string     `json:"nextCursor"`
			}
			if err := m.call(ctx, http.MethodGet, "/v1/events?"+q.Encode(), "", nil, &list); err != nil {
				return nil, fmt.Errorf("payments.lk: list events: %w", err)
			}
			for _, raw := range list.Data {
				ev, err := raw.cardEvent()
				if err != nil {
					return nil, err
				}
				all = append(all, ev)
			}
			if !list.HasMore || list.NextCursor == nil || *list.NextCursor == "" {
				break
			}
			cursor = *list.NextCursor
		}
	}
	slices.SortStableFunc(all, func(a, b CardEvent) int { return a.Created.Compare(b.Created) })
	return all, nil
}

type checkoutJSON struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Mode      string    `json:"mode"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
	Payment   *Payment  `json:"payment"`
}

func (c checkoutJSON) state() CheckoutState {
	return CheckoutState{ID: c.ID, URL: c.URL, Mode: c.Mode, Status: c.Status, ExpiresAt: c.ExpiresAt, Payment: c.Payment}
}

type eventJSON struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Type    string    `json:"type"`
	Mode    string    `json:"mode"`
	Created time.Time `json:"created"`
	Data    struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

func (e eventJSON) cardEvent() (CardEvent, error) {
	if e.ID == "" || e.Type == "" {
		return CardEvent{}, errors.New("payments.lk: event without an id or a type")
	}
	ev := CardEvent{ID: e.ID, Type: e.Type, Mode: e.Mode, Created: e.Created}
	var err error
	switch e.Type {
	case "payment.succeeded":
		ev.Payment = &Payment{}
		err = json.Unmarshal(e.Data.Object, ev.Payment)
	case "checkout.expired":
		var c struct {
			ID string `json:"id"`
		}
		err = json.Unmarshal(e.Data.Object, &c)
		ev.CheckoutID = c.ID
	case "refund.succeeded", "refund.failed":
		ev.Refund = &Refund{}
		err = json.Unmarshal(e.Data.Object, ev.Refund)
	}
	if err != nil {
		return CardEvent{}, fmt.Errorf("payments.lk: read %s event %s: %w", e.Type, e.ID, err)
	}
	return ev, nil
}

// call sends one request with the key and decodes a successful answer into out. key, when
// set, goes as the Idempotency-Key.
func (m Merchant) call(ctx context.Context, method, path, key string, body, out any) error {
	return request(ctx, m.client(), m.base(), m.SecretKey, method, path, key, body, out)
}

func (m Merchant) base() string {
	return PaymentsLK{BaseURL: m.BaseURL}.base()
}

func (m Merchant) client() *http.Client {
	return PaymentsLK{Client: m.Client}.client()
}

// request is the plumbing both Payments.lk clients share.
func request(ctx context.Context, client *http.Client, base, secretKey, method, path, key string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	reply, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read answer: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		e := &APIError{Status: res.StatusCode}
		var fields struct {
			Code    string `json:"code"`
			Reason  string `json:"reason"`
			TraceID string `json:"traceId"`
		}
		if json.Unmarshal(reply, &fields) == nil {
			e.Code, e.Reason, e.TraceID = fields.Code, fields.Reason, fields.TraceID
		}
		return e
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(reply, out); err != nil {
		return fmt.Errorf("decode answer: %w", err)
	}
	return nil
}
