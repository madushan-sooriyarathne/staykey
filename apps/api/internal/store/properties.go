package store

import (
	"context"
	"fmt"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// ListProperties returns the properties the tenant can see, newest first.
func (s *Postgres) ListProperties(ctx context.Context, t tenant.Tenant) ([]domain.Property, error) {
	rows, err := s.db(ctx).ListProperties(ctx, queries.ListPropertiesParams{
		AccountID:   t.AccountID,
		PropertyIds: t.PropertyScope(),
	})
	if err != nil {
		return nil, fmt.Errorf("list properties: %w", err)
	}
	out := make([]domain.Property, len(rows))
	for i, r := range rows {
		out[i] = property(queries.GetPropertyBySlugRow(r))
	}
	return out, nil
}

// CreateProperty adds a property to the tenant's account. It returns domain.ErrSlugTaken when
// the booking page address is already used.
func (s *Postgres) CreateProperty(ctx context.Context, t tenant.Tenant, p domain.NewProperty) (domain.Property, error) {
	var created queries.CreatePropertyRow
	// A savepoint keeps a slug clash from aborting a surrounding transaction.
	err := s.InTx(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.db(ctx).CreateProperty(ctx, queries.CreatePropertyParams{
			ID:            domain.NewID(),
			AccountID:     t.AccountID,
			Slug:          p.Slug,
			Name:          p.Name,
			BookingType:   string(p.BookingType),
			Location:      p.Location,
			Currency:      p.Currency,
			BaseRateMinor: p.BaseRateMinor,
		})
		return err
	})
	if isUniqueViolation(err) {
		return domain.Property{}, domain.ErrSlugTaken
	}
	if err != nil {
		return domain.Property{}, fmt.Errorf("create property: %w", err)
	}
	return property(queries.GetPropertyBySlugRow(created)), nil
}

// GetPropertyBySlug returns the property behind a booking page address. It is public and not
// scoped to an account.
func (s *Postgres) GetPropertyBySlug(ctx context.Context, slug string) (domain.Property, error) {
	r, err := s.db(ctx).GetPropertyBySlug(ctx, slug)
	if err != nil {
		return domain.Property{}, notFound(err)
	}
	return property(r), nil
}

func property(r queries.GetPropertyBySlugRow) domain.Property {
	p := domain.Property{
		ID:            r.ID,
		AccountID:     r.AccountID,
		Slug:          r.Slug,
		Name:          r.Name,
		BookingType:   domain.BookingType(r.BookingType),
		Currency:      r.Currency,
		BaseRateMinor: r.BaseRateMinor,
		CreatedAt:     r.CreatedAt,
	}
	if r.Location != nil {
		p.Location = *r.Location
	}
	return p
}
