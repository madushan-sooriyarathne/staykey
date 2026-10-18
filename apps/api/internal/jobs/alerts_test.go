package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/email"
	"staykey.direct/api/internal/notify"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/store/storetest"
	"staykey.direct/api/internal/tenant"
)

// expoStub records the pushes it receives.
type expoStub struct {
	mu   sync.Mutex
	sent []map[string]any
}

func (e *expoStub) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msgs []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&msgs); err != nil {
			t.Error(err)
		}
		e.mu.Lock()
		e.sent = append(e.sent, msgs...)
		e.mu.Unlock()
		w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

type team struct {
	st        *store.Postgres
	owner     tenant.Tenant
	caretaker tenant.Tenant
	property  domain.PropertyDetail
}

// newTeam makes an account with a villa, its owner and a caretaker for the villa, each signed
// in on a phone with a push token.
func newTeam(t *testing.T) team {
	t.Helper()
	ctx := context.Background()
	st := storetest.New(t)
	owner, _, err := st.UpsertUserByPhone(ctx, "+94771234567")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := st.CreateAccount(ctx, owner.ID, "Kingfisher Collection")
	if err != nil {
		t.Fatal(err)
	}
	ot := tenant.Tenant{AccountID: acc.ID, UserID: owner.ID, MembershipID: acc.MembershipID, Role: domain.RoleOwner}
	p, err := st.CreateProperty(ctx, ot, domain.NewProperty{Slug: "kingfisher", Name: "Kingfisher Villa", BookingType: domain.BookingEntire, Currency: "USD", BaseRateMinor: 10000})
	if err != nil {
		t.Fatal(err)
	}
	ck, _, err := st.UpsertUserByPhone(ctx, "+94715550199")
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.AddMember(ctx, ot, ck.ID, domain.RoleCaretaker, []uuid.UUID{p.ID})
	if err != nil {
		t.Fatal(err)
	}
	for user, token := range map[uuid.UUID]string{owner.ID: "ExponentPushToken[owner]", ck.ID: "ExponentPushToken[caretaker]"} {
		sess := auth.Session{ID: domain.NewID(), UserID: user, RefreshHash: []byte(token), ExpiresAt: time.Now().AddDate(2, 0, 0)}
		if err := st.CreateSession(ctx, sess, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := st.SavePushToken(ctx, user, sess.ID, token, "android"); err != nil {
			t.Fatal(err)
		}
	}
	ct := tenant.Tenant{AccountID: acc.ID, UserID: ck.ID, MembershipID: m.ID, Role: domain.RoleCaretaker, PropertyIDs: []uuid.UUID{p.ID}}
	return team{st: st, owner: ot, caretaker: ct, property: p}
}

func (tm team) book(t *testing.T, from time.Time) {
	t.Helper()
	unit := tm.property.Units[0].ID
	_, err := tm.st.CreateBooking(context.Background(), tm.owner, domain.NewBooking{
		PropertyID: tm.property.ID, Status: domain.StatusConfirmed, Currency: "USD", RefPrefix: "KV", LedgerUnits: []uuid.UUID{unit},
		Stay: domain.StayInput{UnitID: unit, CheckIn: from, CheckOut: from.AddDate(0, 0, 2), Adults: 2, Guest: domain.Guest{Name: "Sam Lee"}, Source: "phone"},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
}

func alertsFor(srv *httptest.Server) Alerts {
	log := slog.New(slog.DiscardHandler)
	return Alerts{Push: notify.Expo{URL: srv.URL}, WhatsApp: notify.LogWhatsApp{Logger: log}, Logger: log}
}

// The phase 4 gate, below the app: the owner adds a stay and only the caretaker's phone hears
// about it.
func TestNewStayAlertsTheCaretakerNotTheOwner(t *testing.T) {
	tm := newTeam(t)
	expo := &expoStub{}
	a := alertsFor(expo.server(t))
	tm.book(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))

	if err := SendAlerts(context.Background(), tm.st, a, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(expo.sent) != 1 || expo.sent[0]["to"] != "ExponentPushToken[caretaker]" || expo.sent[0]["title"] != "A team member added Sam Lee" {
		t.Errorf("pushes = %v, want one to the caretaker", expo.sent)
	}
}

func TestMorningSummaryGoesOutOncePerDay(t *testing.T) {
	tm := newTeam(t)
	ctx := context.Background()
	expo := &expoStub{}
	a := alertsFor(expo.server(t))
	if _, err := tm.st.SetNotificationPrefs(ctx, tm.owner, []domain.NotificationPref{{Event: domain.EventSummary, Push: true}}); err != nil {
		t.Fatal(err)
	}
	// 07:30 in Colombo.
	morning := time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC)
	tm.book(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))

	for range 2 {
		if err := SendSummaries(ctx, tm.st, a, morning); err != nil {
			t.Fatal(err)
		}
	}
	if len(expo.sent) != 1 || expo.sent[0]["body"] != "1 arrival." {
		t.Errorf("pushes = %v, want one summary with one arrival", expo.sent)
	}
}

func TestSummaryText(t *testing.T) {
	tests := []struct {
		sum  store.Summary
		want string
	}{
		{store.Summary{}, "No arrivals, departures or requests today."},
		{store.Summary{Arrivals: 2, Departures: 1, Requests: 3}, "2 arrivals, 1 departure and 3 requests waiting."},
		{store.Summary{Requests: 1}, "1 request waiting."},
	}
	for _, tt := range tests {
		if got := SummaryText(tt.sum); got != tt.want {
			t.Errorf("SummaryText(%+v) = %q, want %q", tt.sum, got, tt.want)
		}
	}
}

// mailbox is an email sender that keeps what it is given.
type mailbox struct {
	mu   sync.Mutex
	sent []email.Message
}

func (m *mailbox) Send(_ context.Context, msg email.Message) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return "msg-1", nil
}

// caretakerWantsEmail turns email on for the caretaker's booking alerts and gives them an
// email, confirmed or not.
func (tm team) caretakerWantsEmail(t *testing.T, confirmed bool) {
	t.Helper()
	ctx := context.Background()
	address := "kamal@example.com"
	if _, err := tm.st.UpdateUser(ctx, tm.caretaker.UserID, domain.UserPatch{Email: &address}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if confirmed {
		if _, err := tm.st.Pool().Exec(ctx, "UPDATE users SET email_confirmed_at = now() WHERE id = $1", tm.caretaker.UserID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tm.st.SetNotificationPrefs(ctx, tm.caretaker, []domain.NotificationPref{{Event: domain.EventBooking, Push: true, Email: true}}); err != nil {
		t.Fatal(err)
	}
}

func TestAConfirmedTeamMemberGetsTheAlertByEmail(t *testing.T) {
	tm := newTeam(t)
	expo, box := &expoStub{}, &mailbox{}
	a := alertsFor(expo.server(t))
	a.Email = box
	tm.caretakerWantsEmail(t, true)
	tm.book(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	if err := SendAlerts(context.Background(), tm.st, a, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(box.sent) != 1 || box.sent[0].To != "kamal@example.com" {
		t.Errorf("emails = %+v, want one to the caretaker", box.sent)
	}
}

func TestAnUnconfirmedTeamMemberGetsNoAlertEmail(t *testing.T) {
	tm := newTeam(t)
	expo, box := &expoStub{}, &mailbox{}
	a := alertsFor(expo.server(t))
	a.Email = box
	tm.caretakerWantsEmail(t, false)
	tm.book(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	if err := SendAlerts(context.Background(), tm.st, a, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(box.sent) != 0 {
		t.Errorf("emails = %+v, want none", box.sent)
	}
}
