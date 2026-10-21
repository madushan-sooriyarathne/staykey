package server

import (
	"context"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// StayKey payouts: the owner's payout details, balances and statements. Owners only.

// GetPayoutProfile returns the owner's payout details, sealed numbers by their last four.
func (s *Server) GetPayoutProfile(ctx context.Context, _ oapi.GetPayoutProfileRequestObject) (oapi.GetPayoutProfileResponseObject, error) {
	t, err := cardOwner(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.store.PayoutProfile(ctx, t.AccountID)
	if err != nil {
		return nil, apiErr(err, "payout_profile")
	}
	return oapi.GetPayoutProfile200JSONResponse(payoutProfileToAPI(p, time.Now())), nil
}

// SetPayoutProfile saves the owner's payout details.
func (s *Server) SetPayoutProfile(ctx context.Context, req oapi.SetPayoutProfileRequestObject) (oapi.SetPayoutProfileResponseObject, error) {
	t, err := cardOwner(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	b := req.Body
	now := time.Now()
	p, err := s.store.SetPayoutProfile(ctx, t, domain.PayoutProfileInput{
		LegalName: b.LegalName, NIC: b.Nic, BankName: b.BankName, Branch: b.Branch,
		AccountName: b.AccountName, AccountNumber: b.AccountNumber, AcceptTerms: b.AcceptTerms,
	}, now)
	if err != nil {
		return nil, apiErr(err, "payout_profile")
	}
	s.log.Info("payout details saved", "account", t.AccountID)
	return oapi.SetPayoutProfile200JSONResponse(payoutProfileToAPI(p, now)), nil
}

// GetPayouts returns the account's payout balances and past payouts.
func (s *Server) GetPayouts(ctx context.Context, _ oapi.GetPayoutsRequestObject) (oapi.GetPayoutsResponseObject, error) {
	t, err := cardOwner(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.store.Payouts(ctx, t.AccountID, time.Now())
	if err != nil {
		return nil, err
	}
	out := oapi.Payouts{
		Available: p.Available, Pending: p.Pending, OnTheWay: p.OnTheWay, NextPayoutOn: day(p.NextPayoutDay),
		Minimum: domain.PayoutMinimum, HeldUntil: p.HeldUntil, Payouts: make([]oapi.Payout, len(p.History)),
	}
	for i, h := range p.History {
		out.Payouts[i] = payoutToAPI(h)
	}
	return oapi.GetPayouts200JSONResponse(out), nil
}

// GetPayout returns one payout with the entries it paid.
func (s *Server) GetPayout(ctx context.Context, req oapi.GetPayoutRequestObject) (oapi.GetPayoutResponseObject, error) {
	t, err := cardOwner(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.store.PayoutStatement(ctx, t.AccountID, req.PayoutId)
	if err != nil {
		return nil, apiErr(err, "payout")
	}
	out := oapi.PayoutStatement{Payout: payoutToAPI(st.Payout), Lines: make([]oapi.PayoutLine, len(st.Lines))}
	for i, l := range st.Lines {
		line := oapi.PayoutLine{
			Id: l.ID, Kind: oapi.PayoutEntryKind(l.Kind), Amount: l.Amount, AvailableOn: day(l.AvailableOn),
			Note: l.Note, BookingId: l.BookingID, BookingRef: optional(l.BookingRef), GuestName: optional(l.GuestName),
		}
		if l.CheckIn != nil {
			d := day(*l.CheckIn)
			line.CheckIn = &d
		}
		out.Lines[i] = line
	}
	return oapi.GetPayout200JSONResponse(out), nil
}

func payoutProfileToAPI(p domain.PayoutProfile, now time.Time) oapi.PayoutProfile {
	return oapi.PayoutProfile{
		LegalName: p.LegalName, NicLast4: p.NICLast4, BankName: p.BankName, Branch: p.Branch,
		AccountName: p.AccountName, AccountLast4: p.AccountLast4, TermsVersion: p.TermsVersion,
		AcceptedAt: p.AcceptedAt, BankChangedAt: p.BankChangedAt, HeldUntil: p.HeldUntil(now),
	}
}

func payoutToAPI(p domain.Payout) oapi.Payout {
	return oapi.Payout{
		Id: p.ID, PeriodEnd: day(p.PeriodEnd), Amount: p.Amount, Status: oapi.PayoutStatus(p.Status),
		Reason: optional(p.Reason), PreparedAt: p.PreparedAt, PaidAt: p.PaidAt, FailedAt: p.FailedAt,
	}
}
