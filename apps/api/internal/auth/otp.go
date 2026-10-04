package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

var codePattern = regexp.MustCompile(`^\d{6}$`)

// CodeSent describes a code that went out.
type CodeSent struct {
	Phone       string // the number in E.164
	ExpiresAt   time.Time
	ResendAfter time.Duration
	// DevCode is the code itself, only when ExposeDevCodes is on.
	DevCode string
}

// RequestCode sends a sign-in code to phone. ip is the caller's address, or the zero Addr when
// unknown. It returns a *domain.ValidationError for a malformed number and a *RateLimitError when
// the number or address has asked too often.
func (s *Service) RequestCode(ctx context.Context, phone string, ip netip.Addr) (CodeSent, error) {
	phone, err := domain.NormalizePhone(phone)
	if err != nil {
		return CodeSent{}, &domain.ValidationError{Field: "phone", Message: err.Error()}
	}

	now := s.now()
	code, err := randomCode()
	if err != nil {
		return CodeSent{}, err
	}
	challenge := OTPChallenge{
		ID:        domain.NewID(),
		Phone:     phone,
		IP:        ip,
		ExpiresAt: now.Add(s.cfg.CodeTTL),
		CreatedAt: now,
	}
	challenge.CodeHash = s.hashCode(challenge.ID, phone, code)

	err = s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.store.LockPhone(ctx, phone); err != nil {
			return err
		}
		if err := s.checkSendLimits(ctx, phone, ip, now); err != nil {
			return err
		}
		return s.store.CreateOTPChallenge(ctx, challenge)
	})
	if err != nil {
		return CodeSent{}, err
	}

	body := fmt.Sprintf("Your StayKey code is %s. It expires in %d minutes.", code, int(s.cfg.CodeTTL.Minutes()))
	if err := s.sms.Send(ctx, phone, body); err != nil {
		return CodeSent{}, fmt.Errorf("send code: %w", err)
	}

	sent := CodeSent{Phone: phone, ExpiresAt: challenge.ExpiresAt, ResendAfter: s.cfg.ResendAfter}
	if s.cfg.ExposeDevCodes {
		sent.DevCode = code
	}
	return sent, nil
}

func (s *Service) checkSendLimits(ctx context.Context, phone string, ip netip.Addr, now time.Time) error {
	last, err := s.store.LastCodeSentAt(ctx, phone)
	switch {
	case errors.Is(err, domain.ErrNotFound):
	case err != nil:
		return err
	case now.Sub(last) < s.cfg.ResendAfter:
		return &RateLimitError{RetryAfter: s.cfg.ResendAfter - now.Sub(last)}
	}

	since := now.Add(-s.cfg.RateWindow)
	byPhone, err := s.store.CodesSentToPhone(ctx, phone, since)
	if err != nil {
		return err
	}
	if byPhone.Count >= s.cfg.CodesPerPhone {
		return &RateLimitError{RetryAfter: byPhone.Oldest.Add(s.cfg.RateWindow).Sub(now)}
	}

	if !ip.IsValid() {
		return nil
	}
	byIP, err := s.store.CodesSentToIP(ctx, ip, since)
	if err != nil {
		return err
	}
	if byIP.Count >= s.cfg.CodesPerIP {
		return &RateLimitError{RetryAfter: byIP.Oldest.Add(s.cfg.RateWindow).Sub(now)}
	}
	return nil
}

// SignIn is the result of a verified code.
type SignIn struct {
	Tokens
	User    domain.User
	NewUser bool
}

// VerifyCode checks code against the latest code sent to phone and opens a session for that
// number, creating the user on first sign-in. Wrong guesses count against the code even though
// the call fails.
func (s *Service) VerifyCode(ctx context.Context, phone, code, deviceName string) (SignIn, error) {
	phone, err := domain.NormalizePhone(phone)
	if err != nil {
		return SignIn{}, &domain.ValidationError{Field: "phone", Message: err.Error()}
	}
	if !codePattern.MatchString(code) {
		return SignIn{}, &domain.ValidationError{Field: "code", Message: "enter the 6-digit code"}
	}
	deviceName = trimDevice(deviceName)

	now := s.now()
	var (
		result   SignIn
		wrongErr error
	)
	err = s.store.InTx(ctx, func(ctx context.Context) error {
		ch, err := s.store.OpenOTPChallengeForUpdate(ctx, phone, now)
		if errors.Is(err, domain.ErrNotFound) {
			return ErrCodeExpired
		}
		if err != nil {
			return err
		}
		if ch.Attempts >= s.cfg.CodeAttempts {
			return ErrTooManyAttempts
		}
		if err := s.store.RecordOTPAttempt(ctx, ch.ID); err != nil {
			return err
		}
		if !hmac.Equal(ch.CodeHash, s.hashCode(ch.ID, phone, code)) {
			// Commit the attempt, then report the wrong code.
			wrongErr = &WrongCodeError{AttemptsLeft: s.cfg.CodeAttempts - ch.Attempts - 1}
			return nil
		}
		if err := s.store.ConsumeOTPChallenge(ctx, ch.ID, now); err != nil {
			return err
		}

		user, created, err := s.store.UpsertUserByPhone(ctx, phone)
		if err != nil {
			return err
		}
		tokens, err := s.openSession(ctx, user.ID, deviceName, now)
		if err != nil {
			return err
		}
		result = SignIn{Tokens: tokens, User: user, NewUser: created}
		return nil
	})
	if err != nil {
		return SignIn{}, err
	}
	if wrongErr != nil {
		return SignIn{}, wrongErr
	}
	return result, nil
}

// hashCode binds a code to its challenge and number, keyed by the server secret, so a leaked
// table cannot be brute forced offline.
func (s *Service) hashCode(id uuid.UUID, phone, code string) []byte {
	m := hmac.New(sha256.New, s.otpKey)
	m.Write(id[:])
	m.Write([]byte(phone))
	m.Write([]byte{0})
	m.Write([]byte(code))
	return m.Sum(nil)
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func trimDevice(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) > 80 {
		r = r[:80]
	}
	return string(r)
}
