package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
)

// DeclineExpiredRequests declines requests the owner didn't answer in time and frees their
// nights. It returns how many it declined.
func (s *Postgres) DeclineExpiredRequests(ctx context.Context, now time.Time) (int, error) {
	var n int
	err := s.InTx(ctx, func(ctx context.Context) error {
		rows, err := s.db(ctx).DeclineExpiredRequests(ctx, now)
		if err != nil {
			return fmt.Errorf("decline expired requests: %w", err)
		}
		n = len(rows)
		for _, r := range rows {
			if err := s.closeBySystem(ctx, r.AccountID, r.ID, domain.StatusRequested, domain.StatusDeclined, "Request expired"); err != nil {
				return err
			}
		}
		return nil
	})
	return n, err
}

// CancelUnpaidBookings cancels stays whose payment deadline passed with nothing paid, freeing
// their nights. It returns how many it cancelled.
func (s *Postgres) CancelUnpaidBookings(ctx context.Context, now time.Time) (int, error) {
	var n int
	err := s.InTx(ctx, func(ctx context.Context) error {
		rows, err := s.db(ctx).CancelUnpaidBookings(ctx, now)
		if err != nil {
			return fmt.Errorf("cancel unpaid bookings: %w", err)
		}
		n = len(rows)
		for _, r := range rows {
			if err := s.closeBySystem(ctx, r.AccountID, r.ID, domain.StatusAwaitingPayment, domain.StatusCancelled, "Not paid in time"); err != nil {
				return err
			}
		}
		return nil
	})
	return n, err
}

// SweepExpired clears checkout holds and idempotency keys that have run out.
func (s *Postgres) SweepExpired(ctx context.Context, now time.Time) error {
	q := s.db(ctx)
	if _, err := q.DeleteExpiredHolds(ctx, now); err != nil {
		return fmt.Errorf("delete expired holds: %w", err)
	}
	if _, err := q.DeleteExpiredIdempotencyKeys(ctx, now); err != nil {
		return fmt.Errorf("delete expired idempotency keys: %w", err)
	}
	return nil
}

func (s *Postgres) closeBySystem(ctx context.Context, accountID, bookingID uuid.UUID, from, to domain.BookingStatus, reason string) error {
	if err := s.db(ctx).DeleteBookingNights(ctx, queries.DeleteBookingNightsParams{BookingID: &bookingID, AccountID: accountID}); err != nil {
		return fmt.Errorf("release nights: %w", err)
	}
	return s.addEvent(ctx, accountID, bookingID, from, to, nil, reason)
}

// FileInUse reports whether a property photo, logo or bank slip points at the file.
func (s *Postgres) FileInUse(ctx context.Context, key string) (bool, error) {
	used, err := s.db(ctx).FileInUse(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check file in use: %w", err)
	}
	return deref(used), nil
}
