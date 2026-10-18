package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/oapi"
)

// Account controls: profile photos, deleting yourself, renaming and exporting an account, and
// erasing a guest's details.

// exportPage is how many stays the export reads at a time.
const exportPage = 500

// CreateMePhotoUpload presigns an upload for the signed-in person's profile photo.
func (s *Server) CreateMePhotoUpload(ctx context.Context, req oapi.CreateMePhotoUploadRequestObject) (oapi.CreateMePhotoUploadResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	limit := files.MaxBytes[files.KindAvatar]
	if req.Body.Size < 1 || req.Body.Size > limit {
		return oapi.CreateMePhotoUpload400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
			fieldErr("file_too_large", "size", fmt.Sprintf("Photos can be up to %d MB.", limit>>20)))}, nil
	}
	key, err := files.NewAvatarKey(p.UserID, string(req.Body.ContentType))
	if err != nil {
		return oapi.CreateMePhotoUpload400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
			fieldErr("invalid_content_type", "contentType", "Use a JPEG, PNG, WebP or HEIC image."))}, nil
	}
	up, err := s.files.PresignUpload(ctx, key, string(req.Body.ContentType), req.Body.Size)
	if err != nil {
		return nil, err
	}
	return oapi.CreateMePhotoUpload201JSONResponse{
		Key: up.Key, UploadUrl: up.URL, Method: oapi.PUT, Headers: up.Headers,
		ExpiresAt: up.ExpiresAt, Url: up.PublicURL,
	}, nil
}

// DeleteMe starts deleting the signed-in person and the accounts they own.
func (s *Server) DeleteMe(ctx context.Context, req oapi.DeleteMeRequestObject) (oapi.DeleteMeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Confirm != "DELETE" {
		return oapi.DeleteMe400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
			fieldErr("confirm_required", "confirm", `Type DELETE to confirm.`))}, nil
	}
	deletesAt, err := s.store.DeleteUser(ctx, p.UserID, time.Now())
	if errors.Is(err, domain.ErrStorePlanRenews) {
		return oapi.DeleteMe409JSONResponse{ConflictJSONResponse: oapi.ConflictJSONResponse{
			Code: oapi.StorePlanRenews, Message: "Cancel your plan in the App Store or Google Play first, then delete your account.",
		}}, nil
	}
	if err != nil {
		return nil, err
	}
	s.log.Info("deletion scheduled", "user_id", p.UserID, "deletes_at", deletesAt)
	return oapi.DeleteMe200JSONResponse{DeletesAt: deletesAt}, nil
}

// RenameAccount renames the account in X-Account-Id. Owners only.
func (s *Server) RenameAccount(ctx context.Context, req oapi.RenameAccountRequestObject) (oapi.RenameAccountResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if t.Role != domain.RoleOwner {
		return nil, errForbidden
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	name, err := domain.NormalizeAccountName(req.Body.Name)
	if err != nil {
		return nil, apiErr(err, "")
	}
	a, err := s.store.RenameAccount(ctx, t, name)
	if err != nil {
		return nil, err
	}
	return oapi.RenameAccount200JSONResponse(accountToAPI(a)), nil
}

// ExportAccount returns everything in the account as one document. Owners only.
func (s *Server) ExportAccount(ctx context.Context, _ oapi.ExportAccountRequestObject) (oapi.ExportAccountResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if t.Role != domain.RoleOwner {
		return nil, errForbidden
	}
	now := time.Now()
	accounts, err := s.store.ListAccounts(ctx, t.UserID)
	if err != nil {
		return nil, err
	}
	var account *oapi.Account
	for _, a := range accounts {
		if a.ID == t.AccountID {
			v := accountToAPI(a)
			account = &v
		}
	}
	if account == nil {
		return nil, errNoAccount
	}

	out := oapi.AccountExport{ExportedAt: now, Account: *account, Properties: []oapi.Property{}, Bookings: []oapi.Booking{}}
	props, err := s.store.ListProperties(ctx, t)
	if err != nil {
		return nil, err
	}
	for _, p := range props {
		out.Properties = append(out.Properties, s.propertyToAPI(ctx, p, true))
	}
	f := domain.BookingFilter{Limit: exportPage}
	for {
		page, err := s.store.ListBookings(ctx, t, f)
		if err != nil {
			return nil, err
		}
		for _, b := range page {
			out.Bookings = append(out.Bookings, s.stayToAPI(ctx, b, true))
		}
		if len(page) < exportPage {
			break
		}
		last := page[len(page)-1]
		f.AfterCheckIn, f.AfterID = &last.CheckIn, &last.ID
	}
	members, invites, err := s.store.ListTeam(ctx, t, now)
	if err != nil {
		return nil, err
	}
	out.Team = oapi.Team{Members: make([]oapi.TeamMember, len(members)), Invites: make([]oapi.Invite, len(invites))}
	for i, m := range members {
		out.Team.Members[i] = memberToAPI(m)
	}
	for i, inv := range invites {
		out.Team.Invites[i] = inviteToAPI(inv)
	}
	sub, err := s.store.Subscription(ctx, t, now)
	if err != nil {
		return nil, err
	}
	out.Subscription = subscriptionToAPI(sub, t, now)
	s.log.Info("account exported", "account_id", t.AccountID, "bookings", len(out.Bookings))
	return oapi.ExportAccount200JSONResponse(out), nil
}

// EraseBookingGuest erases the details of the guest behind a stay. Owners only.
func (s *Server) EraseBookingGuest(ctx context.Context, req oapi.EraseBookingGuestRequestObject) (oapi.EraseBookingGuestResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if t.Role != domain.RoleOwner {
		return nil, errForbidden
	}
	if err := s.store.EraseGuest(ctx, t, req.BookingId); err != nil {
		return nil, apiErr(err, "booking")
	}
	return oapi.EraseBookingGuest204Response{}, nil
}
