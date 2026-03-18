package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"strings"

	"github.com/saaga0h/journal/internal/config"
	"github.com/sirupsen/logrus"
)

// Ollama is a client for the Ollama embedding and chat APIs.
type Ollama struct {
	config config.OllamaConfig
	client *http.Client
	logger *logrus.Logger
}

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  chatOptions   `json:"options"`
}

type chatOptions struct {
	NumCtx int `json:"num_ctx"`
}

type chatResponse struct {
	Message ChatMessage `json:"message"`
}

// NewOllama creates a new Ollama client.
func NewOllama(cfg config.OllamaConfig) *Ollama {
	return &Ollama{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
		logger: logrus.New(),
	}
}

// SetLogger replaces the default logger.
func (o *Ollama) SetLogger(logger *logrus.Logger) {
	o.logger = logger
}

// Embed computes an embedding vector for the given text using the configured model.
func (o *Ollama) Embed(text string) ([]float32, error) {
	reqBody := embedRequest{
		Model: o.config.EmbedModel,
		Input: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	req, err := http.NewRequest("POST", o.config.BaseURL+"/api/embed", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embed returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read embed response: %w", err)
	}

	var embedResp embedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("failed to parse embed response: %w", err)
	}

	if len(embedResp.Embeddings) == 0 {
		return nil, fmt.Errorf("embed response contained no embeddings")
	}

	embedding := embedResp.Embeddings[0]

	o.logger.WithFields(logrus.Fields{
		"model":      o.config.EmbedModel,
		"dimensions": len(embedding),
		"input_len":  len(text),
	}).Debug("Computed embedding")

	return embedding, nil
}

// Chat sends a chat completion request to Ollama and returns the response content.
func (o *Ollama) Chat(messages []ChatMessage, model string, numCtx int) (string, error) {
	reqBody := chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options:  chatOptions{NumCtx: numCtx},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	req, err := http.NewRequest("POST", o.config.BaseURL+"/api/chat", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read chat response: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse chat response: %w\nraw: %s", err, string(body))
	}

	content := strings.TrimSpace(chatResp.Message.Content)

	o.logger.WithFields(logrus.Fields{
		"model":      model,
		"num_ctx":    numCtx,
		"output_len": len(content),
	}).Debug("Chat completion")

	return content, nil
}

// stripMarkdownFences removes markdown code fences from a string.
func StripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
