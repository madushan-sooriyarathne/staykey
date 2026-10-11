package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ActivityKind groups activity the way the owner app shows it.
type ActivityKind string

const (
	ActivityRequest      ActivityKind = "request"
	ActivityBooking      ActivityKind = "booking"
	ActivitySlip         ActivityKind = "slip"
	ActivityPayment      ActivityKind = "payment"
	ActivityCancellation ActivityKind = "cancellation"
	ActivitySync         ActivityKind = "sync"
	ActivityImport       ActivityKind = "import"
)

// HiddenFromCaretakers are the kinds about money, which people without PermPrices never see.
var HiddenFromCaretakers = []ActivityKind{ActivityPayment, ActivitySlip}

// VisibleTo reports whether a role sees activity of this kind.
func (k ActivityKind) VisibleTo(r Role) bool {
	return r.Can(PermPrices) || !slices.Contains(HiddenFromCaretakers, k)
}

// Activity is one thing that happened in an account. Its text is written once, when it happens,
// in the third person, so everyone in the account reads the same line.
type Activity struct {
	ID          uuid.UUID
	PropertyID  *uuid.UUID
	BookingID   *uuid.UUID
	Kind        ActivityKind
	Title       string
	Subtitle    string
	ActorUserID *uuid.UUID
	CreatedAt   time.Time
	// Read is set when activity is listed for one person.
	Read bool
}

// NotificationEvent is what a person can choose to be alerted about.
type NotificationEvent string

const (
	EventBooking      NotificationEvent = "booking"
	EventRequest      NotificationEvent = "request"
	EventPayment      NotificationEvent = "payment"
	EventCancellation NotificationEvent = "cancellation"
	EventSync         NotificationEvent = "sync"
	EventSummary      NotificationEvent = "summary"
)

// NotificationEvents lists every event in the order the app shows them.
var NotificationEvents = []NotificationEvent{EventBooking, EventRequest, EventPayment, EventCancellation, EventSync, EventSummary}

// Valid reports whether e is a known event.
func (e NotificationEvent) Valid() bool { return slices.Contains(NotificationEvents, e) }

// EventFor is the alert preference that covers an activity kind.
func EventFor(k ActivityKind) NotificationEvent {
	switch k {
	case ActivityRequest:
		return EventRequest
	case ActivityPayment, ActivitySlip:
		return EventPayment
	case ActivityCancellation:
		return EventCancellation
	case ActivitySync, ActivityImport:
		return EventSync
	default:
		return EventBooking
	}
}

// Who did something, by first name. Actor is the person's full name; empty means a team member
// who hasn't added their name yet.
func firstName(actor string) string {
	if f := strings.Fields(actor); len(f) > 0 {
		return f[0]
	}
	return "A team member"
}

func stayActivity(b Booking, kind ActivityKind, title, subtitle string) Activity {
	propertyID, bookingID := b.PropertyID, b.ID
	return Activity{PropertyID: &propertyID, BookingID: &bookingID, Kind: kind, Title: title, Subtitle: subtitle}
}

// BookingAdded is a stay someone on the team entered, or a request that came in.
func BookingAdded(b Booking, actor string) Activity {
	if b.Status == StatusRequested {
		return stayActivity(b, ActivityRequest, "Request from "+b.Guest.Name,
			fmt.Sprintf("%s, %s", FormatRange(b.CheckIn, b.CheckOut), guests(b.Adults+b.Children)))
	}
	return stayActivity(b, ActivityBooking, fmt.Sprintf("%s added %s", firstName(actor), b.Guest.Name),
		fmt.Sprintf("%s, %s", FormatRange(b.CheckIn, b.CheckOut), b.Ref))
}

// RequestAnswered is a request approved or declined by someone on the team.
func RequestAnswered(b Booking, approved bool, actor string) Activity {
	if !approved {
		return stayActivity(b, ActivityRequest, fmt.Sprintf("%s declined %s", firstName(actor), b.Guest.Name),
			FormatRange(b.CheckIn, b.CheckOut)+", dates are open again")
	}
	next := "waiting for payment"
	if b.Status == StatusConfirmed {
		next = "confirmed"
	}
	return stayActivity(b, ActivityRequest, fmt.Sprintf("%s approved %s", firstName(actor), b.Guest.Name),
		FormatRange(b.CheckIn, b.CheckOut)+", "+next)
}

// RequestExpired is a request nobody answered in time.
func RequestExpired(b Booking) Activity {
	return stayActivity(b, ActivityRequest, "Request from "+b.Guest.Name+" expired",
		FormatRange(b.CheckIn, b.CheckOut)+", dates are open again")
}

// StayCancelled is a stay cancelled by someone on the team.
func StayCancelled(b Booking, actor string) Activity {
	return stayActivity(b, ActivityCancellation, b.Guest.Name+"'s booking cancelled",
		fmt.Sprintf("%s, cancelled by %s", FormatRange(b.CheckIn, b.CheckOut), firstName(actor)))
}

// StayUnpaid is a stay cancelled because nothing was paid by its deadline.
func StayUnpaid(b Booking) Activity {
	return stayActivity(b, ActivityCancellation, b.Guest.Name+"'s booking cancelled",
		FormatRange(b.CheckIn, b.CheckOut)+", not paid in time")
}

// PaymentRecorded is money someone on the team recorded against a stay.
func PaymentRecorded(b Booking, actor string) Activity {
	return stayActivity(b, ActivityPayment, "Payment from "+b.Guest.Name,
		fmt.Sprintf("%s, recorded by %s", b.Ref, firstName(actor)))
}

// SlipRejected is a bank slip someone on the team turned down.
func SlipRejected(b Booking, actor string) Activity {
	return stayActivity(b, ActivitySlip, "Bank slip from "+b.Guest.Name+" rejected",
		fmt.Sprintf("%s, rejected by %s", b.Ref, firstName(actor)))
}

func guests(n int) string {
	if n == 1 {
		return "1 guest"
	}
	return fmt.Sprintf("%d guests", n)
}

// FormatRange writes a stay's dates the way the owner app does: "12 to 15 Oct", or
// "30 Oct to 2 Nov" across months.
func FormatRange(from, to time.Time) string {
	if from.Month() == to.Month() && from.Year() == to.Year() {
		return fmt.Sprintf("%d to %d %s", from.Day(), to.Day(), to.Format("Jan"))
	}
	return fmt.Sprintf("%d %s to %d %s", from.Day(), from.Format("Jan"), to.Day(), to.Format("Jan"))
}
