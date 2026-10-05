package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/domain/pricing"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// CreateQuote prices a stay the way a booking would store it.
func (s *Server) CreateQuote(ctx context.Context, req oapi.CreateQuoteRequestObject) (oapi.CreateQuoteResponseObject, error) {
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
	b := req.Body
	p, err := s.store.GetProperty(ctx, t, b.PropertyId)
	if err != nil {
		return nil, apiErr(err, "property")
	}
	from, to := b.CheckIn.Time, b.CheckOut.Time
	if !slices.ContainsFunc(p.Units, func(u domain.Unit) bool { return u.ID == b.UnitId }) {
		return nil, apiErr(&domain.ValidationError{Field: "unitId", Message: "must be a unit of this property"}, "")
	}
	if n := domain.Nights(from, to); n < 0 || n > 365 {
		return nil, apiErr(&domain.ValidationError{Field: "checkOut", Message: "must be within a year after check-in"}, "")
	}
	list, err := s.store.RateOverrides(ctx, t, p.ID, from, to.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	overrides := pricing.FromList(list)
	q := pricing.Price(p, pricing.Input{
		UnitID: b.UnitId, From: from, To: to, Adults: b.Adults, Children: deref(b.Children),
		Extras: derefSlice(b.Extras), Promo: deref(b.Promo),
	}, overrides)

	out := oapi.Quote{
		Nights:    q.Nights,
		Lines:     linesToAPI(q.Lines),
		Total:     q.Total,
		Currency:  oapi.Currency(p.Currency),
		MinNights: pricing.MinNights(p, b.UnitId, from, overrides),
		Deposit:   pricing.Deposit(p, q.Total),
	}
	for _, n := range q.Nightly {
		out.Nightly = append(out.Nightly, struct {
			Night oapi.Day   `json:"night"`
			Price oapi.Money `json:"price"`
		}{day(n.Night), n.Price})
	}
	if out.Nightly == nil {
		out.Nightly = []struct {
			Night oapi.Day   `json:"night"`
			Price oapi.Money `json:"price"`
		}{}
	}
	return oapi.CreateQuote200JSONResponse(out), nil
}

// ListBookings returns a page of the stays the caller can see, by check-in.
func (s *Server) ListBookings(ctx context.Context, req oapi.ListBookingsRequestObject) (oapi.ListBookingsResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	prm := req.Params
	limit := 200
	if prm.Limit != nil {
		limit = min(max(*prm.Limit, 1), 500)
	}
	f := domain.BookingFilter{PropertyID: prm.PropertyId, Limit: limit + 1}
	if prm.From != nil {
		f.From = &prm.From.Time
	}
	if prm.To != nil {
		f.To = &prm.To.Time
	}
	if prm.Cursor != nil {
		if f.AfterCheckIn, f.AfterID, err = decodeCursor(*prm.Cursor); err != nil {
			return oapi.ListBookings400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
				fieldErr("invalid_cursor", "cursor", "Send back the nextCursor from the previous page."))}, nil
		}
	}
	bookings, err := s.store.ListBookings(ctx, t, f)
	if err != nil {
		return nil, err
	}
	out := oapi.BookingList{Items: make([]oapi.Booking, 0, min(len(bookings), limit))}
	for i, b := range bookings {
		if i == limit {
			cursor := encodeCursor(bookings[i-1])
			out.NextCursor = &cursor
			break
		}
		out.Items = append(out.Items, s.stayToAPI(ctx, b, t.Can(domain.PermPrices)))
	}
	return oapi.ListBookings200JSONResponse(out), nil
}

// GetBooking returns one stay.
func (s *Server) GetBooking(ctx context.Context, req oapi.GetBookingRequestObject) (oapi.GetBookingResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	b, err := s.store.GetBooking(ctx, t, req.BookingId)
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	return oapi.GetBooking200JSONResponse(s.stayToAPI(ctx, b, t.Can(domain.PermPrices))), nil
}

// CreateBooking adds a confirmed stay the owner took directly, priced unless they agreed a
// price with the guest.
func (s *Server) CreateBooking(ctx context.Context, req oapi.CreateBookingRequestObject) (oapi.CreateBookingResponseObject, error) {
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
	b := req.Body
	p, err := s.store.GetProperty(ctx, t, b.PropertyId)
	if err != nil {
		return nil, apiErr(err, "property")
	}
	in := domain.StayInput{
		UnitID: b.UnitId, CheckIn: b.CheckIn.Time, CheckOut: b.CheckOut.Time, Adults: b.Adults,
		Children: deref(b.Children), Guest: guestFromAPI(b.Guest), Source: domain.Source(b.Source),
		Extras: derefSlice(b.Extras), CustomTotal: b.CustomTotal, OwnerNote: deref(b.OwnerNote),
	}
	in.Normalize()
	if err := in.Validate(p); err != nil {
		return nil, apiErr(err, "")
	}
	price, err := s.priceStay(ctx, t, p, in)
	if err != nil {
		return nil, err
	}
	nb := domain.NewBooking{
		PropertyID: p.ID, Stay: in, Status: domain.StatusConfirmed, Currency: p.Currency,
		RefPrefix: domain.RefPrefix(p.Name), Price: price, LedgerUnits: domain.LedgerUnits(p.Units, in.UnitID),
	}
	if pay := b.Payment; pay != nil {
		if pay.Amount < 1 || !pay.Method.Valid() {
			return nil, apiErr(&domain.ValidationError{Field: "payment", Message: "needs a positive amount and a method of bank, cash or card"}, "")
		}
		nb.FirstPayment = &domain.Payment{Method: string(pay.Method), Amount: pay.Amount}
	}

	out, err := idempotent(ctx, s, t, b.PropertyId, req.Params.IdempotencyKey, http.StatusCreated, b, func(ctx context.Context) (oapi.Booking, error) {
		created, err := s.store.CreateBooking(ctx, t, nb, time.Now())
		if err != nil {
			return oapi.Booking{}, err
		}
		return s.stayToAPI(ctx, created, true), nil
	})
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	return oapi.CreateBooking201JSONResponse(out), nil
}

// UpdateBooking applies an owner's edit, pricing the stay again when what it costs changed.
func (s *Server) UpdateBooking(ctx context.Context, req oapi.UpdateBookingRequestObject) (oapi.UpdateBookingResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermManage) {
		return nil, errForbidden
	}
	cur, err := s.store.GetBooking(ctx, t, req.BookingId)
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	b := req.Body
	p, err := s.store.GetProperty(ctx, t, cur.PropertyID)
	if err != nil {
		return nil, err
	}

	in := domain.StayInput{
		UnitID: cur.UnitID, CheckIn: cur.CheckIn, CheckOut: cur.CheckOut, Adults: cur.Adults, Children: cur.Children,
		Guest: cur.Guest, Source: cur.Source, OwnerNote: cur.OwnerNote, CustomTotal: b.CustomTotal,
	}
	for _, e := range cur.Extras {
		in.Extras = append(in.Extras, e.ExtraID)
	}
	if b.UnitId != nil {
		in.UnitID = *b.UnitId
	}
	if b.CheckIn != nil {
		in.CheckIn = b.CheckIn.Time
	}
	if b.CheckOut != nil {
		in.CheckOut = b.CheckOut.Time
	}
	if b.Adults != nil {
		in.Adults = *b.Adults
	}
	if b.Children != nil {
		in.Children = *b.Children
	}
	if b.Guest != nil {
		id := in.Guest.ID
		in.Guest = guestFromAPI(*b.Guest)
		in.Guest.ID = id
	}
	if b.Source != nil {
		in.Source = domain.Source(*b.Source)
	}
	if b.Extras != nil {
		in.Extras = *b.Extras
	}
	if b.OwnerNote != nil {
		in.OwnerNote = *b.OwnerNote
	}
	in.Normalize()
	if err := in.Validate(p); err != nil {
		return nil, apiErr(err, "")
	}

	price := domain.Pricing{Lines: cur.Lines, Extras: cur.Extras, Total: cur.Total}
	if b.CustomTotal != nil || pricedDifferently(cur, in) {
		if price, err = s.priceStay(ctx, t, p, in); err != nil {
			return nil, err
		}
	}
	updated, err := s.store.UpdateBooking(ctx, t, cur.ID, domain.BookingChange{
		Version: b.Version, Stay: in, Price: price, LedgerUnits: domain.LedgerUnits(p.Units, in.UnitID),
	})
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	return oapi.UpdateBooking200JSONResponse(s.stayToAPI(ctx, updated, true)), nil
}

// TransitionBooking moves a stay on: approve, decline, confirm, check in or check out.
func (s *Server) TransitionBooking(ctx context.Context, req oapi.TransitionBookingRequestObject) (oapi.TransitionBookingResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	b, err := s.store.TransitionBooking(ctx, t, req.BookingId, req.Body.Version, domain.BookingStatus(req.Body.To),
		strings.TrimSpace(deref(req.Body.Reason)), time.Now())
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	return oapi.TransitionBooking200JSONResponse(s.stayToAPI(ctx, b, t.Can(domain.PermPrices))), nil
}

// CancelBooking cancels a stay and records the refund the owner chose.
func (s *Server) CancelBooking(ctx context.Context, req oapi.CancelBookingRequestObject) (oapi.CancelBookingResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermManage) {
		return nil, errForbidden
	}
	cur, err := s.store.GetBooking(ctx, t, req.BookingId)
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	b := req.Body
	reason := strings.TrimSpace(b.Reason)
	if reason == "" || len(reason) > 500 {
		return nil, apiErr(&domain.ValidationError{Field: "reason", Message: "must be between 1 and 500 characters"}, "")
	}
	var refund *domain.Payment
	if r := b.Refund; r != nil {
		if r.Amount < 1 || !r.Method.Valid() {
			return nil, apiErr(&domain.ValidationError{Field: "refund", Message: "needs a positive amount and a method of bank, cash or card"}, "")
		}
		refund = &domain.Payment{Method: string(r.Method), Amount: r.Amount}
	}

	out, err := idempotent(ctx, s, t, cur.ID, req.Params.IdempotencyKey, http.StatusOK, b, func(ctx context.Context) (oapi.Booking, error) {
		cancelled, err := s.store.CancelBooking(ctx, t, cur.ID, b.Version, reason, refund, time.Now())
		if err != nil {
			return oapi.Booking{}, err
		}
		return s.stayToAPI(ctx, cancelled, true), nil
	})
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	return oapi.CancelBooking200JSONResponse(out), nil
}

// RecordPayment adds money the guest paid, accepting their slip when one is named.
func (s *Server) RecordPayment(ctx context.Context, req oapi.RecordPaymentRequestObject) (oapi.RecordPaymentResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermManage) {
		return nil, errForbidden
	}
	cur, err := s.store.GetBooking(ctx, t, req.BookingId)
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	b := req.Body
	note := strings.TrimSpace(deref(b.Note))
	switch {
	case b.Amount < 1:
		return nil, apiErr(&domain.ValidationError{Field: "amount", Message: "must be more than zero"}, "")
	case !b.Method.Valid():
		return nil, apiErr(&domain.ValidationError{Field: "method", Message: "must be bank, cash or card"}, "")
	case len(note) > 500:
		return nil, apiErr(&domain.ValidationError{Field: "note", Message: "must be at most 500 characters"}, "")
	}
	p := domain.Payment{Method: string(b.Method), Amount: b.Amount, Note: note, ReceivedAt: time.Now(), SlipID: b.SlipId}
	if b.ReceivedAt != nil {
		p.ReceivedAt = *b.ReceivedAt
	}

	out, err := idempotent(ctx, s, t, cur.ID, req.Params.IdempotencyKey, http.StatusCreated, b, func(ctx context.Context) (oapi.Booking, error) {
		paid, err := s.store.RecordPayment(ctx, t, cur.ID, p)
		if err != nil {
			return oapi.Booking{}, err
		}
		return s.stayToAPI(ctx, paid, true), nil
	})
	if err != nil {
		return nil, apiErr(err, "booking")
	}
	return oapi.RecordPayment201JSONResponse(out), nil
}

// RejectSlip turns down a guest's bank slip.
func (s *Server) RejectSlip(ctx context.Context, req oapi.RejectSlipRequestObject) (oapi.RejectSlipResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermManage) {
		return nil, errForbidden
	}
	b, err := s.store.RejectSlip(ctx, t, req.SlipId, time.Now())
	if err != nil {
		return nil, apiErr(err, "slip")
	}
	return oapi.RejectSlip200JSONResponse(s.stayToAPI(ctx, b, true)), nil
}

// priceStay prices a stay with the property's overrides, or takes the price agreed with the
// guest. Extras keep their quoted amounts either way.
func (s *Server) priceStay(ctx context.Context, t tenant.Tenant, p domain.PropertyDetail, in domain.StayInput) (domain.Pricing, error) {
	list, err := s.store.RateOverrides(ctx, t, p.ID, in.CheckIn, in.CheckOut)
	if err != nil {
		return domain.Pricing{}, err
	}
	q := pricing.Price(p, pricing.Input{
		UnitID: in.UnitID, From: in.CheckIn, To: in.CheckOut, Adults: in.Adults, Children: in.Children, Extras: in.Extras,
	}, pricing.FromList(list))
	if in.CustomTotal != nil {
		return domain.Pricing{Lines: pricing.CustomLines(q.Nights, *in.CustomTotal), Extras: q.Extras, Total: *in.CustomTotal}, nil
	}
	return domain.Pricing{Lines: q.Lines, Extras: q.Extras, Total: q.Total, PromoID: q.PromoID}, nil
}

// pricedDifferently reports whether an edit changes anything the price depends on.
func pricedDifferently(cur domain.Booking, in domain.StayInput) bool {
	extras := make([]uuid.UUID, 0, len(cur.Extras))
	for _, e := range cur.Extras {
		extras = append(extras, e.ExtraID)
	}
	return cur.UnitID != in.UnitID || !cur.CheckIn.Equal(in.CheckIn) || !cur.CheckOut.Equal(in.CheckOut) ||
		cur.Adults != in.Adults || cur.Children != in.Children || !sameIDs(extras, in.Extras)
}

func sameIDs(a, b []uuid.UUID) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	cmp := func(x, y uuid.UUID) int { return strings.Compare(x.String(), y.String()) }
	slices.SortFunc(a, cmp)
	slices.SortFunc(b, cmp)
	return slices.Equal(a, b)
}

// idempotent runs a write once per Idempotency-Key. Without a key it just runs. The request
// hash covers the record the write targets and its body, so a key reused elsewhere is caught.
func idempotent[T any](ctx context.Context, s *Server, t tenant.Tenant, target uuid.UUID, key *string, status int, body any, run func(ctx context.Context) (T, error)) (T, error) {
	if key == nil {
		return run(ctx)
	}
	var out T
	raw, err := json.Marshal(body)
	if err != nil {
		return out, fmt.Errorf("hash request: %w", err)
	}
	hash := sha256.Sum256(append(target[:], raw...))
	reply, err := s.store.Idempotent(ctx, t, *key, hash[:], status, time.Now(), func(ctx context.Context) ([]byte, error) {
		v, err := run(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(v)
	})
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(reply, &out); err != nil {
		return out, fmt.Errorf("read idempotent reply: %w", err)
	}
	return out, nil
}

// encodeCursor and decodeCursor carry the last stay of a page, by check-in and id.
func encodeCursor(b domain.Booking) string {
	return base64.RawURLEncoding.EncodeToString([]byte(b.CheckIn.Format(time.DateOnly) + "_" + b.ID.String()))
}

func decodeCursor(c string) (*time.Time, *uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return nil, nil, err
	}
	d, id, ok := strings.Cut(string(raw), "_")
	if !ok {
		return nil, nil, errors.New("malformed cursor")
	}
	checkIn, err := time.Parse(time.DateOnly, d)
	if err != nil {
		return nil, nil, err
	}
	after, err := uuid.Parse(id)
	if err != nil {
		return nil, nil, err
	}
	return &checkIn, &after, nil
}

var errNoBody = &apiError{status: http.StatusBadRequest, code: "invalid_body", message: "Request body is required."}

// apiErr turns a store or domain error into the response the contract gives for it. what
// names the record a 404 is about.
func apiErr(err error, what string) error {
	var conflict *domain.ConflictError
	var invalid *domain.ValidationError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return &apiError{status: http.StatusNotFound, code: what + "_not_found", message: "That " + what + " doesn't exist in this account."}
	case errors.Is(err, domain.ErrForbidden):
		return errForbidden
	case errors.Is(err, domain.ErrStale):
		return &apiError{status: http.StatusConflict, code: "stale", message: "This booking changed since you opened it. Refresh and try again."}
	case errors.Is(err, domain.ErrBadTransition):
		return &apiError{status: http.StatusConflict, code: "bad_status", message: sentence(strings.TrimPrefix(err.Error(), domain.ErrBadTransition.Error()+": "))}
	case errors.Is(err, domain.ErrKeyReused):
		return &apiError{status: http.StatusUnprocessableEntity, code: "idempotency_key_reused", message: "That Idempotency-Key was already used for a different request."}
	case errors.As(err, &conflict):
		clash := &oapi.Clash{Night: day(conflict.Night), From: day(conflict.From), To: day(conflict.To),
			BookingId: conflict.BookingID, BlockId: conflict.BlockID, Ref: optional(conflict.Ref), GuestName: optional(conflict.GuestName)}
		message := "Those dates are blocked."
		if conflict.Ref != "" {
			message = fmt.Sprintf("Those dates overlap %s's stay (%s).", conflict.GuestName, conflict.Ref)
		}
		return &apiError{status: http.StatusConflict, code: "dates_taken", message: message, clash: clash}
	case errors.As(err, &invalid):
		e := validationResponse(err, "")
		return &apiError{status: http.StatusBadRequest, code: e.Code, message: e.Message, field: deref(e.Field)}
	}
	return err
}

func guestFromAPI(g oapi.Guest) domain.Guest {
	return domain.Guest{Name: g.Name, Phone: deref(g.Phone), Email: deref(g.Email), Country: deref(g.Country)}
}

// stayToAPI turns a stay into its API shape. Without the prices permission (caretakers), money
// is left out.
func (s *Server) stayToAPI(ctx context.Context, b domain.Booking, money bool) oapi.Booking {
	out := oapi.Booking{
		Id:               b.ID,
		Ref:              b.Ref,
		PropertyId:       b.PropertyID,
		UnitId:           b.UnitID,
		Source:           oapi.BookingSource(b.Source),
		Status:           oapi.BookingStatus(b.Status),
		Guest:            oapi.Guest{Name: b.Guest.Name, Phone: optional(b.Guest.Phone), Email: optional(b.Guest.Email), Country: optional(b.Guest.Country)},
		Adults:           b.Adults,
		Children:         b.Children,
		CheckIn:          day(b.CheckIn),
		CheckOut:         day(b.CheckOut),
		Currency:         oapi.Currency(b.Currency),
		Extras:           make([]openapi_types.UUID, len(b.Extras)),
		GuestNote:        b.GuestNote,
		OwnerNote:        b.OwnerNote,
		RequestExpiresAt: b.RequestExpiresAt,
		PaymentDueAt:     b.PaymentDueAt,
		Version:          b.Version,
		CreatedAt:        b.CreatedAt,
		UpdatedAt:        b.UpdatedAt,
	}
	for i, e := range b.Extras {
		out.Extras[i] = e.ExtraID
	}
	if b.CancelledAt != nil {
		out.Cancel = &struct {
			At     time.Time `json:"at"`
			Reason string    `json:"reason"`
		}{*b.CancelledAt, b.CancelReason}
	}
	if !money {
		return out
	}
	paid, balance := b.Paid(), b.Balance()
	lines := linesToAPI(b.Lines)
	payments := make([]oapi.Payment, len(b.Payments))
	for i, p := range b.Payments {
		payments[i] = oapi.Payment{Id: p.ID, Kind: oapi.PaymentKind(p.Kind), Method: oapi.PaymentMethod(p.Method),
			Amount: p.Amount, Note: p.Note, ReceivedAt: p.ReceivedAt}
	}
	slips := make([]oapi.Slip, len(b.Slips))
	for i, sl := range b.Slips {
		slips[i] = oapi.Slip{Id: sl.ID, Amount: sl.Amount, Status: oapi.SlipStatus(sl.Status), UploadedAt: sl.UploadedAt,
			Url: s.files.URL(ctx, sl.FileKey)}
	}
	out.Total, out.Paid, out.Balance = &b.Total, &paid, &balance
	out.Lines, out.Payments, out.Slips = &lines, &payments, &slips
	return out
}

func linesToAPI(lines []domain.PriceLine) []oapi.PriceLine {
	out := make([]oapi.PriceLine, len(lines))
	for i, l := range lines {
		out[i] = oapi.PriceLine{Label: l.Label, Amount: l.Amount}
		if l.Kind != "" {
			kind := oapi.PriceLineKind(l.Kind)
			out[i].Kind = &kind
		}
	}
	return out
}

func day(t time.Time) oapi.Day { return openapi_types.Date{Time: t} }
