package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewID returns a time-ordered UUIDv7 for a new row. Every primary key is generated here rather
// than in the database.
func NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

// Role is what a person can do inside an account.
type Role string

const (
	RoleOwner     Role = "owner"
	RoleManager   Role = "manager"
	RoleCaretaker Role = "caretaker"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return r == RoleOwner || r == RoleManager || r == RoleCaretaker }

// Permission is an action a role may or may not take. It mirrors useCan in the owner app.
type Permission string

const (
	// PermPrices covers seeing and changing money: rates, totals, payments.
	PermPrices Permission = "prices"
	// PermSettings covers property settings.
	PermSettings Permission = "settings"
	// PermManage covers running bookings: approve, edit, cancel.
	PermManage Permission = "manage"
	// PermBilling covers the subscription and anything that changes the plan, such as adding a
	// property.
	PermBilling Permission = "billing"
	// PermTeam covers inviting and removing people.
	PermTeam Permission = "team"
)

// Can reports whether the role allows p. Owners can do everything, managers run the properties
// without billing or team, and caretakers see stays without prices.
func (r Role) Can(p Permission) bool {
	switch r {
	case RoleOwner:
		return true
	case RoleManager:
		return p != PermBilling && p != PermTeam
	default:
		return false
	}
}

// User is one person, identified by a phone number, who can belong to several accounts.
type User struct {
	ID        uuid.UUID
	Phone     string // E.164, for example +94771234567
	Name      string
	Email     string
	Language  string // "en" or "si"
	CreatedAt time.Time
}

// UserPatch changes some of a user's profile. Nil fields stay as they are.
type UserPatch struct {
	Name     *string
	Email    *string
	Language *string
}

// Normalize trims the patch's text fields.
func (p *UserPatch) Normalize() {
	if p.Name != nil {
		v := strings.Join(strings.Fields(*p.Name), " ")
		p.Name = &v
	}
	if p.Email != nil {
		v := strings.ToLower(strings.TrimSpace(*p.Email))
		p.Email = &v
	}
}

// Validate checks the patch. Call Normalize first.
func (p UserPatch) Validate() error {
	if p.Name != nil && len(*p.Name) > 80 {
		return &ValidationError{"name", "must be at most 80 characters"}
	}
	if p.Email != nil && *p.Email != "" && (len(*p.Email) > 254 || !strings.Contains(*p.Email, "@")) {
		return &ValidationError{"email", "must be a valid email address"}
	}
	if p.Language != nil && *p.Language != "en" && *p.Language != "si" {
		return &ValidationError{"language", `must be "en" or "si"`}
	}
	return nil
}

// AccountStatus tracks the free period and subscription.
type AccountStatus string

// Account is the tenant: the business that owns properties and pays for StayKey.
type Account struct {
	ID             uuid.UUID
	Name           string
	Status         AccountStatus
	TrialStartedAt time.Time
	CreatedAt      time.Time
}

// Membership is a user's place in an account.
type Membership struct {
	ID        uuid.UUID
	AccountID uuid.UUID
	UserID    uuid.UUID
	Role      Role
	// PropertyIDs limits the membership to some properties. Empty means every property.
	PropertyIDs []uuid.UUID
}

// AccountMembership is an account as one of its members sees it.
type AccountMembership struct {
	Account
	MembershipID uuid.UUID
	Role         Role
	PropertyIDs  []uuid.UUID
}

// NormalizeAccountName trims and validates an account name.
func NormalizeAccountName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || len(name) > 120 {
		return "", &ValidationError{"name", "must be between 1 and 120 characters"}
	}
	return name, nil
}

// ErrInvalidPhone means a phone number could not be read as an international number.
var ErrInvalidPhone = errors.New("enter the number with its country code, for example +94 77 123 4567")

// NormalizePhone turns user input such as "+94 77 123-4567" or "0094771234567" into E.164
// ("+94771234567"). Numbers must carry a country code.
func NormalizePhone(input string) (string, error) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(input) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.':
		default:
			return "", ErrInvalidPhone
		}
	}
	s := b.String()
	if strings.HasPrefix(s, "00") {
		s = "+" + s[2:]
	}
	if !strings.HasPrefix(s, "+") {
		return "", ErrInvalidPhone
	}
	digits := s[1:]
	if len(digits) < 7 || len(digits) > 15 || digits[0] == '0' {
		return "", ErrInvalidPhone
	}
	return s, nil
}
