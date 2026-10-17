package pricing

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

func TestGuestStay(t *testing.T) {
	unit := uuid.New()
	noon := 12
	p := domain.PropertyDetail{
		TimeZone: "Asia/Colombo",
		Units:    []domain.Unit{{ID: unit, Name: "Villa", Sleeps: 4}},
		Rules:    domain.StayRules{MinNights: 2, MaxNights: 14, WindowMonths: 12, SameDayCutoff: &noon, ClosedArrival: []int{0}},
	}
	// 09:30 on Thursday 15 Oct 2026 in Colombo.
	now := time.Date(2026, 10, 15, 4, 0, 0, 0, time.UTC)
	closedFriday := Overrides{unit: {"2026-10-23": {ClosedToArrival: true}}}
	stay := func(in, out string, guests int) domain.StayInput {
		return domain.StayInput{UnitID: unit, CheckIn: day(in), CheckOut: day(out), Adults: guests}
	}
	tests := []struct {
		name  string
		in    domain.StayInput
		o     Overrides
		now   time.Time
		field string
	}{
		{"a stay within the rules", stay("2026-10-20", "2026-10-23", 2), nil, now, ""},
		{"in the past", stay("2026-10-14", "2026-10-17", 2), nil, now, "checkIn"},
		{"same day before the cutoff", stay("2026-10-15", "2026-10-17", 2), nil, now, ""},
		{"same day after the cutoff", stay("2026-10-15", "2026-10-17", 2), nil, now.Add(4 * time.Hour), "checkIn"},
		{"too short", stay("2026-10-20", "2026-10-21", 2), nil, now, "checkOut"},
		{"too long", stay("2026-10-20", "2026-11-20", 2), nil, now, "checkOut"},
		{"beyond the booking window", stay("2027-10-20", "2027-10-23", 2), nil, now, "checkOut"},
		{"arriving on a closed weekday", stay("2026-10-18", "2026-10-21", 2), nil, now, "checkIn"},
		{"arriving on a night closed to arrival", stay("2026-10-23", "2026-10-26", 2), closedFriday, now, "checkIn"},
		{"more guests than the unit sleeps", stay("2026-10-20", "2026-10-23", 5), nil, now, "adults"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GuestStay(p, tt.in, tt.o, tt.now)
			var v *domain.ValidationError
			field := ""
			if errors.As(err, &v) {
				field = v.Field
			}
			if field != tt.field || (tt.field == "" && err != nil) {
				t.Errorf("GuestStay() = %v, want an error on %q", err, tt.field)
			}
		})
	}
}

func TestPromoProblem(t *testing.T) {
	tests := []struct {
		name    string
		promo   domain.Promo
		nights  int
		wantErr bool
	}{
		{"usable", domain.Promo{From: "2026-10-01", To: "2026-12-31", Limit: 5, Used: 4, MinNights: 2}, 3, false},
		{"unlimited", domain.Promo{Used: 400}, 1, false},
		{"before it starts", domain.Promo{From: "2026-11-01"}, 3, true},
		{"after it ends", domain.Promo{To: "2026-10-01"}, 3, true},
		{"used up", domain.Promo{Limit: 5, Used: 5}, 3, true},
		{"stay too short", domain.Promo{MinNights: 4}, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := PromoProblem(tt.promo, tt.nights, day("2026-10-20")); (err != nil) != tt.wantErr {
				t.Errorf("PromoProblem() = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}
