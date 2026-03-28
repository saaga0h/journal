package markdown

import (
	"strings"
	"testing"
)

func TestChunkMarkdown(t *testing.T) {
	longPara := strings.Repeat("This is a fairly long sentence that will exceed fifty characters. ", 2)

	cases := []struct {
		name      string
		input     string
		wantCount int          // -1 means "at least 1"
		check     func(t *testing.T, chunks []string)
	}{
		{
			name:      "empty string",
			input:     "",
			wantCount: 0,
			check:     nil,
		},
		{
			name:      "single paragraph",
			input:     longPara,
			wantCount: 1,
			check: func(t *testing.T, chunks []string) {
				if strings.TrimSpace(chunks[0]) != strings.TrimSpace(longPara) {
					t.Errorf("chunk content mismatch")
				}
			},
		},
		{
			name:  "short heading merged into next paragraph",
			input: "# Heading\n\n" + longPara,
			wantCount: 1,
			check: func(t *testing.T, chunks []string) {
				if !strings.Contains(chunks[0], "# Heading") {
					t.Error("expected heading to be merged into chunk")
				}
				if !strings.Contains(chunks[0], "This is a fairly") {
					t.Error("expected paragraph content in merged chunk")
				}
			},
		},
		{
			name:      "trailing short chunk appended to last",
			input:     longPara + "\n\nShort",
			wantCount: 1,
			check: func(t *testing.T, chunks []string) {
				if !strings.Contains(chunks[0], "Short") {
					t.Error("expected trailing short block to be appended to last chunk")
				}
			},
		},
		{
			name:  "long chunk split on single newlines",
			// 25 lines of 101 chars each = 2525 chars total, must split
			input: strings.Repeat(strings.Repeat("x", 100)+"\n", 25),
			wantCount: -1,
			check: func(t *testing.T, chunks []string) {
				if len(chunks) < 2 {
					t.Errorf("expected multiple chunks for >2000 char input, got %d", len(chunks))
				}
				for i, c := range chunks {
					if len(c) > 2000 {
						t.Errorf("chunk %d exceeds 2000 chars: %d", i, len(c))
					}
				}
			},
		},
		{
			name:      "multiple normal paragraphs",
			input:     longPara + "\n\n" + longPara + "\n\n" + longPara,
			wantCount: 3,
			check:     nil,
		},
		{
			name:      "multiple blank lines treated as single separator",
			input:     longPara + "\n\n\n\n" + longPara,
			wantCount: 2,
			check:     nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ChunkMarkdown(tc.input)
			if tc.wantCount >= 0 && len(got) != tc.wantCount {
				t.Errorf("want %d chunks, got %d", tc.wantCount, len(got))
			}
			if tc.wantCount == -1 && len(got) == 0 {
				t.Error("want at least one chunk, got none")
			}
			if tc.check != nil && len(got) > 0 {
				tc.check(t, got)
			}
		})
	}
}
