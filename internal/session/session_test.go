package session

import (
	"net/http"
	"testing"
	"time"
)

// noopLogger is a test logger that discards all output.
type noopLogger struct{}

func (noopLogger) Debug(string, map[string]interface{}) {}
func (noopLogger) Info(string, map[string]interface{})  {}
func (noopLogger) Warn(string, map[string]interface{})  {}

func TestResolveUsesSessionIDHeader(t *testing.T) {
	store := NewStore(time.Hour, 0, noopLogger{})
	headers := http.Header{}
	headers.Set("X-Session-Id", "session-123")

	if got, want := store.Resolve(headers, "user_test"), "session-123"; got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveUsesClaudeCodeSessionIDHeader(t *testing.T) {
	store := NewStore(time.Hour, 0, noopLogger{})
	headers := http.Header{}
	headers.Set("X-Claude-Code-Session-Id", "claude-session-123")

	if got, want := store.Resolve(headers, "user_test"), "claude-session-123"; got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveFallsBackToStoredSession(t *testing.T) {
	store := NewStore(time.Hour, 0, noopLogger{})
	headers := http.Header{}

	first := store.Resolve(headers, "user_test")
	second := store.Resolve(headers, "user_test")
	if first == "" {
		t.Fatal("Resolve() returned empty fallback session")
	}
	if first != second {
		t.Fatalf("Resolve() returned %q then %q, want stable fallback session", first, second)
	}
}
