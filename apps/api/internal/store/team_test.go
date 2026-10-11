package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/tenant"
)

const caretakerPhone = "+94715550199"

// invite sends an invite for the house to caretakerPhone and returns its token.
func (h house) invite(t *testing.T, now time.Time) string {
	t.Helper()
	token, hash, err := domain.NewInviteToken()
	if err != nil {
		t.Fatal(err)
	}
	in := domain.NewInvite{Phone: caretakerPhone, Role: domain.RoleCaretaker, PropertyIDs: []uuid.UUID{h.p.ID}}
	if _, err := h.s.InviteMember(context.Background(), h.t, in, hash, now); err != nil {
		t.Fatal(err)
	}
	return token
}

func (h house) user(t *testing.T, phone string) domain.User {
	t.Helper()
	u, _, err := h.s.UpsertUserByPhone(context.Background(), phone)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestAcceptedInviteLimitsTheMemberToItsProperty(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	now := time.Now()
	token := h.invite(t, now)
	caretaker := h.user(t, caretakerPhone)

	acc, err := h.s.AcceptInvite(context.Background(), caretaker.ID, domain.HashInviteToken(token), now)
	if err != nil || acc.Role != domain.RoleCaretaker || len(acc.PropertyIDs) != 1 || acc.PropertyIDs[0] != h.p.ID {
		t.Errorf("accepted = %+v (err %v), want a caretaker limited to the house", acc, err)
	}
}

func TestInviteForAnotherNumberIsNotFound(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	now := time.Now()
	token := h.invite(t, now)
	stranger := h.user(t, "+94779999999")

	_, err := h.s.AcceptInvite(context.Background(), stranger.ID, domain.HashInviteToken(token), now)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("accept with another number: %v, want not found", err)
	}
}

func TestExpiredInviteIsNotFound(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	now := time.Now()
	token := h.invite(t, now)
	caretaker := h.user(t, caretakerPhone)

	_, err := h.s.AcceptInvite(context.Background(), caretaker.ID, domain.HashInviteToken(token), now.Add(domain.InviteTTL+time.Minute))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("accept after expiry: %v, want not found", err)
	}
}

func TestNewInviteReplacesTheOpenOne(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	now := time.Now()
	first := h.invite(t, now)
	h.invite(t, now)

	_, err := h.s.PreviewInvite(context.Background(), domain.HashInviteToken(first), now)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("preview of the replaced invite: %v, want not found", err)
	}
}

func TestAcceptingTwiceKeepsOneMembership(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	caretaker := h.user(t, caretakerPhone)
	if _, err := h.s.AcceptInvite(ctx, caretaker.ID, domain.HashInviteToken(h.invite(t, now)), now); err != nil {
		t.Fatal(err)
	}

	_, err := h.s.InviteMember(ctx, h.t, domain.NewInvite{Phone: caretakerPhone, Role: domain.RoleCaretaker}, []byte("another"), now)
	if !errors.Is(err, domain.ErrAlreadyMember) {
		t.Errorf("inviting a member again: %v, want ErrAlreadyMember", err)
	}
}

func TestOwnerCantBeRemoved(t *testing.T) {
	t.Parallel()
	h := newHouse(t)

	err := h.s.RemoveMember(context.Background(), h.t, h.t.MembershipID)
	if !errors.Is(err, domain.ErrOwnerMembership) {
		t.Errorf("removing the owner: %v, want ErrOwnerMembership", err)
	}
}

func TestRemovedMemberLosesTheAccount(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	now := time.Now()
	caretaker := h.user(t, caretakerPhone)
	acc, err := h.s.AcceptInvite(ctx, caretaker.ID, domain.HashInviteToken(h.invite(t, now)), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.s.RemoveMember(ctx, h.t, acc.MembershipID); err != nil {
		t.Fatal(err)
	}

	_, err = h.s.Membership(ctx, caretaker.ID, h.t.AccountID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("membership after removal: %v, want not found", err)
	}
}

// caretaker joins the house through an invite and returns their tenant.
func (h house) caretaker(t *testing.T) tenant.Tenant {
	t.Helper()
	now := time.Now()
	u := h.user(t, caretakerPhone)
	acc, err := h.s.AcceptInvite(context.Background(), u.ID, domain.HashInviteToken(h.invite(t, now)), now)
	if err != nil {
		t.Fatal(err)
	}
	return tenant.Tenant{AccountID: acc.ID, UserID: u.ID, MembershipID: acc.MembershipID, Role: acc.Role, PropertyIDs: acc.PropertyIDs}
}

// signIn opens a session for the user and registers a push token on it.
func (h house) signIn(t *testing.T, userID uuid.UUID, pushToken string) {
	t.Helper()
	ctx := context.Background()
	sess := auth.Session{ID: domain.NewID(), UserID: userID, RefreshHash: []byte(pushToken), ExpiresAt: time.Now().Add(time.Hour)}
	if err := h.s.CreateSession(ctx, sess, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := h.s.SavePushToken(ctx, userID, sess.ID, pushToken, "ios"); err != nil {
		t.Fatal(err)
	}
}
