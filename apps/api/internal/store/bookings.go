package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// ListBookings returns the stays the tenant can see that overlap [From, To), by check-in.
func (s *Postgres) ListBookings(ctx context.Context, t tenant.Tenant, f domain.BookingFilter) ([]domain.Booking, error) {
	scope := t.PropertyScope()
	if f.PropertyID != nil {
		if !t.CanSeeProperty(*f.PropertyID) {
			return []domain.Booking{}, nil
		}
		scope = []uuid.UUID{*f.PropertyID}
	}
	rows, err := s.db(ctx).ListBookings(ctx, queries.ListBookingsParams{
		AccountID:    t.AccountID,
		PropertyIds:  scope,
		RangeFrom:    f.From,
		RangeTo:      f.To,
		AfterCheckIn: f.AfterCheckIn,
		AfterID:      f.AfterID,
		RowLimit:     int32(f.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	bookings := make([]domain.Booking, len(rows))
	for i, r := range rows {
		bookings[i] = booking(queries.GetBookingRow(r))
	}
	if err := s.loadBookingDetails(ctx, t.AccountID, bookings); err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	return bookings, nil
}

// GetBooking returns one stay, or domain.ErrNotFound when the tenant can't see it.
func (s *Postgres) GetBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID) (domain.Booking, error) {
	r, err := s.db(ctx).GetBooking(ctx, queries.GetBookingParams{ID: id, AccountID: t.AccountID})
	if err != nil {
		return domain.Booking{}, notFound(err)
	}
	if !t.CanSeeProperty(r.Booking.PropertyID) {
		return domain.Booking{}, domain.ErrNotFound
	}
	bookings := []domain.Booking{booking(r)}
	if err := s.loadBookingDetails(ctx, t.AccountID, bookings); err != nil {
		return domain.Booking{}, fmt.Errorf("get booking: %w", err)
	}
	return bookings[0], nil
}

// CreateBooking adds a stay and holds its nights. It returns a *domain.ConflictError when
// another stay or a block holds one of them.
func (s *Postgres) CreateBooking(ctx context.Context, t tenant.Tenant, in domain.NewBooking, now time.Time) (domain.Booking, error) {
	if !t.CanSeeProperty(in.PropertyID) {
		return domain.Booking{}, domain.ErrNotFound
	}
	id := domain.NewID()
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		n, err := q.NextBookingNumber(ctx, t.AccountID)
		if err != nil {
			return fmt.Errorf("next booking number: %w", err)
		}
		guestID, err := s.matchGuest(ctx, t.AccountID, in.Stay.Guest)
		if err != nil {
			return err
		}
		stay := in.Stay
		if err := q.InsertBooking(ctx, queries.InsertBookingParams{
			ID:         id,
			AccountID:  t.AccountID,
			PropertyID: in.PropertyID,
			UnitID:     stay.UnitID,
			Ref:        fmt.Sprintf("%s-%d", in.RefPrefix, n),
			Source:     string(stay.Source),
			Status:     string(in.Status),
			GuestID:    guestID,
			Adults:     int16(stay.Adults),
			Children:   int16(stay.Children),
			CheckIn:    stay.CheckIn,
			CheckOut:   stay.CheckOut,
			Currency:   in.Currency,
			Total:      in.Price.Total,
			OwnerNote:  stay.OwnerNote,
			GuestNote:  in.GuestNote,
			PromoID:    in.Price.PromoID,
			CreatedBy:  &t.UserID,

			RequestExpiresAt: in.RequestExpiresAt,
		}); err != nil {
			return fmt.Errorf("insert booking: %w", err)
		}
		if err := s.writePrice(ctx, t.AccountID, id, in.Price); err != nil {
			return err
		}
		if err := s.holdNights(ctx, t.AccountID, in.PropertyID, in.LedgerUnits, stay.CheckIn, stay.CheckOut, &id, nil); err != nil {
			return err
		}
		if err := s.addEvent(ctx, t.AccountID, id, "", in.Status, &t.UserID, ""); err != nil {
			return err
		}
		if p := in.FirstPayment; p != nil {
			p.Kind, p.ReceivedAt = "payment", now
			return s.addPayment(ctx, t, id, in.Currency, *p)
		}
		return nil
	})
	if err != nil {
		return domain.Booking{}, err
	}
	return s.GetBooking(ctx, t, id)
}

// UpdateBooking applies an edit made against version c.Version. It returns domain.ErrStale
// when the stay has moved on, and a *domain.ConflictError when the new nights are taken.
func (s *Postgres) UpdateBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID, c domain.BookingChange) (domain.Booking, error) {
	return s.changeBooking(ctx, t, id, &c.Version, func(ctx context.Context, b *domain.Booking) error {
		if !b.Status.Holds() || b.Status == domain.StatusCheckedOut {
			return fmt.Errorf("%w: a %s stay can't be changed", domain.ErrBadTransition, b.Status)
		}
		q := s.db(ctx)
		stay := c.Stay
		if err := q.UpdateGuest(ctx, queries.UpdateGuestParams{
			ID: b.Guest.ID, AccountID: t.AccountID, Name: stay.Guest.Name,
			Phone: nonEmpty(stay.Guest.Phone), Email: nonEmpty(stay.Guest.Email), Country: nonEmpty(stay.Guest.Country),
		}); err != nil {
			return fmt.Errorf("update guest: %w", err)
		}
		b.UnitID, b.Source, b.CheckIn, b.CheckOut = stay.UnitID, stay.Source, stay.CheckIn, stay.CheckOut
		b.Adults, b.Children, b.OwnerNote, b.Total = stay.Adults, stay.Children, stay.OwnerNote, c.Price.Total
		if err := q.DeleteBookingLines(ctx, queries.DeleteBookingLinesParams{BookingID: id, AccountID: t.AccountID}); err != nil {
			return fmt.Errorf("delete booking lines: %w", err)
		}
		if err := q.DeleteBookingExtras(ctx, queries.DeleteBookingExtrasParams{BookingID: id, AccountID: t.AccountID}); err != nil {
			return fmt.Errorf("delete booking extras: %w", err)
		}
		if err := s.writePrice(ctx, t.AccountID, id, c.Price); err != nil {
			return err
		}
		if err := q.DeleteBookingNights(ctx, queries.DeleteBookingNightsParams{BookingID: &id, AccountID: t.AccountID}); err != nil {
			return fmt.Errorf("release nights: %w", err)
		}
		return s.holdNights(ctx, t.AccountID, b.PropertyID, c.LedgerUnits, stay.CheckIn, stay.CheckOut, &id, nil)
	})
}

// TransitionBooking moves a stay to its next status, as the tenant's role allows. Approving a
// request starts the clock on payment when the property cancels unpaid stays.
func (s *Postgres) TransitionBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID, version int, to domain.BookingStatus, reason string, now time.Time) (domain.Booking, error) {
	return s.changeBooking(ctx, t, id, &version, func(ctx context.Context, b *domain.Booking) error {
		if err := domain.CanTransition(t.Role, b.Status, to); err != nil {
			return err
		}
		if to == domain.StatusAwaitingPayment {
			p, err := s.GetProperty(ctx, t, b.PropertyID)
			if err != nil {
				return fmt.Errorf("load property: %w", err)
			}
			if p.Payments.Bank.CancelIfUnpaid {
				due := now.Add(time.Duration(p.Payments.Bank.PayWithinHours) * time.Hour)
				b.PaymentDueAt = &due
			}
		}
		return s.moveTo(ctx, t.AccountID, b, to, &t.UserID, reason)
	})
}

// CancelBooking cancels a stay, frees its nights and records the refund, which can't be more
// than the guest paid.
func (s *Postgres) CancelBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID, version int, reason string, refund *domain.Payment, now time.Time) (domain.Booking, error) {
	return s.changeBooking(ctx, t, id, &version, func(ctx context.Context, b *domain.Booking) error {
		if !b.Status.Cancellable() {
			return fmt.Errorf("%w: a %s stay can't be cancelled", domain.ErrBadTransition, b.Status)
		}
		if refund != nil {
			if refund.Amount > b.Paid() {
				return &domain.ValidationError{Field: "refund.amount", Message: "can't be more than the guest paid"}
			}
			refund.Kind, refund.ReceivedAt = "refund", now
			if err := s.addPayment(ctx, t, b.ID, b.Currency, *refund); err != nil {
				return err
			}
		}
		b.CancelReason, b.CancelledAt = reason, &now
		return s.moveTo(ctx, t.AccountID, b, domain.StatusCancelled, &t.UserID, reason)
	})
}

// RecordPayment adds money the guest paid, accepting their pending slip when p.SlipID is set.
// A first payment confirms a stay that was waiting for one.
func (s *Postgres) RecordPayment(ctx context.Context, t tenant.Tenant, id uuid.UUID, p domain.Payment) (domain.Booking, error) {
	return s.changeBooking(ctx, t, id, nil, func(ctx context.Context, b *domain.Booking) error {
		if !b.Status.Holds() {
			return fmt.Errorf("%w: a %s stay takes no payments", domain.ErrBadTransition, b.Status)
		}
		if p.SlipID != nil {
			if err := s.reviewSlip(ctx, t, *p.SlipID, b.ID, "accepted", p.ReceivedAt); err != nil {
				return err
			}
		}
		p.Kind = "payment"
		if err := s.addPayment(ctx, t, b.ID, b.Currency, p); err != nil {
			return err
		}
		if b.Status != domain.StatusAwaitingPayment {
			return nil
		}
		b.PaymentDueAt = nil
		return s.moveTo(ctx, t.AccountID, b, domain.StatusConfirmed, &t.UserID, "paid")
	})
}

// RejectSlip turns down a guest's pending bank slip and returns its stay.
func (s *Postgres) RejectSlip(ctx context.Context, t tenant.Tenant, slipID uuid.UUID, now time.Time) (domain.Booking, error) {
	var bookingID uuid.UUID
	err := s.InTx(ctx, func(ctx context.Context) error {
		slip, err := s.db(ctx).GetSlipForUpdate(ctx, queries.GetSlipForUpdateParams{ID: slipID, AccountID: t.AccountID})
		if err != nil {
			return notFound(err)
		}
		bookingID = slip.BookingID
		if _, err := s.GetBooking(ctx, t, bookingID); err != nil {
			return err
		}
		return s.reviewSlip(ctx, t, slipID, bookingID, "rejected", now)
	})
	if err != nil {
		return domain.Booking{}, err
	}
	return s.GetBooking(ctx, t, bookingID)
}

// changeBooking locks a stay, checks the caller can see it and that it is still at version
// (when given), runs fn on it and saves what fn changed on the booking row.
func (s *Postgres) changeBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID, version *int, fn func(ctx context.Context, b *domain.Booking) error) (domain.Booking, error) {
	err := s.InTx(ctx, func(ctx context.Context) error {
		r, err := s.db(ctx).GetBookingForUpdate(ctx, queries.GetBookingForUpdateParams{ID: id, AccountID: t.AccountID})
		if err != nil {
			return notFound(err)
		}
		if !t.CanSeeProperty(r.Booking.PropertyID) {
			return domain.ErrNotFound
		}
		bookings := []domain.Booking{booking(queries.GetBookingRow(r))}
		if err := s.loadBookingDetails(ctx, t.AccountID, bookings); err != nil {
			return err
		}
		b := &bookings[0]
		if version != nil && *version != b.Version {
			return domain.ErrStale
		}
		if err := fn(ctx, b); err != nil {
			return err
		}
		return s.saveBooking(ctx, t.AccountID, *b)
	})
	if err != nil {
		return domain.Booking{}, err
	}
	return s.GetBooking(ctx, t, id)
}

// moveTo sets a stay's status, records the event and frees its nights when it stops holding
// them. The caller saves the booking row.
func (s *Postgres) moveTo(ctx context.Context, accountID uuid.UUID, b *domain.Booking, to domain.BookingStatus, actor *uuid.UUID, reason string) error {
	from := b.Status
	b.Status = to
	if to != domain.StatusRequested {
		b.RequestExpiresAt = nil
	}
	if !to.Holds() {
		if err := s.db(ctx).DeleteBookingNights(ctx, queries.DeleteBookingNightsParams{BookingID: &b.ID, AccountID: accountID}); err != nil {
			return fmt.Errorf("release nights: %w", err)
		}
	}
	return s.addEvent(ctx, accountID, b.ID, from, to, actor, reason)
}

func (s *Postgres) saveBooking(ctx context.Context, accountID uuid.UUID, b domain.Booking) error {
	err := s.db(ctx).UpdateBooking(ctx, queries.UpdateBookingParams{
		ID:               b.ID,
		AccountID:        accountID,
		UnitID:           b.UnitID,
		Source:           string(b.Source),
		Status:           string(b.Status),
		Adults:           int16(b.Adults),
		Children:         int16(b.Children),
		CheckIn:          b.CheckIn,
		CheckOut:         b.CheckOut,
		Total:            b.Total,
		GuestNote:        b.GuestNote,
		OwnerNote:        b.OwnerNote,
		RequestExpiresAt: b.RequestExpiresAt,
		PaymentDueAt:     b.PaymentDueAt,
		CancelReason:     nonEmpty(b.CancelReason),
		CancelledAt:      b.CancelledAt,
	})
	if err != nil {
		return fmt.Errorf("update booking: %w", err)
	}
	return nil
}

// holdNights puts every night in [from, to) on the ledger for each unit. A night someone else
// holds rolls back to a savepoint and comes back as a *domain.ConflictError naming the clash.
// Units go in sorted, so two transactions take ledger rows in the same order and can't deadlock.
func (s *Postgres) holdNights(ctx context.Context, accountID, propertyID uuid.UUID, units []uuid.UUID, from, to time.Time, bookingID, blockID *uuid.UUID) error {
	units = slices.Clone(units)
	slices.SortFunc(units, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	err := s.InTx(ctx, func(ctx context.Context) error {
		return s.db(ctx).InsertUnitNights(ctx, queries.InsertUnitNightsParams{
			AccountID: accountID, PropertyID: propertyID, BookingID: bookingID, BlockID: blockID,
			UnitIds: units, CheckIn: from, CheckOut: to,
		})
	})
	if !violates(err, "unit_nights_pkey") {
		if err != nil {
			return fmt.Errorf("hold nights: %w", err)
		}
		return nil
	}
	r, err := s.db(ctx).FirstHeldNight(ctx, queries.FirstHeldNightParams{
		AccountID: accountID, UnitIds: units, CheckIn: from, CheckOut: to,
	})
	if err != nil {
		return fmt.Errorf("find the clash: %w", err)
	}
	c := &domain.ConflictError{Night: r.Night, BookingID: r.BookingID, BlockID: r.BlockID}
	switch {
	case r.BookingID != nil:
		c.Ref, c.GuestName, c.From, c.To = deref(r.BookingRef), deref(r.GuestName), deref(r.BookingCheckIn), deref(r.BookingCheckOut)
	case r.BlockID != nil:
		c.From, c.To = deref(r.BlockStartsOn), deref(r.BlockEndsOn)
	}
	return c
}

// matchGuest finds the account's guest with the same phone or email, bringing their details
// up to date, or adds a new one.
func (s *Postgres) matchGuest(ctx context.Context, accountID uuid.UUID, g domain.Guest) (uuid.UUID, error) {
	q := s.db(ctx)
	id, err := q.FindGuest(ctx, queries.FindGuestParams{AccountID: accountID, Phone: nonEmpty(g.Phone), Email: nonEmpty(g.Email)})
	switch {
	case err == nil:
		err = q.UpdateGuest(ctx, queries.UpdateGuestParams{
			ID: id, AccountID: accountID, Name: g.Name,
			Phone: nonEmpty(g.Phone), Email: nonEmpty(g.Email), Country: nonEmpty(g.Country),
		})
	case errors.Is(notFound(err), domain.ErrNotFound):
		id = domain.NewID()
		err = q.InsertGuest(ctx, queries.InsertGuestParams{
			ID: id, AccountID: accountID, Name: g.Name,
			Phone: nonEmpty(g.Phone), Email: nonEmpty(g.Email), Country: nonEmpty(g.Country),
		})
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("match guest: %w", err)
	}
	return id, nil
}

func (s *Postgres) writePrice(ctx context.Context, accountID, bookingID uuid.UUID, p domain.Pricing) error {
	q := s.db(ctx)
	for i, l := range p.Lines {
		if err := q.InsertBookingLine(ctx, queries.InsertBookingLineParams{
			AccountID: accountID, BookingID: bookingID, Position: int16(i),
			Label: l.Label, Amount: l.Amount, Kind: nonEmpty(l.Kind),
		}); err != nil {
			return fmt.Errorf("insert booking line: %w", err)
		}
	}
	for _, e := range p.Extras {
		if err := q.InsertBookingExtra(ctx, queries.InsertBookingExtraParams{
			AccountID: accountID, BookingID: bookingID, ExtraID: e.ExtraID, Name: e.Name, Amount: e.Amount,
		}); err != nil {
			return fmt.Errorf("insert booking extra: %w", err)
		}
	}
	return nil
}

func (s *Postgres) addPayment(ctx context.Context, t tenant.Tenant, bookingID uuid.UUID, currency string, p domain.Payment) error {
	err := s.db(ctx).InsertPayment(ctx, queries.InsertPaymentParams{
		ID: domain.NewID(), AccountID: t.AccountID, BookingID: bookingID, Kind: p.Kind, Method: p.Method,
		Amount: p.Amount, Currency: currency, Note: p.Note, ReceivedAt: p.ReceivedAt,
		RecordedBy: &t.UserID, SlipID: p.SlipID,
	})
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

// reviewSlip accepts or rejects a pending slip of the booking.
func (s *Postgres) reviewSlip(ctx context.Context, t tenant.Tenant, slipID, bookingID uuid.UUID, status string, now time.Time) error {
	q := s.db(ctx)
	slip, err := q.GetSlipForUpdate(ctx, queries.GetSlipForUpdateParams{ID: slipID, AccountID: t.AccountID})
	if err != nil || slip.BookingID != bookingID {
		return &domain.ValidationError{Field: "slipId", Message: "must be a slip of this booking"}
	}
	if slip.Status != "pending" {
		return fmt.Errorf("%w: the slip was already %s", domain.ErrBadTransition, slip.Status)
	}
	if err := q.ReviewSlip(ctx, queries.ReviewSlipParams{
		ID: slipID, AccountID: t.AccountID, Status: status, ReviewedBy: &t.UserID, ReviewedAt: &now,
	}); err != nil {
		return fmt.Errorf("review slip: %w", err)
	}
	return nil
}

func (s *Postgres) addEvent(ctx context.Context, accountID, bookingID uuid.UUID, from, to domain.BookingStatus, actor *uuid.UUID, reason string) error {
	err := s.db(ctx).InsertBookingEvent(ctx, queries.InsertBookingEventParams{
		ID: domain.NewID(), AccountID: accountID, BookingID: bookingID,
		FromStatus: nonEmpty(string(from)), ToStatus: string(to), ActorUserID: actor, Reason: reason,
	})
	if err != nil {
		return fmt.Errorf("insert booking event: %w", err)
	}
	return nil
}

// loadBookingDetails fills in the price lines, extras, payments and slips of a page of stays.
func (s *Postgres) loadBookingDetails(ctx context.Context, accountID uuid.UUID, bookings []domain.Booking) error {
	if len(bookings) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(bookings))
	index := make(map[uuid.UUID]*domain.Booking, len(bookings))
	for i := range bookings {
		ids[i] = bookings[i].ID
		index[ids[i]] = &bookings[i]
	}
	q := s.db(ctx)
	lines, err := q.ListBookingLines(ctx, queries.ListBookingLinesParams{AccountID: accountID, BookingIds: ids})
	if err != nil {
		return fmt.Errorf("list booking lines: %w", err)
	}
	for _, l := range lines {
		b := index[l.BookingID]
		b.Lines = append(b.Lines, domain.PriceLine{Label: l.Label, Amount: l.Amount, Kind: deref(l.Kind)})
	}
	extras, err := q.ListBookingExtras(ctx, queries.ListBookingExtrasParams{AccountID: accountID, BookingIds: ids})
	if err != nil {
		return fmt.Errorf("list booking extras: %w", err)
	}
	for _, e := range extras {
		b := index[e.BookingID]
		b.Extras = append(b.Extras, domain.BookingExtra{ExtraID: e.ExtraID, Name: e.Name, Amount: e.Amount})
	}
	payments, err := q.ListPayments(ctx, queries.ListPaymentsParams{AccountID: accountID, BookingIds: ids})
	if err != nil {
		return fmt.Errorf("list payments: %w", err)
	}
	for _, p := range payments {
		b := index[p.BookingID]
		b.Payments = append(b.Payments, domain.Payment{
			ID: p.ID, Kind: p.Kind, Method: p.Method, Amount: p.Amount, Note: p.Note, ReceivedAt: p.ReceivedAt, SlipID: p.SlipID,
		})
	}
	slips, err := q.ListSlips(ctx, queries.ListSlipsParams{AccountID: accountID, BookingIds: ids})
	if err != nil {
		return fmt.Errorf("list slips: %w", err)
	}
	for _, sl := range slips {
		b := index[sl.BookingID]
		b.Slips = append(b.Slips, domain.Slip{ID: sl.ID, FileKey: sl.FileKey, Amount: sl.Amount, Status: sl.Status, UploadedAt: sl.UploadedAt})
	}
	return nil
}

func booking(r queries.GetBookingRow) domain.Booking {
	b := r.Booking
	return domain.Booking{
		ID:               b.ID,
		PropertyID:       b.PropertyID,
		UnitID:           b.UnitID,
		Ref:              b.Ref,
		Source:           domain.Source(b.Source),
		Status:           domain.BookingStatus(b.Status),
		Guest:            domain.Guest{ID: b.GuestID, Name: r.GuestName, Phone: deref(r.GuestPhone), Email: deref(r.GuestEmail), Country: deref(r.GuestCountry)},
		Adults:           int(b.Adults),
		Children:         int(b.Children),
		CheckIn:          b.CheckIn,
		CheckOut:         b.CheckOut,
		Currency:         b.Currency,
		Total:            b.Total,
		GuestNote:        b.GuestNote,
		OwnerNote:        b.OwnerNote,
		RequestExpiresAt: b.RequestExpiresAt,
		PaymentDueAt:     b.PaymentDueAt,
		CancelReason:     deref(b.CancelReason),
		CancelledAt:      b.CancelledAt,
		Version:          int(b.Version),
		CreatedAt:        b.CreatedAt,
		UpdatedAt:        b.UpdatedAt,
	}
}

// deref reads an optional column, giving the zero value for NULL.
func deref[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}
	return *v
}

// AddSlip records a bank slip a guest uploaded for a stay.
func (s *Postgres) AddSlip(ctx context.Context, t tenant.Tenant, bookingID uuid.UUID, fileKey string, amount int64, uploadedAt time.Time) (uuid.UUID, error) {
	if _, err := s.GetBooking(ctx, t, bookingID); err != nil {
		return uuid.Nil, err
	}
	id := domain.NewID()
	if err := s.db(ctx).InsertSlip(ctx, queries.InsertSlipParams{
		ID: id, AccountID: t.AccountID, BookingID: bookingID, FileKey: fileKey, Amount: amount, UploadedAt: uploadedAt,
	}); err != nil {
		return uuid.Nil, fmt.Errorf("insert slip: %w", err)
	}
	return id, nil
}
