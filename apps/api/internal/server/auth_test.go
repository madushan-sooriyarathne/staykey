package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/oapi"
)

func TestSignInFlow(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	const phone = "+94 77 123 4567"

	code := e.requestCode(phone)

	// A wrong code counts as an attempt and says how many are left.
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	rec := e.do(call{method: http.MethodPost, path: "/v1/auth/verify", body: oapi.VerifyRequest{Phone: phone, Code: wrong}})
	e.expect(rec, http.StatusBadRequest)
	if body := decode[oapi.Error](t, rec); body.Code != "wrong_code" || !strings.Contains(body.Message, "4 tries left") {
		t.Fatalf("wrong code response = %+v", body)
	}

	device := "Test iPhone"
	rec = e.do(call{method: http.MethodPost, path: "/v1/auth/verify", body: oapi.VerifyRequest{Phone: phone, Code: code, DeviceName: &device}})
	e.expect(rec, http.StatusOK)
	in := decode[oapi.SignIn](t, rec)
	if !in.NewUser || in.User.Phone != "+94771234567" || len(in.Accounts) != 0 {
		t.Fatalf("first sign-in = %+v", in)
	}
	if got := in.Tokens.AccessTokenExpiresAt.Sub(e.clock.Now()); got != 15*time.Minute {
		t.Errorf("access token lasts %s, want 15m", got)
	}
	s := &session{access: in.Tokens.AccessToken, refresh: in.Tokens.RefreshToken}

	// A used code cannot be used again.
	rec = e.do(call{method: http.MethodPost, path: "/v1/auth/verify", body: oapi.VerifyRequest{Phone: phone, Code: code}})
	e.expect(rec, http.StatusBadRequest)
	if body := decode[oapi.Error](t, rec); body.Code != "code_expired" {
		t.Fatalf("reused code = %+v, want code_expired", body)
	}

	// Name the user, then create their account.
	name := "Nimal Perera"
	rec = e.do(call{method: http.MethodPatch, path: "/v1/me", body: oapi.UserPatch{Name: &name}, as: s})
	e.expect(rec, http.StatusOK)
	rec = e.do(call{method: http.MethodPost, path: "/v1/accounts", body: oapi.NewAccount{Name: "Kingfisher Villa"}, as: s})
	e.expect(rec, http.StatusCreated)
	account := decode[oapi.Account](t, rec)
	if account.Role != oapi.Owner || account.Status != oapi.Trial {
		t.Errorf("new account = %+v", account)
	}

	rec = e.do(call{method: http.MethodGet, path: "/v1/me", as: s})
	e.expect(rec, http.StatusOK)
	me := decode[oapi.Me](t, rec)
	if me.User.Name != name || len(me.Accounts) != 1 || me.Accounts[0].Id != account.Id {
		t.Fatalf("me = %+v", me)
	}

	// Signing in again on another device finds the same user and account.
	_, again := e.signInAfterCooldown(phone)
	if again.NewUser || again.User.Id != in.User.Id || len(again.Accounts) != 1 {
		t.Fatalf("second sign-in = %+v", again)
	}

	// Access tokens expire after 15 minutes; the refresh token gets new ones.
	e.clock.Advance(16 * time.Minute)
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/me", as: s}), http.StatusUnauthorized)
	rec = e.refresh(s.refresh)
	e.expect(rec, http.StatusOK)
	tokens := decode[oapi.AuthTokens](t, rec)
	s.access, s.refresh = tokens.AccessToken, tokens.RefreshToken
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/me", as: s}), http.StatusOK)

	// Logging out ends the session at once, for the access token too.
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/auth/logout", body: oapi.RefreshRequest{RefreshToken: s.refresh}}), http.StatusNoContent)
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/me", as: s}), http.StatusUnauthorized)
	e.expect(e.refresh(s.refresh), http.StatusUnauthorized)
}

// signInAfterCooldown waits out the resend limit, then signs in.
func (e *env) signInAfterCooldown(phone string) (*session, oapi.SignIn) {
	e.clock.Advance(31 * time.Second)
	return e.signIn(phone)
}

func TestProtectedRoutesNeedAToken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	for _, c := range []call{
		{method: http.MethodGet, path: "/v1/me"},
		{method: http.MethodGet, path: "/v1/accounts"},
		{method: http.MethodPost, path: "/v1/accounts", raw: []byte("not json")},
		{method: http.MethodGet, path: "/v1/properties", account: "0198c3a2-0000-7000-8000-000000000000"},
		{method: http.MethodGet, path: "/v1/me", as: &session{access: "not-a-jwt"}},
	} {
		rec := e.do(c)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status %d, want 401", c.method, c.path, rec.Code)
		}
	}
}

func TestOwnerRoutesNeedAnAccountHeader(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.owner("+94772000001", "Villa")
	rec := e.do(call{method: http.MethodGet, path: "/v1/properties", as: s})
	e.expect(rec, http.StatusBadRequest)
	if body := decode[oapi.Error](t, rec); body.Code != "account_required" {
		t.Errorf("code = %q, want account_required", body.Code)
	}
	rec = e.do(call{method: http.MethodGet, path: "/v1/properties", as: s, account: "kingfisher"})
	e.expect(rec, http.StatusBadRequest)
}

func TestCodeRequestLimits(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *auth.Config) { c.CodesPerIP = 6 })
	const phone = "+94772000002"

	e.requestCode(phone)

	// One code every 30 seconds per number.
	rec := e.do(call{method: http.MethodPost, path: "/v1/auth/otp", body: oapi.OtpRequest{Phone: phone}})
	e.expect(rec, http.StatusTooManyRequests)
	body := decode[oapi.Error](t, rec)
	if body.RetryAfter == nil || *body.RetryAfter != 30 || rec.Header().Get("Retry-After") != "30" {
		t.Fatalf("retry after = %v / %q, want 30", body.RetryAfter, rec.Header().Get("Retry-After"))
	}

	// Five per number per hour.
	for range 4 {
		e.clock.Advance(31 * time.Second)
		e.requestCode(phone)
	}
	e.clock.Advance(31 * time.Second)
	rec = e.do(call{method: http.MethodPost, path: "/v1/auth/otp", body: oapi.OtpRequest{Phone: phone}})
	e.expect(rec, http.StatusTooManyRequests)
	if retry := *decode[oapi.Error](t, rec).RetryAfter; retry < 55*60 || retry > 60*60 {
		t.Errorf("retry after %ds, want close to an hour", retry)
	}
	e.clock.Advance(time.Hour)
	e.requestCode(phone)

	// Six per address per hour across numbers (configured low for the test). The five codes
	// above have left the window, so two have been sent within it after this one.
	e.requestCode("+94772000003")
	for i, p := range []string{"+94772000004", "+94772000005", "+94772000006", "+94772000007", "+94772000008"} {
		rec := e.do(call{method: http.MethodPost, path: "/v1/auth/otp", body: oapi.OtpRequest{Phone: p}})
		want := http.StatusOK
		if i == 4 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("code %d from the address: status %d, want %d", i+3, rec.Code, want)
		}
	}
}

func TestCodeAttemptsAndExpiry(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	const phone = "+94772000009"
	verify := func(code string) oapi.Error {
		rec := e.do(call{method: http.MethodPost, path: "/v1/auth/verify", body: oapi.VerifyRequest{Phone: phone, Code: code}})
		e.expect(rec, http.StatusBadRequest)
		return decode[oapi.Error](t, rec)
	}

	code := e.requestCode(phone)
	wrong := "123456"
	if code == wrong {
		wrong = "654321"
	}
	for range 5 {
		if got := verify(wrong).Code; got != "wrong_code" {
			t.Fatalf("code = %q, want wrong_code", got)
		}
	}
	// After five misses even the right code is refused.
	if got := verify(code).Code; got != "too_many_attempts" {
		t.Fatalf("code = %q, want too_many_attempts", got)
	}

	// A new code works, but not after five minutes.
	e.clock.Advance(31 * time.Second)
	code = e.requestCode(phone)
	e.clock.Advance(5*time.Minute + time.Second)
	if got := verify(code).Code; got != "code_expired" {
		t.Fatalf("code = %q, want code_expired", got)
	}

	// Malformed input never reaches the code.
	if got := verify("12ab56").Code; got != "invalid_code" {
		t.Fatalf("code = %q, want invalid_code", got)
	}
	rec := e.do(call{method: http.MethodPost, path: "/v1/auth/otp", body: oapi.OtpRequest{Phone: "0771234567"}})
	e.expect(rec, http.StatusBadRequest)
}

func TestRefreshRotationAndReuse(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.signIn("+94772000010")

	// Each refresh rotates the token.
	first := s.refresh
	rec := e.refresh(first)
	e.expect(rec, http.StatusOK)
	second := decode[oapi.AuthTokens](t, rec).RefreshToken
	if second == first {
		t.Fatal("refresh token did not rotate")
	}

	// A retry with the previous token inside the grace period (a lost response) still works and
	// replaces the token the client never received.
	e.clock.Advance(10 * time.Second)
	rec = e.refresh(first)
	e.expect(rec, http.StatusOK)
	third := decode[oapi.AuthTokens](t, rec)
	e.expect(e.refresh(second), http.StatusUnauthorized) // the lost one is now stale...
	// ...which counts as reuse and revokes the session, so the fresh one fails too.
	e.expect(e.refresh(third.RefreshToken), http.StatusUnauthorized)
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/me", as: &session{access: third.AccessToken}}), http.StatusUnauthorized)

	// Outside the grace period, an old token is reuse straight away.
	s, _ = e.signIn("+94772000011")
	rec = e.refresh(s.refresh)
	e.expect(rec, http.StatusOK)
	current := decode[oapi.AuthTokens](t, rec).RefreshToken
	e.clock.Advance(31 * time.Second)
	e.expect(e.refresh(s.refresh), http.StatusUnauthorized)
	e.expect(e.refresh(current), http.StatusUnauthorized)

	// Garbage and expired tokens are refused.
	e.expect(e.refresh("nonsense"), http.StatusUnauthorized)
	s, _ = e.signIn("+94772000012")
	e.clock.Advance(61 * 24 * time.Hour)
	e.expect(e.refresh(s.refresh), http.StatusUnauthorized)
}
