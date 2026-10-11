package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

var errOwnerMembership = &apiError{status: http.StatusForbidden, code: "owner_membership", message: "The owner's role and access can't be changed."}

// GetTeam lists the account's members and open invites.
func (s *Server) GetTeam(ctx context.Context, _ oapi.GetTeamRequestObject) (oapi.GetTeamResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	members, invites, err := s.store.ListTeam(ctx, t, time.Now())
	if err != nil {
		return nil, err
	}
	out := oapi.Team{Members: make([]oapi.TeamMember, len(members)), Invites: make([]oapi.Invite, len(invites))}
	for i, m := range members {
		out.Members[i] = memberToAPI(m)
	}
	for i, inv := range invites {
		out.Invites[i] = inviteToAPI(inv)
	}
	return oapi.GetTeam200JSONResponse(out), nil
}

// CreateInvite invites someone by phone and returns the link to send them.
func (s *Server) CreateInvite(ctx context.Context, req oapi.CreateInviteRequestObject) (oapi.CreateInviteResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermTeam) {
		return nil, errForbidden
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	in := domain.NewInvite{Phone: req.Body.Phone, Name: deref(req.Body.Name), Role: domain.Role(req.Body.Role), PropertyIDs: req.Body.PropertyIds}
	in.Normalize()
	if err := in.Validate(); err != nil {
		return nil, apiErr(err, "")
	}
	token, hash, err := domain.NewInviteToken()
	if err != nil {
		return nil, err
	}
	inv, err := s.store.InviteMember(ctx, t, in, hash, time.Now())
	if errors.Is(err, domain.ErrAlreadyMember) {
		return nil, &apiError{status: http.StatusBadRequest, code: "already_member", field: "phone", message: "That number is already on the team."}
	}
	if err != nil {
		return nil, apiErr(err, "")
	}
	return oapi.CreateInvite201JSONResponse{Invite: inviteToAPI(inv), Link: s.inviteLink(token)}, nil
}

// RevokeInvite stops an open invite's link from working.
func (s *Server) RevokeInvite(ctx context.Context, req oapi.RevokeInviteRequestObject) (oapi.RevokeInviteResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermTeam) {
		return nil, errForbidden
	}
	if err := s.store.RevokeInvite(ctx, t, req.InviteId, time.Now()); err != nil {
		return nil, apiErr(err, "invite")
	}
	return oapi.RevokeInvite204Response{}, nil
}

// UpdateMember changes a member's role or the properties they can see.
func (s *Server) UpdateMember(ctx context.Context, req oapi.UpdateMemberRequestObject) (oapi.UpdateMemberResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermTeam) {
		return nil, errForbidden
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	var p domain.MemberPatch
	if req.Body.Role != nil {
		role := domain.Role(*req.Body.Role)
		p.Role = &role
	}
	if req.Body.PropertyIds != nil {
		ids := append([]uuid.UUID{}, *req.Body.PropertyIds...)
		p.PropertyIDs = &ids
	}
	if err := p.Validate(); err != nil {
		return nil, apiErr(err, "")
	}
	m, err := s.store.UpdateMember(ctx, t, req.MembershipId, p)
	if errors.Is(err, domain.ErrOwnerMembership) {
		return nil, errOwnerMembership
	}
	if err != nil {
		return nil, apiErr(err, "member")
	}
	return oapi.UpdateMember200JSONResponse(memberToAPI(m)), nil
}

// RemoveMember takes someone out of the account.
func (s *Server) RemoveMember(ctx context.Context, req oapi.RemoveMemberRequestObject) (oapi.RemoveMemberResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermTeam) {
		return nil, errForbidden
	}
	err = s.store.RemoveMember(ctx, t, req.MembershipId)
	if errors.Is(err, domain.ErrOwnerMembership) {
		return nil, errOwnerMembership
	}
	if err != nil {
		return nil, apiErr(err, "member")
	}
	return oapi.RemoveMember204Response{}, nil
}

// AcceptInvite adds the signed-in person to the account an invite is for.
func (s *Server) AcceptInvite(ctx context.Context, req oapi.AcceptInviteRequestObject) (oapi.AcceptInviteResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || strings.TrimSpace(req.Body.Token) == "" {
		return nil, errNoBody
	}
	acc, err := s.store.AcceptInvite(ctx, p.UserID, domain.HashInviteToken(strings.TrimSpace(req.Body.Token)), time.Now())
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errInviteNotFound
	}
	if err != nil {
		return nil, err
	}
	return oapi.AcceptInvite200JSONResponse(accountToAPI(acc)), nil
}

// PreviewInvite describes an invite for the join screen, before the person signs in.
func (s *Server) PreviewInvite(ctx context.Context, req oapi.PreviewInviteRequestObject) (oapi.PreviewInviteResponseObject, error) {
	if req.Body == nil || strings.TrimSpace(req.Body.Token) == "" {
		return nil, errNoBody
	}
	pv, err := s.store.PreviewInvite(ctx, domain.HashInviteToken(strings.TrimSpace(req.Body.Token)), time.Now())
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errInviteNotFound
	}
	if err != nil {
		return nil, err
	}
	names := pv.PropertyNames
	if names == nil {
		names = []string{}
	}
	return oapi.PreviewInvite200JSONResponse{
		AccountName: pv.AccountName, InvitedByName: pv.InvitedByName, Role: oapi.MemberRole(pv.Role),
		PropertyNames: names, ExpiresAt: pv.ExpiresAt,
	}, nil
}

var errInviteNotFound = &apiError{
	status: http.StatusNotFound, code: "invite_not_found",
	message: "This invite has expired or was already used. Ask the owner to send a new one.",
}

// inviteLink is the web address that opens the join screen for a token.
func (s *Server) inviteLink(token string) string {
	base := s.opts.InviteURL
	if base == "" {
		base = "https://staykey.direct/join"
	}
	return strings.TrimRight(base, "/") + "/" + token
}

func memberToAPI(m domain.TeamMember) oapi.TeamMember {
	ids := m.PropertyIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return oapi.TeamMember{
		Id: m.MembershipID, UserId: m.UserID, Name: m.Name, Phone: m.Phone,
		Role: oapi.Role(m.Role), PropertyIds: ids, JoinedAt: m.JoinedAt,
	}
}

func inviteToAPI(inv domain.Invite) oapi.Invite {
	ids := inv.PropertyIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return oapi.Invite{
		Id: inv.ID, Phone: inv.Phone, Name: inv.Name, Role: oapi.MemberRole(inv.Role),
		PropertyIds: ids, ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt,
	}
}
