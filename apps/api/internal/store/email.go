package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
)

// Links builds the addresses emails point to.
type Links struct {
	// BookingPage is a property's booking page, from its slug.
	BookingPage func(slug string) string
	// VerifyEmail is where confirmation links go; the token is added as the last path segment.
	VerifyEmail string
}

// WithLinks sets how emails link to booking pages, stay pages and email confirmation.
func WithLinks(l Links) Option { return func(s *Postgres) { s.links = l } }

// Email is a rendered email ready to send.
type Email struct {
	ID       uuid.UUID
	Kind     domain.EmailKind
	To       string
	FromName string
	ReplyTo  string
	Subject  string
	Text     string
	Footer   string
	// Link is the email's main link, which the log sender shows in development.
	Link      string
	BookingID *uuid.UUID
	UserID    *uuid.UUID
}

// emailAttempts is how many times an email is tried before it is marked failed.
const emailAttempts = 3

// emailClaimStale is how long a claim lasts before another run may take the email again.
const emailClaimStale = 15 * time.Minute

// emailPayload holds what an email needs that may be gone or changed by the time it is sent.
type emailPayload struct {
	Name string `json:"name,omitempty"`
	// Token is the confirmation token, sealed for the email row.
	Token   []byte               `json:"token,omitempty"`
	Device  string               `json:"device,omitempty"`
	At      *time.Time           `json:"at,omitempty"`
	FinalOn *time.Time           `json:"finalOn,omitempty"`
	On      string               `json:"on,omitempty"`
	Receipt *domain.PaidPeriod   `json:"receipt,omitempty"`
	Payout  *domain.PayoutNotice `json:"payout,omitempty"`
}

type queuedEmail struct {
	id        uuid.UUID
	accountID *uuid.UUID
	bookingID *uuid.UUID
	userID    *uuid.UUID
	kind      domain.EmailKind
	to        string
	payload   emailPayload
	dedupeKey string
	at        time.Time
}

// queueEmail adds an email to the outbox inside the caller's transaction, so it goes out with
// the change that caused it or not at all. A repeat of a dedupe key is dropped.
func (s *Postgres) queueEmail(ctx context.Context, e queuedEmail) error {
	if e.id == uuid.Nil {
		e.id = domain.NewID()
	}
	payload, err := json.Marshal(e.payload)
	if err != nil {
		return fmt.Errorf("email payload: %w", err)
	}
	if _, err := s.db(ctx).QueueEmail(ctx, queries.QueueEmailParams{
		ID: e.id, AccountID: e.accountID, BookingID: e.bookingID, UserID: e.userID, Kind: string(e.kind),
		ToAddress: e.to, Payload: payload, DedupeKey: nonEmpty(e.dedupeKey), ScheduledFor: e.at,
	}); err != nil {
		return fmt.Errorf("queue email: %w", err)
	}
	return nil
}

// queueStayEmail queues an event email to a stay's guest. on, when set, is the status the email
// is about; it must still hold when the email goes. Stays with no guest email and imported
// stays get none.
func (s *Postgres) queueStayEmail(ctx context.Context, accountID uuid.UUID, b domain.Booking, kind domain.EmailKind, on string, now time.Time) error {
	if b.Guest.Email == "" || b.Source.OTA() {
		return nil
	}
	bookingID := b.ID
	return s.queueEmail(ctx, queuedEmail{
		accountID: &accountID, bookingID: &bookingID, kind: kind, to: b.Guest.Email,
		payload: emailPayload{On: on}, dedupeKey: domain.GuestEmailKey(b.ID, kind, on), at: now,
	})
}

// queueAccountEmail queues an email to a person's confirmed address. It does nothing when the
// person has none.
func (s *Postgres) queueAccountEmail(ctx context.Context, userID uuid.UUID, kind domain.EmailKind, p emailPayload, now time.Time) error {
	u, err := s.db(ctx).ConfirmedEmail(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("confirmed email: %w", err)
	}
	p.Name = u.Name
	return s.queueEmail(ctx, queuedEmail{userID: &userID, kind: kind, to: u.Email, payload: p, at: now})
}

// SendEmails claims up to limit due emails, renders them and passes each to send, which returns
// the provider's message id. The claim and rendering are a short transaction; send runs after it
// commits, so no transaction stays open across a provider call. Emails a stay no longer wants,
// that a template has switched off, or to a reserved example domain become skipped; suppressed addresses become suppressed;
// a send with no message id (the log sender) counts as skipped. A failed send is tried again
// later, up to emailAttempts times. It returns how many were sent.
func (s *Postgres) SendEmails(ctx context.Context, limit int, now time.Time, send func(ctx context.Context, e Email) (string, error)) (int, error) {
	var (
		ready    []Email
		attempts = map[uuid.UUID]int16{}
	)
	err := s.InTx(ctx, func(ctx context.Context) error {
		rows, err := s.db(ctx).ClaimDueEmails(ctx, queries.ClaimDueEmailsParams{
			Now: now, StaleBefore: now.Add(-emailClaimStale), RowLimit: int32(limit),
		})
		if err != nil {
			return fmt.Errorf("claim emails: %w", err)
		}
		for _, r := range rows {
			e, status, err := s.renderEmail(ctx, r, now)
			if err != nil {
				return fmt.Errorf("render email %s: %w", r.ID, err)
			}
			if status != "" {
				if err := s.markEmail(ctx, r.ID, status, nil, ""); err != nil {
					return err
				}
				continue
			}
			attempts[r.ID] = r.Attempts
			ready = append(ready, e)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, e := range ready {
		id, sendErr := send(ctx, e)
		// The job's context may be why a send failed, so recording the result has its own deadline.
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := s.recordSend(recordCtx, e.ID, attempts[e.ID], id, sendErr, now)
		cancel()
		if err != nil {
			return sent, err
		}
		if sendErr == nil && id != "" {
			sent++
		}
	}
	return sent, nil
}

func (s *Postgres) recordSend(ctx context.Context, id uuid.UUID, attempts int16, messageID string, sendErr error, now time.Time) error {
	switch {
	case sendErr == nil && messageID != "":
		return s.markEmail(ctx, id, "sent", &now, messageID)
	case sendErr == nil:
		return s.markEmail(ctx, id, "skipped", nil, "")
	case attempts >= emailAttempts:
		msg := sendErr.Error()
		if err := s.db(ctx).MarkEmail(ctx, queries.MarkEmailParams{ID: id, Status: "failed", LastError: &msg}); err != nil {
			return fmt.Errorf("mark email failed: %w", err)
		}
		return nil
	default:
		retry, msg := now.Add(time.Duration(attempts*attempts)*5*time.Minute), sendErr.Error()
		if err := s.db(ctx).ReleaseEmail(ctx, queries.ReleaseEmailParams{ID: id, RetryAt: retry, LastError: &msg}); err != nil {
			return fmt.Errorf("release email: %w", err)
		}
		return nil
	}
}

func (s *Postgres) markEmail(ctx context.Context, id uuid.UUID, status string, sentAt *time.Time, messageID string) error {
	if err := s.db(ctx).MarkEmail(ctx, queries.MarkEmailParams{
		ID: id, Status: status, SentAt: sentAt, ProviderMessageID: nonEmpty(messageID),
	}); err != nil {
		return fmt.Errorf("mark email %s: %w", status, err)
	}
	return nil
}

// renderEmail writes a claimed email, or returns the status it ends in without sending.
func (s *Postgres) renderEmail(ctx context.Context, r queries.Email, now time.Time) (Email, string, error) {
	var p emailPayload
	if err := json.Unmarshal(r.Payload, &p); err != nil {
		return Email{}, "", fmt.Errorf("email payload: %w", err)
	}
	kind := domain.EmailKind(r.Kind)
	var (
		e      Email
		status string
		err    error
	)
	if kind.IsGuest() {
		e, status, err = s.renderGuestEmail(ctx, r, kind, p, now)
	} else {
		e, status, err = s.renderAccountEmail(ctx, r, kind, p)
	}
	if err != nil || status != "" {
		return Email{}, status, err
	}
	if domain.IsReservedAddress(e.To) {
		return Email{}, "skipped", nil
	}
	suppressed, err := s.db(ctx).IsSuppressed(ctx, e.To)
	if err != nil {
		return Email{}, "", fmt.Errorf("check suppression: %w", err)
	}
	if suppressed {
		return Email{}, "suppressed", nil
	}
	e.ID, e.Kind, e.BookingID, e.UserID = r.ID, kind, r.BookingID, r.UserID
	return e, "", nil
}

func (s *Postgres) renderAccountEmail(ctx context.Context, r queries.Email, kind domain.EmailKind, p emailPayload) (Email, string, error) {
	q := s.db(ctx)
	if r.AccountID != nil {
		closed, err := q.AccountClosed(ctx, *r.AccountID)
		if err != nil {
			return Email{}, "", fmt.Errorf("account status: %w", err)
		}
		if closed {
			return Email{}, "skipped", nil
		}
	}
	v := domain.EmailValues{Name: p.Name, Device: p.Device, Receipt: p.Receipt, Payout: p.Payout}
	if p.At != nil {
		v.At = *p.At
	}
	if p.FinalOn != nil {
		v.FinalOn = *p.FinalOn
	}
	if kind.CarriesConfirmation() {
		// The link only matters while it still confirms the person's current email.
		if r.UserID == nil {
			return Email{}, "skipped", nil
		}
		u, err := q.GetUser(ctx, *r.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Email{}, "skipped", nil
		}
		if err != nil {
			return Email{}, "", fmt.Errorf("get user: %w", err)
		}
		if u.Email == nil || *u.Email != r.ToAddress || u.EmailConfirmedAt != nil || u.DeletedAt != nil {
			return Email{}, "skipped", nil
		}
		token, err := s.secrets.Open(p.Token, r.ID[:])
		if err != nil {
			return Email{}, "", fmt.Errorf("open confirmation token: %w", err)
		}
		v.Link = s.links.VerifyEmail + "/" + string(token)
	}
	subject, text := domain.ComposeEmail(kind, v)
	return Email{
		To: r.ToAddress, FromName: "StayKey", Subject: subject, Text: text, Link: v.Link,
		Footer: "Sent by StayKey. https://staykey.direct",
	}, "", nil
}

func (s *Postgres) renderGuestEmail(ctx context.Context, r queries.Email, kind domain.EmailKind, p emailPayload, now time.Time) (Email, string, error) {
	if r.BookingID == nil || r.AccountID == nil {
		return Email{}, "skipped", nil
	}
	d, err := s.db(ctx).GuestEmailData(ctx, queries.GuestEmailDataParams{
		BookingID: *r.BookingID, AccountID: *r.AccountID, TemplateKey: kind.TemplateKey(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Email{}, "skipped", nil
	}
	if err != nil {
		return Email{}, "", fmt.Errorf("guest email data: %w", err)
	}
	stay := domain.EmailStay{
		Status: domain.BookingStatus(d.Status), CheckIn: d.CheckIn, CheckOut: d.CheckOut,
		PaymentDueAt: d.PaymentDueAt, Balance: d.Total - d.Paid,
	}
	switch {
	case domain.AccountStatus(d.AccountStatus) == domain.StatusClosed,
		d.GuestEmail == nil || *d.GuestEmail == "",
		domain.Source(d.Source).OTA(),
		kind.TemplateKey() != "" && !slices.Contains(d.TemplateChannels, "email"),
		!domain.StillWanted(kind, stay, p.On, now):
		return Email{}, "skipped", nil
	}
	loc, err := time.LoadLocation(d.PropertyTimeZone)
	if err != nil {
		loc = nil
	}
	page := ""
	if s.links.BookingPage != nil {
		page = s.links.BookingPage(d.PropertySlug)
	}
	v := domain.EmailValues{
		Name: d.GuestName, PropertyName: d.PropertyName, CheckIn: d.CheckIn, CheckOut: d.CheckOut,
		Status: stay.Status, PaymentMethod: deref(d.PaymentMethod), PaymentDueAt: d.PaymentDueAt, Location: loc,
		BookingPage: page,
	}
	if kind != domain.EmailDeclined && kind != domain.EmailThanks {
		token, err := s.guestToken(ctx, d)
		if err != nil {
			return Email{}, "", err
		}
		v.StayLink = page + "/stay/" + d.ID.String() + "?t=" + token
	}
	if kind.TemplateKey() != "" {
		mapLink := domain.MapLink(d.PropertyName, d.PropertyLocation, d.PropertyLat, d.PropertyLng)
		v.TemplateText = domain.RenderTemplate(d.TemplateBody, domain.TemplateValues(v, stay.Balance, d.Currency, d.PropertyCheckIn, mapLink))
	}
	subject, text := domain.ComposeEmail(kind, v)
	footer := "Sent by StayKey for " + d.PropertyName + "."
	if page != "" {
		footer += " " + page
	}
	return Email{
		To: *d.GuestEmail, FromName: d.PropertyName + " via StayKey", ReplyTo: d.OwnerEmail,
		Subject: subject, Text: text, Footer: footer, Link: v.StayLink,
	}, "", nil
}

// guestToken opens the stay's sealed guest token, or gives the stay one. A stay that already has
// a sealed token keeps it, so links in emails sent earlier keep working.
func (s *Postgres) guestToken(ctx context.Context, d queries.GuestEmailDataRow) (string, error) {
	if d.GuestTokenSealed != nil {
		token, err := s.secrets.Open(d.GuestTokenSealed, d.ID[:])
		if err != nil {
			return "", fmt.Errorf("open guest token: %w", err)
		}
		return string(token), nil
	}
	token, hash, err := domain.NewGuestToken()
	if err != nil {
		return "", err
	}
	if err := s.setGuestToken(ctx, d.AccountID, d.ID, token, hash); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Postgres) setGuestToken(ctx context.Context, accountID, bookingID uuid.UUID, token string, hash []byte) error {
	sealed, err := s.secrets.Seal([]byte(token), bookingID[:])
	if err != nil {
		return fmt.Errorf("seal guest token: %w", err)
	}
	if _, err := s.db(ctx).SetGuestToken(ctx, queries.SetGuestTokenParams{
		ID: bookingID, AccountID: accountID, TokenHash: hash, TokenSealed: sealed,
	}); err != nil {
		return fmt.Errorf("set guest token: %w", err)
	}
	return nil
}

// QueueTimedGuestEmails queues the reminders, pre-arrival notes and thank-you emails due within
// domain.EmailLookahead, each at its own time. It returns how many it queued.
func (s *Postgres) QueueTimedGuestEmails(ctx context.Context, now time.Time) (int, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	rows, err := s.db(ctx).TimedEmailCandidates(ctx, queries.TimedEmailCandidatesParams{
		DueAfter: now, DueBefore: now.Add(2 * domain.EmailLookahead),
		CheckInFrom: today.AddDate(0, 0, -1), CheckInTo: today.AddDate(0, 0, 4),
		CheckOutFrom: today.AddDate(0, 0, -3), CheckOutTo: today.AddDate(0, 0, 2),
	})
	if err != nil {
		return 0, fmt.Errorf("timed email candidates: %w", err)
	}
	queued := 0
	for _, r := range rows {
		loc, err := time.LoadLocation(r.TimeZone)
		if err != nil {
			loc = time.UTC
		}
		stay := domain.EmailStay{Status: domain.BookingStatus(r.Status), CheckIn: r.CheckIn, CheckOut: r.CheckOut, PaymentDueAt: r.PaymentDueAt}
		for _, kind := range []domain.EmailKind{domain.EmailReminder, domain.EmailPrearrival, domain.EmailThanks} {
			at, on, ok := domain.TimedEmail(kind, stay, loc, now)
			if !ok {
				continue
			}
			payload, err := json.Marshal(emailPayload{On: on})
			if err != nil {
				return queued, fmt.Errorf("email payload: %w", err)
			}
			accountID, bookingID := r.AccountID, r.ID
			n, err := s.db(ctx).QueueEmail(ctx, queries.QueueEmailParams{
				ID: domain.NewID(), AccountID: &accountID, BookingID: &bookingID, Kind: string(kind),
				ToAddress: deref(r.GuestEmail), Payload: payload,
				DedupeKey: nonEmpty(domain.GuestEmailKey(r.ID, kind, on)), ScheduledFor: at,
			})
			if err != nil {
				return queued, fmt.Errorf("queue %s email: %w", kind, err)
			}
			queued += int(n)
		}
	}
	return queued, nil
}

// RecordSuppression stops all email to an address SES reported as a permanent bounce or a
// complaint, and clears its confirmation on any person who uses it.
func (s *Postgres) RecordSuppression(ctx context.Context, address, reason string) error {
	if err := s.db(ctx).Suppress(ctx, queries.SuppressParams{Address: address, Reason: reason}); err != nil {
		return fmt.Errorf("suppress address: %w", err)
	}
	return nil
}

// ConfirmEmail uses a confirmation token and marks the person's email confirmed. It returns
// the address, or domain.ErrNotFound for an unknown, used or expired token, or one for an
// address the person has since changed.
func (s *Postgres) ConfirmEmail(ctx context.Context, token string, now time.Time) (string, error) {
	var address string
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		c, err := q.UseEmailConfirmation(ctx, queries.UseEmailConfirmationParams{
			TokenHash: domain.EmailConfirmationHash(token), Now: now,
		})
		if err != nil {
			return notFound(err)
		}
		n, err := q.ConfirmUserEmail(ctx, queries.ConfirmUserEmailParams{ID: c.UserID, Email: &c.Email, Now: now})
		if err != nil {
			return fmt.Errorf("confirm email: %w", err)
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		address = c.Email
		return nil
	})
	return address, err
}

// ResendConfirmation sends the person a new confirmation link for their email. It returns
// domain.ErrNoEmail when they have none and domain.ErrEmailConfirmed when it is confirmed.
func (s *Postgres) ResendConfirmation(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		u, err := s.db(ctx).GetUser(ctx, userID)
		if err != nil {
			return notFound(err)
		}
		switch {
		case u.Email == nil || *u.Email == "":
			return domain.ErrNoEmail
		case u.EmailConfirmedAt != nil:
			return domain.ErrEmailConfirmed
		}
		return s.sendConfirmation(ctx, userID, u.Name, *u.Email, domain.EmailConfirm, "", now)
	})
}

// sendConfirmation stores a confirmation link for address and queues the email carrying it. Past
// domain.ConfirmationsPerDay in a day it sends nothing; the person can send it again later.
func (s *Postgres) sendConfirmation(ctx context.Context, userID uuid.UUID, name, address string, kind domain.EmailKind, dedupeKey string, now time.Time) error {
	sent, err := s.db(ctx).CountConfirmationsSince(ctx, queries.CountConfirmationsSinceParams{UserID: userID, Since: now.Add(-24 * time.Hour)})
	if err != nil {
		return fmt.Errorf("count confirmations: %w", err)
	}
	if sent >= domain.ConfirmationsPerDay {
		return nil
	}
	token, hash, err := domain.NewEmailConfirmationToken()
	if err != nil {
		return err
	}
	if err := s.db(ctx).CreateEmailConfirmation(ctx, queries.CreateEmailConfirmationParams{
		ID: domain.NewID(), UserID: userID, Email: address, TokenHash: hash, ExpiresAt: now.Add(domain.ConfirmationTTL), Now: now,
	}); err != nil {
		return fmt.Errorf("create email confirmation: %w", err)
	}
	id := domain.NewID()
	sealed, err := s.secrets.Seal([]byte(token), id[:])
	if err != nil {
		return fmt.Errorf("seal confirmation token: %w", err)
	}
	return s.queueEmail(ctx, queuedEmail{
		id: id, userID: &userID, kind: kind, to: address, payload: emailPayload{Name: name, Token: sealed},
		dedupeKey: dedupeKey, at: now,
	})
}

// NoteSignIn queues a new sign-in email when the person has signed in before, but never on a
// device with this name. It runs before the new session is stored.
func (s *Postgres) NoteSignIn(ctx context.Context, userID uuid.UUID, deviceName string, now time.Time) error {
	q := s.db(ctx)
	n, err := q.CountSessions(ctx, userID)
	if err != nil {
		return fmt.Errorf("count sessions: %w", err)
	}
	if n == 0 {
		return nil
	}
	seen, err := q.HasSignedInOnDevice(ctx, queries.HasSignedInOnDeviceParams{UserID: userID, DeviceName: deviceName})
	if err != nil {
		return fmt.Errorf("check device: %w", err)
	}
	if seen {
		return nil
	}
	return s.queueAccountEmail(ctx, userID, domain.EmailNewSignIn, emailPayload{Device: deviceName, At: &now}, now)
}

// SkipPendingEmails marks the account's queued emails as skipped, so sample stays loaded by
// cmd/seed don't email their made-up guests.
func (s *Postgres) SkipPendingEmails(ctx context.Context, accountID uuid.UUID) error {
	if err := s.db(ctx).SkipAccountEmails(ctx, &accountID); err != nil {
		return fmt.Errorf("skip pending emails: %w", err)
	}
	return nil
}
