package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EmailKind is what an email is for. Account kinds go to a person, guest kinds to a stay's guest.
type EmailKind string

const (
	EmailWelcome           EmailKind = "welcome"
	EmailConfirm           EmailKind = "confirm_email"
	EmailChanged           EmailKind = "email_changed"
	EmailNewSignIn         EmailKind = "new_sign_in"
	EmailDeletionScheduled EmailKind = "deletion_scheduled"
	EmailAccountRestored   EmailKind = "account_restored"
	EmailDeletionCompleted EmailKind = "deletion_completed"
	EmailPlanReceipt       EmailKind = "plan_receipt"

	EmailReceived   EmailKind = "received"
	EmailConfirmed  EmailKind = "confirmed"
	EmailDeclined   EmailKind = "declined"
	EmailReminder   EmailKind = "reminder"
	EmailPrearrival EmailKind = "prearrival"
	EmailThanks     EmailKind = "thanks"
)

// EmailKinds lists every kind, account kinds first.
var EmailKinds = []EmailKind{
	EmailWelcome, EmailConfirm, EmailChanged, EmailNewSignIn, EmailDeletionScheduled, EmailAccountRestored,
	EmailDeletionCompleted, EmailPlanReceipt,
	EmailReceived, EmailConfirmed, EmailDeclined, EmailReminder, EmailPrearrival, EmailThanks,
}

// IsGuest reports whether the kind goes to a stay's guest.
func (k EmailKind) IsGuest() bool {
	switch k {
	case EmailReceived, EmailConfirmed, EmailDeclined, EmailReminder, EmailPrearrival, EmailThanks:
		return true
	}
	return false
}

// TemplateKey is the message template the kind's text comes from, or "" for system text.
func (k EmailKind) TemplateKey() string {
	switch k {
	case EmailConfirmed, EmailReminder, EmailPrearrival, EmailThanks:
		return string(k)
	}
	return ""
}

// CarriesConfirmation reports whether the kind is the confirmation link itself, the one email
// that goes to an address before it is confirmed.
func (k EmailKind) CarriesConfirmation() bool { return k == EmailWelcome || k == EmailConfirm }

// IsTimed reports whether the scheduling job queues the kind, rather than an event.
func (k EmailKind) IsTimed() bool {
	return k == EmailReminder || k == EmailPrearrival || k == EmailThanks
}

var (
	// ErrEmailConfirmed means the person's email is already confirmed.
	ErrEmailConfirmed = errors.New("email already confirmed")
	// ErrNoEmail means the person has no email to confirm.
	ErrNoEmail = errors.New("no email")
)

// NewGuestToken returns a token that opens a stay's guest page, and the hash stored for it.
func NewGuestToken() (string, []byte, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("guest token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, GuestTokenHash(token), nil
}

// GuestTokenHash is the stored form of a guest token.
func GuestTokenHash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// IsReservedAddress reports whether an address is on a domain reserved for examples and tests
// (RFC 2606 and RFC 6761), such as the seed's sample guests at example.com. Mail to them can
// only bounce, which counts against the sender, so it is never sent.
func IsReservedAddress(address string) bool {
	_, host, ok := strings.Cut(strings.ToLower(strings.TrimSpace(address)), "@")
	if !ok {
		return true
	}
	switch host {
	case "example.com", "example.net", "example.org", "localhost":
		return true
	}
	for _, tld := range []string{".example", ".test", ".invalid", ".localhost"} {
		if strings.HasSuffix(host, tld) {
			return true
		}
	}
	return strings.HasSuffix(host, ".example.com") || strings.HasSuffix(host, ".example.net") || strings.HasSuffix(host, ".example.org")
}

// ConfirmationsPerDay caps the confirmation emails one person can cause in a day, so typing
// other people's addresses into Profile can't be used to flood their inboxes.
const ConfirmationsPerDay = 5

// ConfirmationTTL is how long an email confirmation link works.
const ConfirmationTTL = 7 * 24 * time.Hour

// NewEmailConfirmationToken returns a confirmation token and the hash stored for it.
func NewEmailConfirmationToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("confirmation token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, EmailConfirmationHash(token), nil
}

// EmailConfirmationHash is the stored form of a confirmation token.
func EmailConfirmationHash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

// RenderTemplate fills {placeholders} in a template body. Unknown placeholders stay as they
// are, so a typo shows rather than vanishing.
func RenderTemplate(body string, values map[string]string) string {
	return placeholder.ReplaceAllStringFunc(body, func(m string) string {
		if v, ok := values[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

// EmailValues is what composed emails are written from. Each kind uses some of them.
type EmailValues struct {
	// Name is the person's or the guest's name; emails use the first word.
	Name string

	// Guest emails.
	PropertyName  string
	CheckIn       time.Time
	CheckOut      time.Time
	Status        BookingStatus
	PaymentMethod string
	PaymentDueAt  *time.Time
	Location      *time.Location
	StayLink      string
	BookingPage   string
	// TemplateText is the account's template, already rendered, for template kinds.
	TemplateText string

	// Account emails.
	Link    string
	Device  string
	At      time.Time
	FinalOn time.Time
	Receipt *PaidPeriod
}

// ComposeEmail writes an email's subject and plain text. Template kinds wrap the account's
// template; the rest are StayKey's own words. Only receipts mention an amount.
func ComposeEmail(kind EmailKind, v EmailValues) (subject, text string) {
	hi := "Hi " + firstWord(v.Name) + ","
	if firstWord(v.Name) == "" {
		hi = "Hi,"
	}
	dates := FormatDay(v.CheckIn) + " to " + FormatDay(v.CheckOut)
	stay := "\n\nYour booking: " + v.StayLink
	switch kind {
	case EmailWelcome:
		return "Welcome to StayKey", hi + "\n\n" +
			"Thanks for setting up StayKey. Please confirm this is your email, so booking updates and account notices reach you:\n\n" +
			v.Link + "\n\n" + confirmFootnote
	case EmailConfirm:
		return "Confirm your email for StayKey", hi + "\n\n" +
			"Please confirm this is the email you want StayKey to use:\n\n" + v.Link + "\n\n" + confirmFootnote
	case EmailChanged:
		return "Your StayKey email was changed", hi + "\n\n" +
			"The email on your StayKey account was changed on " + FormatMoment(v.At, v.Location) +
			", so this address won't get StayKey emails any more.\n\n" +
			"If you didn't make this change, sign in to StayKey and set your email again in Profile."
	case EmailNewSignIn:
		return "New sign-in to StayKey", hi + "\n\n" +
			"Your StayKey account was signed in on " + v.Device + " on " + FormatMoment(v.At, v.Location) + ".\n\n" +
			"If this was you, there's nothing to do. If it wasn't, someone may have your phone or its codes. " +
			"Write to " + SupportEmail + " and we'll sign out every device."
	case EmailDeletionScheduled:
		return "Your StayKey account will be deleted", hi + "\n\n" +
			"You asked StayKey to delete your account. You're signed out everywhere and the booking pages of the accounts you own are offline.\n\n" +
			"Everything is deleted for good on " + FormatDate(v.FinalOn, v.Location) + ". " +
			"Changed your mind? Sign in again with the same phone number before then and it all comes back."
	case EmailAccountRestored:
		return "Your StayKey account is back", hi + "\n\n" +
			"Signing in again cancelled the deletion. Your accounts and booking pages are back as they were."
	case EmailDeletionCompleted:
		return "Your StayKey account has been deleted", hi + "\n\n" +
			"Your StayKey account and the accounts you owned are now deleted for good. Thank you for using StayKey."
	case EmailPlanReceipt:
		r := v.Receipt
		if r == nil {
			r = &PaidPeriod{}
		}
		name, length := string(r.Plan), "monthly"
		if p, ok := FindPlan(r.Plan); ok {
			name = p.Name
		}
		if r.Period == PeriodYearly {
			length = "yearly"
		}
		return "Your StayKey receipt", hi + "\n\n" +
			"Thank you for your payment.\n\n" +
			"Plan: " + name + ", " + length + "\n" +
			"Amount: " + FormatMoney(r.Amount, r.Currency) + "\n" +
			"Period: " + FormatDate(r.Start, v.Location) + " to " + FormatDate(r.End, v.Location) + "\n" +
			"Payment ID: " + r.PaymentID + "\n\n" +
			"Keep this email for your records."

	case EmailReceived:
		switch v.Status {
		case StatusRequested:
			return "Your request to stay at " + v.PropertyName, hi + "\n\n" +
				"Thanks for your request to stay at " + v.PropertyName + ", " + dates + ". " +
				"The owner will reply soon, and we'll email you when they do." + stay
		case StatusAwaitingPayment:
			due := ""
			if v.PaymentDueAt != nil {
				due = " by " + FormatMoment(*v.PaymentDueAt, v.Location)
			}
			return "Your booking at " + v.PropertyName, hi + "\n\n" +
				"Your stay at " + v.PropertyName + ", " + dates + ", is held for you. " +
				"Pay by bank transfer" + due + " and upload the slip on your booking page to confirm it." + stay
		default:
			pay := ""
			if v.PaymentMethod == "property" {
				pay = " You'll pay at the property."
			}
			return "Your stay at " + v.PropertyName + " is confirmed", hi + "\n\n" +
				"Your stay at " + v.PropertyName + ", " + dates + ", is confirmed." + pay + stay
		}
	case EmailDeclined:
		again := ""
		if v.BookingPage != "" {
			again = " You can look for other dates here: " + v.BookingPage
		}
		return "Your request to stay at " + v.PropertyName, hi + "\n\n" +
			"Sorry, " + v.PropertyName + " can't take your stay, " + dates + ". Nothing was charged." + again
	case EmailConfirmed:
		return "Your stay at " + v.PropertyName + " is confirmed", v.TemplateText + stay
	case EmailReminder:
		return "Payment reminder for " + v.PropertyName, v.TemplateText + stay
	case EmailPrearrival:
		return "See you soon at " + v.PropertyName, v.TemplateText + stay
	case EmailThanks:
		return "Thank you for staying at " + v.PropertyName, v.TemplateText
	}
	return "", ""
}

// SupportEmail is where people write to StayKey.
const SupportEmail = "support@staykey.direct"

const confirmFootnote = "The link works once and lasts 7 days. If you didn't add this address to StayKey, you can ignore this email."

// TemplateValues are a stay's values for the template placeholders.
func TemplateValues(v EmailValues, balance int64, currency, checkInTime, mapLink string) map[string]string {
	return map[string]string{
		"guest_name":    firstWord(v.Name),
		"property_name": v.PropertyName,
		"check_in":      FormatDay(v.CheckIn),
		"check_out":     FormatDay(v.CheckOut),
		"balance":       FormatMoney(balance, currency),
		"booking_link":  v.BookingPage,
		"map_link":      mapLink,
		"check_in_time": FormatClock(checkInTime),
	}
}

// MapLink is a Google Maps link for a property: its pin when set, else its name and location.
func MapLink(name string, location *string, lat, lng *float64) string {
	if lat != nil && lng != nil {
		return "https://maps.google.com/?q=" + strconv.FormatFloat(*lat, 'f', -1, 64) + "," + strconv.FormatFloat(*lng, 'f', -1, 64)
	}
	q := name
	if location != nil && *location != "" {
		q += " " + *location
	}
	return "https://maps.google.com/?q=" + strings.ReplaceAll(url.QueryEscape(q), "+", "%20")
}

// EmailStay is what a guest email's timing and relevance are worked out from.
type EmailStay struct {
	Status       BookingStatus
	CheckIn      time.Time
	CheckOut     time.Time
	PaymentDueAt *time.Time
	Balance      int64
}

// EmailLookahead is how far ahead the scheduling job queues timed guest emails.
const EmailLookahead = 24 * time.Hour

// ReminderAt is when a payment reminder goes: a day before the deadline.
func ReminderAt(due time.Time) time.Time { return due.Add(-24 * time.Hour) }

// PrearrivalAt is 09:00 at the property two days before check-in.
func PrearrivalAt(checkIn time.Time, loc *time.Location) time.Time {
	return time.Date(checkIn.Year(), checkIn.Month(), checkIn.Day()-2, 9, 0, 0, 0, loc)
}

// ThanksAt is 10:00 at the property the day after check-out.
func ThanksAt(checkOut time.Time, loc *time.Location) time.Time {
	return time.Date(checkOut.Year(), checkOut.Month(), checkOut.Day()+1, 10, 0, 0, 0, loc)
}

// TimedEmail works out when a timed guest email is due for a stay and whether to queue it at
// now. It is queued once it is due within EmailLookahead, and only while it is still useful: a
// reminder no later than an hour past its time and before the deadline, a pre-arrival note
// before check-in day starts, a thank-you within two days. on is what the email is keyed on, so
// a stay that moves gets a new one.
func TimedEmail(kind EmailKind, s EmailStay, loc *time.Location, now time.Time) (at time.Time, on string, ok bool) {
	var latest time.Time
	switch kind {
	case EmailReminder:
		if s.Status != StatusAwaitingPayment || s.PaymentDueAt == nil {
			return time.Time{}, "", false
		}
		at, on = ReminderAt(*s.PaymentDueAt), s.PaymentDueAt.UTC().Format(time.RFC3339)
		latest = at.Add(time.Hour)
		if s.PaymentDueAt.Before(latest) {
			latest = *s.PaymentDueAt
		}
	case EmailPrearrival:
		if s.Status != StatusConfirmed && s.Status != StatusAwaitingPayment {
			return time.Time{}, "", false
		}
		at, on = PrearrivalAt(s.CheckIn, loc), s.CheckIn.Format(time.DateOnly)
		latest = time.Date(s.CheckIn.Year(), s.CheckIn.Month(), s.CheckIn.Day(), 0, 0, 0, 0, loc)
	case EmailThanks:
		if !stayHappened(s.Status) {
			return time.Time{}, "", false
		}
		at, on = ThanksAt(s.CheckOut, loc), s.CheckOut.Format(time.DateOnly)
		latest = at.Add(48 * time.Hour)
	default:
		return time.Time{}, "", false
	}
	return at, on, !at.After(now.Add(EmailLookahead)) && now.Before(latest)
}

// StillWanted reports whether a queued guest email should still go, given the stay as it is at
// send time and what the email was keyed on when queued: a date or deadline for timed kinds,
// and for a booking email sent again on approval, the status it describes.
func StillWanted(kind EmailKind, s EmailStay, on string, now time.Time) bool {
	switch kind {
	case EmailReceived:
		if on != "" {
			return string(s.Status) == on
		}
		return s.Status != StatusCancelled && s.Status != StatusDeclined
	case EmailConfirmed:
		// An owner entering a stay that already ended, say when moving their bookings over,
		// doesn't email its guest.
		return (s.Status == StatusConfirmed || s.Status == StatusCheckedIn) && s.CheckOut.After(now)
	case EmailDeclined:
		return s.Status == StatusDeclined
	case EmailReminder:
		return s.Status == StatusAwaitingPayment && s.Balance > 0 && s.PaymentDueAt != nil &&
			now.Before(*s.PaymentDueAt) && s.PaymentDueAt.UTC().Format(time.RFC3339) == on
	case EmailPrearrival:
		return (s.Status == StatusConfirmed || s.Status == StatusAwaitingPayment) && s.CheckIn.Format(time.DateOnly) == on
	case EmailThanks:
		return stayHappened(s.Status) && s.CheckOut.Format(time.DateOnly) == on
	}
	return false
}

func stayHappened(s BookingStatus) bool {
	return s == StatusConfirmed || s == StatusCheckedIn || s == StatusCheckedOut
}

// GuestEmailKey is the dedupe key of a guest email: one per stay and kind, and for timed
// kinds one per date they are keyed on.
func GuestEmailKey(bookingID uuid.UUID, kind EmailKind, on string) string {
	key := bookingID.String() + ":" + string(kind)
	if on != "" {
		key += ":" + on
	}
	return key
}

// FormatDay is a stay date the way guests read it, for example "Sun 4 Oct".
func FormatDay(d time.Time) string { return d.Format("Mon 2 Jan") }

// FormatDate is a calendar date in loc, for example "18 Oct 2026".
func FormatDate(t time.Time, loc *time.Location) string {
	return t.In(orColombo(loc)).Format("2 Jan 2006")
}

// FormatMoment is a time of day in loc, for example "18 Oct 2026, 2:05 PM".
func FormatMoment(t time.Time, loc *time.Location) string {
	return t.In(orColombo(loc)).Format("2 Jan 2006, 3:04 PM")
}

// FormatClock turns "14:00" into "2:00 PM".
func FormatClock(hhmm string) string {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return hhmm
	}
	return t.Format("3:04 PM")
}

// FormatMoney writes minor units with the currency, for example "LKR 18,000" or "$180.50".
func FormatMoney(amount int64, currency string) string {
	sign := ""
	if amount < 0 {
		sign, amount = "-", -amount
	}
	whole, cents := amount/100, amount%100
	s := groupThousands(whole)
	if cents != 0 {
		s += fmt.Sprintf(".%02d", cents)
	}
	switch currency {
	case "USD":
		return sign + "$" + s
	case "EUR":
		return sign + "€" + s
	case "GBP":
		return sign + "£" + s
	}
	return sign + currency + " " + s
}

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func firstWord(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}
	return ""
}

func orColombo(loc *time.Location) *time.Location {
	if loc == nil {
		return billingLocation
	}
	return loc
}
