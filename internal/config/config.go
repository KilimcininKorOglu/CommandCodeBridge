package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	Port                 int           `json:"port"`
	Host                 string        `json:"host"`
	APIBase              string        `json:"apiBase"`
	CCAPIKey             string        `json:"cc_apiKey,omitempty"`
	ProxyToken           string        `json:"proxy_token,omitempty"`
	ProjectSlug          string        `json:"projectSlug"`
	LogFile              string        `json:"logFile"`
	LogLevel             string        `json:"logLevel"`
	UseProviderModels    bool          `json:"useProviderModels"`
	ModelRefreshInterval time.Duration `json:"modelRefreshIntervalMs"`
	Fingerprint          *Fingerprint  `json:"fingerprint,omitempty"`
}

// Fingerprint represents machine fingerprint
type Fingerprint struct {
	Thumbmark  string                `json:"thumbmark"`
	Components FingerprintComponents `json:"components"`
}

// FingerprintComponents represents fingerprint components
type FingerprintComponents struct {
	MachineIdHash    string   `json:"machineIdHash"`
	MacHashes        []string `json:"macHashes"`
	OsUserHash       string   `json:"osUserHash"`
	HostnameHash     string   `json:"hostnameHash"`
	GitEmailHash     string   `json:"gitEmailHash"`
	Platform         string   `json:"platform"`
	Arch             string   `json:"arch"`
	OsRelease        string   `json:"osRelease"`
	CpuModel         string   `json:"cpuModel"`
	CpuCount         int      `json:"cpuCount"`
	MemGiB           int      `json:"memGiB"`
	IsContainer      bool     `json:"isContainer"`
	Timezone         string   `json:"timezone"`
	Runtime          string   `json:"runtime"`
	CollectorVersion int      `json:"collectorVersion"`
}

// Default configuration values
var defaults = Config{
	Port:                 3000,
	Host:                 "0.0.0.0",
	APIBase:              "https://api.commandcode.ai",
	ProjectSlug:          "",
	LogFile:              "",
	LogLevel:             "info",
	UseProviderModels:    true,
	ModelRefreshInterval: 5 * time.Minute,
}

// Load loads configuration from JSON file and applies environment variable overrides
func Load(configPath string) (*Config, error) {
	cfg := defaults

	// Load from JSON file if exists
	data, err := os.ReadFile(configPath)
	if err != nil {
		// A missing config file is tolerated (fall back to defaults + env),
		// but any other read failure (permission denied, SELinux label
		// mismatch, I/O error) is fatal — otherwise the proxy silently runs
		// with empty credentials and every authenticated request fails.
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config %q: %w", configPath, err)
		}
	} else {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}

	// Override with environment variables
	if v := os.Getenv("PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}
	if v := os.Getenv("HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("COMMANDCODE_API_BASE"); v != "" {
		cfg.APIBase = v
	}
	if v := os.Getenv("COMMANDCODE_PROXY_TOKEN"); v != "" {
		cfg.ProxyToken = v
	}
	if v := os.Getenv("PROJECT_SLUG"); v != "" {
		cfg.ProjectSlug = v
	}
	if v := os.Getenv("LOG_FILE"); v != "" {
		cfg.LogFile = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("COMMANDCODE_USE_PROVIDER_MODELS"); v != "" {
		cfg.UseProviderModels = strings.ToLower(v) != "false"
	}

	return &cfg, nil
}

// Save saves configuration to JSON file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Validate checks if configuration is valid
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return &ConfigError{Field: "port", Message: "port must be between 1 and 65535"}
	}
	if c.Host == "" {
		return &ConfigError{Field: "host", Message: "host cannot be empty"}
	}
	if c.APIBase == "" {
		return &ConfigError{Field: "apiBase", Message: "apiBase cannot be empty"}
	}
	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}
