package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Model represents an AI model
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ModelList represents a list of models in OpenAI format
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// ModelManager manages model list with caching
type ModelManager struct {
	mu              sync.RWMutex
	models          []Model
	lastRefresh     time.Time
	apiBase         string
	useProvider     bool
	refreshInterval time.Duration
	httpClient      *http.Client
	logger          Logger
}

// Logger interface for model manager logging
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
}

// NewManager creates a new model manager
func NewManager(apiBase string, useProvider bool, refreshInterval time.Duration, logger Logger) *ModelManager {
	return &ModelManager{
		apiBase:         apiBase,
		useProvider:     useProvider,
		refreshInterval: refreshInterval,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GetModels returns the list of models, fetching from Provider API if enabled
func (m *ModelManager) GetModels(ctx context.Context, ccAPIKey string) ([]Model, error) {
	m.mu.RLock()
	needsRefresh := time.Since(m.lastRefresh) > m.refreshInterval
	cached := m.models
	m.mu.RUnlock()

	if !needsRefresh && len(cached) > 0 {
		return cached, nil
	}

	// Fetch fresh models
	var models []Model
	var err error

	if m.useProvider {
		models, err = m.fetchFromProvider(ctx, ccAPIKey)
		if err != nil {
			m.logger.Warn("Failed to fetch models from Provider API, using fallback", map[string]interface{}{
				"error": err.Error(),
			})
			models = GetHardcodedModels()
		}
	} else {
		models = GetHardcodedModels()
	}

	// Update cache
	m.mu.Lock()
	m.models = models
	m.lastRefresh = time.Now()
	m.mu.Unlock()

	return models, nil
}

// fetchFromProvider fetches models from the Provider API
func (m *ModelManager) fetchFromProvider(ctx context.Context, ccAPIKey string) ([]Model, error) {
	url := fmt.Sprintf("%s/provider/v1/models", m.apiBase)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+ccAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-cli-environment", "production")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Provider API returned status %d", resp.StatusCode)
	}

	var modelList ModelList
	if err := json.NewDecoder(resp.Body).Decode(&modelList); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return modelList.Data, nil
}

// GetHardcodedModels returns the fallback list of models
func GetHardcodedModels() []Model {
	return []Model{
		{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6"},
		{ID: "claude-opus-4-8", Name: "Claude Opus 4.8"},
		{ID: "claude-opus-4-7", Name: "Claude Opus 4.7"},
		{ID: "claude-opus-4-6", Name: "Claude Opus 4.6"},
		{ID: "claude-opus-4-5", Name: "Claude Opus 4.5"},
		{ID: "claude-opus-4-4", Name: "Claude Opus 4.4"},
		{ID: "claude-opus-4-3", Name: "Claude Opus 4.3"},
		{ID: "claude-opus-4-2", Name: "Claude Opus 4.2"},
		{ID: "claude-opus-4-1", Name: "Claude Opus 4.1"},
		{ID: "claude-opus-4-0", Name: "Claude Opus 4.0"},
		{ID: "claude-opus-3-7", Name: "Claude Opus 3.7"},
		{ID: "claude-opus-3-6", Name: "Claude Opus 3.6"},
		{ID: "claude-opus-3-5", Name: "Claude Opus 3.5"},
		{ID: "claude-opus-3-4", Name: "Claude Opus 3.4"},
		{ID: "claude-opus-3-3", Name: "Claude Opus 3.3"},
		{ID: "claude-opus-3-2", Name: "Claude Opus 3.2"},
		{ID: "claude-opus-3-1", Name: "Claude Opus 3.1"},
		{ID: "claude-opus-3-0", Name: "Claude Opus 3.0"},
		{ID: "claude-opus-2-0", Name: "Claude Opus 2.0"},
		{ID: "claude-opus-1-0", Name: "Claude Opus 1.0"},
		{ID: "deepseek/deepseek-v4-pro", Name: "DeepSeek V4 Pro"},
		{ID: "deepseek/deepseek-v4-flash", Name: "DeepSeek V4 Flash"},
		{ID: "moonshotai/Kimi-K2.6", Name: "Kimi K2.6"},
		{ID: "moonshotai/Kimi-K2.5", Name: "Kimi K2.5"},
		{ID: "zai-org/GLM-5.1", Name: "GLM 5.1"},
		{ID: "zai-org/GLM-5", Name: "GLM 5"},
		{ID: "MiniMaxAI/MiniMax-M3", Name: "MiniMax M3"},
		{ID: "MiniMaxAI/MiniMax-M2.7", Name: "MiniMax M2.7"},
		{ID: "MiniMaxAI/MiniMax-M2.5", Name: "MiniMax M2.5"},
		{ID: "Qwen/Qwen3.6-Max-Preview", Name: "Qwen 3.6 Max Preview"},
		{ID: "Qwen/Qwen3.6-Plus", Name: "Qwen 3.6 Plus"},
		{ID: "Qwen/Qwen3.7-Max", Name: "Qwen 3.7 Max"},
		{ID: "stepfun/Step-3.7-Flash", Name: "Step 3.7 Flash"},
		{ID: "stepfun/Step-3.5-Flash", Name: "Step 3.5 Flash"},
		{ID: "xiaomi/mimo-v2.5-pro", Name: "MiMo V2.5 Pro"},
		{ID: "xiaomi/mimo-v2.5", Name: "MiMo V2.5"},
		{ID: "google/gemini-3.5-flash", Name: "Gemini 3.5 Flash"},
		{ID: "google/gemini-3.1-flash-lite", Name: "Gemini 3.1 Flash Lite"},
	}
}

// StartRefresh starts a background goroutine to refresh models periodically
func (m *ModelManager) StartRefresh(ctx context.Context, ccAPIKey string) {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := m.GetModels(ctx, ccAPIKey)
			if err != nil {
				m.logger.Error("Failed to refresh models", map[string]interface{}{
					"error": err.Error(),
				})
			}
		case <-ctx.Done():
			return
		}
	}
}
