package markdown

import "strings"

// ChunkMarkdown splits markdown content into semantic chunks on double-newline
// boundaries. Short chunks (< 50 chars, typically headings) are merged into the
// following chunk. Very long chunks are split on single newlines.
func ChunkMarkdown(content string) []string {
	raw := strings.Split(content, "\n\n")

	// Merge short fragments (headings) into the next paragraph
	var merged []string
	var pending string
	for _, block := range raw {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		if pending != "" {
			block = pending + "\n\n" + block
			pending = ""
		}
		if len(block) < 50 {
			pending = block
			continue
		}
		merged = append(merged, block)
	}
	if pending != "" {
		if len(merged) > 0 {
			merged[len(merged)-1] += "\n\n" + pending
		} else {
			merged = append(merged, pending)
		}
	}

	// Split any chunk over 2000 chars on single newlines
	var result []string
	for _, chunk := range merged {
		if len(chunk) <= 2000 {
			result = append(result, chunk)
			continue
		}
		lines := strings.Split(chunk, "\n")
		var cur strings.Builder
		for _, line := range lines {
			if cur.Len()+len(line)+1 > 2000 && cur.Len() > 0 {
				result = append(result, cur.String())
				cur.Reset()
			}
			if cur.Len() > 0 {
				cur.WriteByte('\n')
			}
			cur.WriteString(line)
		}
		if cur.Len() > 0 {
			result = append(result, cur.String())
		}
	}

	return result
}
