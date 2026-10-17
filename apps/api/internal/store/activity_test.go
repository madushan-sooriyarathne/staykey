package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store"
)

func titles(items []domain.Activity) []string {
	out := make([]string, len(items))
	for i, a := range items {
		out[i] = a.Title
	}
	return out
}

func TestApprovingARequestWritesOneActivity(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b := h.request(context.Background(), t, noon.Add(time.Hour))
	if _, err := h.s.TransitionBooking(ctx, h.t, b.ID, b.Version, domain.StatusAwaitingPayment, "", time.Now()); err != nil {
		t.Fatal(err)
	}

	items, err := h.s.ListActivity(ctx, h.t, nil, 50)
	var requests []domain.Activity
	for _, a := range items {
		if a.Kind == domain.ActivityRequest {
			requests = append(requests, a)
		}
	}
	got := strings.Join(titles(requests), " | ")
	if err != nil || got != "A team member approved Ravi Fernando | Request from Ravi Fernando" {
		t.Errorf("activity = %q (err %v)", got, err)
	}
}

func TestCaretakerDoesntSeePayments(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.book(ctx, h.r1, "2026-11-01", "2026-11-03")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.RecordPayment(ctx, h.t, b.ID, domain.Payment{Method: "cash", Amount: 5000, ReceivedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	caretaker := h.caretaker(t)

	items, err := h.s.ListActivity(ctx, caretaker, nil, 50)
	got := strings.Join(titles(items), " | ")
	if err != nil || got != "A team member added Nimali Perera" {
		t.Errorf("caretaker activity = %q (err %v)", got, err)
	}
}

func TestOwnActionsDontCountAsUnread(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.request(context.Background(), t, noon.Add(time.Hour))

	n, err := h.s.UnreadActivity(context.Background(), h.t)
	if err != nil || n != 0 {
		t.Errorf("unread = %d (err %v), want 0", n, err)
	}
}

func TestMarkingReadClearsUnread(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	caretaker := h.caretaker(t)
	h.request(context.Background(), t, noon.Add(time.Hour))
	if err := h.s.MarkActivityRead(ctx, caretaker, time.Now()); err != nil {
		t.Fatal(err)
	}

	n, err := h.s.UnreadActivity(ctx, caretaker)
	if err != nil || n != 0 {
		t.Errorf("unread after marking read = %d (err %v), want 0", n, err)
	}
}

func TestAlertsSkipTheActorAndReachTheCaretaker(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	caretaker := h.caretaker(t)
	// The caretaker's session and device, so the alert has somewhere to go.
	h.signIn(t, caretaker.UserID, "ExponentPushToken[caretaker]")
	h.request(context.Background(), t, noon.Add(time.Hour))

	var got []store.Alert
	_, err := h.s.SendAlerts(ctx, 100, time.Now(), func(_ context.Context, alerts []store.Alert) error {
		got = alerts
		return nil
	})
	if err != nil || len(got) != 1 || len(got[0].Recipients) != 1 || got[0].Recipients[0].PushTokens[0] != "ExponentPushToken[caretaker]" {
		t.Errorf("alerts = %+v (err %v), want one alert for the caretaker's device", got, err)
	}
}

func TestSentAlertsAreNotSentAgain(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.request(context.Background(), t, noon.Add(time.Hour))
	noop := func(context.Context, []store.Alert) error { return nil }
	if _, err := h.s.SendAlerts(ctx, 100, time.Now(), noop); err != nil {
		t.Fatal(err)
	}

	n, err := h.s.SendAlerts(ctx, 100, time.Now(), noop)
	if err != nil || n != 0 {
		t.Errorf("second run claimed %d items (err %v), want 0", n, err)
	}
}

func TestExpiredRequestIsLogged(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.request(ctx, t, noon.Add(-time.Minute))
	if _, err := h.s.DeclineExpiredRequests(ctx, noon.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	items, err := h.s.ListActivity(ctx, h.t, nil, 1)
	if err != nil || len(items) != 1 || items[0].Title != "Request from Ravi Fernando expired" {
		t.Errorf("latest activity = %q (err %v)", titles(items), err)
	}
}

func TestFailedSendIsRetried(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.request(ctx, t, noon.Add(time.Hour))
	failing := func(context.Context, []store.Alert) error { return errors.New("expo unreachable") }
	if _, err := h.s.SendAlerts(ctx, 100, time.Now(), failing); err == nil {
		t.Fatal("a failed send should be reported")
	}

	n, err := h.s.SendAlerts(ctx, 100, time.Now(), func(context.Context, []store.Alert) error { return nil })
	if err != nil || n != 1 {
		t.Errorf("retry claimed %d items (err %v), want 1", n, err)
	}
}
