package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kilimcininkoroglu/commandcode-bridge/internal/config"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/fingerprint"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/logging"
	"github.com/kilimcininkoroglu/commandcode-bridge/pkg/version"
)

// Client represents an HTTP client for CommandCode API
type Client struct {
	httpClient  *http.Client
	apiBase     string
	projectSlug string
	logger      *logging.Logger
}

// New creates a new CommandCode API client
func New(apiBase string, projectSlug string, logger *logging.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		apiBase:     apiBase,
		projectSlug: projectSlug,
		logger:      logger,
	}
}

// Forward forwards a request to the CommandCode API
func (c *Client) Forward(ctx context.Context, body []byte, cc_apiKey string, headers http.Header, sessionID string, fp *config.Fingerprint) (*http.Response, error) {
	url := fmt.Sprintf("%s/alpha/generate", c.apiBase)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		c.logger.Error("Failed to create upstream request", map[string]any{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+cc_apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-cli-environment", "production")
	req.Header.Set("x-command-code-version", version.GetCommandCodeVersion())
	req.Header.Set("x-project-slug", c.projectSlugForSession(sessionID))
	req.Header.Set("x-session-id", sessionID)
	req.Header.Set("x-co-flag", "false")
	req.Header.Set("x-taste-learning", "false")
	req.Header.Set("traceparent", generateTraceparent())
	req.Header.Set("User-Agent", "CommandCode/"+version.GetCommandCodeVersion())

	// Set fingerprint headers if available
	if fp != nil {
		req.Header.Set("x-thumbmark", fp.Thumbmark)
		req.Header.Set("x-machine-id-hash", fp.Components.MachineIdHash)
		req.Header.Set("x-os-user-hash", fp.Components.OsUserHash)
		req.Header.Set("x-hostname-hash", fp.Components.HostnameHash)
		req.Header.Set("x-git-email-hash", fp.Components.GitEmailHash)
		req.Header.Set("x-platform", fp.Components.Platform)
		req.Header.Set("x-arch", fp.Components.Arch)
		req.Header.Set("x-os-release", fp.Components.OsRelease)
		req.Header.Set("x-cpu-model", fp.Components.CpuModel)
		req.Header.Set("x-cpu-count", fmt.Sprintf("%d", fp.Components.CpuCount))
		req.Header.Set("x-mem-gib", fmt.Sprintf("%d", fp.Components.MemGiB))
		req.Header.Set("x-timezone", fp.Components.Timezone)
		req.Header.Set("x-runtime", fp.Components.Runtime)
		req.Header.Set("x-collector-version", fmt.Sprintf("%d", fp.Components.CollectorVersion))
	}

	// Copy safe custom headers without leaking local proxy credentials upstream.
	for key, values := range headers {
		if isForwardBlockedHeader(key) {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	c.logger.Debug("Forwarding request to upstream", map[string]any{
		"session_id": sessionID,
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Upstream request failed", map[string]any{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	c.logger.Debug("Upstream response received", map[string]any{
		"status": resp.StatusCode,
	})

	return resp, nil
}

// ForwardAnthropicCountTokens forwards an Anthropic token count request to the upstream API.
func (c *Client) ForwardAnthropicCountTokens(ctx context.Context, body []byte, cc_apiKey string, headers http.Header, sessionID string, fp *config.Fingerprint, rawQuery string) (*http.Response, error) {
	url := fmt.Sprintf("%s/v1/messages/count_tokens", c.apiBase)
	if rawQuery != "" {
		url += "?" + rawQuery
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		c.logger.Error("Failed to create upstream token count request", map[string]any{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cc_apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-cli-environment", "production")
	req.Header.Set("x-command-code-version", version.GetCommandCodeVersion())
	req.Header.Set("x-project-slug", c.projectSlugForSession(sessionID))
	req.Header.Set("x-session-id", sessionID)
	req.Header.Set("x-co-flag", "false")
	req.Header.Set("x-taste-learning", "false")
	req.Header.Set("traceparent", generateTraceparent())
	req.Header.Set("User-Agent", "CommandCode/"+version.GetCommandCodeVersion())

	if fp != nil {
		req.Header.Set("x-thumbmark", fp.Thumbmark)
		req.Header.Set("x-machine-id-hash", fp.Components.MachineIdHash)
		req.Header.Set("x-os-user-hash", fp.Components.OsUserHash)
		req.Header.Set("x-hostname-hash", fp.Components.HostnameHash)
		req.Header.Set("x-git-email-hash", fp.Components.GitEmailHash)
		req.Header.Set("x-platform", fp.Components.Platform)
		req.Header.Set("x-arch", fp.Components.Arch)
		req.Header.Set("x-os-release", fp.Components.OsRelease)
		req.Header.Set("x-cpu-model", fp.Components.CpuModel)
		req.Header.Set("x-cpu-count", fmt.Sprintf("%d", fp.Components.CpuCount))
		req.Header.Set("x-mem-gib", fmt.Sprintf("%d", fp.Components.MemGiB))
		req.Header.Set("x-timezone", fp.Components.Timezone)
		req.Header.Set("x-runtime", fp.Components.Runtime)
		req.Header.Set("x-collector-version", fmt.Sprintf("%d", fp.Components.CollectorVersion))
	}

	for key, values := range headers {
		if isForwardBlockedHeader(key) {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	c.logger.Debug("Forwarding token count request to upstream", map[string]any{
		"session_id": sessionID,
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Upstream token count request failed", map[string]any{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	c.logger.Debug("Upstream token count response received", map[string]any{
		"status": resp.StatusCode,
	})

	return resp, nil
}

// ProjectSlug returns the configured project slug or a session-derived fallback.
func ProjectSlug(configuredSlug string, sessionID string) string {
	if strings.TrimSpace(configuredSlug) != "" {
		return configuredSlug
	}
	return FakeProjectSlug(sessionID)
}

// projectSlugForSession returns the project slug for this client and session.
func (c *Client) projectSlugForSession(sessionID string) string {
	return ProjectSlug(c.projectSlug, sessionID)
}

// isForwardBlockedHeader reports whether an inbound header must stay local to the proxy.
func isForwardBlockedHeader(key string) bool {
	switch strings.ToLower(key) {
	case "authorization", "x-proxy-token", "x-cli-environment", "x-command-code-version", "x-project-slug", "x-session-id", "x-co-flag", "x-taste-learning", "traceparent", "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade", "host", "content-length":
		return true
	default:
		return false
	}
}

// generateTraceparent creates a W3C traceparent header value.
func generateTraceparent() string {
	traceID := make([]byte, 16)
	parentID := make([]byte, 8)
	if _, err := rand.Read(traceID); err != nil {
		return ""
	}
	if _, err := rand.Read(parentID); err != nil {
		return ""
	}
	return fmt.Sprintf("00-%s-%s-01", hex.EncodeToString(traceID), hex.EncodeToString(parentID))
}

// FetchModels fetches the list of available models from the Provider API
func (c *Client) FetchModels(ctx context.Context, cc_apiKey string) ([]Model, error) {
	url := fmt.Sprintf("%s/v1/models", c.apiBase)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Error("Failed to create models request", map[string]any{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cc_apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to fetch models from upstream", map[string]any{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Models API returned non-200", map[string]any{
			"status": resp.StatusCode,
		})
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Provider API returned status %d: %s", resp.StatusCode, string(body))
	}

	var modelList ModelList
	if err := json.NewDecoder(resp.Body).Decode(&modelList); err != nil {
		c.logger.Error("Failed to decode models response", map[string]any{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Models fetched from upstream", map[string]any{
		"count": len(modelList.Data),
	})

	return modelList.Data, nil
}

// SendFingerprint sends a fingerprint to the CommandCode API
func (c *Client) SendFingerprint(ctx context.Context, fp *fingerprint.Fingerprint, cc_apiKey string) error {
	url := fmt.Sprintf("%s/v1/fingerprint", c.apiBase)

	body, err := json.Marshal(fp)
	if err != nil {
		c.logger.Error("Failed to marshal fingerprint", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to marshal fingerprint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		c.logger.Error("Failed to create fingerprint request", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cc_apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to send fingerprint to upstream", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to send fingerprint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Fingerprint API returned non-200", map[string]any{
			"status": resp.StatusCode,
		})
		return fmt.Errorf("fingerprint API returned status %d", resp.StatusCode)
	}

	c.logger.Debug("Fingerprint sent to upstream", nil)
	return nil
}

// SendLifecycleEvent sends a lifecycle event to the CommandCode API
func (c *Client) SendLifecycleEvent(ctx context.Context, cc_apiKey string, eventType string) error {
	url := fmt.Sprintf("%s/v1/lifecycle", c.apiBase)

	event := map[string]string{
		"type": eventType,
	}

	body, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal lifecycle event", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		c.logger.Error("Failed to create lifecycle event request", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cc_apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to send lifecycle event to upstream", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to send lifecycle event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Lifecycle API returned non-200", map[string]any{
			"status": resp.StatusCode,
		})
		return fmt.Errorf("lifecycle API returned status %d", resp.StatusCode)
	}

	c.logger.Debug("Lifecycle event sent to upstream", map[string]any{
		"event_type": eventType,
	})
	return nil
}

// Model represents an AI model
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ModelList represents a list of models
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}
