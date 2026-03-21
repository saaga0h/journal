package services

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var monthNames = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June,
	"july": time.July, "august": time.August, "september": time.September,
	"october": time.October, "november": time.November, "december": time.December,
}

var (
	// *Emerged: March 20th, 2026... or similar italic metadata lines
	reEmerged = regexp.MustCompile(`(?i)\*[^*]*?([A-Za-z]+ \d{1,2}(?:st|nd|rd|th)?,? \d{4})[^*]*\*`)

	// DD.MM.YYYY
	reDMY = regexp.MustCompile(`\b(\d{1,2})\.(\d{1,2})\.(\d{4})\b`)

	// YYYY-MM-DD (ISO)
	reISO = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)

	// Month Day, Year — "March 20th, 2026" or "March 20, 2026"
	reMonthDayYear = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2})(?:st|nd|rd|th)?,?\s+(\d{4})\b`)

	// Month Year only — "March 2026"
	reMonthYear = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{4})\b`)
)

// ParseDocumentDate scans the full content of a markdown document for date information.
// Returns the parsed date and true if found, or zero time and false if no date found.
// Patterns tried in order: italic metadata line, DD.MM.YYYY, YYYY-MM-DD, Month Day Year, Month Year.
func ParseDocumentDate(content string) (time.Time, bool) {
	// 1. Italic metadata line e.g. *Emerged: March 20th, 2026. Source: ...*
	if m := reEmerged.FindStringSubmatch(content); m != nil {
		if t, ok := parseMonthDayYear(m[1]); ok {
			return t, true
		}
	}

	// 2. DD.MM.YYYY
	if m := reDMY.FindStringSubmatch(content); m != nil {
		day, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		year, _ := strconv.Atoi(m[3])
		if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), true
		}
	}

	// 3. YYYY-MM-DD (ISO)
	if m := reISO.FindStringSubmatch(content); m != nil {
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), true
		}
	}

	// 4. Month Day Year
	if m := reMonthDayYear.FindStringSubmatch(content); m != nil {
		if t, ok := parseMonthDayYear(m[0]); ok {
			return t, true
		}
	}

	// 5. Month Year only → 1st of month
	if m := reMonthYear.FindStringSubmatch(content); m != nil {
		month := monthNames[strings.ToLower(m[1])]
		year, _ := strconv.Atoi(m[2])
		if month != 0 && year > 0 {
			return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC), true
		}
	}

	return time.Time{}, false
}

// parseMonthDayYear parses strings like "March 20th, 2026" or "March 20, 2026".
func parseMonthDayYear(s string) (time.Time, bool) {
	m := reMonthDayYear.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	month := monthNames[strings.ToLower(m[1])]
	day, _ := strconv.Atoi(m[2])
	year, _ := strconv.Atoi(m[3])
	if month == 0 || day == 0 || year == 0 {
		return time.Time{}, false
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), true
}
