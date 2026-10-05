package server

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// GetCalendar returns a property's blocks and rate overrides, the next twelve months unless a
// range is given.
func (s *Server) GetCalendar(ctx context.Context, req oapi.GetCalendarRequestObject) (oapi.GetCalendarResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetProperty(ctx, t, req.Id); err != nil {
		return nil, apiErr(err, "property")
	}
	from := time.Now().UTC().Truncate(24 * time.Hour)
	if req.Params.From != nil {
		from = req.Params.From.Time
	}
	to := from.AddDate(1, 0, 0)
	if req.Params.To != nil {
		to = req.Params.To.Time
	}
	if n := domain.Nights(from, to); n < 1 || n > 800 {
		return nil, apiErr(&domain.ValidationError{Field: "to", Message: "must be between 1 and 800 days after from"}, "")
	}
	blocks, overrides, err := s.store.Calendar(ctx, t, req.Id, from, to)
	if err != nil {
		return nil, err
	}
	out := oapi.Calendar{Blocks: make([]oapi.Block, len(blocks)), RateOverrides: make([]oapi.RateOverride, len(overrides))}
	for i, b := range blocks {
		out.Blocks[i] = blockToAPI(b)
	}
	for i, o := range overrides {
		out.RateOverrides[i] = oapi.RateOverride{UnitId: o.UnitID, Night: day(o.Night), ClosedToArrival: o.ClosedToArrival}
		if o.Price > 0 {
			out.RateOverrides[i].Price = &o.Price
		}
		if o.MinNights > 0 {
			out.RateOverrides[i].MinNights = &o.MinNights
		}
	}
	return oapi.GetCalendar200JSONResponse(out), nil
}

// CreateBlock closes nights on some units. Blocking a whole-house unit holds its rooms too.
func (s *Server) CreateBlock(ctx context.Context, req oapi.CreateBlockRequestObject) (oapi.CreateBlockResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermManage) {
		return nil, errForbidden
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	in := req.Body
	p, err := s.store.GetProperty(ctx, t, in.PropertyId)
	if err != nil {
		return nil, apiErr(err, "property")
	}
	b := domain.Block{
		PropertyID: p.ID, UnitIDs: in.UnitIds, From: in.From.Time, To: in.To.Time,
		Reason: string(in.Reason), Note: strings.TrimSpace(deref(in.Note)),
	}
	if err := b.Validate(p); err != nil {
		return nil, apiErr(err, "")
	}
	var ledger []uuid.UUID
	for _, u := range b.UnitIDs {
		for _, id := range domain.LedgerUnits(p.Units, u) {
			if !slices.Contains(ledger, id) {
				ledger = append(ledger, id)
			}
		}
	}
	created, err := s.store.CreateBlock(ctx, t, b, ledger)
	if err != nil {
		return nil, apiErr(err, "property")
	}
	return oapi.CreateBlock201JSONResponse(blockToAPI(created)), nil
}

// DeleteBlock opens a block's nights again.
func (s *Server) DeleteBlock(ctx context.Context, req oapi.DeleteBlockRequestObject) (oapi.DeleteBlockResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermManage) {
		return nil, errForbidden
	}
	if err := s.store.DeleteBlock(ctx, t, req.BlockId); err != nil {
		return nil, apiErr(err, "block")
	}
	return oapi.DeleteBlock204Response{}, nil
}

// SetRateOverrides sets or clears the price and stay rules for some nights.
func (s *Server) SetRateOverrides(ctx context.Context, req oapi.SetRateOverridesRequestObject) (oapi.SetRateOverridesResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermPrices) {
		return nil, errForbidden
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	in := req.Body
	p, err := s.store.GetProperty(ctx, t, in.PropertyId)
	if err != nil {
		return nil, apiErr(err, "property")
	}
	o := domain.RateOverride{Price: deref(in.Price), MinNights: deref(in.MinNights), ClosedToArrival: deref(in.ClosedToArrival)}
	var invalid error
	switch n := domain.Nights(in.From.Time, in.To.Time); {
	case len(in.UnitIds) == 0:
		invalid = &domain.ValidationError{Field: "unitIds", Message: "must name at least one unit"}
	case n < 1 || n > 366:
		invalid = &domain.ValidationError{Field: "to", Message: "must be between 1 and 366 nights after from"}
	case in.Price != nil && o.Price < 1:
		invalid = &domain.ValidationError{Field: "price", Message: "must be more than zero"}
	case in.MinNights != nil && (o.MinNights < 1 || o.MinNights > 365):
		invalid = &domain.ValidationError{Field: "minNights", Message: "must be between 1 and 365"}
	}
	for _, id := range in.UnitIds {
		if !slices.ContainsFunc(p.Units, func(u domain.Unit) bool { return u.ID == id }) {
			invalid = &domain.ValidationError{Field: "unitIds", Message: "must be units of this property"}
		}
	}
	if invalid != nil {
		return nil, apiErr(invalid, "")
	}
	if err := s.store.SetRateOverrides(ctx, t, p.ID, in.UnitIds, in.From.Time, in.To.Time, o); err != nil {
		return nil, apiErr(err, "property")
	}
	return oapi.SetRateOverrides204Response{}, nil
}

func blockToAPI(b domain.Block) oapi.Block {
	return oapi.Block{
		Id: b.ID, PropertyId: b.PropertyID, UnitIds: b.UnitIDs, From: day(b.From), To: day(b.To),
		Reason: oapi.BlockReason(b.Reason), Note: b.Note,
	}
}
