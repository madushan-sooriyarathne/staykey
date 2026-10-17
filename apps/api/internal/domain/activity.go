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
	ActivityBilling      ActivityKind = "billing"
)

// ActivityKinds lists every kind.
var ActivityKinds = []ActivityKind{
	ActivityRequest, ActivityBooking, ActivitySlip, ActivityPayment, ActivityCancellation, ActivitySync, ActivityImport, ActivityBilling,
}

// HiddenFromCaretakers are the kinds about money, which people without PermPrices never see.
var HiddenFromCaretakers = []ActivityKind{ActivityPayment, ActivitySlip}

// VisibleTo reports whether a role sees activity of this kind. Billing is for owners only.
func (k ActivityKind) VisibleTo(r Role) bool {
	if k == ActivityBilling {
		return r.Can(PermBilling)
	}
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
	// Billing alerts follow the booking preference, since a paused page stops bookings.
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

// DirectBooking is a stay a guest booked on the booking page or widget.
func DirectBooking(b Booking) Activity {
	if b.Status == StatusRequested {
		return BookingAdded(b, "")
	}
	pays := "pays by bank transfer"
	if b.PaymentMethod == "property" {
		pays = "pays at the property"
	}
	return stayActivity(b, ActivityBooking, "New direct booking from "+b.Guest.Name,
		FormatRange(b.CheckIn, b.CheckOut)+", "+pays)
}

// SlipUploaded is a bank slip a guest sent from their stay page.
func SlipUploaded(b Booking) Activity {
	return stayActivity(b, ActivitySlip, "Bank slip from "+b.Guest.Name, b.Ref+", waiting for a check")
}

// StayImported is a stay that arrived through an OTA calendar.
func StayImported(b Booking) Activity {
	return stayActivity(b, ActivityImport, ChannelName(string(b.Source))+" stay imported", FormatRange(b.CheckIn, b.CheckOut))
}

// ImportedStayCancelled is an imported stay that left its OTA calendar before check-out.
func ImportedStayCancelled(b Booking) Activity {
	channel := ChannelName(string(b.Source))
	return stayActivity(b, ActivityImport, channel+" stay cancelled",
		FormatRange(b.CheckIn, b.CheckOut)+", removed from the "+channel+" calendar")
}

// FeedConnected is the first sync of an OTA calendar, which imports quietly and says so once.
func FeedConnected(propertyID uuid.UUID, channel string, stays int) Activity {
	imported := "no upcoming stays"
	switch {
	case stays == 1:
		imported = "1 stay imported"
	case stays > 1:
		imported = fmt.Sprintf("%d stays imported", stays)
	}
	return Activity{PropertyID: &propertyID, Kind: ActivityImport, Title: ChannelName(channel) + " calendar connected", Subtitle: imported}
}

// ImportClash is an imported stay whose nights another stay or a block already holds. other is
// that stay's ref, or empty for a block.
func ImportClash(b Booking, other string) Activity {
	holder := "a block"
	if other != "" {
		holder = other
	}
	return stayActivity(b, ActivitySync, fmt.Sprintf("%s stay overlaps %s", ChannelName(string(b.Source)), holder),
		FormatRange(b.CheckIn, b.CheckOut)+", check both calendars")
}

// FeedFailing is an OTA calendar that couldn't be read several times in a row.
func FeedFailing(propertyID uuid.UUID, channel, propertyName string) Activity {
	return Activity{PropertyID: &propertyID, Kind: ActivitySync, Title: ChannelName(channel) + " sync failed",
		Subtitle: propertyName + ", the calendar link isn't working"}
}

// billingLocation is where billing dates are read. Accounts have no time zone of their own,
// and every owner is in Sri Lanka for now.
var billingLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Colombo")
	if err != nil {
		return time.FixedZone("Asia/Colombo", 5*3600+1800)
	}
	return loc
}()

func billingDate(t time.Time) string { return t.In(billingLocation).Format("2 Jan") }

// BillingChanged is the line an owner sees when the account moves into b's status, and false when
// the change needs no line (a plan refunded during the free period, say). Amounts never appear.
func BillingChanged(b Billing, sub *Subscription) (Activity, bool) {
	line := func(title, subtitle string) (Activity, bool) {
		return Activity{Kind: ActivityBilling, Title: title, Subtitle: subtitle}, true
	}
	switch b.Status {
	case StatusActive:
		name := "Your"
		if sub != nil {
			if p, ok := FindPlan(sub.Plan); ok {
				name = p.Name
			}
		}
		return line(name+" plan active", "Paid until "+billingDate(b.Until))
	case StatusGrace:
		switch b.Reason {
		case GraceFirstBooking:
			return line("Your first direct booking is in", "Free until "+billingDate(b.Until)+", then choose a plan")
		case GracePlanEnded:
			return line("Your plan has ended", "Renew by "+billingDate(b.Until)+" to keep your booking page open")
		default:
			return line("Your free period has ended", "Choose a plan by "+billingDate(b.Until)+" to keep your booking page open")
		}
	case StatusLapsed:
		return line("Your booking page is paused", "Guests are asked to message you on WhatsApp until you choose a plan")
	}
	return Activity{}, false
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
