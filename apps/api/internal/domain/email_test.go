package domain

import (
	"strings"
	"testing"
	"time"
)

func TestRenderTemplateFillsEveryPlaceholder(t *testing.T) {
	values := TemplateValues(EmailValues{
		Name: "Emma Clarke", PropertyName: "Kingfisher Villa", BookingPage: "https://kingfisher.staykey.direct",
		CheckIn: date(2026, 10, 4), CheckOut: date(2026, 10, 8),
	}, 18_000_00, "LKR", "14:00", "https://maps.google.com/?q=6.9,79.8")
	tests := []struct{ placeholder, want string }{
		{"{guest_name}", "Emma"},
		{"{property_name}", "Kingfisher Villa"},
		{"{check_in}", "Sun 4 Oct"},
		{"{check_out}", "Thu 8 Oct"},
		{"{balance}", "LKR 18,000"},
		{"{booking_link}", "https://kingfisher.staykey.direct"},
		{"{map_link}", "https://maps.google.com/?q=6.9,79.8"},
		{"{check_in_time}", "2:00 PM"},
		{"{unknown}", "{unknown}"},
	}
	for _, tt := range tests {
		t.Run(tt.placeholder, func(t *testing.T) {
			if got := RenderTemplate("<"+tt.placeholder+">", values); got != "<"+tt.want+">" {
				t.Errorf("RenderTemplate(%s) = %q, want <%s>", tt.placeholder, got, tt.want)
			}
		})
	}
}

func TestComposedEmailsHaveASubject(t *testing.T) {
	for _, kind := range EmailKinds {
		t.Run(string(kind), func(t *testing.T) {
			if subject, _ := ComposeEmail(kind, sampleValues()); subject == "" {
				t.Errorf("ComposeEmail(%s) has no subject", kind)
			}
		})
	}
}

func TestComposedEmailsHaveNoEmDash(t *testing.T) {
	for _, kind := range EmailKinds {
		t.Run(string(kind), func(t *testing.T) {
			subject, text := ComposeEmail(kind, sampleValues())
			if strings.ContainsRune(subject+text, '—') {
				t.Errorf("ComposeEmail(%s) contains an em dash", kind)
			}
		})
	}
}

func TestOnlyReceiptsMentionAnAmount(t *testing.T) {
	for _, kind := range EmailKinds {
		t.Run(string(kind), func(t *testing.T) {
			_, text := ComposeEmail(kind, sampleValues())
			if mentions := strings.Contains(text, "LKR"); mentions != (kind == EmailPlanReceipt) {
				t.Errorf("ComposeEmail(%s) mentions an amount: %v", kind, mentions)
			}
		})
	}
}

func TestReceiptShowsThePaymentID(t *testing.T) {
	_, text := ComposeEmail(EmailPlanReceipt, sampleValues())
	if !strings.Contains(text, "Payment ID: PLK-123") {
		t.Errorf("receipt text = %q, want the payment id", text)
	}
}

func TestReceivedFollowsTheStatus(t *testing.T) {
	tests := []struct {
		status BookingStatus
		want   string
	}{
		{StatusRequested, "Your request to stay at Kingfisher Villa"},
		{StatusAwaitingPayment, "Your booking at Kingfisher Villa"},
		{StatusConfirmed, "Your stay at Kingfisher Villa is confirmed"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			v := sampleValues()
			v.Status = tt.status
			if subject, _ := ComposeEmail(EmailReceived, v); subject != tt.want {
				t.Errorf("subject = %q, want %q", subject, tt.want)
			}
		})
	}
}

func TestTimedEmailDueTimes(t *testing.T) {
	colomboTZ, _ := time.LoadLocation("Asia/Colombo")
	due := time.Date(2026, 10, 20, 6, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		got  time.Time
		want time.Time
	}{
		{"reminder a day before the deadline", ReminderAt(due), time.Date(2026, 10, 19, 6, 30, 0, 0, time.UTC)},
		{"pre-arrival at 09:00 Colombo two days before", PrearrivalAt(date(2026, 11, 1), colomboTZ), time.Date(2026, 10, 30, 3, 30, 0, 0, time.UTC)},
		{"pre-arrival across a month start", PrearrivalAt(date(2026, 3, 1), time.UTC), time.Date(2026, 2, 27, 9, 0, 0, 0, time.UTC)},
		{"thank-you at 10:00 Colombo the day after", ThanksAt(date(2026, 10, 31), colomboTZ), time.Date(2026, 11, 1, 4, 30, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.got.Equal(tt.want) {
				t.Errorf("got %s, want %s", tt.got.UTC(), tt.want)
			}
		})
	}
}

func TestTimedEmailQueueWindow(t *testing.T) {
	due := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	stay := EmailStay{Status: StatusConfirmed, CheckIn: date(2026, 10, 22), CheckOut: date(2026, 10, 25)}
	unpaid := EmailStay{Status: StatusAwaitingPayment, CheckIn: date(2026, 11, 22), CheckOut: date(2026, 11, 25), PaymentDueAt: &due}
	tests := []struct {
		name string
		kind EmailKind
		stay EmailStay
		now  time.Time
		want bool
	}{
		{"pre-arrival more than a day out", EmailPrearrival, stay, time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC), false},
		{"pre-arrival within a day", EmailPrearrival, stay, time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC), true},
		{"pre-arrival on check-in day", EmailPrearrival, stay, time.Date(2026, 10, 22, 1, 0, 0, 0, time.UTC), false},
		{"pre-arrival for a cancelled stay", EmailPrearrival, EmailStay{Status: StatusCancelled, CheckIn: stay.CheckIn}, time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC), false},
		{"reminder within a day of its time", EmailReminder, unpaid, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), true},
		{"reminder long past its time", EmailReminder, unpaid, time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC), false},
		{"thank-you two days late", EmailThanks, stay, time.Date(2026, 10, 28, 11, 0, 0, 0, time.UTC), false},
		{"thank-you on the day", EmailThanks, stay, time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, ok := TimedEmail(tt.kind, tt.stay, time.UTC, tt.now); ok != tt.want {
				t.Errorf("TimedEmail ok = %v, want %v", ok, tt.want)
			}
		})
	}
}

func TestStillWanted(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	dueOn := due.Format(time.RFC3339)
	unpaid := EmailStay{Status: StatusAwaitingPayment, CheckIn: date(2026, 10, 22), CheckOut: date(2026, 10, 25), PaymentDueAt: &due, Balance: 100}
	tests := []struct {
		name string
		kind EmailKind
		stay EmailStay
		on   string
		want bool
	}{
		{"reminder for an unpaid stay", EmailReminder, unpaid, dueOn, true},
		{"reminder for a paid stay", EmailReminder, EmailStay{Status: StatusConfirmed, PaymentDueAt: &due}, dueOn, false},
		{"reminder for a cancelled stay", EmailReminder, EmailStay{Status: StatusCancelled, PaymentDueAt: &due, Balance: 100}, dueOn, false},
		{"reminder after the deadline moved", EmailReminder, unpaid, now.Format(time.RFC3339), false},
		{"pre-arrival for the same dates", EmailPrearrival, unpaid, "2026-10-22", true},
		{"pre-arrival after the stay moved", EmailPrearrival, unpaid, "2026-10-21", false},
		{"thank-you for a cancelled stay", EmailThanks, EmailStay{Status: StatusCancelled, CheckOut: date(2026, 10, 25)}, "2026-10-25", false},
		{"confirmed for a cancelled stay", EmailConfirmed, EmailStay{Status: StatusCancelled}, "", false},
		{"confirmed for an unpaid stay", EmailConfirmed, unpaid, "", false},
		{"confirmed for a stay that ended", EmailConfirmed, EmailStay{Status: StatusConfirmed, CheckOut: date(2026, 10, 19)}, "", false},
		{"confirmed for a stay ahead", EmailConfirmed, EmailStay{Status: StatusConfirmed, CheckOut: date(2026, 10, 25)}, "", true},
		{"declined for a declined request", EmailDeclined, EmailStay{Status: StatusDeclined}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StillWanted(tt.kind, tt.stay, tt.on, now); got != tt.want {
				t.Errorf("StillWanted = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		amount   int64
		currency string
		want     string
	}{
		{18_000_00, "LKR", "LKR 18,000"},
		{1_234_567_50, "LKR", "LKR 1,234,567.50"},
		{180_50, "USD", "$180.50"},
		{0, "EUR", "€0"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatMoney(tt.amount, tt.currency); got != tt.want {
				t.Errorf("FormatMoney = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfirmationTokenMatchesItsHash(t *testing.T) {
	token, hash, err := NewEmailConfirmationToken()
	if err != nil {
		t.Fatal(err)
	}
	if string(EmailConfirmationHash(token)) != string(hash) {
		t.Error("the stored hash doesn't match the token")
	}
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func sampleValues() EmailValues {
	due := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	return EmailValues{
		Name: "Nimal Perera", PropertyName: "Kingfisher Villa", CheckIn: date(2026, 10, 22), CheckOut: date(2026, 10, 25),
		Status: StatusAwaitingPayment, PaymentDueAt: &due, StayLink: "https://kingfisher.staykey.direct/stay/1?t=x",
		BookingPage: "https://kingfisher.staykey.direct", TemplateText: "Hi Nimal, see you soon.",
		Link: "https://staykey.direct/verify-email/abc", Device: "Nimal's iPhone", At: due, FinalOn: due.AddDate(0, 0, 30),
		Receipt: &PaidPeriod{Plan: PlanVilla, Period: PeriodMonthly, Amount: 2_900_00, Currency: "LKR", PaymentID: "PLK-123", Start: due, End: due.AddDate(0, 1, 0)},
	}
}

func TestIsReservedAddress(t *testing.T) {
	tests := []struct {
		address string
		want    bool
	}{
		{"sofia.rossi@example.com", true},
		{"guest@mail.example.org", true},
		{"guest@villa.test", true},
		{"nimal@gmail.com", false},
		{"owner@staykey.direct", false},
		{"no-at-sign", true},
	}
	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			if got := IsReservedAddress(tt.address); got != tt.want {
				t.Errorf("IsReservedAddress(%q) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}
