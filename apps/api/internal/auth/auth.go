// Package auth signs owners and their teams in with a phone number.
//
// A 6-digit code goes out by SMS, then a verified code opens a session: a 15 minute access token
// (a signed JWT) and a refresh token that rotates on every use. Codes and refresh tokens are only
// stored as hashes. Presenting a refresh token that has already been rotated revokes the session,
// except for a short grace period that lets a client retry a refresh whose response it lost.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

// Config holds the rules for codes and tokens. Zero durations and counts take the defaults.
type Config struct {
	// Secret signs access tokens and keys the code hashes. At least 32 bytes.
	Secret []byte
	Issuer string

	AccessTTL    time.Duration // default 15 minutes
	RefreshTTL   time.Duration // default 60 days, extended on every refresh
	RefreshGrace time.Duration // default 30 seconds

	CodeTTL       time.Duration // default 5 minutes
	CodeAttempts  int           // default 5 per code
	ResendAfter   time.Duration // default 30 seconds between codes to one number
	CodesPerPhone int           // default 5 per number per RateWindow
	CodesPerIP    int           // default 20 per IP address per RateWindow
	RateWindow    time.Duration // default 1 hour

	// ExposeDevCodes returns each code in the response so a development build can show it.
	// Never set in production.
	ExposeDevCodes bool

	Now    func() time.Time
	Logger *slog.Logger
}

func (c *Config) applyDefaults() {
	def := func(d *time.Duration, v time.Duration) {
		if *d == 0 {
			*d = v
		}
	}
	defInt := func(n *int, v int) {
		if *n == 0 {
			*n = v
		}
	}
	if c.Issuer == "" {
		c.Issuer = "staykey.direct"
	}
	def(&c.AccessTTL, 15*time.Minute)
	def(&c.RefreshTTL, 60*24*time.Hour)
	def(&c.RefreshGrace, 30*time.Second)
	def(&c.CodeTTL, 5*time.Minute)
	defInt(&c.CodeAttempts, 5)
	def(&c.ResendAfter, 30*time.Second)
	defInt(&c.CodesPerPhone, 5)
	defInt(&c.CodesPerIP, 20)
	def(&c.RateWindow, time.Hour)
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}

// OTPChallenge is one code sent to one number.
type OTPChallenge struct {
	ID        uuid.UUID
	Phone     string
	CodeHash  []byte
	Attempts  int
	IP        netip.Addr // zero when unknown
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Session is one signed-in device.
type Session struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	RefreshHash         []byte
	PreviousRefreshHash []byte
	RotatedAt           time.Time
	DeviceName          string
	ExpiresAt           time.Time
	Revoked             bool
}

// Sent summarizes recent codes for a number or address inside the rate window.
type Sent struct {
	Count  int
	Oldest time.Time
}

// Store is the persistence auth needs. internal/store implements it. Calls made with the ctx
// that InTx passes to fn run inside that transaction.
type Store interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error

	LockPhone(ctx context.Context, phone string) error
	CodesSentToPhone(ctx context.Context, phone string, since time.Time) (Sent, error)
	CodesSentToIP(ctx context.Context, ip netip.Addr, since time.Time) (Sent, error)
	LastCodeSentAt(ctx context.Context, phone string) (time.Time, error) // domain.ErrNotFound if none
	CreateOTPChallenge(ctx context.Context, c OTPChallenge) error
	OpenOTPChallengeForUpdate(ctx context.Context, phone string, now time.Time) (OTPChallenge, error)
	RecordOTPAttempt(ctx context.Context, id uuid.UUID) error
	ConsumeOTPChallenge(ctx context.Context, id uuid.UUID, now time.Time) error

	UpsertUserByPhone(ctx context.Context, phone string) (u domain.User, created bool, err error)

	CreateSession(ctx context.Context, s Session, now time.Time) error
	SessionForUpdate(ctx context.Context, id uuid.UUID) (Session, error)
	RotateSession(ctx context.Context, id uuid.UUID, refreshHash []byte, now, expiresAt time.Time) error
	ReissueSession(ctx context.Context, id uuid.UUID, refreshHash []byte, now, expiresAt time.Time) error
	RevokeSession(ctx context.Context, id uuid.UUID, reason string, now time.Time) error
	ActiveSessionUser(ctx context.Context, id uuid.UUID, now time.Time) (uuid.UUID, error)
}

// SMSSender delivers a text message. A Sri Lankan gateway or Twilio will implement it; LogSender
// stands in during development.
type SMSSender interface {
	Send(ctx context.Context, to, body string) error
}

// LogSender writes messages to the log instead of sending them.
type LogSender struct{ Logger *slog.Logger }

// Send logs the message.
func (s LogSender) Send(ctx context.Context, to, body string) error {
	log := s.Logger
	if log == nil {
		log = slog.Default()
	}
	log.InfoContext(ctx, "sms (not sent, log sender)", "to", to, "body", body)
	return nil
}

var (
	// ErrCodeExpired means there is no open code for the number: it expired, was used, or was
	// never sent.
	ErrCodeExpired = errors.New("code expired")
	// ErrTooManyAttempts means the code was guessed wrong too often and a new one is needed.
	ErrTooManyAttempts = errors.New("too many attempts")
	// ErrInvalidRefresh means the refresh token is unknown, expired, revoked or reused.
	ErrInvalidRefresh = errors.New("invalid refresh token")
	// ErrUnauthenticated means the access token is missing, invalid, expired or revoked.
	ErrUnauthenticated = errors.New("unauthenticated")
)

// WrongCodeError means the code did not match.
type WrongCodeError struct{ AttemptsLeft int }

func (e *WrongCodeError) Error() string {
	return fmt.Sprintf("wrong code, %d attempts left", e.AttemptsLeft)
}

// RateLimitError means too many codes were requested; try again after RetryAfter.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string { return "rate limited, retry after " + e.RetryAfter.String() }

// Service runs sign-in and sessions.
type Service struct {
	store  Store
	sms    SMSSender
	cfg    Config
	otpKey []byte
	jwtKey []byte
}

// New returns a Service. It fails when the secret is shorter than 32 bytes.
func New(store Store, sms SMSSender, cfg Config) (*Service, error) {
	if len(cfg.Secret) < 32 {
		return nil, errors.New("auth secret must be at least 32 bytes")
	}
	cfg.applyDefaults()
	return &Service{
		store:  store,
		sms:    sms,
		cfg:    cfg,
		otpKey: derive(cfg.Secret, "staykey otp v1"),
		jwtKey: derive(cfg.Secret, "staykey access token v1"),
	}, nil
}

// AccessTTL is how long an access token lasts.
func (s *Service) AccessTTL() time.Duration { return s.cfg.AccessTTL }

func (s *Service) now() time.Time { return s.cfg.Now().UTC() }

// derive gives each use of the secret its own key.
func derive(secret []byte, purpose string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(purpose))
	return m.Sum(nil)
}

type principalKey struct{}

// Principal is the signed-in caller.
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

// WithPrincipal returns ctx carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the caller the middleware authenticated.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
