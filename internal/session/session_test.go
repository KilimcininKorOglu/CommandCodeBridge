package session

import (
	"net/http"
	"testing"
	"time"
)

func TestResolveUsesSessionIDHeader(t *testing.T) {
	store := NewStore(time.Hour, 0)
	headers := http.Header{}
	headers.Set("X-Session-Id", "session-123")

	if got, want := store.Resolve(headers, "user_test"), "session-123"; got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveUsesClaudeCodeSessionIDHeader(t *testing.T) {
	store := NewStore(time.Hour, 0)
	headers := http.Header{}
	headers.Set("X-Claude-Code-Session-Id", "claude-session-123")

	if got, want := store.Resolve(headers, "user_test"), "claude-session-123"; got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveFallsBackToStoredSession(t *testing.T) {
	store := NewStore(time.Hour, 0)
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
