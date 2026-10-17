package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/billing"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

const (
	rcAuth      = "Bearer rc-webhook-secret"
	lkSecret    = "whsec_test"
	forwardKeyT = "forward-secret-forward-secret-forward"
)

// providers stands in for RevenueCat and Payments.lk at the HTTP boundary.
func providers(t *testing.T) func(*Options) {
	t.Helper()
	rc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		end := time.Now().UTC().AddDate(0, 1, 0).Format(time.RFC3339)
		fmt.Fprintf(w, `{"subscriber":{"subscriptions":{"staykey_guesthouse_monthly":{"store":"app_store","expires_date":%q,"store_transaction_id":"tx1"}}}}`, end)
	}))
	t.Cleanup(rc.Close)
	lk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"object":"checkout","id":"chk_1","url":"https://pay.payments.lk/c/chk_1"}`)
	}))
	t.Cleanup(lk.Close)
	return func(o *Options) {
		revenueCat := billing.RevenueCat{SecretKey: "sk_rc", BaseURL: rc.URL}
		o.Subscribers, o.RevenueCatWebhookAuth = revenueCat, rcAuth
		o.WebPayments = billing.PaymentsLK{SecretKey: "sk_test_1", WebhookSecret: lkSecret, BaseURL: lk.URL}
		o.BillingURL = "https://staykey.direct/billing"
		o.ForwardSecret = forwardKeyT
	}
}

func (e *env) member(account string, owner *session, phone string, role domain.Role) *session {
	e.t.Helper()
	s, _ := e.signIn(phone)
	ownerTenant := tenant.Tenant{AccountID: uuid.MustParse(account), UserID: owner.userID, Role: domain.RoleOwner}
	if _, err := e.store.AddMember(context.Background(), ownerTenant, s.userID, role, nil); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *env) subscription(s *session, account string) oapi.Subscription {
	e.t.Helper()
	rec := e.do(call{method: http.MethodGet, path: "/v1/subscription", as: s, account: account})
	e.expect(rec, http.StatusOK)
	return decode[oapi.Subscription](e.t, rec)
}

func (e *env) billingLines(s *session, account string) []string {
	e.t.Helper()
	rec := e.do(call{method: http.MethodGet, path: "/v1/activity", as: s, account: account})
	e.expect(rec, http.StatusOK)
	var out []string
	for _, a := range decode[oapi.ActivityPage](e.t, rec).Items {
		if a.Kind == oapi.ActivityKindBilling {
			out = append(out, a.Title)
		}
	}
	return out
}

func rcHook(eventID, account string) map[string]any {
	return map[string]any{"api_version": "1.0", "event": map[string]any{"id": eventID, "type": "INITIAL_PURCHASE", "app_user_id": account}}
}

func lkSign(body string) string {
	stamp := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(lkSecret))
	fmt.Fprintf(mac, "%d.%s", stamp, body)
	return fmt.Sprintf("t=%d,v1=%s", stamp, hex.EncodeToString(mac.Sum(nil)))
}

func TestACaretakerReadsThePlanButCantBuy(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94775000001", "Plan account")
	caretaker := e.member(account, owner, "+94775000002", domain.RoleCaretaker)
	if got := e.subscription(caretaker, account); got.Status != oapi.AccountStatusTrial || got.CanPurchase {
		t.Errorf("caretaker sees %s, canPurchase %v", got.Status, got.CanPurchase)
	}
}

func TestManagersCantChangeThePlan(t *testing.T) {
	t.Parallel()
	e := newEnvWithOptions(t, providers(t))
	owner, account := e.owner("+94775000003", "Plan account")
	manager := e.member(account, owner, "+94775000004", domain.RoleManager)
	for _, c := range []call{
		{method: http.MethodPost, path: "/v1/subscription/refresh"},
		{method: http.MethodPost, path: "/v1/subscription/checkout", body: map[string]any{"plan": "villa", "period": "monthly"}},
	} {
		c.as, c.account = manager, account
		e.expect(e.do(c), http.StatusForbidden)
	}
}

func TestCheckoutWithoutPaymentsLKIsUnavailable(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94775000005", "Plan account")
	rec := e.do(call{method: http.MethodPost, path: "/v1/subscription/checkout", as: owner, account: account,
		body: map[string]any{"plan": "villa", "period": "monthly"}})
	e.expect(rec, http.StatusServiceUnavailable)
}

func TestARevenueCatWebhookWithTheWrongSecretIsRefused(t *testing.T) {
	t.Parallel()
	e := newEnvWithOptions(t, providers(t))
	_, account := e.owner("+94775000006", "Plan account")
	rec := e.do(call{method: http.MethodPost, path: "/webhooks/revenuecat", body: rcHook("evt-1", account),
		headers: map[string]string{"Authorization": "Bearer guess"}})
	e.expect(rec, http.StatusUnauthorized)
}

func TestARevenueCatEventDeliveredTwiceCountsOnce(t *testing.T) {
	t.Parallel()
	e := newEnvWithOptions(t, providers(t))
	owner, account := e.owner("+94775000007", "Plan account")
	for range 2 {
		rec := e.do(call{method: http.MethodPost, path: "/webhooks/revenuecat", body: rcHook("evt-1", account),
			headers: map[string]string{"Authorization": rcAuth}})
		e.expect(rec, http.StatusOK)
	}
	if got := strings.Join(e.billingLines(owner, account), " | "); got != "Guesthouse plan active" {
		t.Errorf("billing activity = %q", got)
	}
}

func TestAWebPaymentMakesTheAccountActive(t *testing.T) {
	t.Parallel()
	e := newEnvWithOptions(t, providers(t))
	owner, account := e.owner("+94775000008", "Plan account")
	rec := e.do(call{method: http.MethodPost, path: "/v1/subscription/checkout", as: owner, account: account,
		body: map[string]any{"plan": "villa", "period": "yearly"}})
	e.expect(rec, http.StatusCreated)
	checkout := decode[oapi.WebCheckout](t, rec)

	var reference string
	if err := e.store.Pool().QueryRow(context.Background(), "SELECT id::text FROM web_checkouts WHERE provider_id = 'chk_1'").Scan(&reference); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"object":"event","id":"evt_1","type":"payment.succeeded","data":{"object":"payment","id":"pay_1","mode":"test","status":"succeeded","amountCents":%d,"currency":"LKR","reference":%q,"checkoutId":"chk_1"}}`,
		checkout.Amount, reference)
	rec = e.do(call{method: http.MethodPost, path: "/webhooks/payments-lk", raw: []byte(body),
		headers: map[string]string{"Payments-Signature": lkSign(body)}})
	e.expect(rec, http.StatusOK)

	got := e.subscription(owner, account)
	if got.Status != oapi.AccountStatusActive || got.Source == nil || *got.Source != oapi.SubscriptionSourceWeb {
		t.Errorf("subscription = %s from %v", got.Status, got.Source)
	}
}

func TestAPaymentsLKEventThatDoesntVerifyChangesNothing(t *testing.T) {
	t.Parallel()
	e := newEnvWithOptions(t, providers(t))
	owner, account := e.owner("+94775000009", "Plan account")
	body := `{"object":"event","id":"evt_1","type":"payment.succeeded","data":{}}`
	rec := e.do(call{method: http.MethodPost, path: "/webhooks/payments-lk", raw: []byte(body),
		headers: map[string]string{"Payments-Signature": "t=1,v1=" + strings.Repeat("0", 64)}})
	e.expect(rec, http.StatusBadRequest)
	if got := e.subscription(owner, account); got.Status != oapi.AccountStatusTrial {
		t.Errorf("status = %s, want trial", got.Status)
	}
}

// lapse moves the account's free period far enough back that grace has ended.
func (e *env) lapse(account string) {
	e.t.Helper()
	_, err := e.store.Pool().Exec(context.Background(),
		"UPDATE accounts SET trial_started_at = now() - interval '70 days' WHERE id = $1", account)
	if err != nil {
		e.t.Fatal(err)
	}
}

func TestALapsedBookingPageOffersWhatsApp(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000010")
	e.lapse(v.account)
	rec := e.do(call{method: http.MethodGet, path: "/v1/public/properties/" + v.p.Slug})
	e.expect(rec, http.StatusOK)
	got := decode[oapi.PublicProperty](t, rec)
	if got.TakingBookings || got.PausedReason == nil || *got.PausedReason != oapi.PublicPropertyPausedReasonLapsed || deref(got.Whatsapp) != "+94775000010" {
		t.Errorf("taking %v, reason %v, whatsapp %q", got.TakingBookings, got.PausedReason, deref(got.Whatsapp))
	}
}

func TestALapsedBookingPageTakesNoHolds(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000011")
	e.lapse(v.account)
	rec := e.do(call{method: http.MethodPost, path: "/v1/public/properties/" + v.p.Slug + "/holds", body: map[string]any{
		"unitId": v.p.Units[0].Id, "checkIn": future(30), "checkOut": future(32),
	}})
	e.expect(rec, http.StatusConflict)
}

func TestAnOpenBookingPageGivesNoNumber(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000012")
	rec := e.do(call{method: http.MethodGet, path: "/v1/public/properties/" + v.p.Slug})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.PublicProperty](t, rec); got.Whatsapp != nil || got.PausedReason != nil {
		t.Errorf("open page has whatsapp %v, reason %v", got.Whatsapp, got.PausedReason)
	}
}

// bookFrom posts a guest booking from one address, as the booking page would forward it.
func (e *env) bookFrom(v villa, n int, headers map[string]string) int {
	e.t.Helper()
	h := map[string]string{"Idempotency-Key": fmt.Sprintf("form-%d", n)}
	for k, val := range headers {
		h[k] = val
	}
	rec := e.do(call{method: http.MethodPost, path: "/v1/public/properties/" + v.p.Slug + "/bookings", headers: h, body: map[string]any{
		"unitId": v.p.Units[0].Id, "checkIn": future(30 + 3*n), "checkOut": future(31 + 3*n), "adults": 1,
		"guest": map[string]any{"name": "Sam Lee", "phone": "+447700900000"}, "paymentMethod": "property", "source": "page",
	}})
	return rec.Code
}

func TestTheSixthGuestBookingFromOneAddressIsLimited(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000013")
	for n := range 5 {
		e.bookFrom(v, n, nil)
	}
	if got := e.bookFrom(v, 5, nil); got != http.StatusTooManyRequests {
		t.Errorf("sixth booking = %d, want 429", got)
	}
}

func TestAForwardedAddressWithoutTheSecretIsIgnored(t *testing.T) {
	t.Parallel()
	e := newEnvWithOptions(t, providers(t))
	v := e.villa("+94775000014")
	for n := range 5 {
		e.bookFrom(v, n, map[string]string{headerForwardKey: "guess", headerGuestIP: fmt.Sprintf("203.0.113.%d", n)})
	}
	got := e.bookFrom(v, 5, map[string]string{headerForwardKey: "guess", headerGuestIP: "203.0.113.99"})
	if got != http.StatusTooManyRequests {
		t.Errorf("sixth booking = %d, want 429", got)
	}
}

func TestAForwardedAddressWithTheSecretIsCountedOnItsOwn(t *testing.T) {
	t.Parallel()
	e := newEnvWithOptions(t, providers(t))
	v := e.villa("+94775000015")
	for n := range 5 {
		e.bookFrom(v, n, map[string]string{headerForwardKey: forwardKeyT, headerGuestIP: "203.0.113.1"})
	}
	got := e.bookFrom(v, 5, map[string]string{headerForwardKey: forwardKeyT, headerGuestIP: "203.0.113.2"})
	if got != http.StatusCreated {
		t.Errorf("booking from a second guest = %d, want 201", got)
	}
}

func TestPlansListWebPricesInRupees(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rec := e.do(call{method: http.MethodGet, path: "/v1/public/plans"})
	e.expect(rec, http.StatusOK)
	got := decode[oapi.PlanList](t, rec)
	if len(got.Items) != 3 || got.Items[0].Id != oapi.Villa || got.Items[0].Currency != oapi.CurrencyLKR {
		t.Errorf("plans = %+v", got.Items)
	}
}
