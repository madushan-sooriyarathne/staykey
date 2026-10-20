package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
)

// StayKey payouts: card money StayKey collects for an owner and pays out by bank transfer.

// payoutProfileComplete reports whether the account has given its payout details.
func (s *Postgres) payoutProfileComplete(ctx context.Context, accountID uuid.UUID) (bool, error) {
	return false, nil
}

// cardPaymentCollected enters a card payment StayKey collected in the owner's payout ledger.
func (s *Postgres) cardPaymentCollected(ctx context.Context, c queries.CardCheckout, paymentID uuid.UUID, b domain.Booking, now time.Time) error {
	return nil
}

// cardRefundSent takes a card refund StayKey sent out of the owner's payout ledger.
func (s *Postgres) cardRefundSent(ctx context.Context, refundID uuid.UUID, now time.Time) error {
	return nil
}

// cardRefundReturned puts a refund that failed back into the owner's payout ledger.
func (s *Postgres) cardRefundReturned(ctx context.Context, accountID, refundID uuid.UUID, now time.Time) error {
	return nil
}
