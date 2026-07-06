package http

import (
	"context"
	"crypto/subtle"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kilimcininkoroglu/commandcode-bridge/internal/config"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/error"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/logging"
)

// CORS middleware adds CORS headers to all responses
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Proxy-Token")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

var ccAPIKeyPattern = regexp.MustCompile(`user_[a-zA-Z0-9_-]+`)

// AuthMiddleware validates proxy and upstream API key credentials.
func AuthMiddleware(cfg *config.Config, logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !proxyTokenFromRequest(r.Header, cfg.ProxyToken) {
				logger.Warn("Invalid or missing proxy token", nil)
				sendError(w, error.NewAPIError(error.ErrorTypeAuth, "Missing or invalid proxy token").WithCode(http.StatusUnauthorized))
				return
			}

			ccAPIKey, ok := ccAPIKeyFromRequest(r.Header, cfg.CCAPIKey)
			if cfg.ProxyToken != "" {
				ccAPIKey, ok = extractCCAPIKey(cfg.CCAPIKey)
			}
			if !ok {
				logger.Warn("Invalid or missing API key", nil)
				sendError(w, error.NewAPIError(error.ErrorTypeAuth, "Missing or invalid API key").WithCode(http.StatusUnauthorized))
				return
			}

			ctx := context.WithValue(r.Context(), "ccAPIKey", ccAPIKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// proxyTokenFromRequest validates the configured proxy token when it is enabled.
func proxyTokenFromRequest(headers http.Header, proxyToken string) bool {
	if proxyToken == "" {
		return true
	}

	provided := headers.Get("X-Proxy-Token")
	if provided == "" {
		auth := headers.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			provided = auth[7:]
		}
	}

	return subtle.ConstantTimeCompare([]byte(provided), []byte(proxyToken)) == 1
}

// ccAPIKeyFromRequest extracts the first CommandCode API key from a Bearer header or config fallback.
func ccAPIKeyFromRequest(headers http.Header, configCCAPIKey string) (string, bool) {
	auth := headers.Get("Authorization")
	if auth == "" {
		if configCCAPIKey == "" {
			return "", false
		}
		return extractCCAPIKey(configCCAPIKey)
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}

	return extractCCAPIKey(auth[7:])
}

// extractCCAPIKey returns the first user_ key from a credential string.
func extractCCAPIKey(value string) (string, bool) {
	ccAPIKey := ccAPIKeyPattern.FindString(value)
	if ccAPIKey == "" {
		return "", false
	}
	return ccAPIKey, true
}

// RequestSizeLimit middleware limits the maximum request body size
func RequestSizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// RequestTimeout middleware adds a timeout to requests
func RequestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.Info("HTTP request", map[string]any{
				"method":     r.Method,
				"path":       r.URL.Path,
				"status":     wrapped.status,
				"duration":   duration.Milliseconds(),
				"user_agent": r.UserAgent(),
			})
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
