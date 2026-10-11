package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// InviteTTL is how long an invite link works.
const InviteTTL = 14 * 24 * time.Hour

var (
	// ErrOwnerMembership means an edit was aimed at an owner, whose role and access never change
	// through the team routes.
	ErrOwnerMembership = errors.New("the owner's membership can't be changed")
	// ErrAlreadyMember means the invited number already belongs to the account.
	ErrAlreadyMember = errors.New("already on the team")
)

// NewInviteToken returns a random token for an invite link and the hash stored in its place.
func NewInviteToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("invite token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashInviteToken(token), nil
}

// HashInviteToken is how a token from a link is looked up.
func HashInviteToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// TeamMember is a person in the account, as the team list shows them.
type TeamMember struct {
	MembershipID uuid.UUID
	UserID       uuid.UUID
	Name         string
	Phone        string
	Role         Role
	// PropertyIDs limits the member to some properties. Empty means every property.
	PropertyIDs []uuid.UUID
	JoinedAt    time.Time
}

// Invite is an invitation that hasn't been accepted, revoked or let expire.
type Invite struct {
	ID          uuid.UUID
	Phone       string
	Name        string
	Role        Role
	PropertyIDs []uuid.UUID
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// InvitePreview is what the join screen shows before the person signs in.
type InvitePreview struct {
	AccountName   string
	InvitedByName string
	Role          Role
	PropertyNames []string
	ExpiresAt     time.Time
}

// NewInvite is an owner's request to invite someone.
type NewInvite struct {
	Phone       string
	Name        string
	Role        Role
	PropertyIDs []uuid.UUID
}

// Normalize tidies the invite's phone and name. An unreadable phone is left for Validate.
func (in *NewInvite) Normalize() {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	if phone, err := NormalizePhone(in.Phone); err == nil {
		in.Phone = phone
	}
}

// Validate checks the invite. Call Normalize first.
func (in NewInvite) Validate() error {
	if _, err := NormalizePhone(in.Phone); err != nil {
		return &ValidationError{"phone", err.Error()}
	}
	if len(in.Name) > 80 {
		return &ValidationError{"name", "must be at most 80 characters"}
	}
	return validateMemberRole(in.Role)
}

// MemberPatch changes a member's role or the properties they can see. Nil fields stay as they
// are; an empty PropertyIDs means every property.
type MemberPatch struct {
	Role        *Role
	PropertyIDs *[]uuid.UUID
}

// Validate checks the patch.
func (p MemberPatch) Validate() error {
	if p.Role == nil {
		return nil
	}
	return validateMemberRole(*p.Role)
}

// The team routes hand out manager and caretaker roles only; owners come from creating an account.
func validateMemberRole(r Role) error {
	if r != RoleManager && r != RoleCaretaker {
		return &ValidationError{"role", `must be "manager" or "caretaker"`}
	}
	return nil
}
