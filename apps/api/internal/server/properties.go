package server

import (
	"context"
	"errors"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// ListProperties returns the properties the caller can see in the account.
func (s *Server) ListProperties(ctx context.Context, _ oapi.ListPropertiesRequestObject) (oapi.ListPropertiesResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	props, err := s.store.ListProperties(ctx, t)
	if err != nil {
		return nil, err
	}
	items := make([]oapi.Property, 0, len(props))
	for _, p := range props {
		items = append(items, s.toAPI(p))
	}
	return oapi.ListProperties200JSONResponse{Items: items}, nil
}

// CreateProperty validates input, derives the slug when needed and stores the property in the
// caller's account. Only owners can add properties, since they count towards the plan.
func (s *Server) CreateProperty(ctx context.Context, req oapi.CreatePropertyRequestObject) (oapi.CreatePropertyResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermBilling) {
		return nil, errForbidden
	}
	if req.Body == nil {
		return oapi.CreateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(errBody("invalid_body", "Request body is required."))}, nil
	}

	in := domain.NewProperty{
		Name:          req.Body.Name,
		BookingType:   domain.BookingType(req.Body.BookingType),
		Currency:      string(req.Body.Currency),
		BaseRateMinor: req.Body.BaseRate,
	}
	if req.Body.Slug != nil {
		in.Slug = *req.Body.Slug
	}
	if req.Body.Location != nil {
		in.Location = *req.Body.Location
	}
	in.Normalize()

	if err := in.Validate(); err != nil {
		var vErr *domain.ValidationError
		if errors.As(err, &vErr) {
			return oapi.CreateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
				fieldErr("invalid_"+vErr.Field, vErr.Field, vErr.Error()))}, nil
		}
		return nil, err
	}

	created, err := s.store.CreateProperty(ctx, t, in)
	if errors.Is(err, domain.ErrSlugTaken) {
		return oapi.CreateProperty409JSONResponse(fieldErr("slug_taken", "slug", "That booking page address is already taken.")), nil
	}
	if err != nil {
		return nil, err
	}
	return oapi.CreateProperty201JSONResponse(s.toAPI(created)), nil
}

// GetPublicProperty returns what a guest sees on a booking page.
func (s *Server) GetPublicProperty(ctx context.Context, req oapi.GetPublicPropertyRequestObject) (oapi.GetPublicPropertyResponseObject, error) {
	p, err := s.store.GetPropertyBySlug(ctx, req.Slug)
	if errors.Is(err, domain.ErrNotFound) {
		return oapi.GetPublicProperty404JSONResponse{NotFoundJSONResponse: oapi.NotFoundJSONResponse(
			errBody("not_found", "There is no booking page at this address."))}, nil
	}
	if err != nil {
		return nil, err
	}
	return oapi.GetPublicProperty200JSONResponse{
		Slug:        p.Slug,
		Name:        p.Name,
		BookingType: oapi.BookingType(p.BookingType),
		Location:    optional(p.Location),
		Currency:    oapi.Currency(p.Currency),
		BaseRate:    p.BaseRateMinor,
	}, nil
}

func (s *Server) toAPI(p domain.Property) oapi.Property {
	return oapi.Property{
		Id:             p.ID,
		Slug:           p.Slug,
		Name:           p.Name,
		BookingType:    oapi.BookingType(p.BookingType),
		Location:       optional(p.Location),
		Currency:       oapi.Currency(p.Currency),
		BaseRate:       p.BaseRateMinor,
		BookingPageUrl: s.bookingPageURL(p.Slug),
		CreatedAt:      p.CreatedAt,
	}
}

func (s *Server) bookingPageURL(slug string) string {
	return s.opts.BookingScheme + "://" + slug + "." + s.opts.BookingDomain
}
