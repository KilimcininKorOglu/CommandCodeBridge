package session

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// Logger is the interface for session store logging.
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
}

// Session represents a user session
type Session struct {
	ID        string
	CCAPIKey  string
	ExpiresAt time.Time
}

// SessionStore manages sessions with thread-safe operations
type SessionStore struct {
	sessions sync.Map
	duration time.Duration
	jitter   time.Duration
	stopChan chan struct{}
	logger   Logger
}

// NewStore creates a new session store with specified duration and jitter
func NewStore(duration, jitter time.Duration, logger Logger) *SessionStore {
	return &SessionStore{
		duration: duration,
		jitter:   jitter,
		stopChan: make(chan struct{}),
		logger:   logger,
	}
}

// Get retrieves a session ID for the given API key, creating one if needed
func (s *SessionStore) Get(ccAPIKey string) string {
	if val, ok := s.sessions.Load(ccAPIKey); ok {
		session := val.(*Session)
		if time.Now().Before(session.ExpiresAt) {
			return session.ID
		}
	}
	return s.Create(ccAPIKey)
}

// Create creates a new session for the given API key
func (s *SessionStore) Create(ccAPIKey string) string {
	sessionID := generateSessionID()
	jitterDuration := time.Duration(0)
	if s.jitter > 0 {
		jitterDuration = time.Duration(randInt64(int64(s.jitter)))
	}
	expiresAt := time.Now().Add(s.duration + jitterDuration)

	session := &Session{
		ID:        sessionID,
		CCAPIKey:  ccAPIKey,
		ExpiresAt: expiresAt,
	}

	s.sessions.Store(ccAPIKey, session)
	if s.logger != nil {
		s.logger.Debug("Session created", map[string]interface{}{
			"expires_in": s.duration.String(),
		})
	}
	return sessionID
}

// Cleanup removes expired sessions and returns the count of removed sessions
func (s *SessionStore) Cleanup() int {
	var count int
	now := time.Now()

	s.sessions.Range(func(key, value any) bool {
		session := value.(*Session)
		if now.After(session.ExpiresAt) {
			s.sessions.Delete(key)
			count++
		}
		return true
	})

	return count
}

// StartCleanup starts a periodic cleanup goroutine
func (s *SessionStore) StartCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				removed := s.Cleanup()
				if removed > 0 && s.logger != nil {
					s.logger.Debug("Expired sessions cleaned up", map[string]interface{}{
						"removed": removed,
					})
				}
			case <-s.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the periodic cleanup goroutine
func (s *SessionStore) Stop() {
	close(s.stopChan)
}

// GetFromHeaders extracts a compatible upstream session ID from request headers.
func (s *SessionStore) GetFromHeaders(headers http.Header) string {
	for _, key := range []string{"X-Session-Id", "X-Claude-Code-Session-Id"} {
		if sessionID := headers.Get(key); len(sessionID) >= 8 {
			return sessionID
		}
	}
	return ""
}

// Resolve returns a header-provided session ID or the stored session for the API key.
func (s *SessionStore) Resolve(headers http.Header, ccAPIKey string) string {
	if sessionID := s.GetFromHeaders(headers); sessionID != "" {
		return sessionID
	}
	return s.Get(ccAPIKey)
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// randInt64 returns a random non-negative int64
func randInt64(max int64) int64 {
	b := make([]byte, 8)
	rand.Read(b)
	n := int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 | int64(b[3])<<24 |
		int64(b[4])<<32 | int64(b[5])<<40 | int64(b[6])<<48 | int64(b[7])<<56
	if n < 0 {
		n = -n
	}
	return n % max
}
