// Package server implements the HTTP API defined in packages/api-spec/openapi.yaml.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// Store is the persistence the server needs. internal/store.Postgres implements it. Owner data
// is only reachable through a tenant.Tenant.
type Store interface {
	Ping(ctx context.Context) error
	tenant.Memberships

	GetUser(ctx context.Context, id uuid.UUID) (domain.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, p domain.UserPatch) (domain.User, error)
	ListAccounts(ctx context.Context, userID uuid.UUID) ([]domain.AccountMembership, error)
	CreateAccount(ctx context.Context, userID uuid.UUID, name string) (domain.AccountMembership, error)

	ListProperties(ctx context.Context, t tenant.Tenant) ([]domain.PropertyDetail, error)
	GetProperty(ctx context.Context, t tenant.Tenant, id uuid.UUID) (domain.PropertyDetail, error)
	CreateProperty(ctx context.Context, t tenant.Tenant, p domain.NewProperty) (domain.PropertyDetail, error)
	UpdateProperty(ctx context.Context, t tenant.Tenant, id uuid.UUID, p domain.PropertyPatch) (domain.PropertyDetail, error)

	GetPropertyBySlug(ctx context.Context, slug string) (domain.Property, error)

	ListBookings(ctx context.Context, t tenant.Tenant, f domain.BookingFilter) ([]domain.Booking, error)
	GetBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID) (domain.Booking, error)
	CreateBooking(ctx context.Context, t tenant.Tenant, in domain.NewBooking, now time.Time) (domain.Booking, error)
	UpdateBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID, c domain.BookingChange) (domain.Booking, error)
	TransitionBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID, version int, to domain.BookingStatus, reason string, now time.Time) (domain.Booking, error)
	CancelBooking(ctx context.Context, t tenant.Tenant, id uuid.UUID, version int, reason string, refund *domain.Payment, now time.Time) (domain.Booking, error)
	RecordPayment(ctx context.Context, t tenant.Tenant, id uuid.UUID, p domain.Payment) (domain.Booking, error)
	RejectSlip(ctx context.Context, t tenant.Tenant, slipID uuid.UUID, now time.Time) (domain.Booking, error)
	Idempotent(ctx context.Context, t tenant.Tenant, key string, requestHash []byte, status int, now time.Time, fn func(ctx context.Context) ([]byte, error)) ([]byte, error)

	Calendar(ctx context.Context, t tenant.Tenant, propertyID uuid.UUID, from, to time.Time) ([]domain.Block, []domain.RateOverride, error)
	RateOverrides(ctx context.Context, t tenant.Tenant, propertyID uuid.UUID, from, to time.Time) ([]domain.RateOverride, error)
	SetRateOverrides(ctx context.Context, t tenant.Tenant, propertyID uuid.UUID, units []uuid.UUID, from, to time.Time, o domain.RateOverride) error
	CreateBlock(ctx context.Context, t tenant.Tenant, b domain.Block, ledgerUnits []uuid.UUID) (domain.Block, error)
	DeleteBlock(ctx context.Context, t tenant.Tenant, id uuid.UUID) error
}

// Options configures a Server.
type Options struct {
	// BookingDomain and BookingScheme build each property's booking page URL,
	// for example https://kingfisher.staykey.direct.
	BookingDomain string
	BookingScheme string
	// AllowAllOrigins opens CORS on every route. Use in development only.
	AllowAllOrigins bool
	// ClientIPHeader names the header a trusted proxy puts the client address in, for example
	// Fly-Client-IP. Empty uses the connection's address.
	ClientIPHeader string
	// PublicURL is the API's own address, for links to files the local store serves. Empty
	// derives it from each request, which suits development on a LAN.
	PublicURL string
	// Media serves the local file store's files and uploads under /media/. Nil when files live
	// in R2.
	Media  http.Handler
	Logger *slog.Logger
}

// Server implements oapi.StrictServerInterface.
type Server struct {
	store Store
	auth  *auth.Service
	files files.Store
	opts  Options
	log   *slog.Logger
	reqs  map[string]requirement
}

var _ oapi.StrictServerInterface = (*Server)(nil)

// New returns a Server backed by store, signing people in with authService and keeping photos
// in fileStore.
func New(store Store, authService *auth.Service, fileStore files.Store, opts Options) (*Server, error) {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	spec, err := oapi.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("load embedded spec: %w", err)
	}
	reqs, err := requirementsFrom(spec)
	if err != nil {
		return nil, fmt.Errorf("read security requirements: %w", err)
	}
	return &Server{store: store, auth: authService, files: fileStore, opts: opts, log: log, reqs: reqs}, nil
}

// GetHealth reports service health, including whether the database answers.
func (s *Server) GetHealth(ctx context.Context, _ oapi.GetHealthRequestObject) (oapi.GetHealthResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	health := oapi.Health{Status: oapi.HealthStatusOk, Database: oapi.Up}
	if err := s.store.Ping(ctx); err != nil {
		s.log.Warn("database ping failed", "error", err)
		health = oapi.Health{Status: oapi.HealthStatusDegraded, Database: oapi.Down}
	}
	return oapi.GetHealth200JSONResponse(health), nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
