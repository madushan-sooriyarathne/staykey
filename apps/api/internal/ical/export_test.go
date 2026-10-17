package ical

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

func TestWriteRoundTripsThroughParse(t *testing.T) {
	runs := []domain.ExportRun{
		{ID: uuid.New(), Start: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)},
	}
	var b strings.Builder
	if err := Write(&b, "Kingfisher Villa, Galle", runs, today); err != nil {
		t.Fatal(err)
	}
	// Parse skips StayKey's own events, which is what keeps channels from echoing them back.
	events, err := Parse(strings.NewReader(b.String()), colombo, today)
	if err != nil || len(events) != 0 {
		t.Errorf("Parse of the export = %v, %v; want no events and no error", events, err)
	}
}

func TestWriteTitlesBlocksNotAvailable(t *testing.T) {
	runs := []domain.ExportRun{{ID: uuid.New(), Block: true, Start: today, End: today.AddDate(0, 0, 2)}}
	var b strings.Builder
	if err := Write(&b, "Villa", runs, today); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "SUMMARY:Not available\r\n") {
		t.Errorf("export = %q", b.String())
	}
}

func TestWriteEscapesTheCalendarName(t *testing.T) {
	var b strings.Builder
	if err := Write(&b, "Villa; Galle, Lanka", nil, today); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `X-WR-CALNAME:Villa\; Galle\, Lanka`) {
		t.Errorf("export = %q", b.String())
	}
}

func TestFoldKeepsLinesShort(t *testing.T) {
	for line := range strings.SplitSeq(fold(strings.Repeat("ක", 60)), "\r\n") {
		if len(line) > 75 {
			t.Errorf("line of %d octets", len(line))
		}
	}
}
