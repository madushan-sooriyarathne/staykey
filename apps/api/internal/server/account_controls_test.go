package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

func (e *env) deleteMe(s *session, confirm string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(call{method: http.MethodPost, path: "/v1/me/deletion", as: s, body: map[string]any{"confirm": confirm}})
}

// uploadAvatar presigns and uploads a small image as the person's photo, returning its key.
func (e *env) uploadAvatar(s *session) string {
	e.t.Helper()
	data := []byte("\x89PNG\r\n\x1a\nnot really a png")
	rec := e.do(call{method: http.MethodPost, path: "/v1/me/photo", as: s, body: map[string]any{"contentType": "image/png", "size": len(data)}})
	e.expect(rec, http.StatusCreated)
	tk := decode[oapi.UploadTicket](e.t, rec)
	u, err := url.Parse(tk.UploadUrl)
	if err != nil {
		e.t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, u.RequestURI(), bytes.NewReader(data))
	for k, v := range tk.Headers {
		req.Header.Set(k, v)
	}
	put := httptest.NewRecorder()
	e.h.ServeHTTP(put, req)
	if put.Code >= 300 {
		e.t.Fatalf("upload: %d %s", put.Code, put.Body)
	}
	return tk.Key
}

func TestDeletionNeedsTheWordDelete(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.owner("+94776000001", "Delete me")
	e.expect(e.deleteMe(s, "yes"), http.StatusBadRequest)
}

func TestDeletionSignsThePersonOut(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.owner("+94776000002", "Delete me")
	e.expect(e.deleteMe(s, "DELETE"), http.StatusOK)
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/me", as: s}), http.StatusUnauthorized)
}

func TestSigningInAfterDeletionBringsTheAccountBack(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94776000003", "Delete me")
	e.expect(e.deleteMe(s, "DELETE"), http.StatusOK)
	e.clock.Advance(time.Minute) // past the wait before another code
	_, in := e.signIn("+94776000003")
	if len(in.Accounts) != 1 || in.Accounts[0].Id.String() != account {
		t.Errorf("accounts after signing in again = %+v, want %s", in.Accounts, account)
	}
}

func TestACaretakerDeletingThemselvesLeavesTheAccount(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94776000004", "Villa account")
	caretaker := e.member(account, owner, "+94776000005", domain.RoleCaretaker)
	e.expect(e.deleteMe(caretaker, "DELETE"), http.StatusOK)
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/properties", as: owner, account: account}), http.StatusOK)
}

func TestAnotherPersonsPhotoIsRefused(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alice, _ := e.signIn("+94776000006")
	bob, _ := e.signIn("+94776000007")
	key := e.uploadAvatar(alice)
	rec := e.do(call{method: http.MethodPatch, path: "/v1/me", as: bob, body: map[string]any{"photoKey": key}})
	e.expect(rec, http.StatusBadRequest)
}

func TestASavedPhotoComesBackWithTheProfile(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.signIn("+94776000008")
	key := e.uploadAvatar(s)
	e.expect(e.do(call{method: http.MethodPatch, path: "/v1/me", as: s, body: map[string]any{"photoKey": key}}), http.StatusOK)
	rec := e.do(call{method: http.MethodGet, path: "/v1/me", as: s})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.Me](t, rec).User.PhotoUrl; got == nil {
		t.Error("no photoUrl after saving a photo")
	}
}

func TestManagersCantRenameOrExport(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94776000009", "Villa account")
	manager := e.member(account, owner, "+94776000010", domain.RoleManager)
	e.expect(e.do(call{method: http.MethodPatch, path: "/v1/account", as: manager, account: account, body: map[string]any{"name": "Mine now"}}), http.StatusForbidden)
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/account/export", as: manager, account: account}), http.StatusForbidden)
}

func TestAnOwnerRenamesTheAccount(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94776000011", "Villa account")
	rec := e.do(call{method: http.MethodPatch, path: "/v1/account", as: owner, account: account, body: map[string]any{"name": "  Lagoon   Stays "}})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.Account](t, rec).Name; got != "Lagoon Stays" {
		t.Errorf("name = %q, want Lagoon Stays", got)
	}
}

func TestTheExportHoldsTheAccountsStays(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94776000012")
	e.createBooking(v.owner, v.account, v.p, future(10), future(12))
	e.createBooking(v.owner, v.account, v.p, future(20), future(22))
	rec := e.do(call{method: http.MethodGet, path: "/v1/account/export", as: v.owner, account: v.account})
	e.expect(rec, http.StatusOK)
	got := decode[oapi.AccountExport](t, rec)
	if len(got.Bookings) != 2 || len(got.Properties) != 1 || len(got.Team.Members) != 1 {
		t.Errorf("export has %d stays, %d properties, %d members", len(got.Bookings), len(got.Properties), len(got.Team.Members))
	}
}

func TestErasingAGuestElsewhereIsNotFound(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alice := e.villa("+94776000013")
	stay := e.createBooking(alice.owner, alice.account, alice.p, future(10), future(12))
	bob, bobAccount := e.owner("+94776000014", "Bob's account")
	rec := e.do(call{method: http.MethodPost, path: "/v1/bookings/" + stay.Id.String() + "/erase-guest", as: bob, account: bobAccount})
	e.expect(rec, http.StatusNotFound)
}
