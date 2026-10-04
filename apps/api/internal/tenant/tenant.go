// Package tenant resolves which account a request acts on and what the caller may do there.
//
// Owner routes send X-Account-Id. The middleware checks it against the caller's memberships and
// puts a Tenant in the request context; every store query for owner data takes that Tenant, so
// data from another account is never in reach.
package tenant

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

// Tenant is the account a request acts on, seen through the caller's membership.
type Tenant struct {
	AccountID    uuid.UUID
	UserID       uuid.UUID
	MembershipID uuid.UUID
	Role         domain.Role
	// PropertyIDs limits the caller to some properties. Empty means every property.
	PropertyIDs []uuid.UUID
}

// FromMembership builds the Tenant for a membership.
func FromMembership(m domain.Membership) Tenant {
	return Tenant{
		AccountID:    m.AccountID,
		UserID:       m.UserID,
		MembershipID: m.ID,
		Role:         m.Role,
		PropertyIDs:  m.PropertyIDs,
	}
}

// AllProperties reports whether the caller sees every property in the account.
func (t Tenant) AllProperties() bool { return len(t.PropertyIDs) == 0 }

// PropertyScope is the property filter for store queries: nil for every property, otherwise the
// properties the caller is limited to.
func (t Tenant) PropertyScope() []uuid.UUID {
	if t.AllProperties() {
		return nil
	}
	return t.PropertyIDs
}

// CanSeeProperty reports whether the caller may see the property.
func (t Tenant) CanSeeProperty(id uuid.UUID) bool {
	return t.AllProperties() || slices.Contains(t.PropertyIDs, id)
}

// Can reports whether the caller's role allows p.
func (t Tenant) Can(p domain.Permission) bool { return t.Role.Can(p) }

type ctxKey struct{}

// With returns ctx carrying t.
func With(ctx context.Context, t Tenant) context.Context { return context.WithValue(ctx, ctxKey{}, t) }

// From returns the Tenant the middleware resolved for this request.
func From(ctx context.Context) (Tenant, bool) {
	t, ok := ctx.Value(ctxKey{}).(Tenant)
	return t, ok
}

// Header names the account an owner route acts on.
const Header = "X-Account-Id"

var (
	// ErrAccountRequired means the request has no X-Account-Id header.
	ErrAccountRequired = errors.New("account header required")
	// ErrInvalidAccountID means X-Account-Id is not a UUID.
	ErrInvalidAccountID = errors.New("invalid account id")
	// ErrNoAccess means the account does not exist or the caller is not an active member. The two
	// are deliberately indistinguishable.
	ErrNoAccess = errors.New("no access to account")
)

// Memberships looks up a user's active membership in an account, returning domain.ErrNotFound
// when there is none. internal/store implements it.
type Memberships interface {
	Membership(ctx context.Context, userID, accountID uuid.UUID) (domain.Membership, error)
}

// Resolve turns the X-Account-Id header value into the caller's Tenant.
func Resolve(ctx context.Context, m Memberships, userID uuid.UUID, header string) (Tenant, error) {
	if header == "" {
		return Tenant{}, ErrAccountRequired
	}
	accountID, err := uuid.Parse(header)
	if err != nil {
		return Tenant{}, ErrInvalidAccountID
	}
	membership, err := m.Membership(ctx, userID, accountID)
	if errors.Is(err, domain.ErrNotFound) {
		return Tenant{}, ErrNoAccess
	}
	if err != nil {
		return Tenant{}, err
	}
	return FromMembership(membership), nil
}
