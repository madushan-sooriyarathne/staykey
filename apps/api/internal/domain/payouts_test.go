package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNextPayoutDay(t *testing.T) {
	tests := []struct {
		name string
		day  time.Time
		want time.Time
	}{
		{"before the first payout", date(2026, 10, 21), date(2026, 11, 2)},
		{"on the first payout", date(2026, 11, 2), date(2026, 11, 2)},
		{"the day after a payout", date(2026, 11, 3), date(2026, 11, 16)},
		{"the Sunday before a payout", date(2026, 11, 15), date(2026, 11, 16)},
		{"across a month", date(2026, 11, 17), date(2026, 11, 30)},
		{"across a year", date(2026, 12, 29), date(2027, 1, 11)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextPayoutDay(tt.day); !got.Equal(tt.want) {
				t.Errorf("NextPayoutDay(%s) = %s, want %s", tt.day.Format(time.DateOnly), got.Format(time.DateOnly), tt.want.Format(time.DateOnly))
			}
		})
	}
}

func TestLatestPayoutDay(t *testing.T) {
	tests := []struct {
		name string
		day  time.Time
		want time.Time
	}{
		{"on a payout day", date(2026, 11, 16), date(2026, 11, 16)},
		{"the day after a payout", date(2026, 11, 17), date(2026, 11, 16)},
		{"the Sunday before the next", date(2026, 11, 29), date(2026, 11, 16)},
		{"across a year", date(2027, 1, 1), date(2026, 12, 28)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LatestPayoutDay(tt.day)
			if !ok || !got.Equal(tt.want) {
				t.Errorf("LatestPayoutDay(%s) = %s, %v; want %s", tt.day.Format(time.DateOnly), got.Format(time.DateOnly), ok, tt.want.Format(time.DateOnly))
			}
		})
	}
}

func TestThereIsNoPayoutDayBeforeTheFirst(t *testing.T) {
	if _, ok := LatestPayoutDay(date(2026, 11, 1)); ok {
		t.Error("LatestPayoutDay found a payout day before the first one")
	}
}

func TestPayoutDaysAreMondays(t *testing.T) {
	if got := NextPayoutDay(date(2027, 6, 1)).Weekday(); got != time.Monday {
		t.Errorf("payout day falls on %s", got)
	}
}

func TestAPayoutCoversUpToTheSundayBefore(t *testing.T) {
	if got := PeriodEnd(date(2026, 11, 16)); !got.Equal(date(2026, 11, 15)) {
		t.Errorf("PeriodEnd = %s, want 2026-11-15", got.Format(time.DateOnly))
	}
}

func TestThePayoutDayIsCountedInSriLanka(t *testing.T) {
	lateSundayUTC := time.Date(2026, 11, 15, 20, 0, 0, 0, time.UTC) // 01:30 Monday in Colombo
	if got := PayoutDay(lateSundayUTC); !got.Equal(date(2026, 11, 16)) {
		t.Errorf("PayoutDay = %s, want 2026-11-16", got.Format(time.DateOnly))
	}
}

func TestMoneyForAStayIsAvailableTheDayAfterCheckIn(t *testing.T) {
	if got := AvailableOn(date(2026, 12, 31)); !got.Equal(date(2027, 1, 1)) {
		t.Errorf("AvailableOn = %s, want 2027-01-01", got.Format(time.DateOnly))
	}
}

func TestProcessingCharge(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		bps    int
		want   int64
	}{
		{"3% of a round amount", 10_000_000, 300, 300_000},
		{"rounds a part cent up", 10_001, 300, 301},
		{"one cent still pays a cent", 1, 300, 1},
		{"no charge when the rate is zero", 10_000, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProcessingCharge(tt.amount, tt.bps); got != tt.want {
				t.Errorf("ProcessingCharge(%d, %d) = %d, want %d", tt.amount, tt.bps, got, tt.want)
			}
		})
	}
}

func TestValidNIC(t *testing.T) {
	tests := []struct {
		nic  string
		want bool
	}{
		{"851234567V", true},
		{"851234567X", true},
		{"198512345678", true},
		{"85123456V", false},
		{"851234567A", false},
		{"19851234567", false},
		{"19851234567A", false},
	}
	for _, tt := range tests {
		t.Run(tt.nic, func(t *testing.T) {
			if got := ValidNIC(tt.nic); got != tt.want {
				t.Errorf("ValidNIC(%q) = %v, want %v", tt.nic, got, tt.want)
			}
		})
	}
}

func payoutInput() PayoutProfileInput {
	return PayoutProfileInput{
		LegalName: "  Nadeesha   Perera ", NIC: "851234567v", BankName: "Commercial Bank", Branch: "Galle",
		AccountName: "N Perera", AccountNumber: "8001 2345-67", AcceptTerms: PayoutTermsVersion,
	}
}

func TestPayoutDetailsAreNormalized(t *testing.T) {
	in := payoutInput()
	in.Normalize()
	if in.LegalName != "Nadeesha Perera" || in.NIC != "851234567V" || in.AccountNumber != "8001234567" {
		t.Errorf("normalized to %q, %q, %q", in.LegalName, in.NIC, in.AccountNumber)
	}
}

func TestValidPayoutDetailsPass(t *testing.T) {
	in := payoutInput()
	in.Normalize()
	if err := in.Validate(); err != nil {
		t.Errorf("Validate = %v", err)
	}
}

func TestPayoutDetailsCheckEachField(t *testing.T) {
	tests := []struct {
		field  string
		change func(*PayoutProfileInput)
	}{
		{"legalName", func(in *PayoutProfileInput) { in.LegalName = "N" }},
		{"nic", func(in *PayoutProfileInput) { in.NIC = "12345" }},
		{"bankName", func(in *PayoutProfileInput) { in.BankName = "" }},
		{"branch", func(in *PayoutProfileInput) { in.Branch = "" }},
		{"accountName", func(in *PayoutProfileInput) { in.AccountName = "" }},
		{"accountNumber", func(in *PayoutProfileInput) { in.AccountNumber = "12AB56" }},
		{"acceptTerms", func(in *PayoutProfileInput) { in.AcceptTerms = "2020-01" }},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			in := payoutInput()
			tt.change(&in)
			in.Normalize()
			var ve *ValidationError
			if err := in.Validate(); !errors.As(err, &ve) || ve.Field != tt.field {
				t.Errorf("Validate = %v, want an error on %s", err, tt.field)
			}
		})
	}
}

func TestABankChangeHoldsPayoutsFor72Hours(t *testing.T) {
	changed := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	p := PayoutProfile{BankChangedAt: changed}
	if got := p.HeldUntil(changed.Add(71 * time.Hour)); got == nil || !got.Equal(changed.Add(72*time.Hour)) {
		t.Errorf("HeldUntil = %v, want %s", got, changed.Add(72*time.Hour))
	}
}

func TestPayoutsAreNotHeldAfter72Hours(t *testing.T) {
	changed := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	p := PayoutProfile{BankChangedAt: changed}
	if got := p.HeldUntil(changed.Add(72 * time.Hour)); got != nil {
		t.Errorf("HeldUntil = %s, want nil", got)
	}
}
