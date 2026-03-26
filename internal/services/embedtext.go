package services

import (
	"encoding/json"
	"strings"
)

// BuildEmbedText constructs the text to embed from concept extractor output.
// If theoretical data is present: theoretical_territory + summary from engineering.
// Otherwise: concepts + summary from engineering.
// The choice to prefer theoretical territory is deliberate — it captures "what research
// space this lives near" which is what the gravitational model needs.
func BuildEmbedText(engineering, theoretical json.RawMessage) string {
	var summary string
	var topics []string

	if len(theoretical) > 0 && string(theoretical) != "null" {
		// Deep mode: use theoretical territory
		var theo struct {
			TheoreticalTerritory []string `json:"theoretical_territory"`
		}
		if err := json.Unmarshal(theoretical, &theo); err == nil {
			topics = theo.TheoreticalTerritory
		}
	}

	if len(engineering) > 0 && string(engineering) != "null" {
		var eng struct {
			Summary string   `json:"summary"`
			Concepts []string `json:"concepts"`
		}
		if err := json.Unmarshal(engineering, &eng); err == nil {
			summary = eng.Summary
			// If no theoretical territory, fall back to engineering concepts
			if len(topics) == 0 {
				topics = eng.Concepts
			}
		}
	}

	var parts []string
	if summary != "" {
		parts = append(parts, summary)
	}
	if len(topics) > 0 {
		parts = append(parts, strings.Join(topics, ", "))
	}

	return strings.Join(parts, "\n")
}

// TruncateForEmbed truncates text to maxChars characters for embedding.
// qwen3-embedding:8b has a 32k token context limit (~4 chars/token → ~128k chars).
// 24000 chars is the standard limit used across standing doc and entry embedding.
func TruncateForEmbed(text string, maxChars int) string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	return string(runes[:maxChars])
}
