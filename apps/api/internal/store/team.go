package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// ListTeam returns the account's members, owners first, and its open invites, newest first.
func (s *Postgres) ListTeam(ctx context.Context, t tenant.Tenant, now time.Time) ([]domain.TeamMember, []domain.Invite, error) {
	q := s.db(ctx)
	rows, err := q.ListMembers(ctx, t.AccountID)
	if err != nil {
		return nil, nil, fmt.Errorf("list members: %w", err)
	}
	members := make([]domain.TeamMember, len(rows))
	for i, r := range rows {
		members[i] = domain.TeamMember{
			MembershipID: r.ID, UserID: r.UserID, Name: r.Name, Phone: r.Phone,
			Role: domain.Role(r.Role), PropertyIDs: r.PropertyIds, JoinedAt: r.CreatedAt,
		}
	}
	open, err := q.ListOpenInvites(ctx, queries.ListOpenInvitesParams{AccountID: t.AccountID, Now: now})
	if err != nil {
		return nil, nil, fmt.Errorf("list invites: %w", err)
	}
	invites := make([]domain.Invite, len(open))
	for i, r := range open {
		invites[i] = invite(r)
	}
	return members, invites, nil
}

// InviteMember records an invite from the tenant's user, replacing any open invite to the same
// number. Properties must belong to the account. It returns domain.ErrAlreadyMember when the
// number is already on the team.
func (s *Postgres) InviteMember(ctx context.Context, t tenant.Tenant, in domain.NewInvite, tokenHash []byte, now time.Time) (domain.Invite, error) {
	var out domain.Invite
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		if err := s.checkProperties(ctx, t, in.PropertyIDs); err != nil {
			return err
		}
		u, err := q.GetUserByPhone(ctx, in.Phone)
		switch {
		case err == nil:
			member, err := q.IsMember(ctx, queries.IsMemberParams{AccountID: t.AccountID, UserID: u.ID})
			if err != nil {
				return fmt.Errorf("check membership: %w", err)
			}
			if member {
				return domain.ErrAlreadyMember
			}
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("get user: %w", err)
		}
		if err := q.RevokeOpenInvitesForPhone(ctx, queries.RevokeOpenInvitesForPhoneParams{
			Now: now, AccountID: t.AccountID, Phone: in.Phone,
		}); err != nil {
			return fmt.Errorf("revoke open invites: %w", err)
		}
		id, err := s.CreateInvite(ctx, t, Invite{
			Phone: in.Phone, Name: in.Name, Role: in.Role, PropertyIDs: in.PropertyIDs,
			TokenHash: tokenHash, ExpiresAt: now.Add(domain.InviteTTL),
		})
		if err != nil {
			return err
		}
		ids := in.PropertyIDs
		if ids == nil {
			ids = []uuid.UUID{}
		}
		out = domain.Invite{
			ID: id, Phone: in.Phone, Name: in.Name, Role: in.Role, PropertyIDs: ids,
			ExpiresAt: now.Add(domain.InviteTTL), CreatedAt: now,
		}
		return nil
	})
	return out, err
}

// RevokeInvite stops an open invite's link from working.
func (s *Postgres) RevokeInvite(ctx context.Context, t tenant.Tenant, id uuid.UUID, now time.Time) error {
	_, err := s.db(ctx).RevokeInvite(ctx, queries.RevokeInviteParams{Now: now, ID: id, AccountID: t.AccountID})
	return notFound(err)
}

// PreviewInvite describes an open invite for the join screen.
func (s *Postgres) PreviewInvite(ctx context.Context, tokenHash []byte, now time.Time) (domain.InvitePreview, error) {
	q := s.db(ctx)
	r, err := q.GetOpenInvite(ctx, queries.GetOpenInviteParams{TokenHash: tokenHash, Now: now})
	if err != nil {
		return domain.InvitePreview{}, notFound(err)
	}
	names, err := q.PropertyNames(ctx, queries.PropertyNamesParams{AccountID: r.Invite.AccountID, Ids: r.Invite.PropertyIds})
	if err != nil {
		return domain.InvitePreview{}, fmt.Errorf("property names: %w", err)
	}
	return domain.InvitePreview{
		AccountName:   r.AccountName,
		InvitedByName: r.InvitedByName,
		Role:          domain.Role(r.Invite.Role),
		PropertyNames: names,
		ExpiresAt:     r.Invite.ExpiresAt,
	}, nil
}

// AcceptInvite adds the signed-in user to the account an open invite is for. The invite must be
// for their phone; any other number gets domain.ErrNotFound, the same as an unknown token.
// Someone already in the account keeps their membership and the invite is closed.
func (s *Postgres) AcceptInvite(ctx context.Context, userID uuid.UUID, tokenHash []byte, now time.Time) (domain.AccountMembership, error) {
	var accountID uuid.UUID
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		inv, err := q.GetOpenInviteForUpdate(ctx, queries.GetOpenInviteForUpdateParams{TokenHash: tokenHash, Now: now})
		if err != nil {
			return notFound(err)
		}
		u, err := q.GetUser(ctx, userID)
		if err != nil {
			return notFound(err)
		}
		if u.Phone != inv.Phone || !bytes.Equal(inv.TokenHash, tokenHash) {
			return domain.ErrNotFound
		}
		accountID = inv.AccountID
		member, err := q.IsMember(ctx, queries.IsMemberParams{AccountID: inv.AccountID, UserID: userID})
		if err != nil {
			return fmt.Errorf("check membership: %w", err)
		}
		if !member {
			t := tenant.Tenant{AccountID: inv.AccountID, UserID: userID}
			if _, err := s.AddMember(ctx, t, userID, domain.Role(inv.Role), inv.PropertyIds); err != nil {
				return err
			}
		}
		if err := q.MarkInviteAccepted(ctx, queries.MarkInviteAcceptedParams{Now: now, UserID: &userID, ID: inv.ID}); err != nil {
			return fmt.Errorf("accept invite: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.AccountMembership{}, err
	}
	accounts, err := s.ListAccounts(ctx, userID)
	if err != nil {
		return domain.AccountMembership{}, err
	}
	for _, a := range accounts {
		if a.ID == accountID {
			return a, nil
		}
	}
	// A suspended membership, or an account closed since the invite went out.
	return domain.AccountMembership{}, domain.ErrNotFound
}

// UpdateMember changes a member's role or properties. Owners can't be changed.
func (s *Postgres) UpdateMember(ctx context.Context, t tenant.Tenant, membershipID uuid.UUID, p domain.MemberPatch) (domain.TeamMember, error) {
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		m, err := q.GetMemberForUpdate(ctx, queries.GetMemberForUpdateParams{ID: membershipID, AccountID: t.AccountID})
		if err != nil {
			return notFound(err)
		}
		if domain.Role(m.Role) == domain.RoleOwner {
			return domain.ErrOwnerMembership
		}
		if p.Role != nil {
			if err := q.UpdateMemberRole(ctx, queries.UpdateMemberRoleParams{Role: string(*p.Role), ID: m.ID, AccountID: t.AccountID}); err != nil {
				return fmt.Errorf("update role: %w", err)
			}
		}
		if p.PropertyIDs == nil {
			return nil
		}
		if err := s.checkProperties(ctx, t, *p.PropertyIDs); err != nil {
			return err
		}
		if err := q.DeleteMemberProperties(ctx, queries.DeleteMemberPropertiesParams{MembershipID: m.ID, AccountID: t.AccountID}); err != nil {
			return fmt.Errorf("clear member properties: %w", err)
		}
		for _, pid := range *p.PropertyIDs {
			if err := q.AddMembershipProperty(ctx, queries.AddMembershipPropertyParams{
				AccountID: t.AccountID, MembershipID: m.ID, PropertyID: pid,
			}); err != nil {
				return fmt.Errorf("add member property: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.TeamMember{}, err
	}
	members, _, err := s.ListTeam(ctx, t, time.Now())
	if err != nil {
		return domain.TeamMember{}, err
	}
	for _, m := range members {
		if m.MembershipID == membershipID {
			return m, nil
		}
	}
	return domain.TeamMember{}, domain.ErrNotFound
}

// RemoveMember takes a member out of the account, with their alert preferences. Their sessions
// stay signed in but can no longer reach the account. Owners can't be removed.
func (s *Postgres) RemoveMember(ctx context.Context, t tenant.Tenant, membershipID uuid.UUID) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		m, err := q.GetMemberForUpdate(ctx, queries.GetMemberForUpdateParams{ID: membershipID, AccountID: t.AccountID})
		if err != nil {
			return notFound(err)
		}
		if domain.Role(m.Role) == domain.RoleOwner {
			return domain.ErrOwnerMembership
		}
		if err := q.DeleteMember(ctx, queries.DeleteMemberParams{ID: m.ID, AccountID: t.AccountID}); err != nil {
			return fmt.Errorf("delete member: %w", err)
		}
		if err := q.DeleteMemberPrefs(ctx, queries.DeleteMemberPrefsParams{AccountID: t.AccountID, UserID: m.UserID}); err != nil {
			return fmt.Errorf("delete member preferences: %w", err)
		}
		return nil
	})
}

// checkProperties makes sure every id is a property of the tenant's account.
func (s *Postgres) checkProperties(ctx context.Context, t tenant.Tenant, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	names, err := s.db(ctx).PropertyNames(ctx, queries.PropertyNamesParams{AccountID: t.AccountID, Ids: ids})
	if err != nil {
		return fmt.Errorf("check properties: %w", err)
	}
	if len(names) != len(ids) {
		return &domain.ValidationError{Field: "propertyIds", Message: "must be properties in this account"}
	}
	return nil
}

func invite(r queries.Invite) domain.Invite {
	return domain.Invite{
		ID: r.ID, Phone: r.Phone, Name: r.Name, Role: domain.Role(r.Role),
		PropertyIDs: r.PropertyIds, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt,
	}
}
