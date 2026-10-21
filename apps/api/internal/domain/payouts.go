package domain

import (
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// StayKey payouts: card money StayKey collects for owners on the payouts mode, paid out by bank
// transfer every other Monday. Amounts here are rupee cents.

const (
	// PayoutMinimum is the smallest payout; smaller balances roll over to the next one.
	PayoutMinimum int64 = 1_000_00
	// PayoutEveryDays is how often payouts run.
	PayoutEveryDays = 14
	// PayoutHold is how long a change of bank account holds the next payout.
	PayoutHold = 72 * time.Hour
	// PayoutTermsVersion is the version of the payout terms owners accept.
	PayoutTermsVersion = "2026-10"
	// DefaultPayoutFeeBPS is StayKey's processing charge on payments it collects, in basis
	// points: 300 is 3%.
	DefaultPayoutFeeBPS = 300
)

// PayoutAnchor is the first payout Monday. Payouts run every PayoutEveryDays from it.
var PayoutAnchor = time.Date(2026, time.November, 2, 0, 0, 0, 0, time.UTC)

// PayoutEntryKind is what moved money in an owner's payout ledger.
type PayoutEntryKind string

const (
	EntryPayment        PayoutEntryKind = "payment"
	EntryProcessing     PayoutEntryKind = "processing"
	EntryRefund         PayoutEntryKind = "refund"
	EntryRefundReturned PayoutEntryKind = "refund_returned"
	EntryAdjustment     PayoutEntryKind = "adjustment"
	EntryPayout         PayoutEntryKind = "payout"
	EntryPayoutReturned PayoutEntryKind = "payout_returned"
)

// PayoutDay is today's date in Sri Lanka, where payout days are counted, at midnight UTC like
// every other stay date.
func PayoutDay(now time.Time) time.Time {
	y, m, d := now.In(billingLocation).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// NextPayoutDay is the first payout Monday on or after day.
func NextPayoutDay(day time.Time) time.Time {
	if !day.After(PayoutAnchor) {
		return PayoutAnchor
	}
	days := int(day.Sub(PayoutAnchor).Hours() / 24)
	cycles := (days + PayoutEveryDays - 1) / PayoutEveryDays
	return PayoutAnchor.AddDate(0, 0, cycles*PayoutEveryDays)
}

// LatestPayoutDay is the last payout Monday on or before day, and false before the first one.
func LatestPayoutDay(day time.Time) (time.Time, bool) {
	if day.Before(PayoutAnchor) {
		return time.Time{}, false
	}
	days := int(day.Sub(PayoutAnchor).Hours() / 24)
	return PayoutAnchor.AddDate(0, 0, days/PayoutEveryDays*PayoutEveryDays), true
}

// PeriodEnd is the last day a payout covers: the Sunday before its Monday.
func PeriodEnd(payoutDay time.Time) time.Time { return payoutDay.AddDate(0, 0, -1) }

// AvailableOn is when money for a stay becomes payable: the day after check-in.
func AvailableOn(checkIn time.Time) time.Time { return checkIn.AddDate(0, 0, 1) }

// ProcessingCharge is StayKey's charge on a payment it collected, rounded up to the cent.
func ProcessingCharge(amount int64, bps int) int64 {
	if amount <= 0 || bps <= 0 {
		return 0
	}
	return (amount*int64(bps) + 9_999) / 10_000
}

// PayoutProfileInput is the payout details an owner gives.
type PayoutProfileInput struct {
	LegalName     string
	NIC           string
	BankName      string
	Branch        string
	AccountName   string
	AccountNumber string
	AcceptTerms   string // the terms version accepted
}

// Normalize trims the input and strips the separators people type in NICs and account numbers.
func (in *PayoutProfileInput) Normalize() {
	in.LegalName = strings.Join(strings.Fields(in.LegalName), " ")
	in.BankName = strings.Join(strings.Fields(in.BankName), " ")
	in.Branch = strings.Join(strings.Fields(in.Branch), " ")
	in.AccountName = strings.Join(strings.Fields(in.AccountName), " ")
	in.NIC = strings.ToUpper(stripSeparators(in.NIC))
	in.AccountNumber = stripSeparators(in.AccountNumber)
	in.AcceptTerms = strings.TrimSpace(in.AcceptTerms)
}

func stripSeparators(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return r
	}, s)
}

// Validate checks normalized payout details.
func (in PayoutProfileInput) Validate() error {
	switch {
	case !between(in.LegalName, 2, 120):
		return &ValidationError{Field: "legalName", Message: "must be 2 to 120 characters"}
	case !ValidNIC(in.NIC):
		return &ValidationError{Field: "nic", Message: "must be a Sri Lankan NIC number, 9 digits and V or X, or 12 digits"}
	case !between(in.BankName, 2, 80):
		return &ValidationError{Field: "bankName", Message: "must be 2 to 80 characters"}
	case !between(in.Branch, 2, 80):
		return &ValidationError{Field: "branch", Message: "must be 2 to 80 characters"}
	case !between(in.AccountName, 2, 120):
		return &ValidationError{Field: "accountName", Message: "must be 2 to 120 characters"}
	case !validAccountNumber(in.AccountNumber):
		return &ValidationError{Field: "accountNumber", Message: "must be 6 to 20 digits"}
	case in.AcceptTerms != PayoutTermsVersion:
		return &ValidationError{Field: "acceptTerms", Message: "must accept the current payout terms (" + PayoutTermsVersion + ")"}
	}
	return nil
}

func between(s string, lo, hi int) bool {
	n := len([]rune(s))
	return n >= lo && n <= hi
}

// ValidNIC reports whether s is a Sri Lankan NIC number in the old format (9 digits and V or X)
// or the new one (12 digits).
func ValidNIC(s string) bool {
	switch len(s) {
	case 10:
		return allDigits(s[:9]) && (s[9] == 'V' || s[9] == 'X')
	case 12:
		return allDigits(s)
	}
	return false
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// Last4 is the last four characters of s, as shown for sealed details.
func Last4(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}

// PayoutProfile is an owner's payout details as shown back: sealed numbers by their last four.
type PayoutProfile struct {
	LegalName     string
	NICLast4      string
	BankName      string
	Branch        string
	AccountName   string
	AccountLast4  string
	TermsVersion  string
	AcceptedAt    time.Time
	BankChangedAt time.Time
}

// HeldUntil is when a recent change of bank account stops holding payouts, if it still does.
func (p PayoutProfile) HeldUntil(now time.Time) *time.Time {
	until := p.BankChangedAt.Add(PayoutHold)
	if !until.After(now) {
		return nil
	}
	return &until
}

// PayoutStatus is where a payout is.
type PayoutStatus string

const (
	PayoutPrepared PayoutStatus = "prepared"
	PayoutPaid     PayoutStatus = "paid"
	PayoutFailed   PayoutStatus = "failed"
)

// Payout is one bank transfer to an owner.
type Payout struct {
	ID         uuid.UUID
	PeriodEnd  time.Time
	Amount     int64
	Status     PayoutStatus
	Reason     string
	PreparedAt time.Time
	PaidAt     *time.Time
	FailedAt   *time.Time
}

// Payouts is an account's payout balances and history. Available is payable in the next payout,
// Pending waits for stays to start, OnTheWay is in prepared payouts not yet paid.
type Payouts struct {
	Available     int64
	Pending       int64
	OnTheWay      int64
	NextPayoutDay time.Time
	HeldUntil     *time.Time
	History       []Payout
}

// StatementLine is one entry a payout paid.
type StatementLine struct {
	ID          uuid.UUID
	Kind        PayoutEntryKind
	Amount      int64
	AvailableOn time.Time
	Note        string
	BookingID   *uuid.UUID
	BookingRef  string
	GuestName   string
	CheckIn     *time.Time
}

// PayoutStatement is a payout with the entries it paid.
type PayoutStatement struct {
	Payout
	Lines []StatementLine
}
