package services

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const gitTimeFormat = "2006-01-02T15:04:05"

const systemPrompt = `You are a research concept extractor. Given git commit messages and diffs, extract the core scientific, mathematical, and engineering concepts present in the work.

Focus on:
- Algorithms and data structures
- Mathematical or theoretical foundations
- Domain concepts (distributed systems, ML, signal processing, etc.)
- Design patterns or architectural ideas
- Problems being solved at an abstract level

CRITICAL INSTRUCTION: Your response must be a single valid JSON object and nothing else.
Do not write markdown. Do not write explanations. Do not use headers or bullet points.
Start your response with { and end with }.

Required JSON structure:
{
  "concepts": ["concept1", "concept2", ...],
  "summary": "one sentence describing what this work is about at a conceptual level",
  "search_terms": ["term1", "term2", ...]
}

search_terms should be suitable for academic paper search — precise, technical, varied enough to cast a useful net.`

const deepPrompt = `You are a research theorist. Given engineering concepts from software work, identify the underlying theoretical and mathematical territory this work lives near.

Your task is NOT to describe what was built. Find the research space adjacent to it — papers that would inform or illuminate this work even if they share no implementation vocabulary with it.

Think about:
- What mathematical structures underlie these patterns?
- What theoretical CS problems are being approximated?
- What fields study the same problems under different names?
- What are the open research questions in this territory?

CRITICAL INSTRUCTION: Your response must be a single valid JSON object and nothing else.
Do not write markdown. Do not write explanations. Do not use headers or bullet points.
Start your response with { and end with }.

Required JSON structure:
{
  "theoretical_territory": ["area1", "area2", ...],
  "adjacent_fields": ["field1", "field2", ...],
  "arxiv_search_terms": ["term1", "term2", ...],
  "research_questions": ["question1", "question2", ...]
}

arxiv_search_terms must use researcher vocabulary, not engineer vocabulary. Prefer mathematical and theoretical formulations.`

// GetCommitDays returns the unique calendar dates (UTC) on which commits exist
// within the given time range. Used to enumerate active days for per-day extraction.
func GetCommitDays(repoPath string, since time.Time, until time.Time) ([]time.Time, error) {
	sinceStr := since.Format(gitTimeFormat)
	args := []string{"-C", repoPath, "log",
		fmt.Sprintf("--since=%s", sinceStr),
		"--no-color",
		"--pretty=format:%cd",
		"--date=format:%Y-%m-%d",
	}
	if !until.IsZero() {
		args = append(args, fmt.Sprintf("--before=%s", until.Format(gitTimeFormat)))
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log (days) failed: %w", err)
	}

	seen := make(map[string]bool)
	var days []time.Time
	for _, line := range splitLines(string(out)) {
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		t, err := time.ParseInLocation("2006-01-02", line, time.UTC)
		if err != nil {
			continue
		}
		days = append(days, t)
	}

	// Sort ascending so we process oldest day first
	for i := 0; i < len(days); i++ {
		for j := i + 1; j < len(days); j++ {
			if days[j].Before(days[i]) {
				days[i], days[j] = days[j], days[i]
			}
		}
	}
	return days, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// GetCommitMessages returns all commit messages in the time range, oldest first.
// If until is non-zero, only commits before that time are included.
func GetCommitMessages(repoPath string, since time.Time, until time.Time) (string, error) {
	sinceStr := since.Format(gitTimeFormat)
	args := []string{"-C", repoPath, "log",
		fmt.Sprintf("--since=%s", sinceStr),
		"--reverse",
		"--no-color",
		"--pretty=format:COMMIT %h %ad%n%s%n%b%n",
		"--date=short",
	}
	if !until.IsZero() {
		args = append(args, fmt.Sprintf("--before=%s", until.Format(gitTimeFormat)))
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git log (messages) failed: %w", err)
	}
	return string(out), nil
}

// GetNonTestDiff returns diffs of non-test, non-vendor files, oldest first.
// If until is non-zero, only commits before that time are included.
func GetNonTestDiff(repoPath string, since time.Time, until time.Time, maxBytes int) (string, error) {
	sinceStr := since.Format(gitTimeFormat)
	args := []string{"-C", repoPath, "log",
		fmt.Sprintf("--since=%s", sinceStr),
		"--reverse",
		"--no-color",
		"--pretty=format:COMMIT %h %s",
		"-p",
		"--",
		".",
		":(exclude)*_test.go",
		":(exclude)*_test.ts",
		":(exclude)*_test.py",
		":(exclude)vendor/",
		":(exclude)node_modules/",
		":(exclude)*.sum",
		":(exclude)*.lock",
	}
	if !until.IsZero() {
		// Insert --before before the -- path separator
		args = append(args[:9], append([]string{fmt.Sprintf("--before=%s", until.Format(gitTimeFormat))}, args[9:]...)...)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git log (diff) failed: %w", err)
	}
	s := string(out)
	if len(s) > maxBytes {
		s = s[:maxBytes] + "\n... (truncated)"
	}
	return s, nil
}

// ExtractConcepts runs the first-pass concept extraction via Ollama chat.
func ExtractConcepts(ollama *Ollama, model, gitContent string, numCtx int) (map[string]interface{}, error) {
	userMsg := fmt.Sprintf("Extract concepts from these git commits:\n\n%s", gitContent)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMsg},
	}

	raw, err := ollama.Chat(messages, model, numCtx)
	if err != nil {
		return nil, fmt.Errorf("concept extraction failed: %w", err)
	}

	content := StripMarkdownFences(raw)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse concepts JSON failed: %w\ncontent: %s", err, content)
	}

	return result, nil
}

// DeepExtract runs the second-pass theoretical territory extraction.
func DeepExtract(ollama *Ollama, model string, first map[string]interface{}, numCtx int) (map[string]interface{}, error) {
	conceptsRaw, _ := json.Marshal(first["concepts"])
	summaryStr, _ := first["summary"].(string)

	userMsg := fmt.Sprintf(
		"Summary of work: %s\n\nEngineering concepts extracted:\n%s\n\nWhat theoretical and research territory does this work live near?",
		summaryStr, string(conceptsRaw),
	)

	messages := []ChatMessage{
		{Role: "system", Content: deepPrompt},
		{Role: "user", Content: userMsg},
	}

	raw, err := ollama.Chat(messages, model, numCtx)
	if err != nil {
		return nil, fmt.Errorf("deep extraction failed: %w", err)
	}

	content := StripMarkdownFences(raw)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse deep JSON failed: %w\ncontent: %s", err, content)
	}

	return result, nil
}
