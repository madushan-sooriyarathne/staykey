package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/store/storetest"
	"staykey.direct/api/internal/tenant"
)

// firstPayoutRun is 08:00 in Colombo on the first payout Monday.
var firstPayoutRun = time.Date(2026, 11, 2, 2, 30, 0, 0, time.UTC)

func payoutBody() map[string]any {
	return map[string]any{
		"legalName": "Alice Fernando", "nic": "199012345678", "bankName": "Sampath Bank", "branch": "Colombo 03",
		"accountName": "A Fernando", "accountNumber": "1029 3847 5612", "acceptTerms": domain.PayoutTermsVersion,
	}
}

// payout gives the account payout details and a prepared payout of Rs 2,000.00 through an
// adjustment, and returns the payout's id.
func (e *env) payout(t tenant.Tenant) uuid.UUID {
	e.t.Helper()
	ctx := context.Background()
	before := firstPayoutRun.AddDate(0, 0, -10)
	in := domain.PayoutProfileInput{
		LegalName: "Alice Fernando", NIC: "199012345678", BankName: "Sampath Bank", Branch: "Colombo 03",
		AccountName: "A Fernando", AccountNumber: "102938475612", AcceptTerms: domain.PayoutTermsVersion,
	}
	if _, err := e.store.SetPayoutProfile(ctx, t, in, before); err != nil {
		e.t.Fatal(err)
	}
	if err := e.store.AdjustPayouts(ctx, t.AccountID, 200_000, "Opening balance", before); err != nil {
		e.t.Fatal(err)
	}
	if _, _, err := e.store.PreparePayouts(ctx, firstPayoutRun); err != nil {
		e.t.Fatal(err)
	}
	p, err := e.store.Payouts(ctx, t.AccountID, firstPayoutRun)
	if err != nil || len(p.History) != 1 {
		e.t.Fatalf("payouts %+v, %v", p.History, err)
	}
	return p.History[0].ID
}

func TestPayoutDetailsComeBackByTheirLastFour(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94773100001", "Alice's villas")
	rec := e.do(call{method: http.MethodPut, path: "/v1/card-payments/payout-profile", as: owner, account: account, body: payoutBody()})
	e.expect(rec, http.StatusOK)
	if body := rec.Body.String(); strings.Contains(body, "102938475612") || strings.Contains(body, "199012345678") {
		t.Errorf("payout details came back in full: %s", body)
	}
}

func TestPayoutDetailsShowTheAccountsLastFour(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94773100002", "Alice's villas")
	e.expect(e.do(call{method: http.MethodPut, path: "/v1/card-payments/payout-profile", as: owner, account: account, body: payoutBody()}), http.StatusOK)
	rec := e.do(call{method: http.MethodGet, path: "/v1/card-payments/payout-profile", as: owner, account: account})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.PayoutProfile](t, rec).AccountLast4; got != "5612" {
		t.Errorf("accountLast4 = %q, want 5612", got)
	}
}

func TestAWrongNICIsRefused(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94773100003", "Alice's villas")
	body := payoutBody()
	body["nic"] = "12345"
	rec := e.do(call{method: http.MethodPut, path: "/v1/card-payments/payout-profile", as: owner, account: account, body: body})
	e.expect(rec, http.StatusBadRequest)
	if got := deref(decode[oapi.Error](t, rec).Field); got != "nic" {
		t.Errorf("field = %q, want nic", got)
	}
}

func TestManagersCantSeePayouts(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94773100004", "Alice's villas")
	manager, _ := e.signIn("+94773100005")
	tn := tenant.Tenant{AccountID: uuid.MustParse(account), UserID: owner.userID, Role: domain.RoleOwner}
	if _, err := e.store.AddMember(context.Background(), tn, manager.userID, domain.RoleManager, nil); err != nil {
		t.Fatal(err)
	}
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/payouts", as: manager, account: account}), http.StatusForbidden)
}

func TestPayoutsListAPastPayout(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94773100006", "Alice's villas")
	e.payout(tenant.Tenant{AccountID: uuid.MustParse(account), UserID: owner.userID, Role: domain.RoleOwner})
	rec := e.do(call{method: http.MethodGet, path: "/v1/payouts", as: owner, account: account})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.Payouts](t, rec).Payouts; len(got) != 1 || got[0].Amount != 200_000 {
		t.Errorf("payouts = %+v, want one of Rs 2,000.00", got)
	}
}

func TestAPayoutStatementListsItsEntries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94773100007", "Alice's villas")
	id := e.payout(tenant.Tenant{AccountID: uuid.MustParse(account), UserID: owner.userID, Role: domain.RoleOwner})
	rec := e.do(call{method: http.MethodGet, path: "/v1/payouts/" + id.String(), as: owner, account: account})
	e.expect(rec, http.StatusOK)
	if lines := decode[oapi.PayoutStatement](t, rec).Lines; len(lines) != 1 || lines[0].Kind != oapi.PayoutEntryKindAdjustment {
		t.Errorf("statement lines = %+v, want the adjustment", lines)
	}
}

func TestThePayoutsModeNeedsPayoutDetails(t *testing.T) {
	t.Parallel()
	st := storetest.New(t, store.WithCards(store.CardConfig{CollectMode: "test", PayoutsEnabled: true}))
	e := newEnvWithStore(t, st, st)
	owner, account := e.owner("+94773100008", "Alice's villas")
	e.expect(e.do(call{method: http.MethodPut, path: "/v1/card-payments/mode", as: owner, account: account,
		body: map[string]any{"mode": "payouts"}}), http.StatusConflict)
}

func TestThePayoutsModeCanBeChosenWithPayoutDetails(t *testing.T) {
	t.Parallel()
	st := storetest.New(t, store.WithCards(store.CardConfig{CollectMode: "test", PayoutsEnabled: true}))
	e := newEnvWithStore(t, st, st)
	owner, account := e.owner("+94773100009", "Alice's villas")
	e.expect(e.do(call{method: http.MethodPut, path: "/v1/card-payments/payout-profile", as: owner, account: account, body: payoutBody()}), http.StatusOK)
	rec := e.do(call{method: http.MethodPut, path: "/v1/card-payments/mode", as: owner, account: account, body: map[string]any{"mode": "payouts"}})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.CardPayments](t, rec); got.Mode != oapi.CardModePayouts || got.PayoutProfile == nil {
		t.Errorf("card payments = mode %s, profile %v", got.Mode, got.PayoutProfile)
	}
}
