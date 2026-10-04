package server

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// GetMe returns the signed-in user and their accounts.
func (s *Server) GetMe(ctx context.Context, _ oapi.GetMeRequestObject) (oapi.GetMeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	u, err := s.store.GetUser(ctx, p.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errUnauthorized
	}
	if err != nil {
		return nil, err
	}
	accounts, err := s.store.ListAccounts(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return oapi.GetMe200JSONResponse{User: userToAPI(u), Accounts: accountsToAPI(accounts)}, nil
}

// UpdateMe changes the signed-in user's name, email or language.
func (s *Server) UpdateMe(ctx context.Context, req oapi.UpdateMeRequestObject) (oapi.UpdateMeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return oapi.UpdateMe400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(errBody("invalid_body", "Request body is required."))}, nil
	}
	patch := domain.UserPatch{Name: req.Body.Name, Email: req.Body.Email}
	if req.Body.Language != nil {
		lang := string(*req.Body.Language)
		patch.Language = &lang
	}
	patch.Normalize()
	if err := patch.Validate(); err != nil {
		var vErr *domain.ValidationError
		if errors.As(err, &vErr) {
			return oapi.UpdateMe400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
				fieldErr("invalid_"+vErr.Field, vErr.Field, sentence(vErr.Message)))}, nil
		}
		return nil, err
	}
	u, err := s.store.UpdateUser(ctx, p.UserID, patch)
	if err != nil {
		return nil, err
	}
	return oapi.UpdateMe200JSONResponse(userToAPI(u)), nil
}

// ListAccounts returns the accounts the signed-in user belongs to.
func (s *Server) ListAccounts(ctx context.Context, _ oapi.ListAccountsRequestObject) (oapi.ListAccountsResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	accounts, err := s.store.ListAccounts(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return oapi.ListAccounts200JSONResponse{Items: accountsToAPI(accounts)}, nil
}

// CreateAccount creates an account owned by the signed-in user.
func (s *Server) CreateAccount(ctx context.Context, req oapi.CreateAccountRequestObject) (oapi.CreateAccountResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return oapi.CreateAccount400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(errBody("invalid_body", "Request body is required."))}, nil
	}
	name, err := domain.NormalizeAccountName(req.Body.Name)
	if err != nil {
		var vErr *domain.ValidationError
		if errors.As(err, &vErr) {
			return oapi.CreateAccount400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
				fieldErr("invalid_name", "name", sentence(vErr.Message)))}, nil
		}
		return nil, err
	}
	a, err := s.store.CreateAccount(ctx, p.UserID, name)
	if err != nil {
		return nil, err
	}
	return oapi.CreateAccount201JSONResponse(accountToAPI(a)), nil
}

func userToAPI(u domain.User) oapi.User {
	return oapi.User{
		Id:        u.ID,
		Phone:     u.Phone,
		Name:      u.Name,
		Email:     optional(u.Email),
		Language:  oapi.Language(u.Language),
		CreatedAt: u.CreatedAt,
	}
}

func accountToAPI(a domain.AccountMembership) oapi.Account {
	ids := a.PropertyIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return oapi.Account{
		Id:             a.ID,
		Name:           a.Name,
		Status:         oapi.AccountStatus(a.Status),
		Role:           oapi.Role(a.Role),
		PropertyIds:    ids,
		TrialStartedAt: a.TrialStartedAt,
		CreatedAt:      a.CreatedAt,
	}
}

func accountsToAPI(in []domain.AccountMembership) []oapi.Account {
	out := make([]oapi.Account, len(in))
	for i, a := range in {
		out[i] = accountToAPI(a)
	}
	return out
}
