package ical

import (
	"fmt"
	"io"
	"strings"
	"time"

	"staykey.direct/api/internal/domain"
)

// Write writes the export calendar: one all-day event per run of held nights, titled "Booked" or
// "Not available", with nothing about the guest.
func Write(w io.Writer, calendarName string, runs []domain.ExportRun, now time.Time) error {
	var b strings.Builder
	line := func(s string) { b.WriteString(fold(s)) }
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//StayKey//Calendar//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:" + escapeText(calendarName))
	stamp := now.UTC().Format("20060102T150405Z")
	for _, r := range runs {
		summary := "Booked"
		if r.Block {
			summary = "Not available"
		}
		line("BEGIN:VEVENT")
		line(fmt.Sprintf("UID:%s-%s%s", r.ID, r.Start.Format("20060102"), ExportUIDSuffix))
		line("DTSTAMP:" + stamp)
		line("DTSTART;VALUE=DATE:" + r.Start.Format("20060102"))
		line("DTEND;VALUE=DATE:" + r.End.Format("20060102"))
		line("SUMMARY:" + summary)
		line("TRANSP:OPAQUE")
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	_, err := io.WriteString(w, b.String())
	return err
}

var textEscaper = strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`)

func escapeText(s string) string { return textEscaper.Replace(s) }

// fold ends a content line with CRLF, wrapping it at 75 octets without splitting a character.
func fold(s string) string {
	var b strings.Builder
	width := 0
	for _, r := range s {
		n := len(string(r))
		if width+n > 75 {
			b.WriteString("\r\n ")
			width = 1
		}
		b.WriteRune(r)
		width += n
	}
	b.WriteString("\r\n")
	return b.String()
}
