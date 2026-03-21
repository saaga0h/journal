package services

import (
	"testing"
	"time"
)

func TestParseDocumentDate(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		name    string
		content string
		want    time.Time
		found   bool
	}{
		{
			name:    "italic metadata line with ordinal",
			content: "*Emerged: March 20th, 2026. Source: conversation tracing connections.*",
			want:    time.Date(2026, 3, 20, 0, 0, 0, 0, utc),
			found:   true,
		},
		{
			name:    "italic metadata month only",
			content: "Some content\n\n*Emerged: March 2026. Source: something.*",
			want:    time.Date(2026, 3, 1, 0, 0, 0, 0, utc),
			found:   true,
		},
		{
			name:    "DD.MM.YYYY",
			content: "Date of writing: 20.02.2026",
			want:    time.Date(2026, 2, 20, 0, 0, 0, 0, utc),
			found:   true,
		},
		{
			name:    "ISO YYYY-MM-DD",
			content: "Created: 2026-03-15",
			want:    time.Date(2026, 3, 15, 0, 0, 0, 0, utc),
			found:   true,
		},
		{
			name:    "Month Day Year no ordinal",
			content: "Written on March 20, 2026 in a notebook.",
			want:    time.Date(2026, 3, 20, 0, 0, 0, 0, utc),
			found:   true,
		},
		{
			name:    "Month Day Year with ordinal",
			content: "# Some Title\n\nMarch 1st, 2026 — started thinking about this.",
			want:    time.Date(2026, 3, 1, 0, 0, 0, 0, utc),
			found:   true,
		},
		{
			name:    "Month Year only defaults to 1st",
			content: "February 2026 was a strange month.",
			want:    time.Date(2026, 2, 1, 0, 0, 0, 0, utc),
			found:   true,
		},
		{
			name:    "date at bottom of document",
			content: "# My Standing Doc\n\nLots of content here.\n\nMore paragraphs.\n\n*Emerged: January 5th, 2026.*",
			want:    time.Date(2026, 1, 5, 0, 0, 0, 0, utc),
			found:   true,
		},
		{
			name:    "no date in content",
			content: "# Just a title\n\nNo date information anywhere in this document.",
			want:    time.Time{},
			found:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := ParseDocumentDate(tc.content)
			if found != tc.found {
				t.Fatalf("found=%v, want %v", found, tc.found)
			}
			if found && !got.Equal(tc.want) {
				t.Errorf("date=%v, want %v", got, tc.want)
			}
		})
	}
}
