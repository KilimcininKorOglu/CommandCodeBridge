package error

import (
	"fmt"
	"strings"
)

// ErrorType represents the type of API error
type ErrorType string

const (
	ErrorTypeInvalidRequest         ErrorType = "invalid_request_error"
	ErrorTypeAuth                   ErrorType = "authentication_error"
	ErrorTypeRateLimit              ErrorType = "rate_limit_error"
	ErrorTypeNotFound               ErrorType = "not_found"
	ErrorTypeUpstream               ErrorType = "upstream_error"
	ErrorTypeProxy                  ErrorType = "proxy_error"
	ErrorTypeInternal               ErrorType = "internal_error"
	ErrorTypeTemporarilyUnavailable ErrorType = "temporarily_unavailable"
)

// APIError represents an API error with HTTP status code
type APIError struct {
	Type    ErrorType `json:"type"`
	Message string    `json:"message"`
	Code    int       `json:"-"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return e.Message
}

// WithCode sets the HTTP status code
func (e *APIError) WithCode(code int) *APIError {
	e.Code = code
	return e
}

// NewAPIError creates a new API error
func NewAPIError(errorType ErrorType, message string) *APIError {
	return &APIError{
		Type:    errorType,
		Message: message,
	}
}

// StatusMap maps CommandCode HTTP status codes to proxy error types.
var StatusMap = map[int]ErrorType{
	400: ErrorTypeInvalidRequest,
	401: ErrorTypeAuth,
	402: ErrorTypeRateLimit,
	403: ErrorTypeAuth,
	404: ErrorTypeNotFound,
	422: ErrorTypeInvalidRequest,
	429: ErrorTypeRateLimit,
	500: ErrorTypeUpstream,
	502: ErrorTypeUpstream,
	503: ErrorTypeTemporarilyUnavailable,
}

// StatusCodeMap maps CommandCode HTTP status codes to proxy HTTP status codes.
var StatusCodeMap = map[int]int{
	400: 400,
	401: 401,
	402: 429,
	403: 401,
	404: 404,
	422: 400,
	429: 429,
	500: 502,
	502: 502,
	503: 503,
}

// MapStatus maps an HTTP status code and response body to an APIError.
func MapStatus(status int, body string) *APIError {
	errorType, ok := StatusMap[status]
	if !ok {
		errorType = ErrorTypeUpstream
	}

	code, ok := StatusCodeMap[status]
	if !ok {
		code = 502
	}

	message := fmt.Sprintf("CommandCode API error (%d)", status)
	if body != "" {
		truncatedBody := body
		if len(truncatedBody) > 500 {
			truncatedBody = truncatedBody[:500] + "..."
		}
		// Sanitize any control characters/newlines to prevent log injection
		truncatedBody = strings.ReplaceAll(truncatedBody, "\n", " ")
		truncatedBody = strings.ReplaceAll(truncatedBody, "\r", " ")
		message = fmt.Sprintf("CommandCode API error (%d): %s", status, truncatedBody)
	}
	return NewAPIError(errorType, message).WithCode(code)
}

// Usage represents token usage statistics
type Usage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	CachedInputTokens int `json:"cachedInputTokens,omitempty"`
}

// NormalizeUsage normalizes usage stats to prevent false billing when output tokens are zero
func NormalizeUsage(u *Usage) {
	if u == nil {
		return
	}
	if u.OutputTokens == 0 {
		u.InputTokens = 0
		u.CachedInputTokens = 0
	}
}
