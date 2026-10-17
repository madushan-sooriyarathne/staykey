package domain

import (
	"testing"
	"time"
)

func TestBillingState(t *testing.T) {
	start := time.Date(2026, 9, 1, 6, 0, 0, 0, time.UTC)
	day := func(n int) time.Time { return start.AddDate(0, 0, n) }
	at := func(n int) *time.Time { v := day(n); return &v }
	paid := func(status SubscriptionStatus, end time.Time) *Subscription {
		return &Subscription{Source: SourceAppStore, Plan: PlanVilla, Period: PeriodMonthly, Status: status, PeriodEnd: end}
	}
	tests := []struct {
		name        string
		now         time.Time
		firstDirect *time.Time
		sub         *Subscription
		want        Billing
	}{
		{"in the free period", day(10), nil, nil,
			Billing{Status: StatusTrial, Until: day(FreeDays), FreeEndsAt: day(FreeDays)}},
		{"first direct booking starts grace", day(12), at(10), nil,
			Billing{Status: StatusGrace, Until: day(17), FreeEndsAt: day(10), Reason: GraceFirstBooking}},
		{"free days run out", day(FreeDays), nil, nil,
			Billing{Status: StatusGrace, Until: day(FreeDays + GraceDays), FreeEndsAt: day(FreeDays), Reason: GraceFreeDays}},
		{"a booking after the free days doesn't move the end", day(FreeDays + 1), at(FreeDays + 1), nil,
			Billing{Status: StatusGrace, Until: day(FreeDays + GraceDays), FreeEndsAt: day(FreeDays), Reason: GraceFreeDays}},
		{"grace runs out", day(17), at(10), nil,
			Billing{Status: StatusLapsed, Until: day(17), FreeEndsAt: day(10), Reason: GraceFirstBooking}},
		{"a paid plan is active", day(20), at(10), paid(SubActive, day(40)),
			Billing{Status: StatusActive, Until: day(40), FreeEndsAt: day(10)}},
		{"a cancelled plan runs to its end", day(39), at(10), paid(SubCancelled, day(40)),
			Billing{Status: StatusActive, Until: day(40), FreeEndsAt: day(10)}},
		{"an ended plan gets grace", day(41), at(10), paid(SubExpired, day(40)),
			Billing{Status: StatusGrace, Until: day(47), FreeEndsAt: day(10), Reason: GracePlanEnded}},
		{"a plan bought early that ends inside the free period", day(30), nil, paid(SubExpired, day(20)),
			Billing{Status: StatusTrial, Until: day(FreeDays), FreeEndsAt: day(FreeDays)}},
		{"the later of free and paid ends counts", day(FreeDays + 3), nil, paid(SubExpired, day(20)),
			Billing{Status: StatusGrace, Until: day(FreeDays + GraceDays), FreeEndsAt: day(FreeDays), Reason: GraceFreeDays}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BillingState(tt.now, start, tt.firstDirect, tt.sub); got != tt.want {
				t.Errorf("BillingState = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseProduct(t *testing.T) {
	tests := []struct {
		id     string
		plan   PlanID
		period Period
		ok     bool
	}{
		{"staykey_guesthouse_yearly", PlanGuesthouse, PeriodYearly, true},
		{"staykey_villa_monthly:monthly-base", PlanVilla, PeriodMonthly, true},
		{"staykey_collection:yearly", PlanCollection, PeriodYearly, true},
		{"staykey_palace_monthly", "", "", false},
		{"other_villa_monthly", "", "", false},
		{"staykey_villa", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			plan, period, ok := ParseProduct(tt.id)
			if plan != tt.plan || period != tt.period || ok != tt.ok {
				t.Errorf("ParseProduct = %s %s %v, want %s %s %v", plan, period, ok, tt.plan, tt.period, tt.ok)
			}
		})
	}
}

func TestPlanFor(t *testing.T) {
	tests := []struct {
		units int
		want  PlanID
	}{
		{0, PlanVilla}, {1, PlanVilla}, {2, PlanGuesthouse}, {6, PlanGuesthouse}, {7, PlanCollection}, {40, PlanCollection},
	}
	for _, tt := range tests {
		if got := PlanFor(tt.units); got != tt.want {
			t.Errorf("PlanFor(%d) = %s, want %s", tt.units, got, tt.want)
		}
	}
}

func TestTakesBookingsOnlyStopsWhenLapsed(t *testing.T) {
	for _, s := range []AccountStatus{StatusTrial, StatusActive, StatusGrace} {
		if !TakesBookings(s) {
			t.Errorf("TakesBookings(%s) = false", s)
		}
	}
	if TakesBookings(StatusLapsed) {
		t.Error("TakesBookings(lapsed) = true")
	}
}

func TestBillingChanged(t *testing.T) {
	until := time.Date(2026, 10, 24, 4, 0, 0, 0, time.UTC)
	sub := &Subscription{Plan: PlanGuesthouse}
	tests := []struct {
		name            string
		b               Billing
		title, subtitle string
	}{
		{"first direct booking", Billing{Status: StatusGrace, Until: until, Reason: GraceFirstBooking},
			"Your first direct booking is in", "Free until 24 Oct, then choose a plan"},
		{"free days over", Billing{Status: StatusGrace, Until: until, Reason: GraceFreeDays},
			"Your free period has ended", "Choose a plan by 24 Oct to keep your booking page open"},
		{"plan ended", Billing{Status: StatusGrace, Until: until, Reason: GracePlanEnded},
			"Your plan has ended", "Renew by 24 Oct to keep your booking page open"},
		{"lapsed", Billing{Status: StatusLapsed, Until: until},
			"Your booking page is paused", "Guests are asked to message you on WhatsApp until you choose a plan"},
		{"active", Billing{Status: StatusActive, Until: until},
			"Guesthouse plan active", "Paid until 24 Oct"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := BillingChanged(tt.b, sub)
			if !ok || got.Kind != ActivityBilling || got.Title != tt.title || got.Subtitle != tt.subtitle {
				t.Errorf("got %v %s %q %q, want %q %q", ok, got.Kind, got.Title, got.Subtitle, tt.title, tt.subtitle)
			}
		})
	}
}

func TestBillingChangedBackToTrialSaysNothing(t *testing.T) {
	if _, ok := BillingChanged(Billing{Status: StatusTrial}, nil); ok {
		t.Error("expected no line")
	}
}
