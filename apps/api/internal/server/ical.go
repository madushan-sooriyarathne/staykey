package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/ical"
	"staykey.direct/api/internal/oapi"
)

// GetIcalExport serves a property's calendar for OTAs to import: every taken night, with nothing
// about guests. The file name is the slug, a dash and the export token.
func (s *Server) GetIcalExport(ctx context.Context, req oapi.GetIcalExportRequestObject) (oapi.GetIcalExportResponseObject, error) {
	notFound := oapi.GetIcalExport404JSONResponse{NotFoundJSONResponse: oapi.NotFoundJSONResponse(
		errBody("not_found", "There is no calendar at this address."))}
	name, ok := strings.CutSuffix(req.File, ".ics")
	i := strings.LastIndex(name, "-")
	if !ok || i < 1 {
		return notFound, nil
	}
	// Stays that end today still list, so a channel never sees tonight's guest disappear.
	from := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	title, runs, err := s.store.ExportCalendar(ctx, name[:i], name[i+1:], req.Params.Unit, from)
	if errors.Is(err, domain.ErrNotFound) {
		return notFound, nil
	}
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := ical.Write(&buf, title, runs, time.Now()); err != nil {
		return nil, err
	}
	return icalResponse(buf.Bytes()), nil
}

// icalResponse writes the calendar with the headers the generated response leaves out.
type icalResponse []byte

func (r icalResponse) VisitGetIcalExportResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Content-Length", strconv.Itoa(len(r)))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r)
	return err
}

// SyncIcalFeed has the import job read a feed within a minute, for "Sync now" in the app.
func (s *Server) SyncIcalFeed(ctx context.Context, req oapi.SyncIcalFeedRequestObject) (oapi.SyncIcalFeedResponseObject, error) {
	t, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !t.Can(domain.PermSettings) {
		return nil, errForbidden
	}
	f, err := s.store.MarkFeedDue(ctx, t, req.FeedId)
	if err != nil {
		return nil, apiErr(err, "calendar")
	}
	return oapi.SyncIcalFeed200JSONResponse(feedToAPI(f)), nil
}
