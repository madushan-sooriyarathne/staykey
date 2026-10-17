package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/ical"
	"staykey.direct/api/internal/store"
)

var (
	syncNow   = time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	syncToday = day("2026-10-01")
)

// feed connects an Airbnb calendar for the whole house and claims it, as the import job does.
func (h house) feed(t *testing.T) store.DueFeed {
	t.Helper()
	ctx := context.Background()
	if _, err := h.s.UpdateProperty(ctx, h.t, h.p.ID, domain.PropertyPatch{IcalFeeds: &[]domain.IcalFeedInput{
		{Ref: "new", Channel: "airbnb", URL: "https://www.airbnb.com/calendar/ical/1.ics?s=secret"},
	}}); err != nil {
		t.Fatal(err)
	}
	feeds, err := h.s.ClaimDueFeeds(ctx, syncNow, 5*time.Minute, 20)
	if err != nil || len(feeds) != 1 {
		t.Fatalf("ClaimDueFeeds = %d feeds, %v", len(feeds), err)
	}
	return feeds[0]
}

func (h house) sync(t *testing.T, f *store.DueFeed, events ...ical.Event) {
	t.Helper()
	if err := h.s.ApplyFeed(context.Background(), *f, events, "", syncToday, syncNow); err != nil {
		t.Fatal(err)
	}
	f.FirstSync = false
}

func (h house) stays(t *testing.T) []domain.Booking {
	t.Helper()
	all, err := h.s.ListBookings(context.Background(), h.t, domain.BookingFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	return all
}

func event(uid, from, to string) ical.Event {
	return ical.Event{UID: uid, Start: day(from), End: day(to)}
}

func TestSyncingTwiceImportsAStayOnce(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	f := h.feed(t)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	if n := len(h.stays(t)); n != 1 {
		t.Errorf("%d stays, want 1", n)
	}
}

func TestAnImportedStayHoldsTheWholeHouse(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	f := h.feed(t)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	_, err := h.book(context.Background(), h.r2, "2026-10-11", "2026-10-12")
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Errorf("booking a room under an imported whole-house stay = %v, want a clash", err)
	}
}

func TestAMovedEventMovesTheStay(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	f := h.feed(t)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	h.sync(t, &f, event("a@airbnb.com", "2026-10-20", "2026-10-23"))
	if _, err := h.book(context.Background(), h.r1, "2026-10-10", "2026-10-12"); err != nil {
		t.Errorf("the old nights are still held: %v", err)
	}
}

func TestARemovedEventCancelsTheStay(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	f := h.feed(t)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	h.sync(t, &f)
	if s := h.stays(t)[0]; s.Status != domain.StatusCancelled {
		t.Errorf("status = %s, want cancelled", s.Status)
	}
}

func TestAStayTheOwnerCancelledStaysCancelled(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	f := h.feed(t)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	b := h.stays(t)[0]
	if _, err := h.s.CancelBooking(ctx, h.t, b.ID, b.Version, "guest moved to WhatsApp", nil, syncNow); err != nil {
		t.Fatal(err)
	}
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	if _, err := h.book(ctx, h.r1, "2026-10-10", "2026-10-12"); err != nil {
		t.Errorf("the cancelled import took its nights back: %v", err)
	}
}

func TestAClashWithADirectStayAlertsOnceAndHoldsNothing(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.book(ctx, h.r1, "2026-10-10", "2026-10-12"); err != nil {
		t.Fatal(err)
	}
	f := h.feed(t)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	items, err := h.s.ListActivity(ctx, h.t, nil, 20)
	if err != nil {
		t.Fatal(err)
	}
	overlaps := 0
	for _, a := range items {
		if a.Kind == domain.ActivitySync {
			overlaps++
		}
	}
	if overlaps != 1 {
		t.Errorf("%d overlap alerts in %q, want 1", overlaps, titles(items))
	}
}

func TestAClashedImportHoldsOnceTheOtherStayIsCancelled(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	direct, err := h.book(ctx, h.r1, "2026-10-10", "2026-10-12")
	if err != nil {
		t.Fatal(err)
	}
	f := h.feed(t)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	if _, err := h.s.CancelBooking(ctx, h.t, direct.ID, direct.Version, "double booked", nil, syncNow); err != nil {
		t.Fatal(err)
	}
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	_, err = h.book(ctx, h.r2, "2026-10-10", "2026-10-11")
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Errorf("room 2 is free after the retry: %v, want the import to hold it", err)
	}
}

func TestAFirstSyncWritesOneActivityLine(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	f := h.feed(t)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"), event("b@airbnb.com", "2026-11-01", "2026-11-04"))
	items, err := h.s.ListActivity(context.Background(), h.t, nil, 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); len(got) != 1 || got[0] != "Airbnb calendar connected" {
		t.Errorf("activity = %q", got)
	}
}

func TestALaterSyncWritesALinePerNewStay(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	f := h.feed(t)
	h.sync(t, &f)
	h.sync(t, &f, event("a@airbnb.com", "2026-10-10", "2026-10-12"))
	items, err := h.s.ListActivity(context.Background(), h.t, nil, 20)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Title != "Airbnb stay imported" {
		t.Errorf("activity = %q", titles(items))
	}
}

func TestAThirdFailureInARowAlertsTheTeam(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	f := h.feed(t)
	for range 3 {
		n, err := h.s.RecordFeedFailure(ctx, f, "The calendar link wasn't found.", syncNow)
		if err != nil {
			t.Fatal(err)
		}
		f.Failures = n
	}
	items, err := h.s.ListActivity(ctx, h.t, nil, 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); len(got) != 1 || got[0] != "Airbnb sync failed" {
		t.Errorf("activity = %q", got)
	}
}

func TestTheExportListsStaysWithoutGuests(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	if _, err := h.book(ctx, h.whole, "2026-10-10", "2026-10-12"); err != nil {
		t.Fatal(err)
	}
	_, runs, err := h.s.ExportCalendar(ctx, h.p.Slug, h.p.IcalExportToken, &h.r2, syncToday)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || !runs[0].Start.Equal(day("2026-10-10")) {
		t.Errorf("room 2's export = %+v, want the whole-house stay", runs)
	}
}

func TestTheExportRefusesAWrongToken(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	if _, _, err := h.s.ExportCalendar(context.Background(), h.p.Slug, "guess", nil, syncToday); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ExportCalendar with a wrong token = %v, want not found", err)
	}
}

// A busy Booking.com feed imports in one transaction on its first sync.
func TestABusyFirstSyncIsQuick(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	f := h.feed(t)
	events := make([]ical.Event, 200)
	for i := range events {
		start := syncToday.AddDate(0, 0, 3*i)
		events[i] = ical.Event{UID: fmt.Sprintf("%d@booking.com", i), Start: start, End: start.AddDate(0, 0, 2)}
	}
	began := time.Now()
	h.sync(t, &f, events...)
	if took := time.Since(began); took > 10*time.Second {
		t.Errorf("first sync of 200 stays took %v", took)
	}
}
