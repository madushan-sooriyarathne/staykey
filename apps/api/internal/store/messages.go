package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// ListTemplates returns the account's four message templates.
func (s *Postgres) ListTemplates(ctx context.Context, t tenant.Tenant) ([]domain.Template, error) {
	rows, err := s.db(ctx).ListTemplates(ctx, t.AccountID)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	out := make([]domain.Template, len(rows))
	for i, r := range rows {
		out[i] = template(r)
	}
	return out, nil
}

// UpdateTemplate replaces a template's text.
func (s *Postgres) UpdateTemplate(ctx context.Context, t tenant.Tenant, id uuid.UUID, body string) (domain.Template, error) {
	r, err := s.db(ctx).UpdateTemplateBody(ctx, queries.UpdateTemplateBodyParams{Body: body, ID: id, AccountID: t.AccountID})
	if err != nil {
		return domain.Template{}, notFound(err)
	}
	return template(r), nil
}

// NotificationPrefs returns the tenant's user's preferences in the tenant's account.
func (s *Postgres) NotificationPrefs(ctx context.Context, t tenant.Tenant) ([]domain.NotificationPref, error) {
	rows, err := s.db(ctx).ListNotificationPrefs(ctx, queries.ListNotificationPrefsParams{AccountID: t.AccountID, UserID: t.UserID})
	if err != nil {
		return nil, fmt.Errorf("list notification preferences: %w", err)
	}
	out := make([]domain.NotificationPref, len(rows))
	for i, r := range rows {
		out[i] = domain.NotificationPref{Event: domain.NotificationEvent(r.Event), Push: r.Push, WhatsApp: r.Whatsapp}
	}
	return out, nil
}

// SetNotificationPrefs saves the preferences given and returns all of them.
func (s *Postgres) SetNotificationPrefs(ctx context.Context, t tenant.Tenant, prefs []domain.NotificationPref) ([]domain.NotificationPref, error) {
	err := s.InTx(ctx, func(ctx context.Context) error {
		for _, p := range prefs {
			if err := s.db(ctx).SetNotificationPref(ctx, queries.SetNotificationPrefParams{
				ID: domain.NewID(), AccountID: t.AccountID, UserID: t.UserID,
				Event: string(p.Event), Push: p.Push, Whatsapp: p.WhatsApp,
			}); err != nil {
				return fmt.Errorf("set notification preference: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.NotificationPrefs(ctx, t)
}

// SavePushToken ties a device's push token to the session that registered it.
func (s *Postgres) SavePushToken(ctx context.Context, userID, sessionID uuid.UUID, token, platform string) error {
	err := s.db(ctx).UpsertPushToken(ctx, queries.UpsertPushTokenParams{
		ID: domain.NewID(), UserID: userID, SessionID: &sessionID, Token: token, Platform: platform,
	})
	if err != nil {
		return fmt.Errorf("save push token: %w", err)
	}
	return nil
}

// Summary is one person's morning summary for one account.
type Summary struct {
	AccountID  uuid.UUID
	UserID     uuid.UUID
	Phone      string
	Day        time.Time
	PushTokens []string
	WhatsApp   bool
	Arrivals   int
	Departures int
	Requests   int
}

// DueSummaries returns the morning summaries due at now, with the counts for each person's
// properties and the push tokens of their signed-in devices.
func (s *Postgres) DueSummaries(ctx context.Context, now time.Time) ([]Summary, error) {
	q := s.db(ctx)
	rows, err := q.DueSummaries(ctx, now)
	if err != nil {
		return nil, fmt.Errorf("due summaries: %w", err)
	}
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		var scope []uuid.UUID
		if len(r.PropertyIds) > 0 {
			scope = r.PropertyIds
		}
		counts, err := q.SummaryCounts(ctx, queries.SummaryCountsParams{Day: r.LocalDay, AccountID: r.AccountID, PropertyIds: scope})
		if err != nil {
			return nil, fmt.Errorf("summary counts: %w", err)
		}
		sum := Summary{
			AccountID: r.AccountID, UserID: r.UserID, Phone: r.Phone, Day: r.LocalDay, WhatsApp: r.Whatsapp,
			Arrivals: int(counts.Arrivals), Departures: int(counts.Departures), Requests: int(counts.Requests),
		}
		if r.Push {
			tokens, err := q.LivePushTokens(ctx, queries.LivePushTokensParams{UserIds: []uuid.UUID{r.UserID}, Now: now})
			if err != nil {
				return nil, fmt.Errorf("push tokens: %w", err)
			}
			for _, tok := range tokens {
				sum.PushTokens = append(sum.PushTokens, tok.Token)
			}
		}
		out = append(out, sum)
	}
	return out, nil
}

// MarkSummarySent records that a person's summary for day went out.
func (s *Postgres) MarkSummarySent(ctx context.Context, accountID, userID uuid.UUID, day time.Time) error {
	err := s.db(ctx).MarkSummarySent(ctx, queries.MarkSummarySentParams{Day: day, AccountID: accountID, UserID: userID})
	if err != nil {
		return fmt.Errorf("mark summary sent: %w", err)
	}
	return nil
}

func (s *Postgres) addTemplates(ctx context.Context, accountID uuid.UUID) error {
	for _, t := range domain.DefaultTemplates() {
		if err := s.db(ctx).InsertTemplate(ctx, queries.InsertTemplateParams{
			ID: domain.NewID(), AccountID: accountID, Key: t.Key, Name: t.Name,
			Channels: t.Channels, Timing: t.Timing, Body: t.Body,
		}); err != nil {
			return fmt.Errorf("insert template: %w", err)
		}
	}
	return nil
}

func (s *Postgres) addNotificationPrefs(ctx context.Context, accountID, userID uuid.UUID) error {
	for _, p := range domain.DefaultNotificationPrefs() {
		if err := s.db(ctx).InsertNotificationPref(ctx, queries.InsertNotificationPrefParams{
			ID: domain.NewID(), AccountID: accountID, UserID: userID,
			Event: string(p.Event), Push: p.Push, Whatsapp: p.WhatsApp,
		}); err != nil {
			return fmt.Errorf("insert notification preference: %w", err)
		}
	}
	return nil
}

func template(r queries.MessageTemplate) domain.Template {
	return domain.Template{
		ID: r.ID, Key: r.Key, Name: r.Name, Channels: r.Channels, Timing: r.Timing, Body: r.Body, UpdatedAt: r.UpdatedAt,
	}
}
