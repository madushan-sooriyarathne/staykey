package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// ActivityCursor is where a page of activity continues from.
type ActivityCursor struct {
	At time.Time
	ID uuid.UUID
}

// ListActivity returns a page of the account's activity as the tenant may see it, newest
// first, with each item's read state for the tenant's user.
func (s *Postgres) ListActivity(ctx context.Context, t tenant.Tenant, after *ActivityCursor, limit int) ([]domain.Activity, error) {
	q := s.db(ctx)
	params := queries.ListActivityParams{
		AccountID:   t.AccountID,
		PropertyIds: t.PropertyScope(),
		HiddenKinds: hiddenKinds(t),
		RowLimit:    int32(limit),
	}
	if after != nil {
		params.BeforeAt, params.BeforeID = &after.At, &after.ID
	}
	rows, err := q.ListActivity(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list activity: %w", err)
	}
	lastRead, err := s.lastRead(ctx, t)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Activity, len(rows))
	for i, r := range rows {
		a := activity(r)
		a.Read = !r.CreatedAt.After(lastRead) || (r.ActorUserID != nil && *r.ActorUserID == t.UserID)
		out[i] = a
	}
	return out, nil
}

// UnreadActivity counts what the tenant's user hasn't read, leaving out their own actions.
func (s *Postgres) UnreadActivity(ctx context.Context, t tenant.Tenant) (int, error) {
	n, err := s.db(ctx).CountUnreadActivity(ctx, queries.CountUnreadActivityParams{
		AccountID: t.AccountID, PropertyIds: t.PropertyScope(), HiddenKinds: hiddenKinds(t), UserID: t.UserID,
	})
	if err != nil {
		return 0, fmt.Errorf("count unread activity: %w", err)
	}
	return int(n), nil
}

// MarkActivityRead marks everything up to now as read for the tenant's user.
func (s *Postgres) MarkActivityRead(ctx context.Context, t tenant.Tenant, now time.Time) error {
	err := s.db(ctx).SetLastRead(ctx, queries.SetLastReadParams{AccountID: t.AccountID, UserID: t.UserID, LastReadAt: now})
	if err != nil {
		return fmt.Errorf("mark activity read: %w", err)
	}
	return nil
}

func (s *Postgres) lastRead(ctx context.Context, t tenant.Tenant) (time.Time, error) {
	at, err := s.db(ctx).GetLastRead(ctx, queries.GetLastReadParams{AccountID: t.AccountID, UserID: t.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get last read: %w", err)
	}
	return at, nil
}

// addActivity records activity inside the caller's transaction, so it lands with the change it
// describes or not at all.
func (s *Postgres) addActivity(ctx context.Context, accountID uuid.UUID, a domain.Activity, actor *uuid.UUID, at time.Time) error {
	err := s.db(ctx).InsertActivity(ctx, queries.InsertActivityParams{
		ID:          domain.NewID(),
		AccountID:   accountID,
		PropertyID:  a.PropertyID,
		BookingID:   a.BookingID,
		Kind:        string(a.Kind),
		Title:       a.Title,
		Subtitle:    a.Subtitle,
		ActorUserID: actor,
		CreatedAt:   at,
	})
	if err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

// userName is the name activity text uses for the person acting.
func (s *Postgres) userName(ctx context.Context, id uuid.UUID) (string, error) {
	u, err := s.db(ctx).GetUser(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}
	return u.Name, nil
}

// hiddenKinds are the activity kinds the tenant may not see. Never nil: the query reads a null
// array as "hide everything".
func hiddenKinds(t tenant.Tenant) []string {
	out := []string{}
	if t.Can(domain.PermPrices) {
		return out
	}
	for _, k := range domain.HiddenFromCaretakers {
		out = append(out, string(k))
	}
	return out
}

func activity(r queries.Activity) domain.Activity {
	return domain.Activity{
		ID:          r.ID,
		PropertyID:  r.PropertyID,
		BookingID:   r.BookingID,
		Kind:        domain.ActivityKind(r.Kind),
		Title:       r.Title,
		Subtitle:    r.Subtitle,
		ActorUserID: r.ActorUserID,
		CreatedAt:   r.CreatedAt,
	}
}

// Alert is one activity item to send, with the people who should get it.
type Alert struct {
	Activity   domain.Activity
	AccountID  uuid.UUID
	Recipients []Recipient
}

// Recipient is a person an alert goes to and the channels they chose.
type Recipient struct {
	UserID     uuid.UUID
	Phone      string
	PushTokens []string
	WhatsApp   bool
}

// SendAlerts claims up to limit activity items nobody has been alerted about yet and passes them
// to send with their recipients. The claim is a short transaction that stamps the rows, and send
// runs after it commits, so no transaction stays open across network calls. When send reports an
// error the rows are released for the next run; if the process stops mid-send they are not sent
// again. A concurrent run skips rows another run is claiming.
func (s *Postgres) SendAlerts(ctx context.Context, limit int, now time.Time, send func(ctx context.Context, alerts []Alert) error) (int, error) {
	var alerts []Alert
	var ids []uuid.UUID
	err := s.InTx(ctx, func(ctx context.Context) error {
		rows, err := s.db(ctx).ClaimUnnotifiedActivity(ctx, queries.ClaimUnnotifiedActivityParams{Now: now, RowLimit: int32(limit)})
		if err != nil {
			return fmt.Errorf("claim activity: %w", err)
		}
		ids = make([]uuid.UUID, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
			a, err := s.alertFor(ctx, r, now)
			if err != nil {
				return err
			}
			if len(a.Recipients) > 0 {
				alerts = append(alerts, a)
			}
		}
		return nil
	})
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	if err := send(ctx, alerts); err != nil {
		// The job's context may be the reason send failed, so the release gets its own deadline.
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if relErr := s.db(releaseCtx).ReleaseActivity(releaseCtx, ids); relErr != nil {
			return 0, errors.Join(err, fmt.Errorf("release activity: %w", relErr))
		}
		return 0, err
	}
	return len(ids), nil
}

func (s *Postgres) alertFor(ctx context.Context, r queries.Activity, now time.Time) (Alert, error) {
	q := s.db(ctx)
	a := activity(r)
	people, err := q.ActivityRecipients(ctx, queries.ActivityRecipientsParams{
		Event: string(domain.EventFor(a.Kind)), AccountID: r.AccountID, ActorUserID: r.ActorUserID, PropertyID: r.PropertyID,
	})
	if err != nil {
		return Alert{}, fmt.Errorf("activity recipients: %w", err)
	}
	alert := Alert{Activity: a, AccountID: r.AccountID}
	byUser := map[uuid.UUID]int{}
	var pushUsers []uuid.UUID
	for _, p := range people {
		if !a.Kind.VisibleTo(domain.Role(p.Role)) {
			continue
		}
		byUser[p.UserID] = len(alert.Recipients)
		alert.Recipients = append(alert.Recipients, Recipient{UserID: p.UserID, Phone: p.Phone, WhatsApp: p.Whatsapp})
		if p.Push {
			pushUsers = append(pushUsers, p.UserID)
		}
	}
	if len(pushUsers) == 0 {
		return alert, nil
	}
	tokens, err := q.LivePushTokens(ctx, queries.LivePushTokensParams{UserIds: pushUsers, Now: now})
	if err != nil {
		return Alert{}, fmt.Errorf("push tokens: %w", err)
	}
	for _, tok := range tokens {
		i := byUser[tok.UserID]
		alert.Recipients[i].PushTokens = append(alert.Recipients[i].PushTokens, tok.Token)
	}
	return alert, nil
}

// DeletePushTokens forgets tokens Expo says no longer reach a device.
func (s *Postgres) DeletePushTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	if err := s.db(ctx).DeletePushTokens(ctx, tokens); err != nil {
		return fmt.Errorf("delete push tokens: %w", err)
	}
	return nil
}

// SkipPendingAlerts marks the account's activity as already alerted, so sample data loaded by
// cmd/seed doesn't send the team a burst of alerts on the next notify run.
func (s *Postgres) SkipPendingAlerts(ctx context.Context, accountID uuid.UUID, now time.Time) error {
	if err := s.db(ctx).SkipPendingAlerts(ctx, queries.SkipPendingAlertsParams{Now: now, AccountID: accountID}); err != nil {
		return fmt.Errorf("skip pending alerts: %w", err)
	}
	return nil
}
