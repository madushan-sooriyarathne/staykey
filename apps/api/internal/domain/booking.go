package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BookingStatus is where a stay is in its life. The owner app uses the same names.
type BookingStatus string

const (
	StatusRequested       BookingStatus = "requested"
	StatusAwaitingPayment BookingStatus = "awaiting_payment"
	StatusConfirmed       BookingStatus = "confirmed"
	StatusCheckedIn       BookingStatus = "checked_in"
	StatusCheckedOut      BookingStatus = "checked_out"
	StatusCancelled       BookingStatus = "cancelled"
	StatusDeclined        BookingStatus = "declined"
)

// Holds reports whether a booking in this status keeps its nights on the ledger.
func (s BookingStatus) Holds() bool { return s != StatusCancelled && s != StatusDeclined }

// Source is where a booking came from.
type Source string

var sources = []Source{"page", "widget", "whatsapp", "phone", "walkin", "other", "airbnb", "booking", "agoda", "expedia"}

// Valid reports whether s is a known source.
func (s Source) Valid() bool { return slices.Contains(sources, s) }

// OTA reports whether the stay came in through a channel's calendar.
func (s Source) OTA() bool { return s == "airbnb" || s == "booking" || s == "agoda" || s == "expedia" }

// Guest is the person a booking is for.
type Guest struct {
	ID      uuid.UUID
	Name    string
	Phone   string
	Email   string
	Country string
}

// PriceLine is one row of a price breakdown. Kind is empty, discount, charge or extra.
type PriceLine struct {
	Label  string
	Amount int64
	Kind   string
}

// BookingExtra is an extra as it was priced when the stay was booked.
type BookingExtra struct {
	ExtraID uuid.UUID
	Name    string
	Amount  int64
}

// Payment is money received for a booking, or a refund paid back.
type Payment struct {
	ID         uuid.UUID
	Kind       string // payment or refund
	Method     string // bank, cash or card
	Amount     int64
	Note       string
	ReceivedAt time.Time
	SlipID     *uuid.UUID
}

// Slip is a bank transfer receipt a guest uploaded.
type Slip struct {
	ID         uuid.UUID
	FileKey    string
	Amount     int64
	Status     string // pending, accepted or rejected
	UploadedAt time.Time
}

// Booking is a stay on one unit. Dates are local calendar days at UTC midnight; check-out is
// the morning after the last night.
type Booking struct {
	ID               uuid.UUID
	PropertyID       uuid.UUID
	UnitID           uuid.UUID
	Ref              string
	Source           Source
	Status           BookingStatus
	Guest            Guest
	Adults           int
	Children         int
	CheckIn          time.Time
	CheckOut         time.Time
	Currency         string
	Total            int64
	Lines            []PriceLine
	Extras           []BookingExtra
	Payments         []Payment
	Slips            []Slip
	GuestNote        string
	OwnerNote        string
	RequestExpiresAt *time.Time
	PaymentDueAt     *time.Time
	CancelReason     string
	CancelledAt      *time.Time
	Version          int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Paid is what the guest has paid so far, less refunds.
func (b Booking) Paid() int64 {
	var n int64
	for _, p := range b.Payments {
		if p.Kind == "refund" {
			n -= p.Amount
		} else {
			n += p.Amount
		}
	}
	return n
}

// Balance is what the guest still owes. OTA stays are paid through the channel.
func (b Booking) Balance() int64 {
	if b.Source.OTA() || !b.Status.Holds() {
		return 0
	}
	return max(0, b.Total-b.Paid())
}

// StayInput is what the owner enters for a new or changed booking.
type StayInput struct {
	UnitID   uuid.UUID
	CheckIn  time.Time
	CheckOut time.Time
	Adults   int
	Children int
	Guest    Guest
	Source   Source
	Extras   []uuid.UUID
	// CustomTotal replaces the quote with a price agreed with the guest. Nil uses the quote.
	CustomTotal *int64
	OwnerNote   string
}

// Normalize trims the input's text.
func (in *StayInput) Normalize() {
	in.Guest.Name = strings.Join(strings.Fields(in.Guest.Name), " ")
	in.Guest.Phone = strings.Join(strings.Fields(in.Guest.Phone), "")
	in.Guest.Email = strings.ToLower(strings.TrimSpace(in.Guest.Email))
	in.Guest.Country = strings.ToUpper(strings.TrimSpace(in.Guest.Country))
	in.OwnerNote = strings.TrimSpace(in.OwnerNote)
}

// Validate checks the input against the property it books. Call Normalize first.
func (in StayInput) Validate(p PropertyDetail) error {
	if !slices.ContainsFunc(p.Units, func(u Unit) bool { return u.ID == in.UnitID }) {
		return &ValidationError{"unitId", "must be a unit of this property"}
	}
	nights := Nights(in.CheckIn, in.CheckOut)
	switch {
	case nights < 1:
		return &ValidationError{"checkOut", "must be after check-in"}
	case nights > 365:
		return &ValidationError{"checkOut", "must be within a year of check-in"}
	case in.Adults < 1 || in.Adults > 30:
		return &ValidationError{"adults", "must be between 1 and 30"}
	case in.Children < 0 || in.Children > 30:
		return &ValidationError{"children", "must be between 0 and 30"}
	case len(in.Guest.Name) < 2 || len(in.Guest.Name) > 120:
		return &ValidationError{"guest.name", "must be between 2 and 120 characters"}
	case len(in.Guest.Phone) > 32:
		return &ValidationError{"guest.phone", "must be at most 32 characters"}
	case in.Guest.Email != "" && (len(in.Guest.Email) > 254 || !strings.Contains(in.Guest.Email, "@")):
		return &ValidationError{"guest.email", "must be a valid email address"}
	case len(in.Guest.Country) > 2:
		return &ValidationError{"guest.country", "must be a two-letter country code"}
	case !in.Source.Valid():
		return &ValidationError{"source", "is not a known source"}
	case in.CustomTotal != nil && *in.CustomTotal < 0:
		return &ValidationError{"customTotal", "must not be negative"}
	case len(in.OwnerNote) > 2000:
		return &ValidationError{"ownerNote", "must be at most 2000 characters"}
	}
	for _, id := range in.Extras {
		if !slices.ContainsFunc(p.Extras, func(e Extra) bool { return e.ID == id }) {
			return &ValidationError{"extras", "must be extras of this property"}
		}
	}
	return nil
}

// Transition rules: who may move a booking from one status to another. Cancelling has its own
// route, since it refunds; expiry and unpaid cancellation are done by jobs, not people.
var transitions = map[BookingStatus]map[BookingStatus]Permission{
	StatusRequested:       {StatusAwaitingPayment: PermManage, StatusConfirmed: PermManage, StatusDeclined: PermManage},
	StatusAwaitingPayment: {StatusConfirmed: PermManage},
	StatusConfirmed:       {StatusCheckedIn: ""},
	StatusCheckedIn:       {StatusCheckedOut: ""},
}

var (
	// ErrBadTransition means a booking can't move from its status to the one asked for.
	ErrBadTransition = errors.New("booking can't move to that status")
	// ErrStale means the booking changed since the caller read it.
	ErrStale = errors.New("booking changed since it was read")
	// ErrKeyReused means an Idempotency-Key came back with a different request.
	ErrKeyReused = errors.New("idempotency key reused for another request")
	// ErrForbidden means the caller's role doesn't allow the action.
	ErrForbidden = errors.New("forbidden")
)

// CanTransition reports whether role may move a booking from one status to another. Checking
// guests in and out is open to every role, caretakers included.
func CanTransition(role Role, from, to BookingStatus) error {
	perm, ok := transitions[from][to]
	if !ok {
		return fmt.Errorf("%w: %s to %s", ErrBadTransition, from, to)
	}
	if perm != "" && !role.Can(perm) {
		return ErrForbidden
	}
	return nil
}

// Cancellable reports whether a booking can still be cancelled.
func (s BookingStatus) Cancellable() bool {
	return s == StatusRequested || s == StatusAwaitingPayment || s == StatusConfirmed || s == StatusCheckedIn
}

// LedgerUnits are the units whose nights a stay on unitID holds. A whole-house unit holds
// every room it links, so it clashes with each room; a room holds only itself, so two rooms
// sold on the same night don't clash with each other.
func LedgerUnits(units []Unit, unitID uuid.UUID) []uuid.UUID {
	for _, u := range units {
		if u.ID == unitID && len(u.LinkedUnitIDs) > 0 {
			return u.LinkedUnitIDs
		}
	}
	return []uuid.UUID{unitID}
}

// Nights counts the nights between two calendar days.
func Nights(from, to time.Time) int { return int(to.Sub(from).Hours() / 24) }

// RefPrefix is the two-letter start of a property's booking refs, from its initials: "KV" for
// Kingfisher Villa.
func RefPrefix(name string) string {
	var b strings.Builder
	for _, w := range strings.Fields(name) {
		if b.Len() == 2 {
			break
		}
		r := []rune(strings.ToUpper(w))[0]
		if r < 128 {
			b.WriteRune(r)
		}
	}
	return (b.String() + "XX")[:2]
}

// ConflictError means another stay or a block already holds a night.
type ConflictError struct {
	Night     time.Time
	BookingID *uuid.UUID
	Ref       string
	GuestName string
	BlockID   *uuid.UUID
	From      time.Time
	To        time.Time
}

func (e *ConflictError) Error() string {
	return "dates taken from " + e.From.Format(time.DateOnly) + " to " + e.To.Format(time.DateOnly)
}

// Pricing is a stay's price as it will be stored: from the quote, or agreed with the guest.
type Pricing struct {
	Lines   []PriceLine
	Extras  []BookingExtra
	Total   int64
	PromoID *uuid.UUID
}

// NewBooking is everything the store needs to add a stay. LedgerUnits are the units whose
// nights it holds.
type NewBooking struct {
	PropertyID   uuid.UUID
	Stay         StayInput
	Status       BookingStatus
	Currency     string
	RefPrefix    string
	Price        Pricing
	LedgerUnits  []uuid.UUID
	FirstPayment *Payment
	// RequestExpiresAt is when a request the owner hasn't answered is declined.
	RequestExpiresAt *time.Time
}

// BookingChange is an edit to a stay the owner made, priced and checked by the caller.
type BookingChange struct {
	Version     int
	Stay        StayInput
	Price       Pricing
	LedgerUnits []uuid.UUID
}

// Block closes nights on some units of a property. To is exclusive, like a check-out date.
type Block struct {
	ID         uuid.UUID
	PropertyID uuid.UUID
	UnitIDs    []uuid.UUID
	From       time.Time
	To         time.Time
	Reason     string // maintenance, owner or other
	Note       string
}

// Validate checks a new block against its property.
func (b Block) Validate(p PropertyDetail) error {
	for _, id := range b.UnitIDs {
		if !slices.ContainsFunc(p.Units, func(u Unit) bool { return u.ID == id }) {
			return &ValidationError{"unitIds", "must be units of this property"}
		}
	}
	switch n := Nights(b.From, b.To); {
	case len(b.UnitIDs) == 0:
		return &ValidationError{"unitIds", "must name at least one unit"}
	case n < 1 || n > 366:
		return &ValidationError{"to", "must be between 1 and 366 nights after from"}
	case b.Reason != "maintenance" && b.Reason != "owner" && b.Reason != "other":
		return &ValidationError{"reason", "must be maintenance, owner or other"}
	case len(b.Note) > 500:
		return &ValidationError{"note", "must be at most 500 characters"}
	}
	return nil
}

// RateOverride is a manual change to one unit's night. Zero price or minimum stay means none.
type RateOverride struct {
	UnitID          uuid.UUID
	Night           time.Time
	Price           int64
	MinNights       int
	ClosedToArrival bool
}

// BookingFilter narrows a list of stays to those overlapping [From, To). AfterCheckIn and
// AfterID, when set, continue after the stay with that check-in and id.
type BookingFilter struct {
	PropertyID   *uuid.UUID
	From, To     *time.Time
	AfterCheckIn *time.Time
	AfterID      *uuid.UUID
	Limit        int
}
