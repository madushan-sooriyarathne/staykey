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
	"staykey.direct/api/internal/ratelimit"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/tenant"
)

// Store is the persistence the server needs. internal/store.Postgres implements it. Owner data
// is only reachable through a tenant.Tenant.
type Store interface {
	Ping(ctx context.Context) error
	tenant.Memberships

	GetUser(ctx context.Context, id uuid.UUID) (domain.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, p domain.UserPatch, now time.Time) (domain.User, error)
	ResendConfirmation(ctx context.Context, userID uuid.UUID, now time.Time) error
	ConfirmEmail(ctx context.Context, token string, now time.Time) (string, error)
	RecordSuppression(ctx context.Context, address, reason string) error
	ListAccounts(ctx context.Context, userID uuid.UUID) ([]domain.AccountMembership, error)
	CreateAccount(ctx context.Context, userID uuid.UUID, name string) (domain.AccountMembership, error)

	ListProperties(ctx context.Context, t tenant.Tenant) ([]domain.PropertyDetail, error)
	GetProperty(ctx context.Context, t tenant.Tenant, id uuid.UUID) (domain.PropertyDetail, error)
	CreateProperty(ctx context.Context, t tenant.Tenant, p domain.NewProperty) (domain.PropertyDetail, error)
	UpdateProperty(ctx context.Context, t tenant.Tenant, id uuid.UUID, p domain.PropertyPatch) (domain.PropertyDetail, error)

	GetPropertyBySlug(ctx context.Context, slug string) (domain.Property, error)
	PublicProperty(ctx context.Context, slug string) (domain.PropertyDetail, error)
	Availability(ctx context.Context, accountID, propertyID uuid.UUID, from, to, now time.Time) (map[uuid.UUID][]time.Time, error)
	CreateHold(ctx context.Context, accountID, propertyID uuid.UUID, units []uuid.UUID, from, to, expiresAt, now time.Time, replaces *uuid.UUID) (uuid.UUID, error)
	CreateGuestBooking(ctx context.Context, accountID uuid.UUID, in domain.NewBooking, now time.Time) (uuid.UUID, error)
	GuestBooking(ctx context.Context, id uuid.UUID, tokenHash []byte) (domain.GuestBooking, error)
	AddGuestSlip(ctx context.Context, bookingID uuid.UUID, tokenHash []byte, key string, amount int64, now time.Time) error
	MarkFeedDue(ctx context.Context, t tenant.Tenant, feedID uuid.UUID) (domain.IcalFeed, error)
	ExportCalendar(ctx context.Context, slug, token string, unitID *uuid.UUID, from time.Time) (string, []domain.ExportRun, error)

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

	ListTeam(ctx context.Context, t tenant.Tenant, now time.Time) ([]domain.TeamMember, []domain.Invite, error)
	InviteMember(ctx context.Context, t tenant.Tenant, in domain.NewInvite, tokenHash []byte, now time.Time) (domain.Invite, error)
	RevokeInvite(ctx context.Context, t tenant.Tenant, id uuid.UUID, now time.Time) error
	UpdateMember(ctx context.Context, t tenant.Tenant, membershipID uuid.UUID, p domain.MemberPatch) (domain.TeamMember, error)
	RemoveMember(ctx context.Context, t tenant.Tenant, membershipID uuid.UUID) error
	PreviewInvite(ctx context.Context, tokenHash []byte, now time.Time) (domain.InvitePreview, error)
	AcceptInvite(ctx context.Context, userID uuid.UUID, tokenHash []byte, now time.Time) (domain.AccountMembership, error)

	ListActivity(ctx context.Context, t tenant.Tenant, after *store.ActivityCursor, limit int) ([]domain.Activity, error)
	UnreadActivity(ctx context.Context, t tenant.Tenant) (int, error)
	MarkActivityRead(ctx context.Context, t tenant.Tenant, now time.Time) error

	ListTemplates(ctx context.Context, t tenant.Tenant) ([]domain.Template, error)
	UpdateTemplate(ctx context.Context, t tenant.Tenant, id uuid.UUID, body *string, channels []string) (domain.Template, error)
	NotificationPrefs(ctx context.Context, t tenant.Tenant) ([]domain.NotificationPref, error)
	SetNotificationPrefs(ctx context.Context, t tenant.Tenant, prefs []domain.NotificationPref) ([]domain.NotificationPref, error)
	SavePushToken(ctx context.Context, userID, sessionID uuid.UUID, token, platform string) error

	SetUserPhoto(ctx context.Context, userID uuid.UUID, key string) error
	DeleteUser(ctx context.Context, userID uuid.UUID, now time.Time) (time.Time, error)
	RenameAccount(ctx context.Context, t tenant.Tenant, name string) (domain.AccountMembership, error)
	EraseGuest(ctx context.Context, t tenant.Tenant, bookingID uuid.UUID) error

	AccountBilling(ctx context.Context, accountID uuid.UUID, now time.Time) (domain.AccountBilling, error)
	Subscription(ctx context.Context, t tenant.Tenant, now time.Time) (domain.AccountBilling, error)
	ApplyStoreSubscriptions(ctx context.Context, ev store.BillingEvent, accountID uuid.UUID, subs []domain.Subscription, eventAt, now time.Time) error
	RecordBillingEvent(ctx context.Context, ev store.BillingEvent) error
	CreateWebCheckout(ctx context.Context, t tenant.Tenant, plan domain.PlanID, period domain.Period, now time.Time) (uuid.UUID, int64, error)
	SetWebCheckoutProvider(ctx context.Context, t tenant.Tenant, id uuid.UUID, providerID string) error
	PayWebCheckout(ctx context.Context, ev store.BillingEvent, pay store.WebPayment, now time.Time) (domain.PaidPeriod, error)
	ExpireWebCheckout(ctx context.Context, ev store.BillingEvent, providerID string) error
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
	// InviteURL is where invite links point, followed by the token. Empty uses
	// https://staykey.direct/join.
	InviteURL string
	// ForwardSecret lets the booking page and the marketing site pass on the visitor's address
	// in X-StayKey-Guest-IP, for rate limits. Empty ignores those headers.
	ForwardSecret string
	// Subscribers reads store plans from RevenueCat; ExternalPurchases mirrors web periods to it.
	// RevenueCatWebhookAuth is the Authorization value set in RevenueCat's dashboard. Nil or
	// empty leaves those routes answering 503.
	Subscribers           SubscriberSource
	ExternalPurchases     ExternalRecorder
	RevenueCatWebhookAuth string
	// WebPayments takes web payments through Payments.lk, returning owners to BillingURL.
	WebPayments WebGateway
	BillingURL  string
	// SNS checks the bounce and complaint notifications SES sends through SNS. Nil leaves
	// POST /webhooks/ses answering 503.
	SNS SNSChecker
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
	// limiter counts requests per route and caller, in memory.
	limiter *ratelimit.Limiter
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
	return &Server{store: store, auth: authService, files: fileStore, opts: opts, log: log, reqs: reqs, limiter: ratelimit.New(nil)}, nil
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
