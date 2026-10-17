package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"staykey.direct/api/internal/domain"
)

const accountID = "0192a6b8-6c3e-7d5e-8a7f-0f2a4b6c8d10"

func TestSubscriberReadsStorePlans(t *testing.T) {
	fixture, err := os.ReadFile("testdata/revenuecat_subscriber.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/subscribers/"+accountID || r.Header.Get("Authorization") != "Bearer sk_rc" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	subs, err := RevenueCat{SecretKey: "sk_rc", BaseURL: srv.URL}.Subscriber(context.Background(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(subs, func(a, b domain.Subscription) int { return strings.Compare(a.ProductID, b.ProductID) })
	want := []domain.Subscription{
		{Source: domain.SourceAppStore, Plan: domain.PlanGuesthouse, Period: domain.PeriodMonthly, Status: domain.SubActive,
			WillRenew: true, PeriodEnd: time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC),
			ProductID: "staykey_guesthouse_monthly", ExternalID: "2000000712345678"},
		{Source: domain.SourcePlayStore, Plan: domain.PlanVilla, Period: domain.PeriodMonthly, Status: domain.SubCancelled,
			PeriodEnd: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
			ProductID: "staykey_villa:monthly", ExternalID: "GPA.3300-1111-2222-33333"},
	}
	if !slices.Equal(subs, want) {
		t.Errorf("Subscriber =\n%+v\nwant\n%+v", subs, want)
	}
}

func TestSubscriberFailsOnAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"code":7225}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := (RevenueCat{SecretKey: "sk_rc", BaseURL: srv.URL}).Subscriber(context.Background(), accountID); err == nil {
		t.Error("expected an error")
	}
}

func TestRecordExternalPostsThePeriod(t *testing.T) {
	var got map[string]map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/receipts/external" || r.Header.Get("Authorization") != "Bearer sk_ext" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	err := RevenueCat{ExternalKey: "sk_ext", BaseURL: srv.URL}.RecordExternal(context.Background(), ExternalPurchase{
		AccountID: accountID, PaymentID: "pay_1", ProductID: "staykey_villa_yearly",
		PeriodStart: time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2027, 10, 17, 10, 0, 0, 0, time.UTC),
		Amount: 29_000_00, Currency: "LKR", Sandbox: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	summary := fmt.Sprintf("%v %v %v %v %v", got["purchase"]["customer_id"], got["purchase"]["current_period_ends_at"], got["purchase"]["environment"],
		got["payment"]["payment_identifier"], got["payment"]["amount_in_local_currency"])
	if summary != accountID+" 2027-10-17T10:00:00 sandbox pay_1 map[currency:LKR gross:29000]" {
		t.Errorf("posted %s", summary)
	}
}

func TestCreateCheckout(t *testing.T) {
	var idempotency string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkouts" || r.Header.Get("Authorization") != "Bearer sk_test_1" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		idempotency = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"object":"checkout","id":"chk_1","mode":"test","status":"open","url":"https://pay.payments.lk/c/chk_1","expiresAt":"2026-10-17T11:00:00Z","payment":{}}`)
	}))
	defer srv.Close()

	got, err := PaymentsLK{SecretKey: "sk_test_1", BaseURL: srv.URL}.CreateCheckout(context.Background(), CheckoutRequest{
		Reference: "ref-1", AmountCents: 5_900_00, Description: "StayKey Guesthouse, 1 month",
		SuccessURL: "https://staykey.direct/billing/done", CancelURL: "https://staykey.direct/billing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != (Checkout{ID: "chk_1", URL: "https://pay.payments.lk/c/chk_1"}) || idempotency != "ref-1" || body["amountCents"] != float64(5_900_00) || body["reference"] != "ref-1" {
		t.Errorf("checkout %+v, key %q, body %v", got, idempotency, body)
	}
}

func TestCreateCheckoutReportsARefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"code":"VALIDATION","message":"The smallest payment is Rs. 10.","traceId":"tr_1"}`)
	}))
	defer srv.Close()
	_, err := PaymentsLK{SecretKey: "sk_test_1", BaseURL: srv.URL}.CreateCheckout(context.Background(), CheckoutRequest{Reference: "r", AmountCents: 1})
	if err == nil || !strings.Contains(err.Error(), "422 VALIDATION") {
		t.Errorf("err = %v", err)
	}
}

func sign(secret string, stamp int64, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", stamp, body)
	return fmt.Sprintf("t=%d,v1=%s", stamp, hex.EncodeToString(mac.Sum(nil)))
}

const succeeded = `{"object":"event","id":"evt_1","type":"payment.succeeded","createdAt":"2026-10-17T10:00:00Z","data":{"object":"payment","id":"pay_1","mode":"test","status":"succeeded","amountCents":590000,"currency":"LKR","reference":"ref-1","checkoutId":"chk_1"}}`

func TestVerifyEventReadsAPayment(t *testing.T) {
	now := time.Unix(1792231200, 0)
	p := PaymentsLK{WebhookSecret: "whsec_1", Now: func() time.Time { return now }}
	ev, err := p.VerifyEvent([]byte(succeeded), sign("whsec_1", now.Unix(), succeeded))
	if err != nil {
		t.Fatal(err)
	}
	want := Payment{ID: "pay_1", Mode: "test", Status: "succeeded", AmountCents: 590000, Currency: "LKR", Reference: "ref-1", CheckoutID: "chk_1"}
	if ev.ID != "evt_1" || ev.Type != "payment.succeeded" || ev.Payment == nil || *ev.Payment != want {
		t.Errorf("event %+v", ev)
	}
}

func TestVerifyEventReadsAnExpiredCheckout(t *testing.T) {
	now := time.Unix(1792231200, 0)
	body := `{"object":"event","id":"evt_2","type":"checkout.expired","data":{"object":"checkout","id":"chk_9","paymentId":"pay_9"}}`
	ev, err := PaymentsLK{WebhookSecret: "whsec_1", Now: func() time.Time { return now }}.VerifyEvent([]byte(body), sign("whsec_1", now.Unix(), body))
	if err != nil || ev.ExpiredCheckoutID != "chk_9" {
		t.Errorf("event %+v, err %v", ev, err)
	}
}

func TestVerifyEventRefuses(t *testing.T) {
	now := time.Unix(1792231200, 0)
	tests := []struct {
		name, body, header string
	}{
		{"another secret", succeeded, sign("whsec_other", now.Unix(), succeeded)},
		{"an altered body", strings.Replace(succeeded, "590000", "1", 1), sign("whsec_1", now.Unix(), succeeded)},
		{"an old timestamp", succeeded, sign("whsec_1", now.Add(-10*time.Minute).Unix(), succeeded)},
		{"no header", succeeded, ""},
		{"a malformed header", succeeded, "v1=abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PaymentsLK{WebhookSecret: "whsec_1", Now: func() time.Time { return now }}.VerifyEvent([]byte(tt.body), tt.header)
			if !errors.Is(err, ErrBadSignature) {
				t.Errorf("err = %v", err)
			}
		})
	}
}
