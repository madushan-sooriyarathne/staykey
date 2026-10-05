package domain

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		from, to BookingStatus
		want     error
	}{
		{"manager approves a request", RoleManager, StatusRequested, StatusAwaitingPayment, nil},
		{"owner declines a request", RoleOwner, StatusRequested, StatusDeclined, nil},
		{"caretaker can't approve", RoleCaretaker, StatusRequested, StatusAwaitingPayment, ErrForbidden},
		{"caretaker checks a guest in", RoleCaretaker, StatusConfirmed, StatusCheckedIn, nil},
		{"caretaker checks a guest out", RoleCaretaker, StatusCheckedIn, StatusCheckedOut, nil},
		{"no check-in before confirming", RoleOwner, StatusAwaitingPayment, StatusCheckedIn, ErrBadTransition},
		{"no way back from checked out", RoleOwner, StatusCheckedOut, StatusCheckedIn, ErrBadTransition},
		{"cancelling has its own route", RoleOwner, StatusConfirmed, StatusCancelled, ErrBadTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CanTransition(tt.role, tt.from, tt.to); !errors.Is(err, tt.want) {
				t.Errorf("CanTransition = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestLedgerUnits(t *testing.T) {
	house, room1, room2 := uuid.New(), uuid.New(), uuid.New()
	units := []Unit{{ID: house, LinkedUnitIDs: []uuid.UUID{room1, room2}}, {ID: room1}, {ID: room2}}
	tests := []struct {
		name string
		unit uuid.UUID
		want []uuid.UUID
	}{
		{"a whole house holds its rooms", house, []uuid.UUID{room1, room2}},
		{"a room holds only itself", room1, []uuid.UUID{room1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LedgerUnits(units, tt.unit); !slices.Equal(got, tt.want) {
				t.Errorf("LedgerUnits = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefPrefix(t *testing.T) {
	tests := []struct{ name, want string }{
		{"Kingfisher Villa", "KV"},
		{"ella   hideaway house", "EH"},
		{"Sunbird", "SX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RefPrefix(tt.name); got != tt.want {
				t.Errorf("RefPrefix = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBalance(t *testing.T) {
	tests := []struct {
		name string
		b    Booking
		want int64
	}{
		{"total less payments and refunds", Booking{Source: "phone", Status: StatusConfirmed, Total: 1000,
			Payments: []Payment{{Kind: "payment", Amount: 600}, {Kind: "refund", Amount: 100}}}, 500},
		{"never below zero", Booking{Source: "phone", Status: StatusConfirmed, Total: 100,
			Payments: []Payment{{Kind: "payment", Amount: 300}}}, 0},
		{"OTA stays are paid through the channel", Booking{Source: "airbnb", Status: StatusConfirmed, Total: 1000}, 0},
		{"cancelled stays owe nothing", Booking{Source: "phone", Status: StatusCancelled, Total: 1000}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.Balance(); got != tt.want {
				t.Errorf("Balance = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStayInputValidate(t *testing.T) {
	unit, extra := uuid.New(), uuid.New()
	p := PropertyDetail{Units: []Unit{{ID: unit}}, Extras: []Extra{{ID: extra}}}
	valid := func() StayInput {
		return StayInput{UnitID: unit, CheckIn: day("2026-08-01"), CheckOut: day("2026-08-03"), Adults: 2,
			Guest: Guest{Name: "Nimali Perera"}, Source: "whatsapp", Extras: []uuid.UUID{extra}}
	}
	tests := []struct {
		name  string
		edit  func(*StayInput)
		field string
	}{
		{"valid input", func(*StayInput) {}, ""},
		{"unit from another property", func(in *StayInput) { in.UnitID = uuid.New() }, "unitId"},
		{"check-out on check-in day", func(in *StayInput) { in.CheckOut = in.CheckIn }, "checkOut"},
		{"no adults", func(in *StayInput) { in.Adults = 0 }, "adults"},
		{"guest without a name", func(in *StayInput) { in.Guest.Name = "" }, "guest.name"},
		{"unknown source", func(in *StayInput) { in.Source = "fax" }, "source"},
		{"extra from another property", func(in *StayInput) { in.Extras = []uuid.UUID{uuid.New()} }, "extras"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid()
			tt.edit(&in)
			field := ""
			var verr *ValidationError
			if errors.As(in.Validate(p), &verr) {
				field = verr.Field
			}
			if field != tt.field {
				t.Errorf("invalid field = %q, want %q", field, tt.field)
			}
		})
	}
}

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}
