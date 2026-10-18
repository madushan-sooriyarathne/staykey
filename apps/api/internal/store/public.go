package store

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/store/queries"
)

// Booking page and guest methods. They act on the account of the property or booking they are
// given, never on one named by the request, and never as a user.

// PublicProperty returns a property with its full setup by booking page address. Callers decide
// what of it a guest may see; the bank account number is in it.
func (s *Postgres) PublicProperty(ctx context.Context, slug string) (domain.PropertyDetail, error) {
	r, err := s.db(ctx).GetPropertyBySlug(ctx, slug)
	if err != nil {
		return domain.PropertyDetail{}, notFound(err)
	}
	details := []domain.PropertyDetail{propertyDetail(queries.GetPropertyRow(r))}
	if err := s.loadSetup(ctx, r.AccountID, details); err != nil {
		return domain.PropertyDetail{}, fmt.Errorf("public property: %w", err)
	}
	return details[0], nil
}

// Availability returns the ledger nights in [from, to) that a stay, a block or a live hold
// takes, by unit. Holds that lapsed before now don't count.
func (s *Postgres) Availability(ctx context.Context, accountID, propertyID uuid.UUID, from, to, now time.Time) (map[uuid.UUID][]time.Time, error) {
	rows, err := s.db(ctx).HeldNights(ctx, queries.HeldNightsParams{
		AccountID: accountID, PropertyID: propertyID, RangeFrom: from, RangeTo: to, Now: now,
	})
	if err != nil {
		return nil, fmt.Errorf("availability: %w", err)
	}
	out := map[uuid.UUID][]time.Time{}
	for _, r := range rows {
		out[r.UnitID] = append(out[r.UnitID], r.Night)
	}
	return out, nil
}

// CreateHold holds the nights in [from, to) on units until expiresAt, for a guest at checkout.
// replaces, when set, is the guest's previous hold, released first. Taken nights give a
// *domain.ConflictError.
func (s *Postgres) CreateHold(ctx context.Context, accountID, propertyID uuid.UUID, units []uuid.UUID, from, to, expiresAt, now time.Time, replaces *uuid.UUID) (uuid.UUID, error) {
	id := uuid.New()
	err := s.InTx(ctx, func(ctx context.Context) error {
		if replaces != nil {
			if _, err := s.db(ctx).DeleteHold(ctx, queries.DeleteHoldParams{HoldID: replaces, AccountID: accountID}); err != nil {
				return fmt.Errorf("release previous hold: %w", err)
			}
		}
		return s.holdNights(ctx, accountID, propertyID, units, from, to, nightsFor{holdID: &id, holdExpiresAt: &expiresAt}, now)
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// CreateGuestBooking adds a stay a guest booked on the booking page. It matches the guest to
// the account's guests by phone or email without changing what the owner has stored, uses a go
// of the promo code, tells the team, and queues the guest's email.
func (s *Postgres) CreateGuestBooking(ctx context.Context, accountID uuid.UUID, in domain.NewBooking, now time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.InTx(ctx, func(ctx context.Context) error {
		guestID, err := s.findOrAddGuest(ctx, accountID, in.Stay.Guest)
		if err != nil {
			return err
		}
		added, err := s.insertBooking(ctx, accountID, guestID, nil, in, now)
		if err != nil {
			return err
		}
		id = added.ID
		if in.GuestToken != "" {
			if err := s.setGuestToken(ctx, accountID, id, in.GuestToken, in.GuestTokenHash); err != nil {
				return err
			}
		}
		if err := s.queueStayEmail(ctx, accountID, added, domain.EmailReceived, "", now); err != nil {
			return err
		}
		if in.Price.PromoID != nil {
			_, err := s.db(ctx).UsePromo(ctx, queries.UsePromoParams{ID: *in.Price.PromoID, AccountID: accountID})
			if errors.Is(err, pgx.ErrNoRows) {
				return &domain.ValidationError{Field: "promo", Message: "that code has been used up"}
			}
			if err != nil {
				return fmt.Errorf("use promo: %w", err)
			}
		}
		if err := s.addActivity(ctx, accountID, domain.DirectBooking(added), nil, now); err != nil {
			return err
		}
		return s.directBookingMade(ctx, accountID, added, now)
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// GuestBooking returns a booking to the guest holding its token, or domain.ErrNotFound.
func (s *Postgres) GuestBooking(ctx context.Context, id uuid.UUID, tokenHash []byte) (domain.GuestBooking, error) {
	r, err := s.db(ctx).GetBookingForGuest(ctx, id)
	if err != nil {
		return domain.GuestBooking{}, notFound(err)
	}
	if subtle.ConstantTimeCompare(r.Booking.GuestTokenHash, tokenHash) != 1 {
		return domain.GuestBooking{}, domain.ErrNotFound
	}
	bookings := []domain.Booking{booking(queries.GetBookingRow{
		Booking: r.Booking, GuestName: r.GuestName, GuestPhone: r.GuestPhone, GuestEmail: r.GuestEmail, GuestCountry: r.GuestCountry,
	})}
	if err := s.loadBookingDetails(ctx, r.Booking.AccountID, bookings); err != nil {
		return domain.GuestBooking{}, fmt.Errorf("guest booking: %w", err)
	}
	return domain.GuestBooking{
		Booking: bookings[0], AccountID: r.Booking.AccountID, PropertySlug: r.PropertySlug,
		PropertyName: r.PropertyName, PropertyCheckIn: r.PropertyCheckIn, UnitName: r.UnitName,
	}, nil
}

// AddGuestSlip records a bank slip the guest uploaded from their stay page, and tells the team.
// The file must be a slip uploaded to the booking's account.
func (s *Postgres) AddGuestSlip(ctx context.Context, bookingID uuid.UUID, tokenHash []byte, key string, amount int64, now time.Time) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		b, err := s.GuestBooking(ctx, bookingID, tokenHash)
		if err != nil {
			return err
		}
		if !files.Belongs(key, b.AccountID, files.KindSlip) {
			return &domain.ValidationError{Field: "key", Message: "must be a slip uploaded for this booking"}
		}
		if !b.Status.Holds() || b.Status == domain.StatusCheckedOut {
			return fmt.Errorf("%w: a %s stay takes no slips", domain.ErrBadTransition, b.Status)
		}
		if err := s.db(ctx).InsertSlip(ctx, queries.InsertSlipParams{
			ID: domain.NewID(), AccountID: b.AccountID, BookingID: bookingID, FileKey: key, Amount: amount, UploadedAt: now,
		}); err != nil {
			return fmt.Errorf("insert slip: %w", err)
		}
		return s.addActivity(ctx, b.AccountID, domain.SlipUploaded(b.Booking), nil, now)
	})
}

// findOrAddGuest reuses the account's guest with the same name and the same phone or email, or
// adds a new one. Someone on the booking page may type another person's number, so it never
// changes a stored guest, and a different name makes a guest of its own rather than a stay on
// someone else's record.
func (s *Postgres) findOrAddGuest(ctx context.Context, accountID uuid.UUID, g domain.Guest) (uuid.UUID, error) {
	q := s.db(ctx)
	id, err := q.FindNamedGuest(ctx, queries.FindNamedGuestParams{
		AccountID: accountID, Name: g.Name, Phone: nonEmpty(g.Phone), Email: nonEmpty(g.Email),
	})
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("find guest: %w", err)
	}
	id = domain.NewID()
	if err := q.InsertGuest(ctx, queries.InsertGuestParams{
		ID: id, AccountID: accountID, Name: g.Name, Phone: nonEmpty(g.Phone), Email: nonEmpty(g.Email), Country: nonEmpty(g.Country),
	}); err != nil {
		return uuid.Nil, fmt.Errorf("insert guest: %w", err)
	}
	return id, nil
}
