package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Guest card payments through Payments.lk hosted checkouts.

// CardMode is where an account's card money goes: nowhere yet, the owner's own Payments.lk
// account, or StayKey's account to be paid out to the owner.
type CardMode string

const (
	CardModeOff     CardMode = "off"
	CardModeOwn     CardMode = "own"
	CardModePayouts CardMode = "payouts"
)

// Valid reports whether m is a known mode.
func (m CardMode) Valid() bool { return m == CardModeOff || m == CardModeOwn || m == CardModePayouts }

// Collector is who a card payment is taken by.
type Collector string

const (
	CollectedByOwner   Collector = "owner"
	CollectedByStayKey Collector = "staykey"
)

// Collector is who takes card payments in this mode.
func (m CardMode) Collector() Collector {
	if m == CardModePayouts {
		return CollectedByStayKey
	}
	return CollectedByOwner
}

// CardPurpose is the part of a stay a card checkout pays.
type CardPurpose string

const (
	PurposeDeposit CardPurpose = "deposit"
	PurposeFull    CardPurpose = "full"
	PurposeBalance CardPurpose = "balance"
)

// Card charges, the property setting for what a card pays when a guest books.
const (
	CardChargeDeposit = "deposit"
	CardChargeFull    = "full"
)

// CardCurrency is the only currency Payments.lk charges in.
const CardCurrency = "LKR"

// CardCheckoutMinutes is how long a guest has to pay a card checkout, and how long a stay
// booked by card waits for that payment.
const CardCheckoutMinutes = 60

// MinCardCharge and MaxCardCharge are the rupee cents a Payments.lk checkout can charge.
const (
	MinCardCharge int64 = 1000
	MaxCardCharge int64 = 100_000_000
)

// MaxCardRate is the largest rate an owner can set: Rs 100,000.00 per unit of their currency.
const MaxCardRate int64 = 10_000_000

// What can keep cards from being ready on a property, in the order the owner should fix them.
const (
	CardsNeedMode       = "mode"
	CardsNeedConnection = "connection"
	CardsNeedPayouts    = "payout_profile"
	CardsNeedRate       = "rate"
)

var (
	// ErrCardsOff means the property doesn't take cards right now.
	ErrCardsOff = errors.New("card payments are off for this property")
	// ErrNothingToPay means the stay owes nothing for that part.
	ErrNothingToPay = errors.New("nothing is left to pay")
	// ErrPayoutsUnavailable means the payouts mode can't be chosen on this server or account yet.
	ErrPayoutsUnavailable = errors.New("payouts aren't available")
	// ErrInvalidCardKey means Payments.lk refused the key, or it isn't a secret key.
	ErrInvalidCardKey = errors.New("payments.lk refused the key")
	// ErrTestKeyInProduction means a sandbox key was offered to a live server.
	ErrTestKeyInProduction = errors.New("test keys can't take real payments")
)

// CardState is an account's card setup as the server sees it.
type CardState struct {
	Mode       CardMode
	Connection *CardConnection
	// PayoutsReady is true when the server offers payouts and the account's payout details
	// are complete.
	PayoutsReady bool
	// CollectMode is the mode of StayKey's own collection key: test or live, or "" when the
	// server has none.
	CollectMode string
}

// CardConnection is an owner's own Payments.lk account, without its key.
type CardConnection struct {
	Last4     string
	Mode      string // test or live
	Status    string // connected or failed
	CheckedAt time.Time
}

// CardsMissing says what keeps cards from working on a property that has them switched on, or
// nil when they are ready. Properties priced in rupees need no rate.
func CardsMissing(st CardState, pay PaymentSettings, currency string) []string {
	var missing []string
	switch st.Mode {
	case CardModeOwn:
		if st.Connection == nil || st.Connection.Status != "connected" {
			missing = append(missing, CardsNeedConnection)
		}
	case CardModePayouts:
		if !st.PayoutsReady || st.CollectMode == "" {
			missing = append(missing, CardsNeedPayouts)
		}
	default:
		missing = append(missing, CardsNeedMode)
	}
	if currency != CardCurrency && pay.CardRate <= 0 {
		missing = append(missing, CardsNeedRate)
	}
	return missing
}

// KeyMode is the mode, test or live, of the key that takes payments in this state.
func (st CardState) KeyMode() string {
	if st.Mode == CardModePayouts {
		return st.CollectMode
	}
	if st.Connection != nil {
		return st.Connection.Mode
	}
	return ""
}

// CardAmount is what a card checkout for purpose charges, in the booking's currency: the
// deposit or the whole stay less what is paid already, or the balance. deposit is the
// property's deposit for the stay; charge is the property's card charge setting.
func CardAmount(purpose CardPurpose, status BookingStatus, total, deposit, paid int64, charge string) (int64, error) {
	var target int64
	switch {
	case status == StatusAwaitingPayment && purpose == PurposeFull:
		target = total
	case status == StatusAwaitingPayment && purpose == PurposeDeposit && charge == CardChargeDeposit:
		target = deposit
		if target <= 0 {
			target = total
		}
	case status == StatusConfirmed && purpose == PurposeBalance:
		target = total
	case status == StatusAwaitingPayment || status == StatusConfirmed:
		return 0, &ValidationError{Field: "purpose", Message: "can't be paid by card on this stay now"}
	default:
		return 0, fmt.Errorf("%w: a %s stay can't be paid by card", ErrBadTransition, status)
	}
	if left := target - paid; left > 0 {
		return left, nil
	}
	return 0, ErrNothingToPay
}

// ToLKR converts minor units of a two-decimal currency into rupee cents at rate (rupee cents
// per 1.00), rounding up so the owner is never short. A zero rate means the amount is already
// rupees.
func ToLKR(amount, rate int64) int64 {
	if rate <= 0 {
		return amount
	}
	return (amount*rate + 99) / 100
}

// FromLKR converts rupee cents back into minor units at rate, rounding down.
func FromLKR(cents, rate int64) int64 {
	if rate <= 0 {
		return cents
	}
	return cents * 100 / rate
}

// CardKeyMode says whether a Payments.lk key is a test or live secret key.
func CardKeyMode(key string) (string, error) {
	switch {
	case len(key) < 16 || len(key) > 200 || strings.ContainsAny(key, " \t\r\n"):
		return "", ErrInvalidCardKey
	case strings.HasPrefix(key, "sk_test_"):
		return "test", nil
	case strings.HasPrefix(key, "sk_live_"):
		return "live", nil
	}
	return "", ErrInvalidCardKey
}

// CardRefundSplit is how much of a card refund goes back to one card payment.
type CardRefundSplit struct {
	PaymentID string
	Amount    int64 // booking currency
	Charged   int64 // rupee cents
}

// RefundablePayment is a card payment with what is left of it after refunds.
type RefundablePayment struct {
	ID          string
	Rate        int64
	AmountLeft  int64
	ChargedLeft int64
}

// SplitCardRefund spreads a refund over card payments given newest first, taking what is left
// of each in turn. Refunding all that is left of a payment refunds all its rupees, so rounding
// never strands a few cents. It fails when the payments can't cover the amount.
func SplitCardRefund(amount int64, payments []RefundablePayment) ([]CardRefundSplit, error) {
	var out []CardRefundSplit
	left := amount
	for _, p := range payments {
		if left == 0 {
			break
		}
		if p.AmountLeft <= 0 || p.ChargedLeft <= 0 {
			continue
		}
		take := min(left, p.AmountLeft)
		charged := p.ChargedLeft
		if take < p.AmountLeft {
			charged = min(ToLKR(take, p.Rate), p.ChargedLeft)
		}
		out = append(out, CardRefundSplit{PaymentID: p.ID, Amount: take, Charged: charged})
		left -= take
	}
	if left > 0 {
		return nil, &ValidationError{Field: "refund.amount", Message: "can't be more than was paid by card"}
	}
	return out, nil
}
