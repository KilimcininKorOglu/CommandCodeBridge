package protocol

// OpenAIRequest represents an OpenAI API request
type OpenAIRequest struct {
	Model             string          `json:"model"`
	Messages          []OpenAIMessage `json:"messages"`
	MaxTokens         int             `json:"max_tokens,omitempty"`
	Temperature       float64         `json:"temperature,omitempty"`
	TopP              float64         `json:"top_p,omitempty"`
	StopSequences     []string        `json:"stop,omitempty"`
	Metadata          any             `json:"metadata,omitempty"`
	Tools             []OpenAITool    `json:"tools,omitempty"`
	Stream            bool            `json:"stream,omitempty"`
	ReasoningEffort   string          `json:"reasoning_effort,omitempty"`
	Thinking          any             `json:"thinking,omitempty"`
	ContextManagement any             `json:"context_management,omitempty"`
	OutputConfig      any             `json:"output_config,omitempty"`
	ToolChoice        any             `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls,omitempty"`
}

// OpenAIMessage represents a message in OpenAI format
type OpenAIMessage struct {
	Role             string     `json:"role"`
	Content          any        `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
}

// OpenAITool represents a tool definition in OpenAI format
type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIToolFunction `json:"function"`
}

// OpenAIToolFunction represents a function tool in OpenAI format
type OpenAIToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
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

// OpenAIResponse represents an OpenAI API response
type OpenAIResponse struct {
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
	Message      *OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIMessage `json:"delta,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
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

// AnthropicRequest represents an Anthropic API request
type AnthropicRequest struct {
	Model             string             `json:"model"`
	Messages          []AnthropicMessage `json:"messages"`
	MaxTokens         int                `json:"max_tokens"`
	Stream            bool               `json:"stream,omitempty"`
	System            any                `json:"system,omitempty"`
	Tools             []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice        any                `json:"tool_choice,omitempty"`
	Temperature       float64            `json:"temperature,omitempty"`
	TopP              float64            `json:"top_p,omitempty"`
	StopSequences     []string           `json:"stop_sequences,omitempty"`
	Metadata          *AnthropicMetadata `json:"metadata,omitempty"`
	Thinking          *AnthropicThinking `json:"thinking,omitempty"`
	ContextManagement any                `json:"context_management,omitempty"`
	OutputConfig      any                `json:"output_config,omitempty"`
}

// AnthropicMessage represents a message in Anthropic format
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// AnthropicTool represents a tool definition in Anthropic format
type AnthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
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

// AnthropicResponse represents an Anthropic API response
type AnthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []AnthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason,omitempty"`
	StopSequence *string            `json:"stop_sequence,omitempty"`
	Usage        *AnthropicUsage    `json:"usage,omitempty"`
}

// AnthropicContent represents content in Anthropic format
type AnthropicContent struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Thinking  string         `json:"thinking,omitempty"`
	Signature string         `json:"signature,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
}

// AnthropicUsage represents usage in Anthropic format
type AnthropicUsage struct {
	InputTokens              int  `json:"input_tokens"`
	OutputTokens             int  `json:"output_tokens"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int  `json:"cache_read_input_tokens,omitempty"`
}

// CommandCodeRequest represents a CommandCode API request
type CommandCodeRequest struct {
	Config         CommandCodeConfig `json:"config"`
	Memory         any               `json:"memory"`
	Taste          any               `json:"taste"`
	Skills         string            `json:"skills"`
	PermissionMode string            `json:"permissionMode"`
	Params         CommandCodeParams `json:"params"`
}

// CommandCodeConfig represents configuration in CommandCode format
type CommandCodeConfig struct {
	WorkingDir    string   `json:"workingDir"`
	Date          string   `json:"date"`
	Environment   string   `json:"environment"`
	Structure     []string `json:"structure"`
	IsGitRepo     bool     `json:"isGitRepo"`
	CurrentBranch string   `json:"currentBranch"`
	MainBranch    string   `json:"mainBranch"`
	GitStatus     string   `json:"gitStatus"`
	RecentCommits []string `json:"recentCommits"`
}

// CommandCodeParams represents parameters in CommandCode format
type CommandCodeParams struct {
	Model             string               `json:"model"`
	Messages          []CommandCodeMessage `json:"messages"`
	MaxTokens         int                  `json:"max_tokens"`
	Stream            bool                 `json:"stream"`
	System            string               `json:"system,omitempty"`
	Temperature       float64              `json:"temperature,omitempty"`
	TopP              float64              `json:"top_p,omitempty"`
	StopSequences     []string             `json:"stop_sequences,omitempty"`
	Metadata          any                  `json:"metadata,omitempty"`
	ReasoningEffort   string               `json:"reasoning_effort,omitempty"`
	Thinking          any                  `json:"thinking,omitempty"`
	ContextManagement any                  `json:"context_management,omitempty"`
	OutputConfig      any                  `json:"output_config,omitempty"`
	Tools             []CommandCodeTool    `json:"tools,omitempty"`
	ToolChoice        any                  `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool                `json:"parallel_tool_calls,omitempty"`
}

// CommandCodeMessage represents a message in CommandCode format
type CommandCodeMessage struct {
	Role    string               `json:"role"`
	Content []CommandCodeContent `json:"content"`
}

// CommandCodeContent represents content in CommandCode format
type CommandCodeContent struct {
	Type       string                 `json:"type"`
	Text       string                 `json:"text,omitempty"`
	Image      string                 `json:"image,omitempty"`
	ToolCallID string                 `json:"toolCallId,omitempty"`
	ToolName   string                 `json:"toolName,omitempty"`
	Input      any                    `json:"input,omitempty"`
	Output     *CommandCodeToolOutput `json:"output,omitempty"`
}

// CommandCodeToolOutput represents tool output in CommandCode format
type CommandCodeToolOutput struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// CommandCodeTool represents a tool in CommandCode format
type CommandCodeTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// CommandCodeResponse represents a CommandCode API response
type CommandCodeResponse struct {
	ID           string `json:"id"`
	Created      int64  `json:"created"`
	Model        string `json:"model"`
	Content      string `json:"content"`
	FinishReason string `json:"finishReason,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
}

// Usage represents token usage statistics
type Usage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	CachedInputTokens int `json:"cachedInputTokens,omitempty"`
}
