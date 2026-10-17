package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func sampleStay(status BookingStatus) Booking {
	return Booking{
		ID: uuid.New(), PropertyID: uuid.New(), Ref: "KV-2041", Status: status,
		Guest:   Guest{Name: "Sam Lee"},
		Adults:  2,
		CheckIn: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), CheckOut: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
	}
}

func paysAtProperty() Booking {
	b := sampleStay(StatusAwaitingPayment)
	b.PaymentMethod = "property"
	return b
}

func fromAirbnb() Booking {
	b := sampleStay(StatusConfirmed)
	b.Source = "airbnb"
	return b
}

func TestActivityText(t *testing.T) {
	tests := []struct {
		name            string
		got             Activity
		kind            ActivityKind
		title, subtitle string
	}{
		{"owner adds a stay", BookingAdded(sampleStay(StatusConfirmed), "Nadeesha Perera"), ActivityBooking,
			"Nadeesha added Sam Lee", "12 to 15 Oct, KV-2041"},
		{"a request comes in", BookingAdded(sampleStay(StatusRequested), ""), ActivityRequest,
			"Request from Sam Lee", "12 to 15 Oct, 2 guests"},
		{"request approved", RequestAnswered(sampleStay(StatusAwaitingPayment), true, "Ruwan Silva"), ActivityRequest,
			"Ruwan approved Sam Lee", "12 to 15 Oct, waiting for payment"},
		{"request declined by someone without a name", RequestAnswered(sampleStay(StatusDeclined), false, ""), ActivityRequest,
			"A team member declined Sam Lee", "12 to 15 Oct, dates are open again"},
		{"request expired", RequestExpired(sampleStay(StatusDeclined)), ActivityRequest,
			"Request from Sam Lee expired", "12 to 15 Oct, dates are open again"},
		{"stay cancelled", StayCancelled(sampleStay(StatusCancelled), "Nadeesha"), ActivityCancellation,
			"Sam Lee's booking cancelled", "12 to 15 Oct, cancelled by Nadeesha"},
		{"stay unpaid", StayUnpaid(sampleStay(StatusCancelled)), ActivityCancellation,
			"Sam Lee's booking cancelled", "12 to 15 Oct, not paid in time"},
		{"payment recorded", PaymentRecorded(sampleStay(StatusConfirmed), "Nadeesha"), ActivityPayment,
			"Payment from Sam Lee", "KV-2041, recorded by Nadeesha"},
		{"direct booking paying by bank", DirectBooking(sampleStay(StatusAwaitingPayment)), ActivityBooking,
			"New direct booking from Sam Lee", "12 to 15 Oct, pays by bank transfer"},
		{"direct booking paying at the property", DirectBooking(paysAtProperty()), ActivityBooking,
			"New direct booking from Sam Lee", "12 to 15 Oct, pays at the property"},
		{"direct request", DirectBooking(sampleStay(StatusRequested)), ActivityRequest,
			"Request from Sam Lee", "12 to 15 Oct, 2 guests"},
		{"slip uploaded", SlipUploaded(sampleStay(StatusAwaitingPayment)), ActivitySlip,
			"Bank slip from Sam Lee", "KV-2041, waiting for a check"},
		{"stay imported", StayImported(fromAirbnb()), ActivityImport,
			"Airbnb stay imported", "12 to 15 Oct"},
		{"imported stay cancelled", ImportedStayCancelled(fromAirbnb()), ActivityImport,
			"Airbnb stay cancelled", "12 to 15 Oct, removed from the Airbnb calendar"},
		{"feed connected", FeedConnected(uuid.New(), "booking", 12), ActivityImport,
			"Booking.com calendar connected", "12 stays imported"},
		{"empty feed connected", FeedConnected(uuid.New(), "airbnb", 0), ActivityImport,
			"Airbnb calendar connected", "no upcoming stays"},
		{"import clash with a stay", ImportClash(fromAirbnb(), "KV-2041"), ActivitySync,
			"Airbnb stay overlaps KV-2041", "12 to 15 Oct, check both calendars"},
		{"import clash with a block", ImportClash(fromAirbnb(), ""), ActivitySync,
			"Airbnb stay overlaps a block", "12 to 15 Oct, check both calendars"},
		{"feed failing", FeedFailing(uuid.New(), "agoda", "Kingfisher Villa"), ActivitySync,
			"Agoda sync failed", "Kingfisher Villa, the calendar link isn't working"},
		{"slip rejected", SlipRejected(sampleStay(StatusAwaitingPayment), "Nadeesha"), ActivitySlip,
			"Bank slip from Sam Lee rejected", "KV-2041, rejected by Nadeesha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got.Kind != tt.kind || tt.got.Title != tt.title || tt.got.Subtitle != tt.subtitle {
				t.Errorf("got %s %q %q, want %s %q %q", tt.got.Kind, tt.got.Title, tt.got.Subtitle, tt.kind, tt.title, tt.subtitle)
			}
		})
	}
}

func TestFormatRangeAcrossMonths(t *testing.T) {
	got := FormatRange(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))
	if got != "30 Oct to 2 Nov" {
		t.Errorf("FormatRange = %q", got)
	}
}

func TestEventFor(t *testing.T) {
	tests := []struct {
		kind ActivityKind
		want NotificationEvent
	}{
		{ActivityBooking, EventBooking},
		{ActivityRequest, EventRequest},
		{ActivityPayment, EventPayment},
		{ActivitySlip, EventPayment},
		{ActivityCancellation, EventCancellation},
		{ActivitySync, EventSync},
		{ActivityImport, EventSync},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := EventFor(tt.kind); got != tt.want {
				t.Errorf("EventFor(%s) = %s, want %s", tt.kind, got, tt.want)
			}
		})
	}
}

func TestActivityVisibleTo(t *testing.T) {
	tests := []struct {
		name string
		kind ActivityKind
		role Role
		want bool
	}{
		{"manager sees payments", ActivityPayment, RoleManager, true},
		{"caretaker doesn't see payments", ActivityPayment, RoleCaretaker, false},
		{"caretaker doesn't see slips", ActivitySlip, RoleCaretaker, false},
		{"caretaker sees bookings", ActivityBooking, RoleCaretaker, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.VisibleTo(tt.role); got != tt.want {
				t.Errorf("VisibleTo = %v, want %v", got, tt.want)
			}
		})
	}
}
