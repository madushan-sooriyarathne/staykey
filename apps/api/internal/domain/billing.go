package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Account statuses. The owner app works in every one; only the booking page stops taking stays
// once an account lapses.
const (
	StatusTrial  AccountStatus = "trial"
	StatusActive AccountStatus = "active"
	StatusGrace  AccountStatus = "grace"
	StatusLapsed AccountStatus = "lapsed"
	StatusClosed AccountStatus = "closed"
)

const (
	// FreeDays is the longest free period. The first direct booking ends it sooner.
	FreeDays = 60
	// GraceDays follow the end of free or paid access before the booking page pauses.
	GraceDays = 7
)

// PlanID names a plan. Every plan has every feature; they differ only in bookable units.
type PlanID string

const (
	PlanVilla      PlanID = "villa"
	PlanGuesthouse PlanID = "guesthouse"
	PlanCollection PlanID = "collection"
)

// Period is how long one payment lasts.
type Period string

const (
	PeriodMonthly Period = "monthly"
	PeriodYearly  Period = "yearly"
)

// Valid reports whether p is a known period.
func (p Period) Valid() bool { return p == PeriodMonthly || p == PeriodYearly }

// After is the end of one period that starts at t.
func (p Period) After(t time.Time) time.Time {
	if p == PeriodYearly {
		return t.AddDate(1, 0, 0)
	}
	return t.AddDate(0, 1, 0)
}

// Plan is one plan and its web prices. Store prices are set in App Store Connect and Google Play.
type Plan struct {
	ID    PlanID
	Name  string
	Units int
	// Web prices in LKR cents, since Payments.lk charges only in rupees.
	Monthly int64
	Yearly  int64
}

// WebCurrency is the currency web purchases are charged in.
const WebCurrency = "LKR"

// Plans are in order of size; the last one also covers anything larger.
var Plans = []Plan{
	{ID: PlanVilla, Name: "Villa", Units: 1, Monthly: 2_900_00, Yearly: 29_000_00},
	{ID: PlanGuesthouse, Name: "Guesthouse", Units: 6, Monthly: 5_900_00, Yearly: 59_000_00},
	{ID: PlanCollection, Name: "Collection", Units: 20, Monthly: 11_900_00, Yearly: 119_000_00},
}

// FindPlan returns the plan with this id.
func FindPlan(id PlanID) (Plan, bool) {
	for _, p := range Plans {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// Price is the web price for one period of the plan.
func (p Plan) Price(period Period) int64 {
	if period == PeriodYearly {
		return p.Yearly
	}
	return p.Monthly
}

// PlanFor is the smallest plan that covers this many bookable units.
func PlanFor(units int) PlanID {
	for _, p := range Plans {
		if units <= p.Units {
			return p.ID
		}
	}
	return Plans[len(Plans)-1].ID
}

// ParseProduct reads a store product id: staykey_<plan>_<monthly|yearly> on the App Store, and
// the same followed by ":<base plan>" on Google Play, or staykey_<plan>:<monthly|yearly> when the
// base plan names the period.
func ParseProduct(productID string) (PlanID, Period, bool) {
	id, basePlan, _ := strings.Cut(productID, ":")
	rest, ok := strings.CutPrefix(id, "staykey_")
	if !ok {
		return "", "", false
	}
	plan, period, hasPeriod := strings.Cut(rest, "_")
	if !hasPeriod {
		period = basePlan
	}
	if _, ok := FindPlan(PlanID(plan)); !ok || !Period(period).Valid() {
		return "", "", false
	}
	return PlanID(plan), Period(period), true
}

// SubscriptionSource is where a plan was bought.
type SubscriptionSource string

const (
	SourceAppStore  SubscriptionSource = "app_store"
	SourcePlayStore SubscriptionSource = "play_store"
	SourceWeb       SubscriptionSource = "web"
)

// IsStore reports whether the plan renews through an app store.
func (s SubscriptionSource) IsStore() bool { return s == SourceAppStore || s == SourcePlayStore }

// SubscriptionStatus is a plan's standing with its store.
type SubscriptionStatus string

const (
	SubActive       SubscriptionStatus = "active"
	SubCancelled    SubscriptionStatus = "cancelled"
	SubBillingIssue SubscriptionStatus = "billing_issue"
	SubExpired      SubscriptionStatus = "expired"
)

// Subscription is the plan an account pays for. A cancelled plan still gives access until
// PeriodEnd.
type Subscription struct {
	Source    SubscriptionSource
	Plan      PlanID
	Period    Period
	Status    SubscriptionStatus
	WillRenew bool
	PeriodEnd time.Time
	ProductID string
	// ExternalID is the store's original transaction id, or the web checkout id.
	ExternalID string
}

// GivesAccess reports whether the plan is paid up at now.
func (s *Subscription) GivesAccess(now time.Time) bool {
	return s != nil && s.Status != SubExpired && now.Before(s.PeriodEnd)
}

// GraceReason says why an account is in grace or lapsed.
type GraceReason string

const (
	GraceFirstBooking GraceReason = "first_booking"
	GraceFreeDays     GraceReason = "free_days"
	GracePlanEnded    GraceReason = "plan_ended"
)

// Billing is an account's standing at one moment.
type Billing struct {
	Status AccountStatus
	// Until is when the status next changes on its own: the end of the free period, of the paid
	// period, or of grace. For a lapsed account it is when grace ended.
	Until      time.Time
	FreeEndsAt time.Time
	Reason     GraceReason
}

// BillingState works out an account's status. Free access runs from the trial's start until
// the first direct booking, at most FreeDays. Paid access runs to the plan's period end. Grace
// follows whichever ends later, then the account lapses.
func BillingState(now, trialStartedAt time.Time, firstDirectAt *time.Time, sub *Subscription) Billing {
	freeEnd := trialStartedAt.AddDate(0, 0, FreeDays)
	reason := GraceFreeDays
	if firstDirectAt != nil && firstDirectAt.Before(freeEnd) {
		freeEnd = *firstDirectAt
		reason = GraceFirstBooking
	}
	if sub.GivesAccess(now) {
		return Billing{Status: StatusActive, Until: sub.PeriodEnd, FreeEndsAt: freeEnd}
	}
	if now.Before(freeEnd) {
		return Billing{Status: StatusTrial, Until: freeEnd, FreeEndsAt: freeEnd}
	}
	accessEnd := freeEnd
	if sub != nil && sub.PeriodEnd.After(freeEnd) {
		accessEnd = sub.PeriodEnd
		reason = GracePlanEnded
	}
	graceEnd := accessEnd.AddDate(0, 0, GraceDays)
	status := StatusGrace
	if !now.Before(graceEnd) {
		status = StatusLapsed
	}
	return Billing{Status: status, Until: graceEnd, FreeEndsAt: freeEnd, Reason: reason}
}

// ErrAlreadySubscribed means a store plan still gives access, so a web period can't be bought.
var ErrAlreadySubscribed = errors.New("an App Store or Google Play plan is active")

// AccountBilling is everything the subscription screen and the booking page need about an
// account's plan.
type AccountBilling struct {
	Billing
	// Sub is the plan bought last, or nil.
	Sub *Subscription
	// Units is the bookable units across the account's properties.
	Units int
	// OwnerPhone is the first owner's number.
	OwnerPhone string
}

// PaidPeriod is a web period that was just paid for.
type PaidPeriod struct {
	AccountID uuid.UUID
	PaymentID string
	Plan      PlanID
	Period    Period
	Start     time.Time
	End       time.Time
	Amount    int64
	Currency  string
}

// ProductID is the product id a period is recorded under, the same shape as the store ids.
func ProductID(plan PlanID, period Period) string {
	return "staykey_" + string(plan) + "_" + string(period)
}

// TakesBookings reports whether the booking page takes holds and stays in this status.
func TakesBookings(s AccountStatus) bool { return s != StatusLapsed && s != StatusClosed }
