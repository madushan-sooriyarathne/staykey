package store

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/store/queries"
)

var _ auth.Store = (*Postgres)(nil)

// LockPhone holds a transaction-scoped lock for one number.
func (s *Postgres) LockPhone(ctx context.Context, phone string) error {
	return s.db(ctx).LockPhone(ctx, phone)
}

// CodesSentToPhone counts codes sent to a number since a time.
func (s *Postgres) CodesSentToPhone(ctx context.Context, phone string, since time.Time) (auth.Sent, error) {
	r, err := s.db(ctx).OTPsForPhoneSince(ctx, queries.OTPsForPhoneSinceParams{Phone: phone, Since: since})
	return auth.Sent{Count: int(r.Sent), Oldest: r.Oldest}, err
}

// CodesSentToIP counts codes requested from an address since a time.
func (s *Postgres) CodesSentToIP(ctx context.Context, ip netip.Addr, since time.Time) (auth.Sent, error) {
	r, err := s.db(ctx).OTPsForIPSince(ctx, queries.OTPsForIPSinceParams{Ip: &ip, Since: since})
	return auth.Sent{Count: int(r.Sent), Oldest: r.Oldest}, err
}

// LastCodeSentAt returns when the latest code went to a number, or domain.ErrNotFound.
func (s *Postgres) LastCodeSentAt(ctx context.Context, phone string) (time.Time, error) {
	t, err := s.db(ctx).LastOTPSentAt(ctx, phone)
	return t, notFound(err)
}

// CreateOTPChallenge stores a sent code.
func (s *Postgres) CreateOTPChallenge(ctx context.Context, c auth.OTPChallenge) error {
	var ip *netip.Addr
	if c.IP.IsValid() {
		ip = &c.IP
	}
	return s.db(ctx).CreateOTPChallenge(ctx, queries.CreateOTPChallengeParams{
		ID: c.ID, Phone: c.Phone, CodeHash: c.CodeHash, Ip: ip, ExpiresAt: c.ExpiresAt, CreatedAt: c.CreatedAt,
	})
}

// OpenOTPChallengeForUpdate locks and returns the latest open code for a number.
func (s *Postgres) OpenOTPChallengeForUpdate(ctx context.Context, phone string, now time.Time) (auth.OTPChallenge, error) {
	r, err := s.db(ctx).GetOpenOTPForUpdate(ctx, queries.GetOpenOTPForUpdateParams{Phone: phone, Now: now})
	if err != nil {
		return auth.OTPChallenge{}, notFound(err)
	}
	c := auth.OTPChallenge{
		ID: r.ID, Phone: r.Phone, CodeHash: r.CodeHash, Attempts: int(r.Attempts),
		ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt,
	}
	if r.Ip != nil {
		c.IP = *r.Ip
	}
	return c, nil
}

// RecordOTPAttempt counts a guess against a code.
func (s *Postgres) RecordOTPAttempt(ctx context.Context, id uuid.UUID) error {
	return s.db(ctx).RecordOTPAttempt(ctx, id)
}

// ConsumeOTPChallenge marks a code as used.
func (s *Postgres) ConsumeOTPChallenge(ctx context.Context, id uuid.UUID, now time.Time) error {
	return s.db(ctx).ConsumeOTP(ctx, queries.ConsumeOTPParams{ID: id, Now: now})
}

// CreateSession stores a new device session.
func (s *Postgres) CreateSession(ctx context.Context, sess auth.Session, now time.Time) error {
	return s.db(ctx).CreateSession(ctx, queries.CreateSessionParams{
		ID: sess.ID, UserID: sess.UserID, RefreshHash: sess.RefreshHash,
		DeviceName: sess.DeviceName, Now: now, ExpiresAt: sess.ExpiresAt,
	})
}

// SessionForUpdate locks and returns a session.
func (s *Postgres) SessionForUpdate(ctx context.Context, id uuid.UUID) (auth.Session, error) {
	r, err := s.db(ctx).GetSessionForUpdate(ctx, id)
	if err != nil {
		return auth.Session{}, notFound(err)
	}
	return auth.Session{
		ID: r.ID, UserID: r.UserID, RefreshHash: r.RefreshHash, PreviousRefreshHash: r.PreviousRefreshHash,
		RotatedAt: r.RotatedAt, DeviceName: r.DeviceName, ExpiresAt: r.ExpiresAt, Revoked: r.RevokedAt != nil,
	}, nil
}

// RotateSession replaces the refresh token and remembers the old one for the grace period.
func (s *Postgres) RotateSession(ctx context.Context, id uuid.UUID, refreshHash []byte, now, expiresAt time.Time) error {
	return s.db(ctx).RotateSession(ctx, queries.RotateSessionParams{ID: id, RefreshHash: refreshHash, Now: now, ExpiresAt: expiresAt})
}

// ReissueSession replaces the refresh token during the grace period.
func (s *Postgres) ReissueSession(ctx context.Context, id uuid.UUID, refreshHash []byte, now, expiresAt time.Time) error {
	return s.db(ctx).ReissueSession(ctx, queries.ReissueSessionParams{ID: id, RefreshHash: refreshHash, Now: now, ExpiresAt: expiresAt})
}

// RevokeSession ends a session.
func (s *Postgres) RevokeSession(ctx context.Context, id uuid.UUID, reason string, now time.Time) error {
	return s.db(ctx).RevokeSession(ctx, queries.RevokeSessionParams{ID: id, Reason: reason, Now: now})
}

// ActiveSessionUser returns the user of an open session, or domain.ErrNotFound.
func (s *Postgres) ActiveSessionUser(ctx context.Context, id uuid.UUID, now time.Time) (uuid.UUID, error) {
	u, err := s.db(ctx).GetActiveSessionUser(ctx, queries.GetActiveSessionUserParams{ID: id, Now: now})
	return u, notFound(err)
}
