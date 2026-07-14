package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFallsBackToDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load("/tmp/nonexistent-config-test.json")
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for missing file", err)
	}
	if cfg.Port != 3000 {
		t.Fatalf("Port = %d, want default 3000", cfg.Port)
	}
	if cfg.APIBase != "https://api.commandcode.ai" {
		t.Fatalf("APIBase = %q, want default", cfg.APIBase)
	}
	if cfg.UseProviderModels != true {
		t.Fatalf("UseProviderModels = %v, want true", cfg.UseProviderModels)
	}
}

func TestLoadFailsOnPermissionDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"port":3050}`), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(path, 0644) })

	// Skip if running as root (root bypasses file permissions).
	if os.Getuid() == 0 {
		t.Skip("running as root, permission test not applicable")
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want error for unreadable file")
	}
}

func TestLoadParsesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"port":8080,"host":"127.0.0.1","apiBase":"https://custom.api","cc_apiKey":"user_abc123","proxy_token":"tok"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.APIBase != "https://custom.api" {
		t.Fatalf("APIBase = %q, want https://custom.api", cfg.APIBase)
	}
	if cfg.CCAPIKey != "user_abc123" {
		t.Fatalf("CCAPIKey = %q, want user_abc123", cfg.CCAPIKey)
	}
	if cfg.ProxyToken != "tok" {
		t.Fatalf("ProxyToken = %q, want tok", cfg.ProxyToken)
	}
}

func TestLoadFailsOnInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid JSON")
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("HOST", "0.0.0.0")
	t.Setenv("COMMANDCODE_PROXY_TOKEN", "envtoken")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load("/tmp/nonexistent-config-test.json")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9999 {
		t.Fatalf("Port = %d, want 9999 from env", cfg.Port)
	}
	if cfg.ProxyToken != "envtoken" {
		t.Fatalf("ProxyToken = %q, want envtoken from env", cfg.ProxyToken)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug from env", cfg.LogLevel)
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	cfg := &Config{Port: 0, Host: "0.0.0.0", APIBase: "https://api.test"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for port 0")
	}
}

func TestValidateRejectsEmptyHost(t *testing.T) {
	cfg := &Config{Port: 3050, Host: "", APIBase: "https://api.test"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for empty host")
	}
}

func TestDefaultModelRefreshInterval(t *testing.T) {
	if defaults.ModelRefreshInterval != 5*time.Minute {
		t.Fatalf("default ModelRefreshInterval = %v, want 5m", defaults.ModelRefreshInterval)
	}
}
