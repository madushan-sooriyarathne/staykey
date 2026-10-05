package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// CreateBlock closes b's nights on its units. ledgerUnits are the units whose nights it holds
// (b's units with their linked rooms). It returns a *domain.ConflictError when a stay or
// another block holds one of them.
func (s *Postgres) CreateBlock(ctx context.Context, t tenant.Tenant, b domain.Block, ledgerUnits []uuid.UUID) (domain.Block, error) {
	if !t.CanSeeProperty(b.PropertyID) {
		return domain.Block{}, domain.ErrNotFound
	}
	b.ID = domain.NewID()
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		if err := q.InsertBlock(ctx, queries.InsertBlockParams{
			ID: b.ID, AccountID: t.AccountID, PropertyID: b.PropertyID, StartsOn: b.From, EndsOn: b.To,
			Reason: b.Reason, Note: b.Note, CreatedBy: &t.UserID,
		}); err != nil {
			return fmt.Errorf("insert block: %w", err)
		}
		for _, unit := range b.UnitIDs {
			if err := q.InsertBlockUnit(ctx, queries.InsertBlockUnitParams{
				AccountID: t.AccountID, PropertyID: b.PropertyID, BlockID: b.ID, UnitID: unit,
			}); err != nil {
				return fmt.Errorf("insert block unit: %w", err)
			}
		}
		return s.holdNights(ctx, t.AccountID, b.PropertyID, ledgerUnits, b.From, b.To, nil, &b.ID)
	})
	if err != nil {
		return domain.Block{}, err
	}
	return b, nil
}

// DeleteBlock opens a block's nights again.
func (s *Postgres) DeleteBlock(ctx context.Context, t tenant.Tenant, id uuid.UUID) error {
	q := s.db(ctx)
	b, err := q.GetBlock(ctx, queries.GetBlockParams{ID: id, AccountID: t.AccountID})
	if err != nil {
		return notFound(err)
	}
	if !t.CanSeeProperty(b.PropertyID) {
		return domain.ErrNotFound
	}
	if err := q.DeleteBlock(ctx, queries.DeleteBlockParams{ID: id, AccountID: t.AccountID}); err != nil {
		return fmt.Errorf("delete block: %w", err)
	}
	return nil
}

// Calendar returns a property's blocks and rate overrides that touch [from, to).
func (s *Postgres) Calendar(ctx context.Context, t tenant.Tenant, propertyID uuid.UUID, from, to time.Time) ([]domain.Block, []domain.RateOverride, error) {
	if !t.CanSeeProperty(propertyID) {
		return nil, nil, domain.ErrNotFound
	}
	q := s.db(ctx)
	rows, err := q.ListBlocks(ctx, queries.ListBlocksParams{AccountID: t.AccountID, PropertyID: propertyID, RangeFrom: from, RangeTo: to})
	if err != nil {
		return nil, nil, fmt.Errorf("list blocks: %w", err)
	}
	blocks := make([]domain.Block, len(rows))
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]*domain.Block, len(rows))
	for i, r := range rows {
		blocks[i] = domain.Block{ID: r.ID, PropertyID: r.PropertyID, From: r.StartsOn, To: r.EndsOn, Reason: r.Reason, Note: r.Note, UnitIDs: []uuid.UUID{}}
		ids[i] = r.ID
		index[r.ID] = &blocks[i]
	}
	units, err := q.ListBlockUnits(ctx, queries.ListBlockUnitsParams{AccountID: t.AccountID, BlockIds: ids})
	if err != nil {
		return nil, nil, fmt.Errorf("list block units: %w", err)
	}
	for _, u := range units {
		b := index[u.BlockID]
		b.UnitIDs = append(b.UnitIDs, u.UnitID)
	}
	overrides, err := s.RateOverrides(ctx, t, propertyID, from, to)
	if err != nil {
		return nil, nil, err
	}
	return blocks, overrides, nil
}

// RateOverrides returns a property's overrides for the nights in [from, to).
func (s *Postgres) RateOverrides(ctx context.Context, t tenant.Tenant, propertyID uuid.UUID, from, to time.Time) ([]domain.RateOverride, error) {
	rows, err := s.db(ctx).ListRateOverrides(ctx, queries.ListRateOverridesParams{
		AccountID: t.AccountID, PropertyID: propertyID, RangeFrom: from, RangeTo: to,
	})
	if err != nil {
		return nil, fmt.Errorf("list rate overrides: %w", err)
	}
	out := make([]domain.RateOverride, len(rows))
	for i, r := range rows {
		out[i] = domain.RateOverride{
			UnitID: r.UnitID, Night: r.Night, Price: deref(r.Price), MinNights: int(deref(r.MinNights)), ClosedToArrival: r.ClosedToArrival,
		}
	}
	return out, nil
}

// SetRateOverrides replaces the overrides on units for every night in [from, to). An override
// with nothing set clears them.
func (s *Postgres) SetRateOverrides(ctx context.Context, t tenant.Tenant, propertyID uuid.UUID, units []uuid.UUID, from, to time.Time, o domain.RateOverride) error {
	if !t.CanSeeProperty(propertyID) {
		return domain.ErrNotFound
	}
	return s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		if err := q.DeleteRateOverrides(ctx, queries.DeleteRateOverridesParams{
			AccountID: t.AccountID, PropertyID: propertyID, UnitIds: units, RangeFrom: from, RangeTo: to,
		}); err != nil {
			return fmt.Errorf("delete rate overrides: %w", err)
		}
		if o.Price == 0 && o.MinNights == 0 && !o.ClosedToArrival {
			return nil
		}
		var minNights *int16
		if o.MinNights > 0 {
			n := int16(o.MinNights)
			minNights = &n
		}
		var price *int64
		if o.Price > 0 {
			price = &o.Price
		}
		if err := q.InsertRateOverrides(ctx, queries.InsertRateOverridesParams{
			AccountID: t.AccountID, PropertyID: propertyID, UnitIds: units, RangeFrom: from, RangeTo: to,
			Price: price, MinNights: minNights, ClosedToArrival: o.ClosedToArrival,
		}); err != nil {
			return fmt.Errorf("insert rate overrides: %w", err)
		}
		return nil
	})
}
