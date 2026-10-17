package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/domain/pricing"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// Booking page routes. They have no signed-in user: the account comes from the property's slug
// or the guest's booking, never from the request.

var (
	errNoPage = &apiError{status: http.StatusNotFound, code: "not_found", message: "There is no booking page at this address."}
	errNoStay = &apiError{status: http.StatusNotFound, code: "booking_not_found", message: "That booking doesn't exist, or the link is incomplete."}
	errTaken  = &apiError{status: http.StatusConflict, code: "dates_taken", message: "Those dates were just taken. Pick other dates."}
	errPaused = &apiError{status: http.StatusConflict, code: "not_taking_bookings", message: "This property isn't taking bookings online right now."}
)

// maxAvailabilityMonths is the longest range the booking page asks availability for.
const maxAvailabilityMonths = 13

// systemTenant lets booking page reads reuse the owner read paths for an account. It has no
// user, so it must never reach a write: booking page writes go through the store's guest
// methods, which take the account from the property or booking.
func systemTenant(accountID uuid.UUID) tenant.Tenant {
	return tenant.Tenant{AccountID: accountID, Role: domain.RoleOwner}
}

// GetPublicProperty returns what a guest sees on a booking page.
func (s *Server) GetPublicProperty(ctx context.Context, req oapi.GetPublicPropertyRequestObject) (oapi.GetPublicPropertyResponseObject, error) {
	p, err := s.publicProperty(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	out := oapi.PublicProperty{
		Slug: p.Slug, Name: p.Name, BookingType: oapi.BookingType(p.BookingType), Location: optional(p.Location),
		Currency: oapi.Currency(p.Currency), BaseRate: p.BaseRateMinor, Description: p.Description,
		Amenities: nonNil(p.Amenities), CheckIn: p.CheckIn, CheckOut: p.CheckOut, TimeZone: p.TimeZone,
		HouseRules: nonNil(p.HouseRules), Policy: oapi.Policy(p.Policy), DepositPercent: p.DepositPercent,
		Photos: make([]oapi.PublicPhoto, len(p.Photos)), Units: make([]oapi.PublicUnit, len(p.Units)),
		Extras: []oapi.PublicExtra{},
		Rules: oapi.StayRules{
			MinNights: p.Rules.MinNights, MaxNights: p.Rules.MaxNights, SameDayCutoff: p.Rules.SameDayCutoff,
			WindowMonths: p.Rules.WindowMonths, ClosedArrival: nonNil(p.Rules.ClosedArrival),
		},
		Booking:        oapi.PublicBookingSettings{Mode: oapi.PublicBookingSettingsMode(p.Booking.Mode), HoldMinutes: p.Booking.HoldMinutes},
		Payments:       oapi.PublicPayments{Bank: p.Payments.Bank.Enabled, AtProperty: p.Payments.AtProperty},
		Branding:       oapi.PublicBranding{Color: p.Branding.Color},
		TakingBookings: takingBookings(p),
	}
	for i, ph := range p.Photos {
		out.Photos[i] = oapi.PublicPhoto{Url: s.files.URL(ctx, ph.Key), Caption: ph.Caption}
	}
	for i, u := range p.Units {
		out.Units[i] = oapi.PublicUnit{Id: u.ID, Name: u.Name, Sleeps: u.Sleeps, Beds: u.Beds, LinkedUnitIds: nonNil(u.LinkedUnitIDs)}
	}
	for _, e := range p.Extras {
		if e.Enabled {
			out.Extras = append(out.Extras, oapi.PublicExtra{Id: e.ID, Name: e.Name, Price: e.Price, Per: oapi.ExtraPer(e.Per), OnRequest: e.OnRequest})
		}
	}
	if p.Branding.LogoKey != "" {
		url := s.files.URL(ctx, p.Branding.LogoKey)
		out.Branding.LogoUrl = &url
	}
	return oapi.GetPublicProperty200JSONResponse(out), nil
}

// GetPublicAvailability lists, per unit, the nights in [from, to) that can't be booked and the
// days guests can't arrive on. A unit's nights are taken when any unit it books is held, so a
// whole house is taken when one of its rooms is, and a room when the whole house is.
func (s *Server) GetPublicAvailability(ctx context.Context, req oapi.GetPublicAvailabilityRequestObject) (oapi.GetPublicAvailabilityResponseObject, error) {
	p, err := s.publicProperty(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	from, to := req.Params.From.Time, req.Params.To.Time
	if !to.After(from) || to.After(from.AddDate(0, maxAvailabilityMonths, 0)) {
		return nil, apiErr(&domain.ValidationError{Field: "to", Message: fmt.Sprintf("must be after from and within %d months of it", maxAvailabilityMonths)}, "")
	}
	held, err := s.store.Availability(ctx, p.AccountID, p.ID, from, to, time.Now())
	if err != nil {
		return nil, err
	}
	list, err := s.store.RateOverrides(ctx, systemTenant(p.AccountID), p.ID, from, to)
	if err != nil {
		return nil, err
	}
	closed := map[uuid.UUID]map[time.Time]bool{}
	for _, o := range list {
		if o.ClosedToArrival {
			if closed[o.UnitID] == nil {
				closed[o.UnitID] = map[time.Time]bool{}
			}
			closed[o.UnitID][o.Night] = true
		}
	}

	out := oapi.Availability{Units: make([]oapi.UnitAvailability, len(p.Units))}
	for i, u := range p.Units {
		taken := map[time.Time]bool{}
		for _, id := range domain.LedgerUnits(p.Units, u.ID) {
			for _, n := range held[id] {
				taken[n] = true
			}
		}
		ua := oapi.UnitAvailability{UnitId: u.ID, Taken: []oapi.Day{}, ClosedArrival: []oapi.Day{}}
		for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
			if taken[d] {
				ua.Taken = append(ua.Taken, day(d))
			}
			if closed[u.ID][d] || slices.Contains(p.Rules.ClosedArrival, int(d.Weekday())) {
				ua.ClosedArrival = append(ua.ClosedArrival, day(d))
			}
		}
		out.Units[i] = ua
	}
	return oapi.GetPublicAvailability200JSONResponse(out), nil
}

// CreatePublicQuote prices a stay a guest is choosing, after checking it against the
// property's rules and the promo code.
func (s *Server) CreatePublicQuote(ctx context.Context, req oapi.CreatePublicQuoteRequestObject) (oapi.CreatePublicQuoteResponseObject, error) {
	if req.Body == nil {
		return nil, errNoBody
	}
	b := req.Body
	p, err := s.publicProperty(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	in := domain.StayInput{
		UnitID: b.UnitId, CheckIn: b.CheckIn.Time, CheckOut: b.CheckOut.Time, Adults: b.Adults,
		Children: deref(b.Children), Extras: derefSlice(b.Extras),
	}
	q, overrides, err := s.guestQuote(ctx, p, in, deref(b.Promo))
	if err != nil {
		return nil, err
	}
	return oapi.CreatePublicQuote200JSONResponse(quoteToAPI(p, q, pricing.MinNights(p, in.UnitID, in.CheckIn, overrides))), nil
}

// CreateHold holds a stay's nights while the guest fills in the checkout.
func (s *Server) CreateHold(ctx context.Context, req oapi.CreateHoldRequestObject) (oapi.CreateHoldResponseObject, error) {
	if req.Body == nil {
		return nil, errNoBody
	}
	b := req.Body
	p, err := s.publicProperty(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	if !takingBookings(p) {
		return nil, errPaused
	}
	in := domain.StayInput{UnitID: b.UnitId, CheckIn: b.CheckIn.Time, CheckOut: b.CheckOut.Time}
	now := time.Now()
	if err := s.checkGuestStay(ctx, p, in, now); err != nil {
		return nil, err
	}
	expires := now.Add(time.Duration(p.Booking.HoldMinutes) * time.Minute)
	id, err := s.store.CreateHold(ctx, p.AccountID, p.ID, domain.LedgerUnits(p.Units, in.UnitID), in.CheckIn, in.CheckOut, expires, now, b.Replaces)
	if err != nil {
		return nil, guestErr(err)
	}
	return oapi.CreateHold201JSONResponse{HoldId: id, ExpiresAt: expires}, nil
}

// CreateGuestBooking books a stay from the booking page or widget. Request mode makes it a
// request the owner answers; instant mode books it, waiting for payment.
func (s *Server) CreateGuestBooking(ctx context.Context, req oapi.CreateGuestBookingRequestObject) (oapi.CreateGuestBookingResponseObject, error) {
	if req.Body == nil {
		return nil, errNoBody
	}
	if req.Params.IdempotencyKey == nil || *req.Params.IdempotencyKey == "" {
		return nil, &apiError{status: http.StatusBadRequest, code: "idempotency_key_required", message: "Send an Idempotency-Key with the booking.", field: "Idempotency-Key"}
	}
	b := req.Body
	p, err := s.publicProperty(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	if !takingBookings(p) {
		return nil, errPaused
	}
	in := domain.StayInput{
		UnitID: b.UnitId, CheckIn: b.CheckIn.Time, CheckOut: b.CheckOut.Time, Adults: b.Adults,
		Children: deref(b.Children), Guest: guestFromAPI(b.Guest), Source: domain.Source(b.Source),
		Extras: derefSlice(b.Extras),
	}
	in.Normalize()
	note := strings.TrimSpace(deref(b.Note))
	method := string(b.PaymentMethod)
	switch {
	case b.Source != oapi.GuestBookingRequestSourcePage && b.Source != oapi.GuestBookingRequestSourceWidget:
		return nil, apiErr(&domain.ValidationError{Field: "source", Message: "must be page or widget"}, "")
	case in.Guest.Phone == "":
		return nil, apiErr(&domain.ValidationError{Field: "guest.phone", Message: "is needed so the property can reach you"}, "")
	case len(note) > 2000:
		return nil, apiErr(&domain.ValidationError{Field: "note", Message: "must be at most 2000 characters"}, "")
	case method == "bank" && !p.Payments.Bank.Enabled, method == "property" && !p.Payments.AtProperty, method != "bank" && method != "property":
		return nil, apiErr(&domain.ValidationError{Field: "paymentMethod", Message: "isn't a way this property takes payment"}, "")
	}
	if err := in.Validate(p); err != nil {
		return nil, apiErr(err, "")
	}
	q, _, err := s.guestQuote(ctx, p, in, deref(b.Promo))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	nb := domain.NewBooking{
		PropertyID: p.ID, Stay: in, Status: domain.StatusAwaitingPayment, Currency: p.Currency,
		RefPrefix:   domain.RefPrefix(p.Name),
		Price:       domain.Pricing{Lines: q.Lines, Extras: q.Extras, Total: q.Total, PromoID: q.PromoID},
		LedgerUnits: domain.LedgerUnits(p.Units, in.UnitID), GuestNote: note, PaymentMethod: method, HoldID: b.HoldId,
	}
	switch {
	case p.Booking.Mode == "request":
		nb.Status = domain.StatusRequested
		expires := now.Add(time.Duration(p.Booking.ReplyHours) * time.Hour)
		nb.RequestExpiresAt = &expires
	case method == "bank" && p.Payments.Bank.CancelIfUnpaid:
		due := now.Add(time.Duration(p.Payments.Bank.PayWithinHours) * time.Hour)
		nb.PaymentDueAt = &due
	}

	out, err := idempotent(ctx, s, systemTenant(p.AccountID), p.ID, req.Params.IdempotencyKey, http.StatusCreated, b, func(ctx context.Context) (oapi.GuestBookingCreated, error) {
		token, hash, err := newBookingToken()
		if err != nil {
			return oapi.GuestBookingCreated{}, err
		}
		nb.GuestTokenHash = hash
		id, err := s.store.CreateGuestBooking(ctx, p.AccountID, nb, now)
		if err != nil {
			return oapi.GuestBookingCreated{}, err
		}
		gb, err := s.store.GuestBooking(ctx, id, hash)
		if err != nil {
			return oapi.GuestBookingCreated{}, err
		}
		return oapi.GuestBookingCreated{Booking: s.guestBookingToAPI(gb, p), Token: token}, nil
	})
	if err != nil {
		return nil, guestErr(err)
	}
	return oapi.CreateGuestBooking201JSONResponse(out), nil
}

// GetGuestBooking returns a booking to the guest holding its token.
func (s *Server) GetGuestBooking(ctx context.Context, req oapi.GetGuestBookingRequestObject) (oapi.GetGuestBookingResponseObject, error) {
	gb, p, err := s.guestBooking(ctx, req.BookingId, req.Params.XBookingToken)
	if err != nil {
		return nil, err
	}
	return oapi.GetGuestBooking200JSONResponse(s.guestBookingToAPI(gb, p)), nil
}

// CreateGuestUpload presigns the upload of a bank slip for the guest's booking.
func (s *Server) CreateGuestUpload(ctx context.Context, req oapi.CreateGuestUploadRequestObject) (oapi.CreateGuestUploadResponseObject, error) {
	if req.Body == nil {
		return nil, errNoBody
	}
	gb, _, err := s.guestBooking(ctx, req.BookingId, req.Params.XBookingToken)
	if err != nil {
		return nil, err
	}
	if !takesSlips(gb.Status) {
		return nil, apiErr(fmt.Errorf("%w: a %s stay takes no slips", domain.ErrBadTransition, gb.Status), "")
	}
	if limit := files.MaxBytes[files.KindSlip]; req.Body.Size < 1 || req.Body.Size > limit {
		return nil, &apiError{status: http.StatusBadRequest, code: "file_too_large", field: "size", message: fmt.Sprintf("Slips can be up to %d MB.", limit>>20)}
	}
	key, err := files.NewKey(gb.AccountID, files.KindSlip, string(req.Body.ContentType))
	if err != nil {
		return nil, &apiError{status: http.StatusBadRequest, code: "invalid_content_type", field: "contentType", message: "Use a JPEG, PNG, WebP or HEIC image."}
	}
	up, err := s.files.PresignUpload(ctx, key, string(req.Body.ContentType), req.Body.Size)
	if err != nil {
		return nil, err
	}
	return oapi.CreateGuestUpload201JSONResponse{
		Key: up.Key, UploadUrl: up.URL, Method: oapi.PUT, Headers: up.Headers, ExpiresAt: up.ExpiresAt, Url: up.PublicURL,
	}, nil
}

// AddGuestSlip sends an uploaded bank slip to the owner.
func (s *Server) AddGuestSlip(ctx context.Context, req oapi.AddGuestSlipRequestObject) (oapi.AddGuestSlipResponseObject, error) {
	if req.Body == nil {
		return nil, errNoBody
	}
	b := req.Body
	if b.Amount < 1 {
		return nil, apiErr(&domain.ValidationError{Field: "amount", Message: "must be more than zero"}, "")
	}
	hash := bookingTokenHash(req.Params.XBookingToken)
	if err := s.store.AddGuestSlip(ctx, req.BookingId, hash, b.Key, b.Amount, time.Now()); err != nil {
		return nil, guestErr(err)
	}
	gb, p, err := s.guestBooking(ctx, req.BookingId, req.Params.XBookingToken)
	if err != nil {
		return nil, err
	}
	return oapi.AddGuestSlip201JSONResponse(s.guestBookingToAPI(gb, p)), nil
}

// guestQuote checks a guest's stay and promo code, and prices it.
func (s *Server) guestQuote(ctx context.Context, p domain.PropertyDetail, in domain.StayInput, promo string) (pricing.Quote, pricing.Overrides, error) {
	switch {
	case in.Adults < 1 || in.Adults > 30:
		return pricing.Quote{}, nil, apiErr(&domain.ValidationError{Field: "adults", Message: "must be between 1 and 30"}, "")
	case in.Children < 0 || in.Children > 30:
		return pricing.Quote{}, nil, apiErr(&domain.ValidationError{Field: "children", Message: "must be between 0 and 30"}, "")
	case len(in.Extras) > 50:
		return pricing.Quote{}, nil, apiErr(&domain.ValidationError{Field: "extras", Message: "can list at most 50 extras"}, "")
	}
	for _, id := range in.Extras {
		if !slices.ContainsFunc(p.Extras, func(e domain.Extra) bool { return e.ID == id && e.Enabled }) {
			return pricing.Quote{}, nil, apiErr(&domain.ValidationError{Field: "extras", Message: "must be extras this property offers"}, "")
		}
	}
	overrides, err := s.guestRules(ctx, p, in, time.Now())
	if err != nil {
		return pricing.Quote{}, nil, err
	}
	promo = strings.ToUpper(strings.TrimSpace(promo))
	if promo != "" {
		i := slices.IndexFunc(p.Promos, func(pr domain.Promo) bool { return pr.Code == promo })
		if i < 0 {
			return pricing.Quote{}, nil, apiErr(&domain.ValidationError{Field: "promo", Message: "that code isn't valid"}, "")
		}
		if err := pricing.PromoProblem(p.Promos[i], domain.Nights(in.CheckIn, in.CheckOut), in.CheckIn); err != nil {
			return pricing.Quote{}, nil, apiErr(err, "")
		}
	}
	q := pricing.Price(p, pricing.Input{
		UnitID: in.UnitID, From: in.CheckIn, To: in.CheckOut, Adults: in.Adults, Children: in.Children,
		Extras: in.Extras, Promo: promo,
	}, overrides)
	return q, overrides, nil
}

// checkGuestStay checks the unit, the dates and the property's rules for a guest.
func (s *Server) checkGuestStay(ctx context.Context, p domain.PropertyDetail, in domain.StayInput, now time.Time) error {
	_, err := s.guestRules(ctx, p, in, now)
	return err
}

func (s *Server) guestRules(ctx context.Context, p domain.PropertyDetail, in domain.StayInput, now time.Time) (pricing.Overrides, error) {
	if !slices.ContainsFunc(p.Units, func(u domain.Unit) bool { return u.ID == in.UnitID }) {
		return nil, apiErr(&domain.ValidationError{Field: "unitId", Message: "must be a unit of this property"}, "")
	}
	if n := domain.Nights(in.CheckIn, in.CheckOut); n < 1 || n > 365 {
		return nil, apiErr(&domain.ValidationError{Field: "checkOut", Message: "must be after check-in and within a year of it"}, "")
	}
	list, err := s.store.RateOverrides(ctx, systemTenant(p.AccountID), p.ID, in.CheckIn, in.CheckOut.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	overrides := pricing.FromList(list)
	if err := pricing.GuestStay(p, in, overrides, now); err != nil {
		return nil, apiErr(err, "")
	}
	return overrides, nil
}

func (s *Server) publicProperty(ctx context.Context, slug string) (domain.PropertyDetail, error) {
	p, err := s.store.PublicProperty(ctx, slug)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.PropertyDetail{}, errNoPage
	}
	return p, err
}

// guestBooking loads the guest's booking by its token, with its property.
func (s *Server) guestBooking(ctx context.Context, id uuid.UUID, token string) (domain.GuestBooking, domain.PropertyDetail, error) {
	gb, err := s.store.GuestBooking(ctx, id, bookingTokenHash(token))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.GuestBooking{}, domain.PropertyDetail{}, errNoStay
	}
	if err != nil {
		return domain.GuestBooking{}, domain.PropertyDetail{}, err
	}
	p, err := s.store.PublicProperty(ctx, gb.PropertySlug)
	if err != nil {
		return domain.GuestBooking{}, domain.PropertyDetail{}, fmt.Errorf("load the booking's property: %w", err)
	}
	return gb, p, nil
}

// guestBookingToAPI shows a guest their own booking. Bank details are for guests paying by bank
// transfer only.
func (s *Server) guestBookingToAPI(gb domain.GuestBooking, p domain.PropertyDetail) oapi.GuestBooking {
	out := oapi.GuestBooking{
		Id: gb.ID, Ref: gb.Ref, Status: oapi.BookingStatus(gb.Status), PropertyName: gb.PropertyName,
		PropertySlug: gb.PropertySlug, PropertyCheckIn: gb.PropertyCheckIn, UnitName: gb.UnitName,
		CheckIn: day(gb.CheckIn), CheckOut: day(gb.CheckOut), Adults: gb.Adults, Children: gb.Children,
		Currency: oapi.Currency(gb.Currency), Lines: linesToAPI(gb.Lines), Total: gb.Total,
		Deposit: pricing.Deposit(p, gb.Total), Paid: gb.Paid(), Balance: gb.Balance(),
		PaymentDueAt: gb.PaymentDueAt, RequestExpiresAt: gb.RequestExpiresAt, Slips: make([]oapi.GuestSlip, len(gb.Slips)),
	}
	if gb.PaymentMethod != "" {
		method := oapi.GuestPaymentMethod(gb.PaymentMethod)
		out.PaymentMethod = &method
	}
	if bank := p.Payments.Bank; gb.PaymentMethod == "bank" && bank.Enabled {
		out.Bank = &oapi.GuestBank{BankName: bank.BankName, AccountName: bank.AccountName, AccountNumber: bank.AccountNumber}
	}
	for i, sl := range gb.Slips {
		out.Slips[i] = oapi.GuestSlip{Amount: sl.Amount, Status: oapi.GuestSlipStatus(sl.Status), UploadedAt: sl.UploadedAt}
	}
	return out
}

// guestErr maps store errors for guests. A clash names nothing about the other stay.
func guestErr(err error) error {
	var conflict *domain.ConflictError
	switch {
	case errors.As(err, &conflict):
		return errTaken
	case errors.Is(err, domain.ErrNotFound):
		return errNoStay
	}
	return apiErr(err, "")
}

// takingBookings reports whether guests can book: the property isn't paused and offers a way
// to pay. Cards wait for PayHere.
func takingBookings(p domain.PropertyDetail) bool {
	return p.Status != "paused" && (p.Payments.Bank.Enabled || p.Payments.AtProperty)
}

func takesSlips(s domain.BookingStatus) bool {
	return s.Holds() && s != domain.StatusCheckedOut
}

// newBookingToken returns a guest's booking token and the hash stored for it.
func newBookingToken() (string, []byte, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("booking token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, bookingTokenHash(token), nil
}

func bookingTokenHash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
