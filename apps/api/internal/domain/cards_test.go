package domain

import (
	"errors"
	"slices"
	"testing"
)

func TestCardAmount(t *testing.T) {
	tests := []struct {
		name    string
		purpose CardPurpose
		status  BookingStatus
		paid    int64
		charge  string
		want    int64
	}{
		{"deposit when the property asks for one", PurposeDeposit, StatusAwaitingPayment, 0, CardChargeDeposit, 30_000},
		{"deposit less a part already paid", PurposeDeposit, StatusAwaitingPayment, 10_000, CardChargeDeposit, 20_000},
		{"the whole stay", PurposeFull, StatusAwaitingPayment, 0, CardChargeDeposit, 100_000},
		{"the whole stay on a property that charges in full", PurposeFull, StatusAwaitingPayment, 0, CardChargeFull, 100_000},
		{"the balance of a confirmed stay", PurposeBalance, StatusConfirmed, 30_000, CardChargeDeposit, 70_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CardAmount(tt.purpose, tt.status, 100_000, 30_000, tt.paid, tt.charge)
			if err != nil || got != tt.want {
				t.Errorf("CardAmount = %d, %v; want %d", got, err, tt.want)
			}
		})
	}
}

func TestCardAmountWithNoDepositIsTheWholeStay(t *testing.T) {
	got, err := CardAmount(PurposeDeposit, StatusAwaitingPayment, 100_000, 0, 0, CardChargeDeposit)
	if err != nil || got != 100_000 {
		t.Errorf("CardAmount = %d, %v; want 100000", got, err)
	}
}

func TestCardAmountRefusesWhatDoesntApply(t *testing.T) {
	tests := []struct {
		name    string
		purpose CardPurpose
		status  BookingStatus
		charge  string
	}{
		{"a deposit where the property charges in full", PurposeDeposit, StatusAwaitingPayment, CardChargeFull},
		{"a balance before the stay is confirmed", PurposeBalance, StatusAwaitingPayment, CardChargeDeposit},
		{"a deposit on a confirmed stay", PurposeDeposit, StatusConfirmed, CardChargeDeposit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CardAmount(tt.purpose, tt.status, 100_000, 30_000, 0, tt.charge)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("CardAmount error = %v, want a validation error", err)
			}
		})
	}
}

func TestCardAmountRefusesStaysThatTakeNoPayment(t *testing.T) {
	for _, status := range []BookingStatus{StatusRequested, StatusCheckedIn, StatusCancelled, StatusDeclined} {
		t.Run(string(status), func(t *testing.T) {
			if _, err := CardAmount(PurposeFull, status, 100_000, 30_000, 0, CardChargeDeposit); !errors.Is(err, ErrBadTransition) {
				t.Errorf("CardAmount error = %v, want ErrBadTransition", err)
			}
		})
	}
}

func TestCardAmountWithNothingLeft(t *testing.T) {
	if _, err := CardAmount(PurposeDeposit, StatusAwaitingPayment, 100_000, 30_000, 30_000, CardChargeDeposit); !errors.Is(err, ErrNothingToPay) {
		t.Errorf("CardAmount error = %v, want ErrNothingToPay", err)
	}
}

func TestToLKR(t *testing.T) {
	tests := []struct {
		name         string
		amount, rate int64
		want         int64
	}{
		{"rupees stay rupees", 1_674_000, 0, 1_674_000},
		{"whole dollars at a whole rate", 18_000, 30_000, 5_400_000},
		{"a fraction of a cent rounds up", 1, 30_050, 301},
		{"cents at an uneven rate", 12_345, 29_876, 3_688_193},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToLKR(tt.amount, tt.rate); got != tt.want {
				t.Errorf("ToLKR(%d, %d) = %d, want %d", tt.amount, tt.rate, got, tt.want)
			}
		})
	}
}

func TestFromLKR(t *testing.T) {
	tests := []struct {
		name        string
		cents, rate int64
		want        int64
	}{
		{"rupees stay rupees", 1_674_000, 0, 1_674_000},
		{"a whole conversion", 5_400_000, 30_000, 18_000},
		{"a fraction of a cent rounds down", 3_688_193, 29_876, 12_345},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromLKR(tt.cents, tt.rate); got != tt.want {
				t.Errorf("FromLKR(%d, %d) = %d, want %d", tt.cents, tt.rate, got, tt.want)
			}
		})
	}
}

func TestCardsMissing(t *testing.T) {
	connected := &CardConnection{Last4: "a1b2", Mode: "test", Status: "connected"}
	failed := &CardConnection{Last4: "a1b2", Mode: "test", Status: "failed"}
	rated := PaymentSettings{Cards: "on", CardRate: 30_000}
	tests := []struct {
		name     string
		st       CardState
		pay      PaymentSettings
		currency string
		want     []string
	}{
		{"own account, connected, priced in rupees", CardState{Mode: CardModeOwn, Connection: connected}, PaymentSettings{Cards: "on"}, "LKR", nil},
		{"own account with a rate", CardState{Mode: CardModeOwn, Connection: connected}, rated, "USD", nil},
		{"no mode chosen", CardState{Mode: CardModeOff}, rated, "USD", []string{CardsNeedMode}},
		{"own account without a connection", CardState{Mode: CardModeOwn}, rated, "USD", []string{CardsNeedConnection}},
		{"own account whose key stopped working", CardState{Mode: CardModeOwn, Connection: failed}, rated, "USD", []string{CardsNeedConnection}},
		{"no rate for dollars", CardState{Mode: CardModeOwn, Connection: connected}, PaymentSettings{Cards: "on"}, "USD", []string{CardsNeedRate}},
		{"payouts ready", CardState{Mode: CardModePayouts, PayoutsReady: true, CollectMode: "live"}, rated, "USD", nil},
		{"payouts without details", CardState{Mode: CardModePayouts, CollectMode: "live"}, rated, "USD", []string{CardsNeedPayouts}},
		{"payouts on a server without a collection key", CardState{Mode: CardModePayouts, PayoutsReady: true}, rated, "USD", []string{CardsNeedPayouts}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CardsMissing(tt.st, tt.pay, tt.currency); !slices.Equal(got, tt.want) {
				t.Errorf("CardsMissing = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCardKeyMode(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"sk_test_fixture-key-0001", "test"},
		{"sk_live_fixture-key-0001", "live"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got, err := CardKeyMode(tt.key); err != nil || got != tt.want {
				t.Errorf("CardKeyMode = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestCardKeyModeRefusesOtherKeys(t *testing.T) {
	for _, key := range []string{"pk_live_fixture-key-0001", "ak_test_fixture-key-0001", "sk_test_short", "sk_live_fixture key-0001", ""} {
		t.Run(key, func(t *testing.T) {
			if _, err := CardKeyMode(key); !errors.Is(err, ErrInvalidCardKey) {
				t.Errorf("CardKeyMode(%q) error = %v, want ErrInvalidCardKey", key, err)
			}
		})
	}
}

func TestSplitCardRefund(t *testing.T) {
	payments := []RefundablePayment{
		{ID: "pay_new", Rate: 30_000, AmountLeft: 7_000, ChargedLeft: 2_100_000},
		{ID: "pay_old", Rate: 29_000, AmountLeft: 3_000, ChargedLeft: 870_000},
	}
	tests := []struct {
		name   string
		amount int64
		want   []CardRefundSplit
	}{
		{"part of the newest payment", 1_000, []CardRefundSplit{{"pay_new", 1_000, 300_000}}},
		{"all of the newest refunds all its rupees", 7_000, []CardRefundSplit{{"pay_new", 7_000, 2_100_000}}},
		{"over two payments", 8_000, []CardRefundSplit{{"pay_new", 7_000, 2_100_000}, {"pay_old", 1_000, 290_000}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitCardRefund(tt.amount, payments)
			if err != nil || !slices.Equal(got, tt.want) {
				t.Errorf("SplitCardRefund = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestSplitCardRefundRefusesMoreThanWasPaidByCard(t *testing.T) {
	_, err := SplitCardRefund(10_001, []RefundablePayment{{ID: "pay_1", AmountLeft: 10_000, ChargedLeft: 10_000}})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("SplitCardRefund error = %v, want a validation error", err)
	}
}

func TestPaidCountsPendingRefundsAndLeavesOutFailedOnes(t *testing.T) {
	b := Booking{Payments: []Payment{
		{Kind: "payment", Amount: 50_000, Status: "succeeded"},
		{Kind: "refund", Amount: 10_000, Status: "pending"},
		{Kind: "refund", Amount: 5_000, Status: "failed"},
	}}
	if got := b.Paid(); got != 40_000 {
		t.Errorf("Paid = %d, want 40000", got)
	}
}
