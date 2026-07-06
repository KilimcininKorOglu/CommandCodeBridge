package types

// Common types used across packages

// Usage represents token usage statistics
type Usage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	CachedInputTokens int `json:"cachedInputTokens,omitempty"`
	InputTokenDetails *InputTokenDetails `json:"inputTokenDetails,omitempty"`
}

// InputTokenDetails contains detailed input token information
type InputTokenDetails struct {
	CacheWriteTokens int `json:"cacheWriteTokens,omitempty"`
}

// StreamContext tracks streaming state
type StreamContext struct {
	BytesReceived      int
	LastCommandCodeEvent string
	InputTokens        int
	OutputTokens       int
	CachedInputTokens  int
}

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
}
