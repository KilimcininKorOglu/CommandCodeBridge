package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// Version is the proxy version
	Version = "1.1.0"

	// CommandCodeVersionFallback is the fallback version if fetch fails
	CommandCodeVersionFallback = "0.34.1"

	// CommandCodeVersionRefreshInterval is the interval for version refresh
	CommandCodeVersionRefreshInterval = 24 * time.Hour
)

var (
	currentVersion string
	versionMutex   sync.RWMutex
)

func init() {
	currentVersion = CommandCodeVersionFallback
}

// GetCommandCodeVersion returns the current CommandCode CLI version
func GetCommandCodeVersion() string {
	versionMutex.RLock()
	defer versionMutex.RUnlock()
	return currentVersion
}

// RefreshCommandCodeVersion fetches the latest version from npm registry
func RefreshCommandCodeVersion() error {
	url := "https://registry.npmjs.org/command-code"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch version: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var npmResp struct {
		DistTags struct {
			Latest string `json:"latest"`
		} `json:"dist-tags"`
	}

	if err := json.Unmarshal(body, &npmResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if npmResp.DistTags.Latest != "" {
		versionMutex.Lock()
		currentVersion = npmResp.DistTags.Latest
		versionMutex.Unlock()
	}

	return nil
}

// StartAutoRefresh starts automatic version refresh in a goroutine
func StartAutoRefresh(ctx context.Context, logger Logger) {
	ticker := time.NewTicker(CommandCodeVersionRefreshInterval)
	defer ticker.Stop()

	// Initial refresh
	if err := RefreshCommandCodeVersion(); err != nil {
		logger.Debug("Failed to refresh CommandCode version on startup", map[string]interface{}{
			"error": err.Error(),
		})
	}

	for {
		select {
		case <-ticker.C:
			if err := RefreshCommandCodeVersion(); err != nil {
				logger.Debug("Failed to refresh CommandCode version", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				logger.Info("CommandCode version refreshed", map[string]interface{}{
					"version": GetCommandCodeVersion(),
				})
			}
		case <-ctx.Done():
			return
		}
	}
}

// Logger interface for version logging
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
}
