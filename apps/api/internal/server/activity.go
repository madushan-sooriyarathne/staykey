package server

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/store"
)

// ListActivity returns a page of the account's activity as the caller may see it.
func (s *Server) ListActivity(ctx context.Context, req oapi.ListActivityRequestObject) (oapi.ListActivityResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	limit := 50
	if req.Params.Limit != nil {
		limit = min(max(*req.Params.Limit, 1), 100)
	}
	var after *store.ActivityCursor
	if req.Params.Cursor != nil {
		if after, err = decodeActivityCursor(*req.Params.Cursor); err != nil {
			return oapi.ListActivity400JSONResponse{BadRequestJSONResponse: oapi.BadRequestJSONResponse(
				fieldErr("invalid_cursor", "cursor", "Send back the nextCursor from the previous page."))}, nil
		}
	}
	items, err := s.store.ListActivity(ctx, t, after, limit+1)
	if err != nil {
		return nil, err
	}
	unread, err := s.store.UnreadActivity(ctx, t)
	if err != nil {
		return nil, err
	}
	out := oapi.ActivityPage{Items: make([]oapi.Activity, 0, min(len(items), limit)), Unread: unread}
	for i, a := range items {
		if i == limit {
			cursor := encodeActivityCursor(items[i-1])
			out.NextCursor = &cursor
			break
		}
		out.Items = append(out.Items, oapi.Activity{
			Id: a.ID, Kind: oapi.ActivityKind(a.Kind), Title: a.Title, Subtitle: a.Subtitle,
			At: a.CreatedAt, Read: a.Read, PropertyId: a.PropertyID, BookingId: a.BookingID,
		})
	}
	return oapi.ListActivity200JSONResponse(out), nil
}

// MarkActivityRead marks everything up to now as read for the caller.
func (s *Server) MarkActivityRead(ctx context.Context, _ oapi.MarkActivityReadRequestObject) (oapi.MarkActivityReadResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.MarkActivityRead(ctx, t, time.Now()); err != nil {
		return nil, err
	}
	return oapi.MarkActivityRead204Response{}, nil
}

func encodeActivityCursor(a domain.Activity) string {
	return base64.RawURLEncoding.EncodeToString([]byte(a.CreatedAt.Format(time.RFC3339Nano) + "_" + a.ID.String()))
}

func decodeActivityCursor(c string) (*store.ActivityCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return nil, err
	}
	at, id, ok := strings.Cut(string(raw), "_")
	if !ok {
		return nil, errors.New("malformed cursor")
	}
	when, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return nil, err
	}
	after, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	return &store.ActivityCursor{At: when, ID: after}, nil
}
