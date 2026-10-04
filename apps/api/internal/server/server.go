// Package server implements the HTTP API defined in packages/api-spec/openapi.yaml.
package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// Store is the persistence the server needs. internal/store.Postgres implements it.
type Store interface {
	Ping(ctx context.Context) error
	ListProperties(ctx context.Context) ([]domain.Property, error)
	CreateProperty(ctx context.Context, p domain.NewProperty) (domain.Property, error)
	GetPropertyBySlug(ctx context.Context, slug string) (domain.Property, error)
}

// Options configures a Server.
type Options struct {
	// BookingDomain and BookingScheme build each property's booking page URL,
	// for example https://kingfisher.staykey.direct.
	BookingDomain string
	BookingScheme string
	// AllowAllOrigins opens CORS on every route. Use in development only.
	AllowAllOrigins bool
	Logger          *slog.Logger
}

// Server implements oapi.StrictServerInterface.
type Server struct {
	store Store
	opts  Options
	log   *slog.Logger
}

var _ oapi.StrictServerInterface = (*Server)(nil)

// New returns a Server backed by store.
func New(store Store, opts Options) *Server {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Server{store: store, opts: opts, log: log}
}

// GetHealth reports service health, including whether the database answers.
func (s *Server) GetHealth(ctx context.Context, _ oapi.GetHealthRequestObject) (oapi.GetHealthResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	health := oapi.Health{Status: oapi.Ok, Database: oapi.Up}
	if err := s.store.Ping(ctx); err != nil {
		s.log.Warn("database ping failed", "error", err)
		health = oapi.Health{Status: oapi.Degraded, Database: oapi.Down}
	}
	return oapi.GetHealth200JSONResponse(health), nil
}

// ListProperties returns the owner's properties.
func (s *Server) ListProperties(ctx context.Context, _ oapi.ListPropertiesRequestObject) (oapi.ListPropertiesResponseObject, error) {
	props, err := s.store.ListProperties(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]oapi.Property, 0, len(props))
	for _, p := range props {
		items = append(items, s.toAPI(p))
	}
	return oapi.ListProperties200JSONResponse{Items: items}, nil
}

// CreateProperty validates input, derives the slug when needed and stores the property.
func (s *Server) CreateProperty(ctx context.Context, req oapi.CreatePropertyRequestObject) (oapi.CreatePropertyResponseObject, error) {
	if req.Body == nil {
		return badRequest("invalid_body", "Request body is required."), nil
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
			return badRequest("invalid_"+vErr.Field, vErr.Error()), nil
		}
		return nil, err
	}

	created, err := s.store.CreateProperty(ctx, in)
	if errors.Is(err, domain.ErrSlugTaken) {
		return oapi.CreateProperty409JSONResponse{
			Code:    "slug_taken",
			Message: "That booking page address is already taken.",
		}, nil
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
		return oapi.GetPublicProperty404JSONResponse{NotFoundJSONResponse: oapi.NotFoundJSONResponse{
			Code:    "not_found",
			Message: "There is no booking page at this address.",
		}}, nil
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

func badRequest(code, message string) oapi.CreateProperty400JSONResponse {
	return oapi.CreateProperty400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse{Code: code, Message: message}}
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
