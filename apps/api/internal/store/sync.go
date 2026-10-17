package store

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/ical"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// DueFeed is an imported calendar the import job is to read now.
type DueFeed struct {
	ID         uuid.UUID
	AccountID  uuid.UUID
	PropertyID uuid.UUID
	UnitID     *uuid.UUID
	Channel    string
	URL        string
	ETag       string
	Failures   int
	// FirstSync is true until the feed has been read once.
	FirstSync    bool
	TimeZone     string
	PropertyName string
}

// cancelledFromFeed is the reason recorded on imported stays that left their calendar.
const cancelledFromFeed = "removed from the channel's calendar"

// ClaimDueFeeds leases up to limit feeds that are due, so a run that overlaps this one skips
// them. A feed whose run never reports back is due again after lease.
func (s *Postgres) ClaimDueFeeds(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]DueFeed, error) {
	rows, err := s.db(ctx).ClaimDueFeeds(ctx, queries.ClaimDueFeedsParams{LeaseUntil: now.Add(lease), Now: now, RowLimit: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("claim due feeds: %w", err)
	}
	feeds := make([]DueFeed, len(rows))
	for i, r := range rows {
		feeds[i] = DueFeed{
			ID: r.ID, AccountID: r.AccountID, PropertyID: r.PropertyID, UnitID: r.UnitID, Channel: r.Channel,
			URL: r.Url, ETag: deref(r.Etag), Failures: int(r.ConsecutiveFailures), FirstSync: r.LastSyncedAt == nil,
			TimeZone: r.TimeZone, PropertyName: r.PropertyName,
		}
	}
	return feeds, nil
}

// ApplyFeed brings a feed's imported stays in line with its events, in one transaction: new
// events become confirmed stays, moved ones move, and stays whose event went away before
// check-out are cancelled. Stays the owner cancelled stay cancelled. A stay whose nights are
// taken is kept without nights and tried again on every sync; the team hears once when the
// nights belong to a stay that wasn't imported, or to a block. The first sync of a feed writes
// one activity line instead of one per stay.
func (s *Postgres) ApplyFeed(ctx context.Context, f DueFeed, events []ical.Event, etag string, today, now time.Time) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		r, err := q.GetProperty(ctx, queries.GetPropertyParams{ID: f.PropertyID, AccountID: f.AccountID})
		if err != nil {
			return fmt.Errorf("load property: %w", notFound(err))
		}
		details := []domain.PropertyDetail{propertyDetail(r)}
		if err := s.loadSetup(ctx, f.AccountID, details); err != nil {
			return fmt.Errorf("load property: %w", err)
		}
		p := details[0]
		unit, ledger, ok := domain.ImportUnits(p.Units, f.UnitID)
		if !ok {
			return errors.New("the feed's property has no unit to import onto")
		}

		existing, err := q.ListFeedBookings(ctx, queries.ListFeedBookingsParams{FeedID: &f.ID, AccountID: f.AccountID})
		if err != nil {
			return fmt.Errorf("list imported stays: %w", err)
		}
		byUID := make(map[string]queries.ListFeedBookingsRow, len(existing))
		for _, b := range existing {
			byUID[deref(b.ExternalUid)] = b
		}

		im := importer{s: s, feed: f, property: p, unit: unit, ledger: ledger, now: now}
		upcoming := 0
		inFeed := make(map[string]bool, len(events))
		for _, e := range events {
			inFeed[e.UID] = true
			old, found := byUID[e.UID]
			switch {
			case !found:
				err = im.add(ctx, e)
			case !domain.BookingStatus(old.Status).Holds():
				continue
			case !old.CheckIn.Equal(e.Start) || !old.CheckOut.Equal(e.End) || old.UnitID != unit:
				err = im.move(ctx, old.ID, e)
			case !old.HoldsNights:
				err = im.retry(ctx, old.ID)
			}
			if err != nil {
				return err
			}
			upcoming++
		}
		for _, old := range existing {
			if inFeed[deref(old.ExternalUid)] || !old.CheckOut.After(today) || !domain.BookingStatus(old.Status).Cancellable() {
				continue
			}
			if err := im.cancel(ctx, old.ID); err != nil {
				return err
			}
		}

		if f.FirstSync {
			if err := s.addActivity(ctx, f.AccountID, domain.FeedConnected(f.PropertyID, f.Channel, upcoming), nil, now); err != nil {
				return err
			}
		}
		if err := q.FeedSucceeded(ctx, queries.FeedSucceededParams{
			Now: now, Etag: nonEmpty(etag), Upcoming: int32(upcoming), NextSyncAt: domain.NextSync(0, now), ID: f.ID, AccountID: f.AccountID,
		}); err != nil {
			return fmt.Errorf("record sync: %w", err)
		}
		return nil
	})
}

// FeedNotModified records a read that found the feed unchanged since the last one.
func (s *Postgres) FeedNotModified(ctx context.Context, f DueFeed, now time.Time) error {
	if err := s.db(ctx).FeedNotModified(ctx, queries.FeedNotModifiedParams{
		Now: now, NextSyncAt: domain.NextSync(0, now), ID: f.ID, AccountID: f.AccountID,
	}); err != nil {
		return fmt.Errorf("record sync: %w", err)
	}
	return nil
}

// RecordFeedFailure records a read that failed with a message for the owner, and backs the feed
// off. The team is told once, when it has failed domain.FeedAlertAfter times in a row.
func (s *Postgres) RecordFeedFailure(ctx context.Context, f DueFeed, message string, now time.Time) (int, error) {
	var failures int
	err := s.InTx(ctx, func(ctx context.Context) error {
		n, err := s.db(ctx).FeedFailed(ctx, queries.FeedFailedParams{
			LastError: &message, NextSyncAt: domain.NextSync(f.Failures+1, now), ID: f.ID, AccountID: f.AccountID,
		})
		if err != nil {
			return fmt.Errorf("record failure: %w", err)
		}
		failures = int(n)
		if failures != domain.FeedAlertAfter {
			return nil
		}
		return s.addActivity(ctx, f.AccountID, domain.FeedFailing(f.PropertyID, f.Channel, f.PropertyName), nil, now)
	})
	return failures, err
}

// MarkFeedDue has the import job read a feed on its next run, for the owner's "Sync now".
func (s *Postgres) MarkFeedDue(ctx context.Context, t tenant.Tenant, feedID uuid.UUID) (domain.IcalFeed, error) {
	q := s.db(ctx)
	propertyID, err := q.GetFeedProperty(ctx, queries.GetFeedPropertyParams{ID: feedID, AccountID: t.AccountID})
	if err != nil {
		return domain.IcalFeed{}, notFound(err)
	}
	if !t.CanSeeProperty(propertyID) {
		return domain.IcalFeed{}, domain.ErrNotFound
	}
	if _, err := q.MarkFeedDue(ctx, queries.MarkFeedDueParams{ID: feedID, AccountID: t.AccountID}); errors.Is(err, pgx.ErrNoRows) {
		return domain.IcalFeed{}, &domain.ValidationError{Field: "url", Message: "add the calendar link first"}
	} else if err != nil {
		return domain.IcalFeed{}, fmt.Errorf("mark feed due: %w", err)
	}
	p, err := s.GetProperty(ctx, t, propertyID)
	if err != nil {
		return domain.IcalFeed{}, err
	}
	for _, f := range p.IcalFeeds {
		if f.ID == feedID {
			return f, nil
		}
	}
	return domain.IcalFeed{}, domain.ErrNotFound
}

// ExportCalendar returns the property's name and its taken nights from from on, for the export
// link with token. unitID limits it to the nights that unit can't be sold. A wrong slug, token
// or unit is domain.ErrNotFound.
func (s *Postgres) ExportCalendar(ctx context.Context, slug, token string, unitID *uuid.UUID, from time.Time) (string, []domain.ExportRun, error) {
	r, err := s.db(ctx).GetExportProperty(ctx, slug)
	if err != nil {
		return "", nil, notFound(err)
	}
	if subtle.ConstantTimeCompare([]byte(r.IcalExportToken), []byte(token)) != 1 {
		return "", nil, domain.ErrNotFound
	}
	var units []uuid.UUID
	if unitID != nil {
		details := []domain.PropertyDetail{{Property: domain.Property{ID: r.ID}}}
		if err := s.loadSetup(ctx, r.AccountID, details); err != nil {
			return "", nil, fmt.Errorf("export calendar: %w", err)
		}
		if !slices.ContainsFunc(details[0].Units, func(u domain.Unit) bool { return u.ID == *unitID }) {
			return "", nil, domain.ErrNotFound
		}
		units = domain.LedgerUnits(details[0].Units, *unitID)
	}
	rows, err := s.db(ctx).ExportNights(ctx, queries.ExportNightsParams{AccountID: r.AccountID, PropertyID: r.ID, RangeFrom: from})
	if err != nil {
		return "", nil, fmt.Errorf("export calendar: %w", err)
	}
	nights := make([]domain.HeldNight, len(rows))
	for i, n := range rows {
		nights[i] = domain.HeldNight{UnitID: n.UnitID, Night: n.Night, BookingID: n.BookingID, BlockID: n.BlockID}
	}
	return r.Name, domain.ExportRuns(nights, units), nil
}

// importer writes one feed's stays inside ApplyFeed's transaction.
type importer struct {
	s        *Postgres
	feed     DueFeed
	property domain.PropertyDetail
	unit     uuid.UUID
	ledger   []uuid.UUID
	now      time.Time
}

// add records a new event as a confirmed stay, without nights when they are taken.
func (im importer) add(ctx context.Context, e ical.Event) error {
	channel := domain.ChannelName(im.feed.Channel)
	in := domain.NewBooking{
		PropertyID: im.feed.PropertyID,
		Stay: domain.StayInput{
			UnitID: im.unit, CheckIn: e.Start, CheckOut: e.End, Adults: 1,
			Guest: domain.Guest{Name: channel + " stay"}, Source: domain.Source(im.feed.Channel),
		},
		Status:      domain.StatusConfirmed,
		Currency:    im.property.Currency,
		RefPrefix:   domain.RefPrefix(im.property.Name),
		LedgerUnits: im.ledger,
		FeedID:      &im.feed.ID,
		ExternalUID: e.UID,
	}
	var added domain.Booking
	var clash *domain.ConflictError
	err := im.s.InTx(ctx, func(ctx context.Context) error {
		var err error
		added, err = im.insert(ctx, in)
		return err
	})
	if errors.As(err, &clash) {
		in.LedgerUnits = nil
		added, err = im.insert(ctx, in)
	}
	if err != nil {
		return err
	}
	if clash != nil {
		return im.reportClash(ctx, added, clash)
	}
	if im.feed.FirstSync {
		return nil
	}
	return im.s.addActivity(ctx, im.feed.AccountID, domain.StayImported(added), nil, im.now)
}

// insert adds an imported stay with a guest record of its own, since it has no phone or email
// to match on.
func (im importer) insert(ctx context.Context, in domain.NewBooking) (domain.Booking, error) {
	guestID := domain.NewID()
	if err := im.s.db(ctx).InsertGuest(ctx, queries.InsertGuestParams{ID: guestID, AccountID: im.feed.AccountID, Name: in.Stay.Guest.Name}); err != nil {
		return domain.Booking{}, fmt.Errorf("insert guest: %w", err)
	}
	return im.s.insertBooking(ctx, im.feed.AccountID, guestID, nil, in, im.now)
}

// move re-dates an imported stay, or moves it to the feed's unit, and holds its new nights.
func (im importer) move(ctx context.Context, id uuid.UUID, e ical.Event) error {
	b, err := im.lock(ctx, id)
	if err != nil {
		return err
	}
	if err := im.s.db(ctx).DeleteBookingNights(ctx, queries.DeleteBookingNightsParams{BookingID: &id, AccountID: im.feed.AccountID}); err != nil {
		return fmt.Errorf("release nights: %w", err)
	}
	b.CheckIn, b.CheckOut, b.UnitID = e.Start, e.End, im.unit
	if err := im.s.saveBooking(ctx, im.feed.AccountID, b); err != nil {
		return err
	}
	return im.hold(ctx, b)
}

// retry tries again to hold the nights of an imported stay that clashed before.
func (im importer) retry(ctx context.Context, id uuid.UUID) error {
	b, err := im.lock(ctx, id)
	if err != nil {
		return err
	}
	return im.hold(ctx, b)
}

func (im importer) hold(ctx context.Context, b domain.Booking) error {
	err := im.s.holdNights(ctx, im.feed.AccountID, b.PropertyID, im.ledger, b.CheckIn, b.CheckOut, nightsFor{bookingID: &b.ID}, im.now)
	var clash *domain.ConflictError
	if errors.As(err, &clash) {
		return im.reportClash(ctx, b, clash)
	}
	return err
}

// cancel cancels an imported stay whose event left the feed, freeing its nights.
func (im importer) cancel(ctx context.Context, id uuid.UUID) error {
	b, err := im.lock(ctx, id)
	if err != nil {
		return err
	}
	now := im.now
	b.CancelReason, b.CancelledAt = cancelledFromFeed, &now
	if err := im.s.moveTo(ctx, im.feed.AccountID, &b, domain.StatusCancelled, nil, cancelledFromFeed); err != nil {
		return err
	}
	if err := im.s.saveBooking(ctx, im.feed.AccountID, b); err != nil {
		return err
	}
	return im.s.addActivity(ctx, im.feed.AccountID, domain.ImportedStayCancelled(b), nil, im.now)
}

func (im importer) lock(ctx context.Context, id uuid.UUID) (domain.Booking, error) {
	r, err := im.s.db(ctx).GetBookingForUpdate(ctx, queries.GetBookingForUpdateParams{ID: id, AccountID: im.feed.AccountID})
	if err != nil {
		return domain.Booking{}, fmt.Errorf("lock imported stay: %w", notFound(err))
	}
	return booking(queries.GetBookingRow(r)), nil
}

// reportClash tells the team, once per stay, when an imported stay's nights belong to a block
// or to a stay that wasn't imported. A clash between two imports is a channel echoing another,
// and a clash with a checkout hold clears up on its own; both stay quiet.
func (im importer) reportClash(ctx context.Context, b domain.Booking, clash *domain.ConflictError) error {
	q := im.s.db(ctx)
	other := ""
	switch {
	case clash.BookingID != nil:
		r, err := q.GetBooking(ctx, queries.GetBookingParams{ID: *clash.BookingID, AccountID: im.feed.AccountID})
		if err != nil {
			return fmt.Errorf("load the clash: %w", err)
		}
		if r.Booking.FeedID != nil {
			return nil
		}
		other = r.Booking.Ref
	case clash.BlockID == nil:
		return nil
	}
	told, err := q.HasActivity(ctx, queries.HasActivityParams{AccountID: im.feed.AccountID, BookingID: &b.ID, Kind: string(domain.ActivitySync)})
	if err != nil {
		return fmt.Errorf("check earlier alerts: %w", err)
	}
	if told {
		return nil
	}
	return im.s.addActivity(ctx, im.feed.AccountID, domain.ImportClash(b, other), nil, im.now)
}

// ParkFeeds keeps an account's feeds from being read until until, and shows them as working.
// The development seed uses it for sample feeds whose links are made up.
func (s *Postgres) ParkFeeds(ctx context.Context, accountID uuid.UUID, until time.Time) error {
	if err := s.db(ctx).ParkFeeds(ctx, queries.ParkFeedsParams{Until: until, AccountID: accountID}); err != nil {
		return fmt.Errorf("park feeds: %w", err)
	}
	return nil
}
