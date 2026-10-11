package server

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// ListTemplates returns the account's message templates.
func (s *Server) ListTemplates(ctx context.Context, _ oapi.ListTemplatesRequestObject) (oapi.ListTemplatesResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	list, err := s.store.ListTemplates(ctx, t)
	if err != nil {
		return nil, err
	}
	out := oapi.TemplateList{Items: make([]oapi.Template, len(list))}
	for i, tpl := range list {
		out.Items[i] = templateToAPI(tpl)
	}
	return oapi.ListTemplates200JSONResponse(out), nil
}

// UpdateTemplate changes a template's text.
func (s *Server) UpdateTemplate(ctx context.Context, req oapi.UpdateTemplateRequestObject) (oapi.UpdateTemplateResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermSettings) {
		return nil, errForbidden
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	// Another account's template is not found, whatever the body says.
	list, err := s.store.ListTemplates(ctx, t)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(list, func(tpl domain.Template) bool { return tpl.ID == req.TemplateId }) {
		return nil, apiErr(domain.ErrNotFound, "template")
	}
	body, err := domain.NormalizeTemplateBody(req.Body.Body)
	if err != nil {
		return nil, apiErr(err, "")
	}
	tpl, err := s.store.UpdateTemplate(ctx, t, req.TemplateId, body)
	if err != nil {
		return nil, apiErr(err, "template")
	}
	return oapi.UpdateTemplate200JSONResponse(templateToAPI(tpl)), nil
}

// GetNotificationPrefs returns how the caller wants to hear about each event.
func (s *Server) GetNotificationPrefs(ctx context.Context, _ oapi.GetNotificationPrefsRequestObject) (oapi.GetNotificationPrefsResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	prefs, err := s.store.NotificationPrefs(ctx, t)
	if err != nil {
		return nil, err
	}
	return oapi.GetNotificationPrefs200JSONResponse(prefsToAPI(prefs)), nil
}

// SetNotificationPrefs saves the caller's preferences for the events sent.
func (s *Server) SetNotificationPrefs(ctx context.Context, req oapi.SetNotificationPrefsRequestObject) (oapi.SetNotificationPrefsResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, errNoBody
	}
	in := make([]domain.NotificationPref, 0, len(req.Body.Items))
	seen := map[domain.NotificationEvent]bool{}
	for _, p := range req.Body.Items {
		e := domain.NotificationEvent(p.Event)
		if !e.Valid() || seen[e] {
			return nil, &apiError{status: http.StatusBadRequest, code: "invalid_event", field: "items", message: "Send each event once."}
		}
		seen[e] = true
		in = append(in, domain.NotificationPref{Event: e, Push: p.Push, WhatsApp: p.Whatsapp})
	}
	prefs, err := s.store.SetNotificationPrefs(ctx, t, in)
	if err != nil {
		return nil, err
	}
	return oapi.SetNotificationPrefs200JSONResponse(prefsToAPI(prefs)), nil
}

// RegisterPushToken ties this device's push token to the signed-in session.
func (s *Server) RegisterPushToken(ctx context.Context, req oapi.RegisterPushTokenRequestObject) (oapi.RegisterPushTokenResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Token == "" {
		return nil, errNoBody
	}
	if !isExpoPushToken(req.Body.Token) {
		return nil, &apiError{status: http.StatusBadRequest, code: "invalid_token", field: "token", message: "Send the Expo push token for this device."}
	}
	switch req.Body.Platform {
	case oapi.Ios, oapi.Android, oapi.Web:
	default:
		return nil, &apiError{status: http.StatusBadRequest, code: "invalid_platform", field: "platform", message: "Platform is ios, android or web."}
	}
	if err := s.store.SavePushToken(ctx, p.UserID, p.SessionID, req.Body.Token, string(req.Body.Platform)); err != nil {
		return nil, err
	}
	return oapi.RegisterPushToken204Response{}, nil
}

// isExpoPushToken matches the tokens Expo issues, ExponentPushToken[...] or ExpoPushToken[...],
// within the spec's 300 characters. The server doesn't validate bodies against the spec, so the
// limit is checked here.
func isExpoPushToken(token string) bool {
	if len(token) > 300 || !strings.HasSuffix(token, "]") {
		return false
	}
	return strings.HasPrefix(token, "ExponentPushToken[") || strings.HasPrefix(token, "ExpoPushToken[")
}

func templateToAPI(t domain.Template) oapi.Template {
	channels := make([]oapi.TemplateChannel, len(t.Channels))
	for i, c := range t.Channels {
		channels[i] = oapi.TemplateChannel(c)
	}
	return oapi.Template{
		Id: t.ID, Key: oapi.TemplateKey(t.Key), Name: t.Name, Channels: channels, Timing: t.Timing, Body: t.Body,
	}
}

func prefsToAPI(prefs []domain.NotificationPref) oapi.NotificationPrefs {
	out := oapi.NotificationPrefs{Items: make([]oapi.NotificationPref, len(prefs))}
	for i, p := range prefs {
		out.Items[i] = oapi.NotificationPref{Event: oapi.NotificationEvent(p.Event), Push: p.Push, Whatsapp: p.WhatsApp}
	}
	return out
}
