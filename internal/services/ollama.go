package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saaga0h/journal/internal/config"
	"github.com/sirupsen/logrus"
)

// Ollama is a client for the Ollama embedding API.
type Ollama struct {
	config config.OllamaConfig
	client *http.Client
	logger *logrus.Logger
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
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
