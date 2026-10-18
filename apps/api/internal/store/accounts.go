package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// GetUser returns a user by id.
func (s *Postgres) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	u, err := s.db(ctx).GetUser(ctx, id)
	if err != nil {
		return domain.User{}, notFound(err)
	}
	return user(u), nil
}

// UpdateUser applies a profile patch.
func (s *Postgres) UpdateUser(ctx context.Context, id uuid.UUID, p domain.UserPatch) (domain.User, error) {
	u, err := s.db(ctx).UpdateUser(ctx, queries.UpdateUserParams{
		ID:       id,
		Name:     p.Name,
		Email:    p.Email,
		Language: p.Language,
	})
	if err != nil {
		return domain.User{}, notFound(err)
	}
	return user(queries.User{
		ID: u.ID, Phone: u.Phone, Name: u.Name, Email: u.Email, Language: u.Language, PhotoKey: u.PhotoKey,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt, DeletedAt: u.DeletedAt, EmailConfirmedAt: u.EmailConfirmedAt,
	}), nil
}

// UpsertUserByPhone returns the user with this number, creating them on first sign-in.
func (s *Postgres) UpsertUserByPhone(ctx context.Context, phone string) (domain.User, bool, error) {
	id := domain.NewID()
	if err := s.db(ctx).InsertUserIfMissing(ctx, queries.InsertUserIfMissingParams{ID: id, Phone: phone}); err != nil {
		return domain.User{}, false, fmt.Errorf("insert user: %w", err)
	}
	u, err := s.db(ctx).GetUserByPhone(ctx, phone)
	if err != nil {
		return domain.User{}, false, fmt.Errorf("get user: %w", err)
	}
	return user(u), u.ID == id, nil
}

// ListAccounts returns the accounts a user belongs to, oldest membership first.
func (s *Postgres) ListAccounts(ctx context.Context, userID uuid.UUID) ([]domain.AccountMembership, error) {
	rows, err := s.db(ctx).ListAccountsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	out := make([]domain.AccountMembership, len(rows))
	for i, r := range rows {
		out[i] = domain.AccountMembership{
			Account: domain.Account{
				ID:             r.ID,
				Name:           r.Name,
				Status:         domain.AccountStatus(r.Status),
				TrialStartedAt: r.TrialStartedAt,
				CreatedAt:      r.CreatedAt,
			},
			MembershipID: r.MembershipID,
			Role:         domain.Role(r.Role),
			PropertyIDs:  r.PropertyIds,
		}
	}
	return out, nil
}

// Membership returns a user's active membership in an account, or domain.ErrNotFound.
func (s *Postgres) Membership(ctx context.Context, userID, accountID uuid.UUID) (domain.Membership, error) {
	r, err := s.db(ctx).GetMembership(ctx, queries.GetMembershipParams{UserID: userID, AccountID: accountID})
	if err != nil {
		return domain.Membership{}, notFound(err)
	}
	return domain.Membership{
		ID:          r.ID,
		AccountID:   r.AccountID,
		UserID:      r.UserID,
		Role:        domain.Role(r.Role),
		PropertyIDs: r.PropertyIds,
	}, nil
}

// CreateAccount creates an account with userID as its owner.
func (s *Postgres) CreateAccount(ctx context.Context, userID uuid.UUID, name string) (domain.AccountMembership, error) {
	var out domain.AccountMembership
	err := s.InTx(ctx, func(ctx context.Context) error {
		a, err := s.db(ctx).CreateAccount(ctx, queries.CreateAccountParams{ID: domain.NewID(), Name: name})
		if err != nil {
			return err
		}
		m, err := s.db(ctx).CreateMembership(ctx, queries.CreateMembershipParams{
			ID: domain.NewID(), AccountID: a.ID, UserID: userID, Role: string(domain.RoleOwner),
		})
		if err != nil {
			return err
		}
		if err := s.addTemplates(ctx, a.ID); err != nil {
			return err
		}
		if err := s.addNotificationPrefs(ctx, a.ID, userID); err != nil {
			return err
		}
		out = domain.AccountMembership{
			Account: domain.Account{
				ID: a.ID, Name: a.Name, Status: domain.AccountStatus(a.Status),
				TrialStartedAt: a.TrialStartedAt, CreatedAt: a.CreatedAt,
			},
			MembershipID: m.ID,
			Role:         domain.RoleOwner,
			PropertyIDs:  []uuid.UUID{},
		}
		return nil
	})
	if err != nil {
		return domain.AccountMembership{}, fmt.Errorf("create account: %w", err)
	}
	return out, nil
}

// AddMember adds a user to the tenant's account. propertyIDs limits them to those properties,
// which must belong to the same account; empty means every property.
func (s *Postgres) AddMember(ctx context.Context, t tenant.Tenant, userID uuid.UUID, role domain.Role, propertyIDs []uuid.UUID) (domain.Membership, error) {
	var out domain.Membership
	err := s.InTx(ctx, func(ctx context.Context) error {
		m, err := s.db(ctx).CreateMembership(ctx, queries.CreateMembershipParams{
			ID: domain.NewID(), AccountID: t.AccountID, UserID: userID, Role: string(role),
		})
		if err != nil {
			return err
		}
		for _, pid := range propertyIDs {
			if err := s.db(ctx).AddMembershipProperty(ctx, queries.AddMembershipPropertyParams{
				AccountID: t.AccountID, MembershipID: m.ID, PropertyID: pid,
			}); err != nil {
				return err
			}
		}
		if err := s.addNotificationPrefs(ctx, t.AccountID, userID); err != nil {
			return err
		}
		out = domain.Membership{ID: m.ID, AccountID: t.AccountID, UserID: userID, Role: role, PropertyIDs: propertyIDs}
		return nil
	})
	if err != nil {
		return domain.Membership{}, fmt.Errorf("add member: %w", err)
	}
	return out, nil
}

// Invite is a pending invitation to join an account.
type Invite struct {
	Phone       string
	Name        string
	Role        domain.Role
	PropertyIDs []uuid.UUID
	TokenHash   []byte
	ExpiresAt   time.Time
}

// CreateInvite records an invitation from the tenant's user into the tenant's account and
// returns its id.
func (s *Postgres) CreateInvite(ctx context.Context, t tenant.Tenant, in Invite) (uuid.UUID, error) {
	id := domain.NewID()
	invitedBy := t.UserID
	propertyIDs := in.PropertyIDs
	if propertyIDs == nil {
		propertyIDs = []uuid.UUID{}
	}
	err := s.db(ctx).CreateInvite(ctx, queries.CreateInviteParams{
		ID:          id,
		AccountID:   t.AccountID,
		Phone:       in.Phone,
		Name:        in.Name,
		Role:        string(in.Role),
		PropertyIds: propertyIDs,
		TokenHash:   in.TokenHash,
		InvitedBy:   &invitedBy,
		ExpiresAt:   in.ExpiresAt,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create invite: %w", err)
	}
	return id, nil
}

// DeleteAccount removes the tenant's account and everything in it. Owners only.
func (s *Postgres) DeleteAccount(ctx context.Context, t tenant.Tenant) error {
	if t.Role != domain.RoleOwner {
		return fmt.Errorf("delete account: role %s is not the owner", t.Role)
	}
	return s.db(ctx).DeleteAccount(ctx, t.AccountID)
}

func user(u queries.User) domain.User {
	out := domain.User{
		ID:        u.ID,
		Phone:     u.Phone,
		Name:      u.Name,
		Language:  u.Language,
		CreatedAt: u.CreatedAt,
	}
	if u.Email != nil {
		out.Email = *u.Email
	}
	if u.PhotoKey != nil {
		out.PhotoKey = *u.PhotoKey
	}
	return out
}

// SetUserPhoto sets the person's profile photo; an empty key removes it. The old file is left
// for the upload sweep, which deletes files nothing points at.
func (s *Postgres) SetUserPhoto(ctx context.Context, userID uuid.UUID, key string) error {
	var k *string
	if key != "" {
		k = &key
	}
	if err := s.db(ctx).SetUserPhoto(ctx, queries.SetUserPhotoParams{ID: userID, PhotoKey: k}); err != nil {
		return fmt.Errorf("set user photo: %w", err)
	}
	return nil
}

// RenameAccount renames the tenant's account. Owners only.
func (s *Postgres) RenameAccount(ctx context.Context, t tenant.Tenant, name string) (domain.AccountMembership, error) {
	if t.Role != domain.RoleOwner {
		return domain.AccountMembership{}, fmt.Errorf("rename account: role %s is not the owner", t.Role)
	}
	a, err := s.db(ctx).RenameAccount(ctx, queries.RenameAccountParams{ID: t.AccountID, Name: name})
	if err != nil {
		return domain.AccountMembership{}, notFound(err)
	}
	return domain.AccountMembership{
		Account: domain.Account{
			ID: a.ID, Name: a.Name, Status: domain.AccountStatus(a.Status),
			TrialStartedAt: a.TrialStartedAt, CreatedAt: a.CreatedAt,
		},
		MembershipID: t.MembershipID,
		Role:         t.Role,
		PropertyIDs:  t.PropertyIDs,
	}, nil
}

// DeleteUser starts deleting a person: they are signed out everywhere, leave the teams of
// accounts they don't own, and the accounts they own close, which takes their booking pages
// offline. PurgeDeletedUsers finishes the job after domain.DeletionDays; RestoreUser undoes it
// before then. It returns when the deletion becomes final, and refuses with
// domain.ErrStorePlanRenews while an owned account's store plan will renew.
func (s *Postgres) DeleteUser(ctx context.Context, userID uuid.UUID, now time.Time) (time.Time, error) {
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		owned, err := q.OwnedAccountIDs(ctx, userID)
		if err != nil {
			return fmt.Errorf("owned accounts: %w", err)
		}
		for _, id := range owned {
			b, err := s.AccountBilling(ctx, id, now)
			if err != nil {
				return err
			}
			if b.Sub != nil && b.Sub.Source.IsStore() && b.Sub.WillRenew && b.Sub.GivesAccess(now) {
				return domain.ErrStorePlanRenews
			}
		}
		for _, id := range owned {
			if err := q.CloseAccount(ctx, queries.CloseAccountParams{ID: id, Now: now}); err != nil {
				return fmt.Errorf("close account: %w", err)
			}
		}
		if err := q.DeleteTeamMemberships(ctx, userID); err != nil {
			return fmt.Errorf("leave teams: %w", err)
		}
		if err := q.MarkUserDeleted(ctx, queries.MarkUserDeletedParams{ID: userID, Now: now}); err != nil {
			return fmt.Errorf("mark user deleted: %w", err)
		}
		if err := q.RevokeUserSessions(ctx, queries.RevokeUserSessionsParams{UserID: userID, Now: now, Reason: "deleted"}); err != nil {
			return fmt.Errorf("sign out: %w", err)
		}
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}
	return now.AddDate(0, 0, domain.DeletionDays), nil
}

// RestoreUser undoes a deletion that hasn't become final: the person is back and the accounts
// they own reopen with the status their plan gives them. It reports whether there was anything
// to restore. Team memberships in other accounts don't come back; their owners invite again.
func (s *Postgres) RestoreUser(ctx context.Context, userID uuid.UUID, now time.Time) (bool, error) {
	restored := false
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		n, err := q.RestoreUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("restore user: %w", err)
		}
		if n == 0 {
			return nil
		}
		restored = true
		ids, err := q.ReopenOwnedAccounts(ctx, userID)
		if err != nil {
			return fmt.Errorf("reopen accounts: %w", err)
		}
		for _, id := range ids {
			if err := s.recomputeBilling(ctx, id, now); err != nil {
				return err
			}
		}
		return nil
	})
	return restored, err
}

// PurgeDeletedUsers finishes deletions older than domain.DeletionDays: the accounts the person
// owns go with every row in them, then the person. Files go with the upload sweep. One person
// per transaction.
func (s *Postgres) PurgeDeletedUsers(ctx context.Context, now time.Time) (int, error) {
	ids, err := s.db(ctx).UsersDeletedBefore(ctx, now.AddDate(0, 0, -domain.DeletionDays))
	if err != nil {
		return 0, fmt.Errorf("deleted users: %w", err)
	}
	for i, id := range ids {
		err := s.InTx(ctx, func(ctx context.Context) error {
			if err := s.db(ctx).DeleteClosedOwnedAccounts(ctx, id); err != nil {
				return fmt.Errorf("delete accounts: %w", err)
			}
			if err := s.db(ctx).DeleteUser(ctx, id); err != nil {
				return fmt.Errorf("delete user: %w", err)
			}
			return nil
		})
		if err != nil {
			return i, fmt.Errorf("user %s: %w", id, err)
		}
	}
	return len(ids), nil
}
