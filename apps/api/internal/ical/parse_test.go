package ical

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

var (
	colombo, _ = time.LoadLocation("Asia/Colombo")
	today      = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
)

func parseFixture(t *testing.T, name string) []string {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := Parse(f, colombo, today)
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	got := make([]string, len(events))
	for i, e := range events {
		got[i] = e.UID + " " + e.Start.Format(time.DateOnly) + " " + e.End.Format(time.DateOnly)
	}
	return got
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []string
	}{
		{"airbnb stays and closed nights, past ones left out", "airbnb.ics", []string{
			"1418fb94e984-f2a86c6e1ae6e8ff7f4c4a5a1ddc3e47@airbnb.com 2026-11-01 2026-11-05",
			"9c0d2e1a7b3f-5be1f0a0c1d2e3f4a5b6c7d8e9f0a1b2@airbnb.com 2026-11-18 2026-11-20",
		}},
		{"booking.com closed nights", "booking.ics", []string{
			"a1b2c3d4e5f60718293a4b5c6d7e8f90@booking.com 2026-12-01 2026-12-04",
		}},
		{"date-times land on the property's days", "datetime.ics", []string{
			"expedia-77001 2026-11-10 2026-11-12",
			"agoda-55002 2026-11-20 2026-11-22",
		}},
		{"own exports and cancelled events skipped, a missing end is one night", "echo.ics", []string{
			"one-night 2026-11-15 2026-11-16",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseFixture(t, tt.fixture); !slices.Equal(got, tt.want) {
				t.Errorf("events = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseReadsFoldedLines(t *testing.T) {
	f, err := os.Open("testdata/airbnb.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := Parse(f, colombo, today)
	if err != nil {
		t.Fatal(err)
	}
	if events[0].Summary != "Reserved" {
		t.Errorf("summary = %q", events[0].Summary)
	}
}

func TestParseClipsToTheHorizon(t *testing.T) {
	feed := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:long\r\nDTSTART;VALUE=DATE:20281001\r\nDTEND;VALUE=DATE:20290101\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	events, err := Parse(strings.NewReader(feed), colombo, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].End.Equal(time.Date(2028, 10, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("events = %+v, want one ending at the horizon", events)
	}
}

func TestParseRefusesSomethingElse(t *testing.T) {
	if _, err := Parse(strings.NewReader("<html>Sign in</html>"), colombo, today); err == nil {
		t.Error("an HTML page parsed as a calendar")
	}
}
