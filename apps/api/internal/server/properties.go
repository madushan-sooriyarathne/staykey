package server

import (
	"context"
	"errors"
	"fmt"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// ListProperties returns the properties the caller can see in the account, with their setup.
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
		items = append(items, s.propertyToAPI(ctx, p, t.Can(domain.PermSettings)))
	}
	return oapi.ListProperties200JSONResponse{Items: items}, nil
}

// GetProperty returns one property with its setup.
func (s *Server) GetProperty(ctx context.Context, req oapi.GetPropertyRequestObject) (oapi.GetPropertyResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.store.GetProperty(ctx, t, req.Id)
	if errors.Is(err, domain.ErrNotFound) {
		return oapi.GetProperty404JSONResponse{NotFoundJSONResponse: propertyNotFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	return oapi.GetProperty200JSONResponse(s.propertyToAPI(ctx, p, t.Can(domain.PermSettings))), nil
}

// CreateProperty validates input, derives the slug when needed and stores the property, with
// its full setup when onboarding sends it. Only owners can add properties, since they count
// towards the plan.
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
		BaseRateMinor: deref(req.Body.BaseRate),
		Slug:          deref(req.Body.Slug),
		Location:      deref(req.Body.Location),
	}
	if req.Body.Setup != nil {
		in.Setup = patchFromAPI(*req.Body.Setup)
	}
	in.Normalize()
	if err := in.Validate(); err != nil {
		if bad := validationResponse(err, "setup."); bad != nil {
			return oapi.CreateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(*bad)}, nil
		}
		return nil, err
	}
	if bad, err := s.checkUploads(ctx, t, nil, in.Setup); err != nil || bad != nil {
		if bad != nil {
			return oapi.CreateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(*bad)}, nil
		}
		return nil, err
	}

	created, err := s.store.CreateProperty(ctx, t, in)
	if errors.Is(err, domain.ErrSlugTaken) {
		return oapi.CreateProperty409JSONResponse(fieldErr("slug_taken", "slug", "That booking page address is already taken.")), nil
	}
	if bad := validationResponse(err, "setup."); bad != nil {
		return oapi.CreateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(*bad)}, nil
	}
	if err != nil {
		return nil, err
	}
	return oapi.CreateProperty201JSONResponse(s.propertyToAPI(ctx, created, true)), nil
}

// UpdateProperty applies the sections the owner changed. Owners and managers only.
func (s *Server) UpdateProperty(ctx context.Context, req oapi.UpdatePropertyRequestObject) (oapi.UpdatePropertyResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	current, err := s.store.GetProperty(ctx, t, req.Id)
	if errors.Is(err, domain.ErrNotFound) {
		return oapi.UpdateProperty404JSONResponse{NotFoundJSONResponse: propertyNotFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return oapi.UpdateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(errBody("invalid_body", "Request body is required."))}, nil
	}

	patch := patchFromAPI(*req.Body)
	if !t.Can(domain.PermSettings) || (patch.TouchesPrices() && !t.Can(domain.PermPrices)) {
		return nil, errForbidden
	}
	patch.Normalize()
	if err := patch.Validate(); err != nil {
		if bad := validationResponse(err, ""); bad != nil {
			return oapi.UpdateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(*bad)}, nil
		}
		return nil, err
	}
	if bad, err := s.checkUploads(ctx, t, &current, patch); err != nil || bad != nil {
		if bad != nil {
			return oapi.UpdateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(*bad)}, nil
		}
		return nil, err
	}

	updated, err := s.store.UpdateProperty(ctx, t, req.Id, patch)
	if errors.Is(err, domain.ErrNotFound) {
		return oapi.UpdateProperty404JSONResponse{NotFoundJSONResponse: propertyNotFound()}, nil
	}
	if bad := validationResponse(err, ""); bad != nil {
		return oapi.UpdateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(*bad)}, nil
	}
	if err != nil {
		return nil, err
	}
	return oapi.UpdateProperty200JSONResponse(s.propertyToAPI(ctx, updated, true)), nil
}

// checkUploads makes sure every photo and logo the patch adds was uploaded to this account.
// Keys the property already has pass as they are.
func (s *Server) checkUploads(ctx context.Context, t tenant.Tenant, current *domain.PropertyDetail, p domain.PropertyPatch) (*oapi.Error, error) {
	known := map[string]bool{}
	if current != nil {
		for _, ph := range current.Photos {
			known[ph.Key] = true
		}
		known[current.Branding.LogoKey] = true
	}
	check := func(field, key string, kind files.Kind) (*oapi.Error, error) {
		if key == "" || known[key] {
			return nil, nil
		}
		if !files.Belongs(key, t.AccountID, kind) {
			e := fieldErr("invalid_upload", field, "That file wasn't uploaded for this account.")
			return &e, nil
		}
		ok, err := s.files.Exists(ctx, key)
		if err != nil {
			return nil, err
		}
		if !ok {
			e := fieldErr("upload_missing", field, "That file hasn't finished uploading. Try again.")
			return &e, nil
		}
		return nil, nil
	}
	if p.Photos != nil {
		for i, ph := range *p.Photos {
			if bad, err := check(fmt.Sprintf("photos[%d].key", i), ph.Key, files.KindPhoto); bad != nil || err != nil {
				return bad, err
			}
		}
	}
	if p.Branding != nil {
		return check("branding.logoKey", p.Branding.LogoKey, files.KindLogo)
	}
	return nil, nil
}

// validationResponse turns a domain validation error into a 400 body, or returns nil for other
// errors. prefix places nested fields, such as "setup." for a new property's setup.
func validationResponse(err error, prefix string) *oapi.Error {
	var v *domain.ValidationError
	if !errors.As(err, &v) {
		return nil
	}
	field := v.Field
	if prefix != "" && field != "name" && field != "slug" && field != "bookingType" &&
		field != "currency" && field != "baseRate" && field != "location" {
		field = prefix + field
	}
	e := fieldErr("invalid_"+fieldOf(v.Field), field, sentence(v.Message))
	return &e
}

func propertyNotFound() oapi.NotFoundJSONResponse {
	return oapi.NotFoundJSONResponse(errBody("property_not_found", "That property doesn't exist in this account."))
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

func (s *Server) bookingPageURL(slug string) string {
	return s.opts.BookingScheme + "://" + slug + "." + s.opts.BookingDomain
}
