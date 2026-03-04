package calendar

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// EventData holds the fields needed to generate calendar entries.
type EventData struct {
	ID          string
	Title       string
	Description string
	Location    string
	EventDate   time.Time
	EndDate     *time.Time
	Timezone    string
	URL         string // Public invite URL
}

// GenerateICS produces an RFC 5545 compliant ICS string for the event.
func GenerateICS(e EventData) string {
	now := time.Now().UTC()
	dtstamp := now.Format("20060102T150405Z")

	tz := e.Timezone
	if tz == "" {
		tz = "America/New_York"
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
		tz = "UTC"
	}

	// Convert event date to the event's timezone for display.
	eventStart := e.EventDate.In(loc)
	dtstart := eventStart.Format("20060102T150405")

	var eventEnd time.Time
	if e.EndDate != nil {
		eventEnd = e.EndDate.In(loc)
	} else {
		// Default to 2 hours duration if no end date.
		eventEnd = eventStart.Add(2 * time.Hour)
	}
	dtend := eventEnd.Format("20060102T150405")

	uid := e.ID + "@openrsvp"

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//OpenRSVP//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")

	// VTIMEZONE component.
	b.WriteString(generateVTIMEZONE(tz, loc, eventStart))

	b.WriteString("BEGIN:VEVENT\r\n")
	writeFoldedLine(&b, "UID:"+uid)
	writeFoldedLine(&b, "DTSTAMP:"+dtstamp)
	writeFoldedLine(&b, fmt.Sprintf("DTSTART;TZID=%s:%s", tz, dtstart))
	writeFoldedLine(&b, fmt.Sprintf("DTEND;TZID=%s:%s", tz, dtend))
	writeFoldedLine(&b, "SUMMARY:"+escapeICSText(e.Title))
	if e.Description != "" {
		writeFoldedLine(&b, "DESCRIPTION:"+escapeICSText(e.Description))
	}
	if e.Location != "" {
		writeFoldedLine(&b, "LOCATION:"+escapeICSText(e.Location))
	}
	if e.URL != "" {
		writeFoldedLine(&b, "URL:"+e.URL)
	}
	b.WriteString("STATUS:CONFIRMED\r\n")
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")

	return b.String()
}

// GoogleCalendarURL returns a URL that opens Google Calendar's event creation
// form pre-filled with the event details.
func GoogleCalendarURL(e EventData) string {
	start := e.EventDate.UTC().Format("20060102T150405Z")

	var end string
	if e.EndDate != nil {
		end = e.EndDate.UTC().Format("20060102T150405Z")
	} else {
		end = e.EventDate.Add(2 * time.Hour).UTC().Format("20060102T150405Z")
	}

	desc := e.Description
	if len(desc) > 1500 {
		desc = desc[:1500]
	}

	params := url.Values{}
	params.Set("action", "TEMPLATE")
	params.Set("text", e.Title)
	params.Set("dates", start+"/"+end)
	params.Set("location", e.Location)
	params.Set("details", desc)

	return "https://calendar.google.com/calendar/render?" + params.Encode()
}

// escapeICSText escapes special characters per RFC 5545 rules.
func escapeICSText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// writeFoldedLine writes a content line to the builder, folding at 75 octets
// per RFC 5545 section 3.1.
func writeFoldedLine(b *strings.Builder, line string) {
	const maxLineLen = 75

	if len(line) <= maxLineLen {
		b.WriteString(line)
		b.WriteString("\r\n")
		return
	}

	// First line: up to 75 octets.
	b.WriteString(line[:maxLineLen])
	b.WriteString("\r\n")
	line = line[maxLineLen:]

	// Continuation lines: space + up to 74 octets.
	for len(line) > 0 {
		chunkLen := maxLineLen - 1 // -1 for the leading space
		if chunkLen > len(line) {
			chunkLen = len(line)
		}
		b.WriteString(" ")
		b.WriteString(line[:chunkLen])
		b.WriteString("\r\n")
		line = line[chunkLen:]
	}
}

// generateVTIMEZONE generates a minimal VTIMEZONE component for the given
// timezone. It uses the current UTC offset to produce STANDARD and DAYLIGHT
// components.
func generateVTIMEZONE(tzName string, loc *time.Location, refTime time.Time) string {
	var b strings.Builder

	b.WriteString("BEGIN:VTIMEZONE\r\n")
	b.WriteString("TZID:" + tzName + "\r\n")

	// Determine if the timezone has DST by checking offsets at different
	// points in the year.
	jan := time.Date(refTime.Year(), time.January, 1, 12, 0, 0, 0, loc)
	jul := time.Date(refTime.Year(), time.July, 1, 12, 0, 0, 0, loc)

	_, janOffset := jan.Zone()
	julName, julOffset := jul.Zone()
	janName, _ := jan.Zone()

	if janOffset == julOffset {
		// No DST — single STANDARD component.
		b.WriteString("BEGIN:STANDARD\r\n")
		b.WriteString("DTSTART:19700101T000000\r\n")
		b.WriteString("TZOFFSETFROM:" + formatUTCOffset(janOffset) + "\r\n")
		b.WriteString("TZOFFSETTO:" + formatUTCOffset(janOffset) + "\r\n")
		if janName != "" {
			b.WriteString("TZNAME:" + janName + "\r\n")
		}
		b.WriteString("END:STANDARD\r\n")
	} else {
		// Has DST. In the northern hemisphere, summer (Jul) is usually DST.
		// In the southern hemisphere, winter (Jan) is usually DST.
		var stdOffset, dstOffset int
		var stdName, dstName string
		if julOffset > janOffset {
			// Northern hemisphere: Jan is STANDARD, Jul is DAYLIGHT.
			stdOffset = janOffset
			dstOffset = julOffset
			stdName = janName
			dstName = julName
		} else {
			// Southern hemisphere: Jul is STANDARD, Jan is DAYLIGHT.
			stdOffset = julOffset
			dstOffset = janOffset
			stdName = julName
			dstName = janName
		}

		// STANDARD component (typically Nov first Sunday in US).
		b.WriteString("BEGIN:STANDARD\r\n")
		b.WriteString("DTSTART:19701101T020000\r\n")
		b.WriteString("RRULE:FREQ=YEARLY;BYDAY=1SU;BYMONTH=11\r\n")
		b.WriteString("TZOFFSETFROM:" + formatUTCOffset(dstOffset) + "\r\n")
		b.WriteString("TZOFFSETTO:" + formatUTCOffset(stdOffset) + "\r\n")
		if stdName != "" {
			b.WriteString("TZNAME:" + stdName + "\r\n")
		}
		b.WriteString("END:STANDARD\r\n")

		// DAYLIGHT component (typically Mar second Sunday in US).
		b.WriteString("BEGIN:DAYLIGHT\r\n")
		b.WriteString("DTSTART:19700308T020000\r\n")
		b.WriteString("RRULE:FREQ=YEARLY;BYDAY=2SU;BYMONTH=3\r\n")
		b.WriteString("TZOFFSETFROM:" + formatUTCOffset(stdOffset) + "\r\n")
		b.WriteString("TZOFFSETTO:" + formatUTCOffset(dstOffset) + "\r\n")
		if dstName != "" {
			b.WriteString("TZNAME:" + dstName + "\r\n")
		}
		b.WriteString("END:DAYLIGHT\r\n")
	}

	b.WriteString("END:VTIMEZONE\r\n")
	return b.String()
}

// formatUTCOffset formats a UTC offset in seconds to the iCalendar format
// (+/-HHMM).
func formatUTCOffset(offsetSec int) string {
	sign := "+"
	if offsetSec < 0 {
		sign = "-"
		offsetSec = -offsetSec
	}
	hours := offsetSec / 3600
	minutes := (offsetSec % 3600) / 60
	return fmt.Sprintf("%s%02d%02d", sign, hours, minutes)
}
