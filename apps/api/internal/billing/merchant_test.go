package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

const ownerKey = "sk_test_owner_fixture-01"

func TestCreateCardCheckoutSendsTheAmountAndKeysTheRequest(t *testing.T) {
	var got map[string]any
	var key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/checkouts" || r.Header.Get("Authorization") != "Bearer "+ownerKey {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		key = r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"object":"checkout","id":"chk_1","mode":"test","status":"open","url":"https://pay.payments.lk/c/chk_1","expiresAt":"2026-10-21T10:00:00Z"}`))
	}))
	defer srv.Close()

	c, err := Merchant{SecretKey: ownerKey, BaseURL: srv.URL}.CreateCheckout(context.Background(), CardCheckout{
		Reference: "0192a6b8-6c3e-7d5e-8a7f-0f2a4b6c8d10", AmountCents: 1_674_000, Description: "Kingfisher Villa, KV-2041",
		SuccessURL: "https://kingfisher.staykey.direct/stay/1?paid=1", CancelURL: "https://kingfisher.staykey.direct/stay/1", ExpiresInMinutes: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"amountCents": float64(1_674_000), "description": "Kingfisher Villa, KV-2041", "reference": "0192a6b8-6c3e-7d5e-8a7f-0f2a4b6c8d10",
		"successUrl": "https://kingfisher.staykey.direct/stay/1?paid=1", "cancelUrl": "https://kingfisher.staykey.direct/stay/1", "expiresInMinutes": float64(60),
	}
	if c.ID != "chk_1" || key != "0192a6b8-6c3e-7d5e-8a7f-0f2a4b6c8d10" || !equalMaps(got, want) {
		t.Errorf("checkout %+v, key %q, body %v", c, key, got)
	}
}

func TestGetCheckoutReadsItsPayment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkouts/chk_1" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"object":"checkout","id":"chk_1","mode":"test","status":"completed","url":"https://pay.payments.lk/c/chk_1",
			"payment":{"object":"payment","id":"pay_1","mode":"test","status":"succeeded","amountCents":1674000,"currency":"LKR","reference":"ref","checkoutId":"chk_1","feeCents":null}}`))
	}))
	defer srv.Close()

	c, err := Merchant{SecretKey: ownerKey, BaseURL: srv.URL}.GetCheckout(context.Background(), "chk_1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Payment == nil || c.Payment.ID != "pay_1" || c.Payment.AmountCents != 1_674_000 || c.Payment.Status != "succeeded" {
		t.Errorf("GetCheckout payment = %+v", c.Payment)
	}
}

func TestRefundKeysTheRequestWithOurRefund(t *testing.T) {
	var got map[string]any
	var key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"object":"refund","id":"re_1","paymentId":"pay_1","status":"pending","amountCents":500000,"currency":"LKR"}`))
	}))
	defer srv.Close()

	r, err := Merchant{SecretKey: ownerKey, BaseURL: srv.URL}.Refund(context.Background(), "pay_1", 500_000, "Stay cancelled", "refund-row-1")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"paymentId": "pay_1", "amountCents": float64(500_000), "reason": "Stay cancelled"}
	if r.ID != "re_1" || key != "refund-row-1" || !equalMaps(got, want) {
		t.Errorf("refund %+v, key %q, body %v", r, key, got)
	}
}

func TestMerchantErrorsSayWhetherToRetry(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		unauthorized bool
		refused      bool
	}{
		{"a revoked key", http.StatusUnauthorized, `{"code":"UNAUTHENTICATED","reason":"unknown_api_key"}`, true, false},
		{"not refundable", http.StatusConflict, `{"code":"CONFLICT","reason":"not_refundable"}`, false, true},
		{"still running", http.StatusConflict, `{"code":"CONFLICT","reason":"idempotency_key_in_progress"}`, false, false},
		{"a server error", http.StatusInternalServerError, `{"code":"INTERNAL"}`, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			_, err := Merchant{SecretKey: ownerKey, BaseURL: srv.URL}.Refund(context.Background(), "pay_1", 100, "", "refund-row-1")
			if Unauthorized(err) != tt.unauthorized || Refused(err) != tt.refused {
				t.Errorf("error %v: Unauthorized = %v, Refused = %v", err, Unauthorized(err), Refused(err))
			}
		})
	}
}

func TestEventsReadEveryPageOfEachTypeOldestFirst(t *testing.T) {
	pages := map[string]string{
		"payment.succeeded|": `{"object":"list","hasMore":true,"nextCursor":"c2","data":[
			{"id":"evt_3","object":"event","type":"payment.succeeded","mode":"test","created":"2026-10-21T09:03:00Z","data":{"object":{"id":"pay_3","checkoutId":"chk_3","amountCents":1000}}}]}`,
		"payment.succeeded|c2": `{"object":"list","hasMore":false,"nextCursor":null,"data":[
			{"id":"evt_1","object":"event","type":"payment.succeeded","mode":"test","created":"2026-10-21T09:01:00Z","data":{"object":{"id":"pay_1","checkoutId":"chk_1","amountCents":1000}}}]}`,
		"refund.failed|": `{"object":"list","hasMore":false,"data":[
			{"id":"evt_2","object":"event","type":"refund.failed","mode":"test","created":"2026-10-21T09:02:00Z","data":{"object":{"id":"re_1","paymentId":"pay_0","status":"pending","failureMessage":"Card closed"}}}]}`,
	}
	var since string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		since = q.Get("since")
		page, ok := pages[q.Get("type")+"|"+q.Get("cursor")]
		if !ok {
			page = `{"object":"list","hasMore":false,"data":[]}`
		}
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	events, err := Merchant{SecretKey: ownerKey, BaseURL: srv.URL}.Events(context.Background(), time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, ev := range events {
		ids = append(ids, ev.ID)
	}
	if !slices.Equal(ids, []string{"evt_1", "evt_2", "evt_3"}) || since != "2026-10-21T09:00:00Z" {
		t.Errorf("Events = %v since %q", ids, since)
	}
}

func TestEventsReadTheirObjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "refund.failed" {
			_, _ = w.Write([]byte(`{"object":"list","hasMore":false,"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"object":"list","hasMore":false,"data":[
			{"id":"evt_2","object":"event","type":"refund.failed","mode":"test","created":"2026-10-21T09:02:00Z","data":{"object":{"id":"re_1","paymentId":"pay_0","status":"pending","failureMessage":"Card closed"}}}]}`))
	}))
	defer srv.Close()

	events, err := Merchant{SecretKey: ownerKey, BaseURL: srv.URL}.Events(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Refund == nil || events[0].Refund.ID != "re_1" || events[0].Refund.FailureMessage != "Card closed" {
		t.Errorf("Events = %+v", events)
	}
}

func equalMaps(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
