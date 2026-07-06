package types

// OpenAIRequest represents an OpenAI API request
type OpenAIRequest struct {
	Model             string                 `json:"model"`
	Messages          []OpenAIMessage        `json:"messages"`
	MaxTokens         int                    `json:"max_tokens,omitempty"`
	Temperature       float64                `json:"temperature,omitempty"`
	Tools             []OpenAITool           `json:"tools,omitempty"`
	Stream            bool                   `json:"stream,omitempty"`
	ReasoningEffort   string                 `json:"reasoning_effort,omitempty"`
	ToolChoice        interface{}            `json:"tool_choice,omitempty"`
	ParallelToolCalls bool                   `json:"parallel_tool_calls,omitempty"`
}

// OpenAIMessage represents a message in OpenAI format
type OpenAIMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// OpenAITool represents a tool definition in OpenAI format
type OpenAITool struct {
	Type     string              `json:"type"`
	Function OpenAIToolFunction `json:"function,omitempty"`
}

// OpenAIToolFunction represents a function tool in OpenAI format
type OpenAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
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

// AnthropicRequest represents an Anthropic API request
type AnthropicRequest struct {
	Model          string              `json:"model"`
	Messages       []AnthropicMessage  `json:"messages"`
	MaxTokens      int                 `json:"max_tokens"`
	Stream         bool                `json:"stream,omitempty"`
	System         interface{}         `json:"system,omitempty"`
	Tools          []AnthropicTool     `json:"tools,omitempty"`
	ToolChoice     interface{}         `json:"tool_choice,omitempty"`
	Temperature    float64             `json:"temperature,omitempty"`
	TopP           float64             `json:"top_p,omitempty"`
	StopSequences  []string            `json:"stop_sequences,omitempty"`
	Metadata       *AnthropicMetadata  `json:"metadata,omitempty"`
	Thinking       *AnthropicThinking   `json:"thinking,omitempty"`
}

// AnthropicMessage represents a message in Anthropic format
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// AnthropicTool represents a tool definition in Anthropic format
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// AnthropicMetadata represents metadata in Anthropic format
type AnthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// AnthropicThinking represents thinking configuration
type AnthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	Effort       string `json:"effort,omitempty"`
}
