// Package ical reads OTA calendars and writes the calendar StayKey exports for them.
package ical

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	goical "github.com/emersion/go-ical"
)

// Event is one stay or closed stretch read from a feed. Start and End are local calendar days at
// UTC midnight, End exclusive like a check-out date.
type Event struct {
	UID     string
	Summary string
	Start   time.Time
	End     time.Time
}

// ExportUIDSuffix ends the UID of every event StayKey exports. Imports skip those UIDs, so a
// channel that echoes StayKey's calendar back never makes a stay twice.
const ExportUIDSuffix = "@staykey.direct"

// Horizon is how far ahead imports look. Nothing on a villa's calendar is sold further out.
const Horizon = 24 // months

// ErrNotCalendar means the link answered with something that isn't an iCalendar file.
var ErrNotCalendar = errors.New("ical: not a calendar")

// Parse reads a feed's events in the property's time zone. It leaves out cancelled events,
// events that ended before today, StayKey's own exported events and anything past the horizon.
// today is the property's local date at UTC midnight.
func Parse(r io.Reader, loc *time.Location, today time.Time) ([]Event, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("ical: read: %w", err)
	}
	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))
	cal, err := goical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotCalendar, err)
	}

	horizon := today.AddDate(0, Horizon, 0)
	seen := map[string]bool{}
	var events []Event
	for _, e := range cal.Events() {
		uid := strings.TrimSpace(propText(e.Props, goical.PropUID))
		if uid == "" || seen[uid] || strings.HasSuffix(uid, ExportUIDSuffix) {
			continue
		}
		if strings.EqualFold(propText(e.Props, goical.PropStatus), string(goical.EventCancelled)) {
			continue
		}
		start, end, err := stayDates(e, loc)
		if err != nil {
			return nil, fmt.Errorf("ical: event %s: %w", uid, err)
		}
		if !end.After(today) || !start.Before(horizon) {
			continue
		}
		if end.After(horizon) {
			end = horizon
		}
		seen[uid] = true
		events = append(events, Event{UID: uid, Summary: propText(e.Props, goical.PropSummary), Start: start, End: end})
	}
	return events, nil
}

// stayDates turns an event's start and end into local calendar days. A date-time is read in its
// own zone and converted to the property's; a missing end means one night.
func stayDates(e goical.Event, loc *time.Location) (time.Time, time.Time, error) {
	startProp := e.Props.Get(goical.PropDateTimeStart)
	if startProp == nil {
		return time.Time{}, time.Time{}, errors.New("no DTSTART")
	}
	start, err := localDay(startProp, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end := start.AddDate(0, 0, 1)
	if endProp := e.Props.Get(goical.PropDateTimeEnd); endProp != nil {
		if end, err = localDay(endProp, loc); err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else if durProp := e.Props.Get(goical.PropDuration); durProp != nil {
		dur, err := durProp.Duration()
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end = start.Add(dur.Truncate(24 * time.Hour))
	}
	if !end.After(start) {
		end = start.AddDate(0, 0, 1)
	}
	return start, end, nil
}

// localDay reads a DATE or DATE-TIME as a calendar day in loc. A TZID Go doesn't know, such as a
// Windows zone name, is read as the property's zone rather than failing the whole feed.
func localDay(p *goical.Prop, loc *time.Location) (time.Time, error) {
	t, err := p.DateTime(loc)
	if err != nil && p.Params.Get(goical.PropTimezoneID) != "" {
		bare := *p
		bare.Params = goical.Params{}
		t, err = bare.DateTime(loc)
	}
	if err != nil {
		return time.Time{}, err
	}
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}

func propText(props goical.Props, name string) string {
	p := props.Get(name)
	if p == nil {
		return ""
	}
	s, err := p.Text()
	if err != nil {
		return p.Value
	}
	return s
}
