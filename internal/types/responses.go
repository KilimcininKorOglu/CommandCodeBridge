package types

// OpenAIResponse represents an OpenAI API response
type OpenAIResponse struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// OpenAIChoice represents a choice in OpenAI response
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      *OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIMessage `json:"delta,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
}

// OpenAIChunk represents a streaming chunk in OpenAI format
type OpenAIChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// AnthropicResponse represents an Anthropic API response
type AnthropicResponse struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	Role        string              `json:"role"`
	Content     []AnthropicContent  `json:"content"`
	Model        string              `json:"model"`
	StopReason  string              `json:"stop_reason,omitempty"`
	StopSequence *string             `json:"stop_sequence,omitempty"`
	Usage       *AnthropicUsage     `json:"usage,omitempty"`
}

// AnthropicContent represents content in Anthropic format
type AnthropicContent struct {
	Type       string                 `json:"type"`
	Text       string                 `json:"text,omitempty"`
	ID         string                 `json:"id,omitempty"`
	Name       string                 `json:"name,omitempty"`
	Input      map[string]interface{} `json:"input,omitempty"`
}

// AnthropicUsage represents usage in Anthropic format
type AnthropicUsage struct {
	InputTokens                int  `json:"input_tokens"`
	OutputTokens               int  `json:"output_tokens"`
	CacheCreationInputTokens  *int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens      int  `json:"cache_read_input_tokens,omitempty"`
}

// CommandCodeRequest represents a CommandCode API request
type CommandCodeRequest struct {
	Config         CommandCodeConfig `json:"config"`
	Memory         interface{}        `json:"memory"`
	Taste          interface{}        `json:"taste"`
	Skills         string             `json:"skills"`
	PermissionMode string             `json:"permissionMode"`
	Params         CommandCodeParams  `json:"params"`
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
	Model             string                 `json:"model"`
	Messages          []CommandCodeMessage  `json:"messages"`
	MaxTokens         int                    `json:"max_tokens"`
	Stream            bool                   `json:"stream"`
	System            string                 `json:"system,omitempty"`
	Temperature       float64                `json:"temperature,omitempty"`
	ReasoningEffort   string                 `json:"reasoning_effort,omitempty"`
	Tools             []CommandCodeTool      `json:"tools,omitempty"`
	ToolChoice        interface{}            `json:"tool_choice,omitempty"`
	ParallelToolCalls bool                   `json:"parallel_tool_calls,omitempty"`
}

// CommandCodeMessage represents a message in CommandCode format
type CommandCodeMessage struct {
	Role    string                 `json:"role"`
	Content []CommandCodeContent  `json:"content"`
}

// CommandCodeContent represents content in CommandCode format
type CommandCodeContent struct {
	Type        string                 `json:"type"`
	Text        string                 `json:"text,omitempty"`
	Image       string                 `json:"image,omitempty"`
	ToolCallID  string                 `json:"toolCallId,omitempty"`
	ToolName    string                 `json:"toolName,omitempty"`
	Input       interface{}            `json:"input,omitempty"`
	Output      *CommandCodeToolOutput `json:"output,omitempty"`
}

// CommandCodeToolOutput represents tool output in CommandCode format
type CommandCodeToolOutput struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// CommandCodeTool represents a tool in CommandCode format
type CommandCodeTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// CommandCodeEvent represents an event in CommandCode NDJSON stream
type CommandCodeEvent struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	Delta        string                 `json:"delta,omitempty"`
	ToolCallID   string                 `json:"toolCallId,omitempty"`
	ToolName     string                 `json:"toolName,omitempty"`
	Input        interface{}            `json:"input,omitempty"`
	FinishReason string                 `json:"finishReason,omitempty"`
	Usage        *Usage                 `json:"usage,omitempty"`
	TotalUsage   *Usage                 `json:"totalUsage,omitempty"`
	Error        *CommandCodeError      `json:"error,omitempty"`
}

// CommandCodeError represents an error in CommandCode format
type CommandCodeError struct {
	Message string `json:"message,omitempty"`
}
