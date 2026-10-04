package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// ListProperties returns the properties the tenant can see with their full setup, newest first.
func (s *Postgres) ListProperties(ctx context.Context, t tenant.Tenant) ([]domain.PropertyDetail, error) {
	rows, err := s.db(ctx).ListProperties(ctx, queries.ListPropertiesParams{
		AccountID:   t.AccountID,
		PropertyIds: t.PropertyScope(),
	})
	if err != nil {
		return nil, fmt.Errorf("list properties: %w", err)
	}
	details := make([]domain.PropertyDetail, len(rows))
	for i, r := range rows {
		details[i] = propertyDetail(queries.GetPropertyRow(r))
	}
	if err := s.loadSetup(ctx, t.AccountID, details); err != nil {
		return nil, fmt.Errorf("list properties: %w", err)
	}
	return details, nil
}

// GetProperty returns one property with its full setup, or domain.ErrNotFound when it is not
// in the tenant's account or outside a caretaker's properties.
func (s *Postgres) GetProperty(ctx context.Context, t tenant.Tenant, id uuid.UUID) (domain.PropertyDetail, error) {
	if !t.CanSeeProperty(id) {
		return domain.PropertyDetail{}, domain.ErrNotFound
	}
	r, err := s.db(ctx).GetProperty(ctx, queries.GetPropertyParams{ID: id, AccountID: t.AccountID})
	if err != nil {
		return domain.PropertyDetail{}, notFound(err)
	}
	details := []domain.PropertyDetail{propertyDetail(r)}
	if err := s.loadSetup(ctx, t.AccountID, details); err != nil {
		return domain.PropertyDetail{}, fmt.Errorf("get property: %w", err)
	}
	return details[0], nil
}

// CreateProperty adds a property to the tenant's account with the setup in p, filling in the
// defaults for anything p leaves out. It returns domain.ErrSlugTaken when the booking page
// address is already used.
func (s *Postgres) CreateProperty(ctx context.Context, t tenant.Tenant, p domain.NewProperty) (domain.PropertyDetail, error) {
	id := domain.NewID()
	setup := p.Setup
	if setup.Units == nil {
		name, sleeps := p.Name, 4
		if p.BookingType == domain.BookingRooms {
			name, sleeps = "Room 1", 2
		}
		setup.Units = &[]domain.UnitInput{{Ref: "unit", Name: name, Sleeps: sleeps, Rate: p.BaseRateMinor}}
	}
	if setup.Charges == nil {
		charges := domain.DefaultCharges()
		setup.Charges = &charges
	}

	err := s.InTx(ctx, func(ctx context.Context) error {
		token, err := exportToken()
		if err != nil {
			return err
		}
		defaults := domain.Defaults(p.Currency)
		if err := s.db(ctx).InsertProperty(ctx, queries.InsertPropertyParams{
			ID:                id,
			AccountID:         t.AccountID,
			Slug:              p.Slug,
			Name:              p.Name,
			BookingType:       string(p.BookingType),
			Location:          p.Location,
			Currency:          p.Currency,
			IcalExportToken:   token,
			DisplayCurrencies: defaults.Booking.DisplayCurrencies,
		}); err != nil {
			return err
		}
		if err := s.db(ctx).UpsertPaymentSettings(ctx, queries.UpsertPaymentSettingsParams{
			PropertyID: id, AccountID: t.AccountID, PayWithinHours: 24, CancelIfUnpaid: true,
			AtProperty: true, CardsStatus: "off",
		}); err != nil {
			return err
		}
		return s.applySetup(ctx, t.AccountID, id, setup)
	})
	if violates(err, "properties_slug_key") {
		return domain.PropertyDetail{}, domain.ErrSlugTaken
	}
	if err != nil {
		return domain.PropertyDetail{}, setupError("create property", err)
	}
	return s.GetProperty(ctx, t, id)
}

// UpdateProperty applies a patch to one property's setup in one transaction and returns the
// result.
func (s *Postgres) UpdateProperty(ctx context.Context, t tenant.Tenant, id uuid.UUID, patch domain.PropertyPatch) (domain.PropertyDetail, error) {
	if !t.CanSeeProperty(id) {
		return domain.PropertyDetail{}, domain.ErrNotFound
	}
	err := s.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.db(ctx).LockProperty(ctx, queries.LockPropertyParams{ID: id, AccountID: t.AccountID}); err != nil {
			return notFound(err)
		}
		return s.applySetup(ctx, t.AccountID, id, patch)
	})
	if err != nil {
		return domain.PropertyDetail{}, setupError("update property", err)
	}
	return s.GetProperty(ctx, t, id)
}

// GetPropertyBySlug returns the property behind a booking page address. It is public and not
// scoped to an account.
func (s *Postgres) GetPropertyBySlug(ctx context.Context, slug string) (domain.Property, error) {
	r, err := s.db(ctx).GetPropertyBySlug(ctx, slug)
	if err != nil {
		return domain.Property{}, notFound(err)
	}
	return propertyDetail(queries.GetPropertyRow(r)).Property, nil
}

// setupError keeps not-found and validation errors recognisable and wraps the rest.
func setupError(op string, err error) error {
	var v *domain.ValidationError
	if errors.Is(err, domain.ErrNotFound) || errors.As(err, &v) {
		return err
	}
	if violates(err, "promos_property_id_code_key") {
		return &domain.ValidationError{Field: "promos", Message: "two codes can't be the same"}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func exportToken() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func propertyDetail(r queries.GetPropertyRow) domain.PropertyDetail {
	d := domain.PropertyDetail{
		Property: domain.Property{
			ID:            r.ID,
			AccountID:     r.AccountID,
			Slug:          r.Slug,
			Name:          r.Name,
			BookingType:   domain.BookingType(r.BookingType),
			Currency:      r.Currency,
			BaseRateMinor: r.BaseRate,
			CreatedAt:     r.CreatedAt,
		},
		Description:    r.Description,
		Amenities:      r.Amenities,
		CheckIn:        r.CheckInTime,
		CheckOut:       r.CheckOutTime,
		TimeZone:       r.TimeZone,
		ExtraGuest:     domain.ExtraGuest{Above: int(r.ExtraGuestAbove), Amount: r.ExtraGuestAmount},
		Policy:         r.Policy,
		DepositPercent: int(r.DepositPercent),
		BalanceDueDays: int(r.BalanceDueDays),
		HouseRules:     r.HouseRules,
		Rules: domain.StayRules{
			MinNights:     int(r.MinNights),
			MaxNights:     int(r.MaxNights),
			WindowMonths:  int(r.WindowMonths),
			ClosedArrival: ints(r.ClosedArrival),
		},
		Booking: domain.BookingSettings{
			Mode:              r.BookingMode,
			ReplyHours:        int(r.ReplyHours),
			HoldMinutes:       int(r.HoldMinutes),
			DisplayCurrencies: r.DisplayCurrencies,
		},
		Branding:        domain.Branding{Color: r.BrandColor},
		IcalExportToken: r.IcalExportToken,
		Photos:          []domain.Photo{},
		Units:           []domain.Unit{},
		Seasons:         []domain.Season{},
		LengthDiscounts: []domain.LengthDiscount{},
		Charges:         []domain.Charge{},
		Extras:          []domain.Extra{},
		Promos:          []domain.Promo{},
		IcalFeeds:       []domain.IcalFeed{},
	}
	if r.Location != nil {
		d.Location = *r.Location
	}
	if r.SameDayCutoff != nil {
		h := int(*r.SameDayCutoff)
		d.Rules.SameDayCutoff = &h
	}
	if r.LogoKey != nil {
		d.Branding.LogoKey = *r.LogoKey
	}
	return d
}

func ints(in []int16) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}
