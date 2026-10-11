package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"staykey.direct/api/internal/oapi"
)

func inviteToken(t *testing.T, link string) string {
	t.Helper()
	i := strings.LastIndex(link, "/")
	if !strings.HasPrefix(link, "https://staykey.direct/join/") || i < 0 {
		t.Fatalf("invite link %q", link)
	}
	return link[i+1:]
}

// A caretaker joins by link, sees only the property they were invited to, and their alert
// preferences are in place.
func TestCaretakerJoinsByInvite(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94774000001", "Two villas")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	e.createProperty(owner, account, "Coral Bay House")

	rec := e.do(call{method: http.MethodPost, path: "/v1/team/invites", as: owner, account: account, body: map[string]any{
		"phone": "+94 77 400 0002", "role": "caretaker", "propertyIds": []any{villa.Id},
	}})
	e.expect(rec, http.StatusCreated)
	token := inviteToken(t, decode[oapi.InviteCreated](t, rec).Link)

	rec = e.do(call{method: http.MethodPost, path: "/v1/public/invites/preview", body: oapi.InviteToken{Token: token}})
	e.expect(rec, http.StatusOK)
	if pv := decode[oapi.InvitePreview](t, rec); pv.AccountName != "Two villas" || len(pv.PropertyNames) != 1 || pv.PropertyNames[0] != "Kingfisher Villa" {
		t.Errorf("preview = %+v", pv)
	}

	caretaker, _ := e.signIn("+94774000002")
	rec = e.do(call{method: http.MethodPost, path: "/v1/invites/accept", as: caretaker, body: oapi.InviteToken{Token: token}})
	e.expect(rec, http.StatusOK)

	rec = e.do(call{method: http.MethodGet, path: "/v1/properties", as: caretaker, account: account})
	e.expect(rec, http.StatusOK)
	if list := decode[oapi.PropertyList](t, rec); len(list.Items) != 1 || list.Items[0].Id != villa.Id {
		t.Errorf("caretaker sees %v, want only the villa", ids(list.Items))
	}
	rec = e.do(call{method: http.MethodGet, path: "/v1/notification-prefs", as: caretaker, account: account})
	e.expect(rec, http.StatusOK)
	if prefs := decode[oapi.NotificationPrefs](t, rec); len(prefs.Items) != 6 {
		t.Errorf("caretaker has %d preferences, want 6", len(prefs.Items))
	}
}

func TestInviteForAnotherNumberIsRefused(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94774000011", "Kingfisher Villa")
	rec := e.do(call{method: http.MethodPost, path: "/v1/team/invites", as: owner, account: account, body: map[string]any{
		"phone": "+94774000012", "role": "manager", "propertyIds": []any{},
	}})
	e.expect(rec, http.StatusCreated)
	token := inviteToken(t, decode[oapi.InviteCreated](t, rec).Link)

	someoneElse, _ := e.signIn("+94774000013")
	rec = e.do(call{method: http.MethodPost, path: "/v1/invites/accept", as: someoneElse, body: oapi.InviteToken{Token: token}})
	e.expect(rec, http.StatusNotFound)
}

func TestOnlyOwnersInvite(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94774000021", "Kingfisher Villa")
	rec := e.do(call{method: http.MethodPost, path: "/v1/team/invites", as: owner, account: account, body: map[string]any{
		"phone": "+94774000022", "role": "manager", "propertyIds": []any{},
	}})
	e.expect(rec, http.StatusCreated)
	manager, _ := e.signIn("+94774000022")
	token := inviteToken(t, decode[oapi.InviteCreated](t, rec).Link)
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/invites/accept", as: manager, body: oapi.InviteToken{Token: token}}), http.StatusOK)

	rec = e.do(call{method: http.MethodPost, path: "/v1/team/invites", as: manager, account: account, body: map[string]any{
		"phone": "+94774000023", "role": "caretaker", "propertyIds": []any{},
	}})
	e.expect(rec, http.StatusForbidden)
}

func TestOwnerMembershipCantBeChanged(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94774000031", "Kingfisher Villa")
	rec := e.do(call{method: http.MethodGet, path: "/v1/team", as: owner, account: account})
	e.expect(rec, http.StatusOK)
	self := decode[oapi.Team](t, rec).Members[0]

	rec = e.do(call{method: http.MethodPatch, path: "/v1/team/members/" + self.Id.String(), as: owner, account: account, body: map[string]any{"role": "manager"}})
	e.expect(rec, http.StatusForbidden)
}

func TestCaretakerCantEditTemplates(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94774000041", "Kingfisher Villa")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	rec := e.do(call{method: http.MethodPost, path: "/v1/team/invites", as: owner, account: account, body: map[string]any{
		"phone": "+94774000042", "role": "caretaker", "propertyIds": []uuid.UUID{villa.Id},
	}})
	e.expect(rec, http.StatusCreated)
	caretaker, _ := e.signIn("+94774000042")
	token := inviteToken(t, decode[oapi.InviteCreated](t, rec).Link)
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/invites/accept", as: caretaker, body: oapi.InviteToken{Token: token}}), http.StatusOK)
	rec = e.do(call{method: http.MethodGet, path: "/v1/templates", as: caretaker, account: account})
	e.expect(rec, http.StatusOK)
	tpl := decode[oapi.TemplateList](t, rec).Items[0]

	rec = e.do(call{method: http.MethodPatch, path: "/v1/templates/" + tpl.Id.String(), as: caretaker, account: account, body: map[string]any{"body": "Hello"}})
	e.expect(rec, http.StatusForbidden)
}

func TestApprovingShowsInActivityForTheTeam(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94774000051", "Kingfisher Villa")
	villa := e.createProperty(owner, account, "Kingfisher Villa")
	e.createBooking(owner, account, villa, "2026-12-10", "2026-12-12")

	rec := e.do(call{method: http.MethodGet, path: "/v1/activity", as: owner, account: account})
	e.expect(rec, http.StatusOK)
	page := decode[oapi.ActivityPage](t, rec)
	if len(page.Items) != 1 || page.Unread != 0 || !page.Items[0].Read || page.Items[0].Kind != oapi.ActivityKindBooking {
		t.Errorf("owner's activity = %+v, want their own booking, already read", page)
	}
}

func TestRegisterPushToken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.signIn("+94774000061")

	rec := e.do(call{method: http.MethodPost, path: "/v1/push-tokens", as: s, body: map[string]any{"token": "ExponentPushToken[abc]", "platform": "ios"}})
	e.expect(rec, http.StatusNoContent)
}

func TestPushTokenMustBeAnExpoToken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, _ := e.signIn("+94774000071")

	rec := e.do(call{method: http.MethodPost, path: "/v1/push-tokens", as: s, body: map[string]any{"token": "not-a-token", "platform": "ios"}})
	e.expect(rec, http.StatusBadRequest)
}

func TestInviteToAnInternationalNumberIsRefused(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94774000081", "Kingfisher Villa")

	rec := e.do(call{method: http.MethodPost, path: "/v1/team/invites", as: owner, account: account, body: map[string]any{
		"phone": "+44 7700 900123", "role": "caretaker", "propertyIds": []any{},
	}})
	e.expect(rec, http.StatusBadRequest)
}
