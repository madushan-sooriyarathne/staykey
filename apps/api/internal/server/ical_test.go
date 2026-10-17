package server

import (
	"net/http"
	"strings"
	"testing"

	"staykey.direct/api/internal/oapi"
)

func exportPath(p oapi.Property) string {
	return "/ical/" + p.Slug + "-" + p.IcalExportToken + ".ics"
}

func TestTheExportListsStaysAndBlocksWithoutGuests(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000001")
	e.createBooking(v.owner, v.account, v.p, future(10), future(12))
	rec := e.do(call{method: http.MethodPost, path: "/v1/blocks", as: v.owner, account: v.account, body: map[string]any{
		"propertyId": v.p.Id, "unitIds": []any{v.p.Units[0].Id}, "from": future(20), "to": future(22), "reason": "owner",
	}})
	e.expect(rec, http.StatusCreated)

	rec = e.do(call{method: http.MethodGet, path: exportPath(v.p)})
	e.expect(rec, http.StatusOK)
	body := rec.Body.String()
	if !strings.Contains(body, "SUMMARY:Booked") || !strings.Contains(body, "SUMMARY:Not available") || strings.Contains(body, "Nimali") {
		t.Errorf("export = %s", body)
	}
}

func TestTheExportIsACalendar(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000002")
	rec := e.do(call{method: http.MethodGet, path: exportPath(v.p)})
	e.expect(rec, http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "text/calendar; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestTheExportRefusesAWrongToken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000003")
	rec := e.do(call{method: http.MethodGet, path: "/ical/" + v.p.Slug + "-guess.ics"})
	e.expect(rec, http.StatusNotFound)
}

func TestARoomsExportIncludesAWholeHouseStay(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000004")
	rec := e.do(call{method: http.MethodPatch, path: "/v1/properties/" + v.p.Id.String(), as: v.owner, account: v.account, body: map[string]any{
		"units": []any{
			map[string]any{"id": v.p.Units[0].Id, "name": "Whole villa", "sleeps": 6, "rate": 30000, "linkedUnitIds": []any{"r1", "r2"}},
			map[string]any{"id": "r1", "name": "Room 1", "sleeps": 2, "rate": 12000},
			map[string]any{"id": "r2", "name": "Room 2", "sleeps": 2, "rate": 12000},
		},
	}})
	e.expect(rec, http.StatusOK)
	v.p = decode[oapi.Property](t, rec)
	e.createBooking(v.owner, v.account, v.p, future(10), future(12))
	var room string
	for _, u := range v.p.Units {
		if u.Name == "Room 2" {
			room = u.Id.String()
		}
	}

	rec = e.do(call{method: http.MethodGet, path: exportPath(v.p) + "?unit=" + room})
	e.expect(rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), "SUMMARY:Booked") {
		t.Errorf("room 2's export = %s, want the whole-villa stay", rec.Body)
	}
}

func TestSyncNowNeedsALink(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.villa("+94775000005")
	rec := e.do(call{method: http.MethodPatch, path: "/v1/properties/" + v.p.Id.String(), as: v.owner, account: v.account, body: map[string]any{
		"icalFeeds": []any{map[string]any{"channel": "airbnb", "url": ""}},
	}})
	e.expect(rec, http.StatusOK)
	feed := decode[oapi.Property](t, rec).IcalFeeds[0]
	rec = e.do(call{method: http.MethodPost, path: "/v1/ical-feeds/" + feed.Id.String() + "/sync", as: v.owner, account: v.account})
	e.expect(rec, http.StatusBadRequest)
}
