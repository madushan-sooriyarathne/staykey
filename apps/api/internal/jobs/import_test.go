package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/ical"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/store/storetest"
	"staykey.direct/api/internal/tenant"
)

const airbnbFeed = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Airbnb Inc//Hosting Calendar 0.8.8//EN\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20261110\r\nDTEND;VALUE=DATE:20261113\r\n" +
	"UID:1418fb94e984-f2a86c6e1ae6e8ff7f4c4a5a1ddc3e47@airbnb.com\r\nSUMMARY:Reserved\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

// importSetup is a villa with an Airbnb feed served by srv.
func importSetup(t *testing.T, feedURL string) (*store.Postgres, domain.PropertyDetail) {
	t.Helper()
	ctx := context.Background()
	st := storetest.New(t)
	u, _, err := st.UpsertUserByPhone(ctx, "+94771234567")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := st.CreateAccount(ctx, u.ID, "Imports")
	if err != nil {
		t.Fatal(err)
	}
	tn := tenant.Tenant{AccountID: acc.ID, UserID: u.ID, Role: domain.RoleOwner}
	p, err := st.CreateProperty(ctx, tn, domain.NewProperty{
		Slug: "imports", Name: "Imports Villa", BookingType: domain.BookingEntire, Currency: "USD", BaseRateMinor: 10000,
		Setup: domain.PropertyPatch{IcalFeeds: &[]domain.IcalFeedInput{{Ref: "new", Channel: "airbnb", URL: feedURL}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return st, p
}

var (
	importNow        = time.Date(2026, 10, 15, 4, 0, 0, 0, time.UTC)
	stayFrom, stayTo = time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	quiet            = slog.New(slog.NewTextHandler(io.Discard, nil))
	airbnbServer     = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, airbnbFeed) })
)

// The phase 5 gate below the app: a stay on the Airbnb calendar closes those nights to guests
// after one run of the import.
func TestAnAirbnbStayClosesItsNightsAfterOneImport(t *testing.T) {
	srv := httptest.NewServer(airbnbServer)
	defer srv.Close()
	ctx := context.Background()
	st, p := importSetup(t, srv.URL+"/calendar.ics")
	if err := ImportFeeds(ctx, st, ical.NewFetcher(true), quiet, importNow); err != nil {
		t.Fatal(err)
	}

	taken, err := st.Availability(ctx, p.AccountID, p.ID, stayFrom, stayTo, importNow)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(taken[p.Units[0].ID]); n != 3 {
		t.Errorf("%d nights taken, want 3", n)
	}
}

func TestAHoldOnImportedNightsClashes(t *testing.T) {
	srv := httptest.NewServer(airbnbServer)
	defer srv.Close()
	ctx := context.Background()
	st, p := importSetup(t, srv.URL+"/calendar.ics")
	if err := ImportFeeds(ctx, st, ical.NewFetcher(true), quiet, importNow); err != nil {
		t.Fatal(err)
	}

	_, err := st.CreateHold(ctx, p.AccountID, p.ID, domain.LedgerUnits(p.Units, p.Units[0].ID), stayFrom, stayTo, importNow.Add(15*time.Minute), importNow, nil)
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Errorf("hold on the imported nights = %v, want a clash", err)
	}
}

func TestAFeedOnAPrivateAddressFailsInProduction(t *testing.T) {
	srv := httptest.NewServer(airbnbServer)
	defer srv.Close()
	ctx := context.Background()
	st, p := importSetup(t, srv.URL+"/calendar.ics")
	if err := ImportFeeds(ctx, st, ical.NewFetcher(false), quiet, importNow); err != nil {
		t.Fatal(err)
	}

	tn := tenant.Tenant{AccountID: p.AccountID, Role: domain.RoleOwner}
	got, err := st.GetProperty(ctx, tn, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if f := got.IcalFeeds[0]; f.Error != "That link points to a private address." {
		t.Errorf("feed error = %q", f.Error)
	}
}
