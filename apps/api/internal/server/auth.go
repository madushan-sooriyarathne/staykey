package server

import (
	"context"
	"errors"
	"fmt"
	"math"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// RequestOtp texts a sign-in code.
func (s *Server) RequestOtp(ctx context.Context, req oapi.RequestOtpRequestObject) (oapi.RequestOtpResponseObject, error) {
	if req.Body == nil {
		return oapi.RequestOtp400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(errBody("invalid_body", "Request body is required."))}, nil
	}
	sent, err := s.auth.RequestCode(ctx, req.Body.Phone, clientIP(ctx))

	var vErr *domain.ValidationError
	var rl *auth.RateLimitError
	switch {
	case errors.As(err, &vErr):
		return oapi.RequestOtp400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
			fieldErr("invalid_phone", "phone", sentence(vErr.Message)))}, nil
	case errors.As(err, &rl):
		secs := int(math.Ceil(rl.RetryAfter.Seconds()))
		body := errBody("too_many_requests", tryAgainIn(secs))
		body.RetryAfter = &secs
		return oapi.RequestOtp429JSONResponse{TooManyRequestsJSONResponse: oapi.TooManyRequestsJSONResponse{
			Body: body, Headers: oapi.TooManyRequestsResponseHeaders{RetryAfter: &secs},
		}}, nil
	case err != nil:
		return nil, err
	}

	out := oapi.RequestOtp200JSONResponse{
		Phone:       sent.Phone,
		ExpiresAt:   sent.ExpiresAt,
		ResendAfter: int(sent.ResendAfter.Seconds()),
	}
	if sent.DevCode != "" {
		out.DevCode = &sent.DevCode
	}
	return out, nil
}

// VerifyOtp checks a code and signs the user in.
func (s *Server) VerifyOtp(ctx context.Context, req oapi.VerifyOtpRequestObject) (oapi.VerifyOtpResponseObject, error) {
	if req.Body == nil {
		return oapi.VerifyOtp400JSONResponse(errBody("invalid_body", "Request body is required.")), nil
	}
	device := ""
	if req.Body.DeviceName != nil {
		device = *req.Body.DeviceName
	}
	in, err := s.auth.VerifyCode(ctx, req.Body.Phone, req.Body.Code, device)

	var vErr *domain.ValidationError
	var wrong *auth.WrongCodeError
	switch {
	case errors.As(err, &vErr):
		return oapi.VerifyOtp400JSONResponse(fieldErr("invalid_"+vErr.Field, vErr.Field, sentence(vErr.Message))), nil
	case errors.As(err, &wrong):
		msg := "That code didn't match. Request a new one."
		if wrong.AttemptsLeft > 0 {
			msg = fmt.Sprintf("That code didn't match. %d %s left.", wrong.AttemptsLeft, plural(wrong.AttemptsLeft, "try", "tries"))
		}
		return oapi.VerifyOtp400JSONResponse(fieldErr("wrong_code", "code", msg)), nil
	case errors.Is(err, auth.ErrCodeExpired):
		return oapi.VerifyOtp400JSONResponse(fieldErr("code_expired", "code", "That code has expired. Request a new one.")), nil
	case errors.Is(err, auth.ErrTooManyAttempts):
		return oapi.VerifyOtp400JSONResponse(fieldErr("too_many_attempts", "code", "Too many wrong codes. Request a new one.")), nil
	case err != nil:
		return nil, err
	}

	accounts, err := s.store.ListAccounts(ctx, in.User.ID)
	if err != nil {
		return nil, err
	}
	return oapi.VerifyOtp200JSONResponse{
		Tokens:   tokensToAPI(in.Tokens),
		User:     userToAPI(in.User),
		Accounts: accountsToAPI(accounts),
		NewUser:  in.NewUser,
	}, nil
}

// RefreshSession rotates the refresh token.
func (s *Server) RefreshSession(ctx context.Context, req oapi.RefreshSessionRequestObject) (oapi.RefreshSessionResponseObject, error) {
	if req.Body == nil {
		return nil, errUnauthorized
	}
	tokens, err := s.auth.Refresh(ctx, req.Body.RefreshToken)
	if errors.Is(err, auth.ErrInvalidRefresh) {
		return oapi.RefreshSession401JSONResponse{UnauthorizedJSONResponse: oapi.UnauthorizedJSONResponse(
			errBody("invalid_refresh", "Your session has ended. Sign in again."))}, nil
	}
	if err != nil {
		return nil, err
	}
	return oapi.RefreshSession200JSONResponse(tokensToAPI(tokens)), nil
}

// Logout ends a session.
func (s *Server) Logout(ctx context.Context, req oapi.LogoutRequestObject) (oapi.LogoutResponseObject, error) {
	if req.Body != nil {
		if err := s.auth.Logout(ctx, req.Body.RefreshToken); err != nil {
			return nil, err
		}
	}
	return oapi.Logout204Response{}, nil
}

func tokensToAPI(t auth.Tokens) oapi.AuthTokens {
	return oapi.AuthTokens{
		AccessToken:           t.AccessToken,
		AccessTokenExpiresAt:  t.AccessExpiresAt,
		RefreshToken:          t.RefreshToken,
		RefreshTokenExpiresAt: t.RefreshExpiresAt,
	}
}

// sentence capitalizes a domain message and ends it with a full stop.
func sentence(msg string) string {
	if msg == "" {
		return msg
	}
	if msg[0] >= 'a' && msg[0] <= 'z' {
		msg = string(msg[0]-'a'+'A') + msg[1:]
	}
	if msg[len(msg)-1] != '.' {
		msg += "."
	}
	return msg
}

func tryAgainIn(secs int) string {
	if secs < 90 {
		return fmt.Sprintf("Please wait %d %s before asking for another code.", secs, plural(secs, "second", "seconds"))
	}
	mins := (secs + 59) / 60
	return fmt.Sprintf("Too many codes requested. Try again in %d %s.", mins, plural(mins, "minute", "minutes"))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
