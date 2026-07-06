package streaming

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
}

// CommandCodeEvent represents an event in CommandCode NDJSON stream
type CommandCodeEvent struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	Delta        string                 `json:"delta,omitempty"`
	ToolCallID   string                 `json:"toolCallId,omitempty"`
	ToolName     string                 `json:"toolName,omitempty"`
	Input        any                    `json:"input,omitempty"`
	Output       *CommandCodeToolOutput `json:"output,omitempty"`
	FinishReason string                 `json:"finishReason,omitempty"`
	Usage        *Usage                 `json:"usage,omitempty"`
	TotalUsage   *Usage                 `json:"totalUsage,omitempty"`
	Message      string                 `json:"message,omitempty"`
	Error        *CommandCodeError      `json:"error,omitempty"`
}

// CommandCodeError represents an error in CommandCode format
type CommandCodeError struct {
	Message string `json:"message,omitempty"`
}

// CommandCodeToolOutput represents tool output in CommandCode format
type CommandCodeToolOutput struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Usage represents token usage statistics
type Usage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	CachedInputTokens int `json:"cachedInputTokens,omitempty"`
}

// OpenAIChunk represents a streaming chunk in OpenAI format
type OpenAIChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// OpenAIChoice represents a choice in OpenAI response
type OpenAIChoice struct {
	Index        int            `json:"index"`
	Delta        *OpenAIMessage `json:"delta,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
}

// OpenAIMessage represents a message in OpenAI format
type OpenAIMessage struct {
	Role             string     `json:"role,omitempty"`
	Content          any        `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool call in OpenAI format
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call invocation
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
