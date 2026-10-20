package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"staykey.direct/api/internal/cards"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/domain/pricing"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// Guest card payments through Payments.lk: the owner's choice of where card money goes, and
// the checkouts guests pay through.

var (
	errCardsOff     = &apiError{status: http.StatusConflict, code: "cards_off", message: "This property isn't taking card payments right now."}
	errNothingToPay = &apiError{status: http.StatusConflict, code: "nothing_to_pay", message: "Nothing is left to pay for that part of the stay."}
	errUnreachable  = &apiError{status: http.StatusServiceUnavailable, code: "payments_unreachable", message: "Payments.lk can't be reached right now. Try again in a few minutes."}
	errInvalidKey   = &apiError{status: http.StatusBadRequest, code: "invalid_key", field: "secretKey", message: "Payments.lk didn't accept that key. Copy the secret key from your Payments.lk dashboard."}
	errTestKey      = &apiError{status: http.StatusBadRequest, code: "test_key_in_production", field: "secretKey", message: "That is a sandbox key. Use your live secret key to take real payments."}
	errNoPayouts    = &apiError{status: http.StatusConflict, code: "payouts_unavailable", message: "StayKey payouts aren't available for this account yet."}
)

// GetCardPayments returns where the account's card payments go. Owners only.
func (s *Server) GetCardPayments(ctx context.Context, _ oapi.GetCardPaymentsRequestObject) (oapi.GetCardPaymentsResponseObject, error) {
	t, err := cardOwner(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.store.CardState(ctx, t.AccountID)
	if err != nil {
		return nil, err
	}
	return oapi.GetCardPayments200JSONResponse(s.cardPaymentsToAPI(st)), nil
}

// SetCardMode chooses where the account's card payments go. Owners only.
func (s *Server) SetCardMode(ctx context.Context, req oapi.SetCardModeRequestObject) (oapi.SetCardModeResponseObject, error) {
	t, err := cardOwner(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	st, err := s.store.SetCardMode(ctx, t, domain.CardMode(req.Body.Mode))
	if err != nil {
		return nil, cardErr(err)
	}
	return oapi.SetCardMode200JSONResponse(s.cardPaymentsToAPI(st)), nil
}

// ConnectCards checks the owner's Payments.lk key and keeps it. Owners only.
func (s *Server) ConnectCards(ctx context.Context, req oapi.ConnectCardsRequestObject) (oapi.ConnectCardsResponseObject, error) {
	t, err := cardOwner(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	if s.opts.Cards == nil {
		return nil, errUnreachable
	}
	key := strings.TrimSpace(req.Body.SecretKey)
	mode, err := s.opts.Cards.CheckKey(ctx, key)
	if err != nil {
		return nil, cardErr(err)
	}
	st, err := s.store.ConnectCards(ctx, t, key, mode, time.Now())
	if err != nil {
		return nil, err
	}
	s.log.Info("card account connected", "account", t.AccountID, "mode", mode)
	return oapi.ConnectCards200JSONResponse(s.cardPaymentsToAPI(st)), nil
}

// DisconnectCards forgets the owner's Payments.lk key. Owners only.
func (s *Server) DisconnectCards(ctx context.Context, _ oapi.DisconnectCardsRequestObject) (oapi.DisconnectCardsResponseObject, error) {
	t, err := cardOwner(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.store.DisconnectCards(ctx, t)
	if err != nil {
		return nil, err
	}
	return oapi.DisconnectCards200JSONResponse(s.cardPaymentsToAPI(st)), nil
}

// CreateCardCheckout opens a Payments.lk checkout for part of a guest's stay.
func (s *Server) CreateCardCheckout(ctx context.Context, req oapi.CreateCardCheckoutRequestObject) (oapi.CreateCardCheckoutResponseObject, error) {
	if req.Body == nil {
		return nil, errNoBody
	}
	purpose := domain.CardPurpose(req.Body.Purpose)
	if !req.Body.Purpose.Valid() {
		return nil, apiErr(&domain.ValidationError{Field: "purpose", Message: "must be deposit, full or balance"}, "")
	}
	if s.opts.Cards == nil {
		return nil, errCardsOff
	}
	c, err := s.opts.Cards.StartCheckout(ctx, req.BookingId, req.Params.XBookingToken, purpose)
	if err != nil {
		return nil, cardErr(err)
	}
	return oapi.CreateCardCheckout201JSONResponse{
		Url: c.URL, Amount: c.Amount, Currency: oapi.Currency(c.Currency), Charged: c.Charged,
		ChargedCurrency: oapi.Currency(domain.CardCurrency), TestMode: c.TestMode,
	}, nil
}

// CheckCardCheckout reads the stay's open checkouts from Payments.lk now, for a guest coming
// back from paying, and returns the stay.
func (s *Server) CheckCardCheckout(ctx context.Context, req oapi.CheckCardCheckoutRequestObject) (oapi.CheckCardCheckoutResponseObject, error) {
	if s.opts.Cards != nil {
		if err := s.opts.Cards.Check(ctx, req.BookingId, req.Params.XBookingToken); err != nil {
			return nil, guestErr(err)
		}
	}
	gb, p, err := s.guestBooking(ctx, req.BookingId, req.Params.XBookingToken)
	if err != nil {
		return nil, err
	}
	return oapi.CheckCardCheckout200JSONResponse(s.guestBookingToAPI(gb, p)), nil
}

// sendCardRefunds makes one attempt at a stay's card refunds right after it is cancelled. The
// refunds job sends whatever this leaves.
func (s *Server) sendCardRefunds(ctx context.Context, b domain.Booking) {
	if s.opts.Cards == nil {
		return
	}
	for _, p := range b.Payments {
		if p.Kind == "refund" && p.Method == "card" && p.Status == "pending" {
			if _, err := s.opts.Cards.SendRefunds(ctx, &b.ID); err != nil {
				s.log.Warn("card refunds not sent after cancel", "booking", b.ID, "error", err)
			}
			return
		}
	}
}

func cardOwner(ctx context.Context) (tenant.Tenant, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return tenant.Tenant{}, err
	}
	if !t.Can(domain.PermBilling) {
		return tenant.Tenant{}, errForbidden
	}
	return t, nil
}

func cardErr(err error) error {
	switch {
	case errors.Is(err, domain.ErrCardsOff):
		return errCardsOff
	case errors.Is(err, domain.ErrNothingToPay):
		return errNothingToPay
	case errors.Is(err, domain.ErrInvalidCardKey):
		return errInvalidKey
	case errors.Is(err, domain.ErrTestKeyInProduction):
		return errTestKey
	case errors.Is(err, domain.ErrPayoutsUnavailable):
		return errNoPayouts
	case errors.Is(err, cards.ErrUnreachable):
		return errUnreachable
	}
	return guestErr(err)
}

func (s *Server) cardPaymentsToAPI(st domain.CardState) oapi.CardPayments {
	out := oapi.CardPayments{Mode: oapi.CardMode(st.Mode), PayoutsAvailable: s.store.PayoutsAvailable()}
	if c := st.Connection; c != nil {
		out.Connection = &oapi.CardConnection{
			Last4: c.Last4, Mode: oapi.CardConnectionMode(c.Mode), Status: oapi.CardConnectionStatus(c.Status), CheckedAt: c.CheckedAt,
		}
	}
	return out
}

// cardsToAPI is what the owner reads back about cards: on only when guests can pay by card.
func cardsToAPI(pay domain.PaymentSettings) (oapi.PaymentSettingsCards, *[]oapi.PaymentSettingsCardsMissing) {
	switch {
	case pay.Cards != "on":
		return oapi.PaymentSettingsCardsOff, nil
	case len(pay.CardsMissing) > 0:
		missing := make([]oapi.PaymentSettingsCardsMissing, len(pay.CardsMissing))
		for i, m := range pay.CardsMissing {
			missing[i] = oapi.PaymentSettingsCardsMissing(m)
		}
		return oapi.PaymentSettingsCardsPending, &missing
	}
	return oapi.PaymentSettingsCardsOn, nil
}

// publicCard is what the booking page needs to offer cards, or nil when the property doesn't
// take them.
func publicCard(p domain.PropertyDetail) *oapi.PublicCard {
	if !p.Payments.CardsReady() {
		return nil
	}
	out := &oapi.PublicCard{Charge: oapi.PublicCardCharge(p.Payments.CardCharge), TestMode: p.Payments.CardsTest}
	if p.Currency != domain.CardCurrency {
		out.Rate = &p.Payments.CardRate
	}
	return out
}

// guestCard lists what the guest can pay by card on their stay now, or nil when they can't.
func guestCard(gb domain.GuestBooking, p domain.PropertyDetail) *oapi.GuestCard {
	if !p.Payments.CardsReady() || (gb.Source != "page" && gb.Source != "widget") {
		return nil
	}
	rate := int64(0)
	if gb.Currency != domain.CardCurrency {
		rate = p.Payments.CardRate
	}
	out := &oapi.GuestCard{TestMode: p.Payments.CardsTest, Options: []oapi.CardOption{}}
	for _, purpose := range []domain.CardPurpose{domain.PurposeDeposit, domain.PurposeFull, domain.PurposeBalance} {
		amount, err := domain.CardAmount(purpose, gb.Status, gb.Total, pricing.Deposit(p, gb.Total), gb.Paid(), p.Payments.CardCharge)
		if err != nil {
			continue
		}
		charged := domain.ToLKR(amount, rate)
		if charged < domain.MinCardCharge || charged > domain.MaxCardCharge {
			continue
		}
		out.Options = append(out.Options, oapi.CardOption{Purpose: oapi.CardPurpose(purpose), Amount: amount, Charged: charged})
	}
	if len(out.Options) == 0 {
		return nil
	}
	return out
}

func paymentToAPI(p domain.Payment) oapi.Payment {
	out := oapi.Payment{
		Id: p.ID, Kind: oapi.PaymentKind(p.Kind), Method: oapi.PaymentMethod(p.Method),
		Amount: p.Amount, Note: p.Note, ReceivedAt: p.ReceivedAt, Failure: optional(p.Failure),
	}
	if p.Status != "" {
		status := oapi.PaymentStatus(p.Status)
		out.Status = &status
	}
	if p.Charged > 0 {
		currency := oapi.Currency(domain.CardCurrency)
		out.Charged, out.ChargedCurrency = &p.Charged, &currency
	}
	if p.Rate > 0 {
		out.Rate = &p.Rate
	}
	return out
}
