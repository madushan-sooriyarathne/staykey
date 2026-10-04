package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

// Tokens is a signed-in session as the client holds it.
type Tokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// accessClaims are the claims in an access token. sub is the user, sid the session.
type accessClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
}

// Refresh exchanges a refresh token for new tokens. The old refresh token stops working, except
// that within the grace period a client may retry with it once more and get a fresh pair. Any
// other use of an old token is treated as theft and revokes the session.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	sessionID, ok := parseRefresh(refreshToken)
	if !ok {
		return Tokens{}, ErrInvalidRefresh
	}
	presented := hashRefresh(refreshToken)
	now := s.now()

	var (
		tokens Tokens
		reused bool
	)
	err := s.store.InTx(ctx, func(ctx context.Context) error {
		sess, err := s.store.SessionForUpdate(ctx, sessionID)
		if errors.Is(err, domain.ErrNotFound) {
			return ErrInvalidRefresh
		}
		if err != nil {
			return err
		}
		if sess.Revoked || !now.Before(sess.ExpiresAt) {
			return ErrInvalidRefresh
		}

		refresh, hash, err := newRefreshToken(sess.ID)
		if err != nil {
			return err
		}
		expiresAt := now.Add(s.cfg.RefreshTTL)

		switch {
		case hmac.Equal(presented, sess.RefreshHash):
			err = s.store.RotateSession(ctx, sess.ID, hash, now, expiresAt)
		case sess.PreviousRefreshHash != nil && hmac.Equal(presented, sess.PreviousRefreshHash) &&
			now.Sub(sess.RotatedAt) <= s.cfg.RefreshGrace:
			err = s.store.ReissueSession(ctx, sess.ID, hash, now, expiresAt)
		default:
			// Commit the revocation, then fail.
			reused = true
			return s.store.RevokeSession(ctx, sess.ID, "reuse", now)
		}
		if err != nil {
			return err
		}

		access, accessExp, err := s.signAccess(sess.UserID, sess.ID, now)
		if err != nil {
			return err
		}
		tokens = Tokens{AccessToken: access, AccessExpiresAt: accessExp, RefreshToken: refresh, RefreshExpiresAt: expiresAt}
		return nil
	})
	if err != nil {
		return Tokens{}, err
	}
	if reused {
		s.cfg.Logger.WarnContext(ctx, "refresh token reused, session revoked", "session", sessionID)
		return Tokens{}, ErrInvalidRefresh
	}
	return tokens, nil
}

// Logout revokes the session behind a refresh token. Unknown or stale tokens are ignored, so
// signing out always succeeds from the client's point of view.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	sessionID, ok := parseRefresh(refreshToken)
	if !ok {
		return nil
	}
	presented := hashRefresh(refreshToken)
	now := s.now()
	return s.store.InTx(ctx, func(ctx context.Context) error {
		sess, err := s.store.SessionForUpdate(ctx, sessionID)
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		current := hmac.Equal(presented, sess.RefreshHash)
		previous := sess.PreviousRefreshHash != nil && hmac.Equal(presented, sess.PreviousRefreshHash)
		if sess.Revoked || (!current && !previous) {
			return nil
		}
		return s.store.RevokeSession(ctx, sess.ID, "logout", now)
	})
}

// Authenticate checks an access token and that its session is still open, so signing out takes
// effect at once rather than when the token expires.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (Principal, error) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(accessToken, &claims,
		func(*jwt.Token) (any, error) { return s.jwtKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.cfg.Issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	userID, err1 := uuid.Parse(claims.Subject)
	sessionID, err2 := uuid.Parse(claims.SessionID)
	if err1 != nil || err2 != nil {
		return Principal{}, ErrUnauthenticated
	}

	owner, err := s.store.ActiveSessionUser(ctx, sessionID, s.now())
	if errors.Is(err, domain.ErrNotFound) || (err == nil && owner != userID) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	return Principal{UserID: userID, SessionID: sessionID}, nil
}

func (s *Service) openSession(ctx context.Context, userID uuid.UUID, deviceName string, now time.Time) (Tokens, error) {
	id := domain.NewID()
	refresh, hash, err := newRefreshToken(id)
	if err != nil {
		return Tokens{}, err
	}
	sess := Session{
		ID:          id,
		UserID:      userID,
		RefreshHash: hash,
		DeviceName:  deviceName,
		ExpiresAt:   now.Add(s.cfg.RefreshTTL),
	}
	if err := s.store.CreateSession(ctx, sess, now); err != nil {
		return Tokens{}, err
	}
	access, accessExp, err := s.signAccess(userID, id, now)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, AccessExpiresAt: accessExp, RefreshToken: refresh, RefreshExpiresAt: sess.ExpiresAt}, nil
}

func (s *Service) signAccess(userID, sessionID uuid.UUID, now time.Time) (string, time.Time, error) {
	exp := now.Add(s.cfg.AccessTTL)
	claims := accessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		SessionID: sessionID.String(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtKey)
	return signed, exp, err
}

// A refresh token is "<session id>.<32 random bytes, base64url>". Carrying the session id lets
// the API find the session and recognise an old token from that session as reuse.
func newRefreshToken(sessionID uuid.UUID) (token string, hash []byte, err error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, err
	}
	token = sessionID.String() + "." + base64.RawURLEncoding.EncodeToString(secret)
	return token, hashRefresh(token), nil
}

func parseRefresh(token string) (uuid.UUID, bool) {
	id, secret, ok := strings.Cut(token, ".")
	if !ok || len(secret) != 43 {
		return uuid.Nil, false
	}
	sessionID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, false
	}
	return sessionID, true
}

func hashRefresh(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
