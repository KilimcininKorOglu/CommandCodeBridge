package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/kilimcininkoroglu/commandcode-bridge/internal/config"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/logging"
	"github.com/kilimcininkoroglu/commandcode-bridge/pkg/version"
)

const (
	// InitRefreshInterval is the interval for initialization refresh
	InitRefreshInterval = 8 * time.Hour
	// InitJitter is the jitter for initialization refresh
	InitJitter = 2 * time.Hour
)

// InitManager manages initialization requests (fingerprint and lifecycle events)
type InitManager struct {
	apiBase     string
	projectSlug string
	logger      *logging.Logger
	nextInitAt  time.Time
	mu          sync.Mutex
}

// NewInitManager creates a new initialization manager
func NewInitManager(apiBase string, projectSlug string, logger *logging.Logger) *InitManager {
	return &InitManager{
		apiBase:     apiBase,
		projectSlug: projectSlug,
		logger:      logger,
	}
}

// EnsureInitialized ensures initialization requests are sent if needed
func (m *InitManager) EnsureInitialized(ctx context.Context, cc_apiKey string, fp *config.Fingerprint) error {
	m.mu.Lock()
	now := time.Now()
	if now.Before(m.nextInitAt) {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	headers := map[string]string{
		"Content-Type":           "application/json",
		"x-cli-environment":      "production",
		"Authorization":          "Bearer " + cc_apiKey,
		"x-command-code-version": version.GetCommandCodeVersion(),
	}

	// Send fingerprint record
	if err := m.sendFingerprintRecord(ctx, fp, headers); err != nil {
		m.logger.Warn("Fingerprint record failed", map[string]any{
			"error": err.Error(),
		})
	} else {
		m.logger.Info("Fingerprint recorded", nil)
	}

	// Send lifecycle event
	if err := m.sendLifecycleEvent(ctx, fp, headers); err != nil {
		m.logger.Warn("Lifecycle event failed", map[string]any{
			"error": err.Error(),
		})
	} else {
		m.logger.Info("Lifecycle event sent", nil)
	}

	// Update next init time with jitter
	jitter := time.Duration(randInt64(int64(InitJitter)))
	m.mu.Lock()
	m.nextInitAt = time.Now().Add(InitRefreshInterval + jitter)
	m.mu.Unlock()

	m.logger.Info("Fingerprint/lifecycle next refresh", map[string]any{
		"nextIn": fmt.Sprintf("%.1fh", (InitRefreshInterval + jitter).Hours()),
	})

	return nil
}

// sendFingerprintRecord sends fingerprint to CommandCode API
func (m *InitManager) sendFingerprintRecord(ctx context.Context, fp *config.Fingerprint, headers map[string]string) error {
	url := fmt.Sprintf("%s/alpha/fingerprint/record", m.apiBase)

	body, err := json.Marshal(fp)
	if err != nil {
		m.logger.Error("Failed to marshal fingerprint record", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to marshal fingerprint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		m.logger.Error("Failed to create fingerprint record request", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		m.logger.Error("Failed to send fingerprint record to upstream", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		m.logger.Warn("Fingerprint record API returned non-200", map[string]any{
			"status": resp.StatusCode,
		})
		return fmt.Errorf("fingerprint record returned status %d", resp.StatusCode)
	}

	return nil
}

// sendLifecycleEvent sends lifecycle event to CommandCode API
func (m *InitManager) sendLifecycleEvent(ctx context.Context, fp *config.Fingerprint, headers map[string]string) error {
	url := fmt.Sprintf("%s/alpha/lifecycle-events", m.apiBase)

	sessionID := generateSessionID()
	projectSlug := ProjectSlug(m.projectSlug, sessionID)

	event := map[string]any{
		"eventType": "cli_session_exists",
		"metadata": map[string]any{
			"sessionId":  sessionID,
			"cliVersion": version.GetCommandCodeVersion(),
			"mode":       "interactive",
			"os":         fmt.Sprintf("%s-%s", fp.Components.Platform, fp.Components.Arch),
		},
	}

	body, err := json.Marshal(event)
	if err != nil {
		m.logger.Error("Failed to marshal lifecycle event", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		m.logger.Error("Failed to create lifecycle event request", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("x-project-slug", projectSlug)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		m.logger.Error("Failed to send lifecycle event to upstream", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		m.logger.Warn("Lifecycle event API returned non-200", map[string]any{
			"status": resp.StatusCode,
		})
		return fmt.Errorf("lifecycle event returned status %d", resp.StatusCode)
	}

	return nil
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "sess_" + hex.EncodeToString(b)
}

// randInt64 returns a random non-negative int64
func randInt64(max int64) int64 {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0
	}
	return n.Int64()
}

// FakeProjectSlug generates a fake project slug from session ID
func FakeProjectSlug(sessionID string) string {
	names := []string{
		"app", "api", "backend", "bot", "cli", "core", "data", "frontend",
		"lib", "plugin", "proxy", "server", "service", "tool", "web", "worker",
	}

	if len(sessionID) < 4 {
		return "cc-bridge"
	}

	// Parse first 4 chars as hex
	var idx int64
	for i := 0; i < 4 && i < len(sessionID); i++ {
		c := sessionID[i]
		var val int64
		if c >= '0' && c <= '9' {
			val = int64(c - '0')
		} else if c >= 'a' && c <= 'f' {
			val = int64(c - 'a' + 10)
		} else if c >= 'A' && c <= 'F' {
			val = int64(c - 'A' + 10)
		} else {
			continue
		}
		idx = idx*16 + val
	}

	nameIdx := int(idx) % len(names)
	name := names[nameIdx]
	suffix := sessionID[:4]

	return fmt.Sprintf("d-users-dev-projects-%s-%s", name, suffix)
}
