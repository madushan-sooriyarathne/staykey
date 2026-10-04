// Package domain holds StayKey's core types and business rules, independent of
// HTTP and storage.
package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BookingType says whether guests book the whole place or individual rooms.
type BookingType string

const (
	BookingEntire BookingType = "entire"
	BookingRooms  BookingType = "rooms"
)

// Valid reports whether t is a known booking type.
func (t BookingType) Valid() bool { return t == BookingEntire || t == BookingRooms }

// supportedCurrencies are the currencies a property can charge in.
var supportedCurrencies = map[string]bool{"USD": true, "LKR": true, "EUR": true, "GBP": true}

// Property is a villa, guesthouse or similar listing with its own booking page.
type Property struct {
	ID            uuid.UUID
	AccountID     uuid.UUID
	Slug          string
	Name          string
	BookingType   BookingType
	Location      string
	Currency      string
	BaseRateMinor int64 // lowest nightly unit rate in minor units, for example cents
	CreatedAt     time.Time
}

// NewProperty is the input for creating a property. Setup carries the rest of the property's
// setup, as onboarding publishes it; without units, the property gets one unit at BaseRateMinor.
type NewProperty struct {
	Slug          string
	Name          string
	BookingType   BookingType
	Location      string
	Currency      string
	BaseRateMinor int64
	Setup         PropertyPatch
}

var (
	// ErrNotFound means the requested record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrSlugTaken means another property already uses the booking page address.
	ErrSlugTaken = errors.New("slug already taken")
)

// ValidationError describes invalid input on a single field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

// Normalize trims input and derives the slug from the name when none was given.
func (p *NewProperty) Normalize() {
	p.Name = strings.Join(strings.Fields(p.Name), " ")
	p.Location = strings.TrimSpace(p.Location)
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
	if p.Slug == "" {
		p.Slug = Slugify(p.Name)
	}
	p.Setup.Normalize()
}

// Validate checks the input against StayKey's rules. Call Normalize first.
func (p NewProperty) Validate() error {
	switch {
	case len(p.Name) < 2 || len(p.Name) > 80:
		return &ValidationError{"name", "must be between 2 and 80 characters"}
	case !p.BookingType.Valid():
		return &ValidationError{"bookingType", `must be "entire" or "rooms"`}
	case !supportedCurrencies[p.Currency]:
		return &ValidationError{"currency", "must be one of USD, LKR, EUR or GBP"}
	case p.Setup.Units == nil && p.BaseRateMinor <= 0:
		return &ValidationError{"baseRate", "must be greater than zero"}
	case p.BaseRateMinor < 0 || p.BaseRateMinor > MaxMoneyMinor:
		return &ValidationError{"baseRate", "must be greater than zero"}
	case len(p.Location) > 160:
		return &ValidationError{"location", "must be at most 160 characters"}
	}
	if err := ValidateSlug(p.Slug); err != nil {
		return &ValidationError{"slug", err.Error()}
	}
	if p.Setup.Name != nil || p.Setup.BookingType != nil || p.Setup.Location != nil {
		return &ValidationError{"name", "send the name, booking type and location once, at the top level"}
	}
	return p.Setup.Validate()
}
