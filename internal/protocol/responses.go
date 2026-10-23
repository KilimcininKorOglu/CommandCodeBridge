package protocol

import (
	"errors"
	"strings"

	"github.com/kilimcininkoroglu/commandcode-bridge/internal/config"
)

// OpenAIResponsesRequest represents an OpenAI Responses API request.
type OpenAIResponsesRequest struct {
	Model             string       `json:"model"`
	Input             any          `json:"input"`
	Instructions      string       `json:"instructions,omitempty"`
	MaxOutputTokens   int          `json:"max_output_tokens,omitempty"`
	Temperature       float64      `json:"temperature,omitempty"`
	TopP              float64      `json:"top_p,omitempty"`
	Metadata          any          `json:"metadata,omitempty"`
	Tools             []OpenAITool `json:"tools,omitempty"`
	Stream            bool         `json:"stream,omitempty"`
	ReasoningEffort   string       `json:"reasoning_effort,omitempty"`
	Thinking          any          `json:"thinking,omitempty"`
	ContextManagement any          `json:"context_management,omitempty"`
	OutputConfig      any          `json:"output_config,omitempty"`
	ToolChoice        any          `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool        `json:"parallel_tool_calls,omitempty"`
}

// OpenAIResponsesCompactRequest represents an OpenAI Responses compaction request.
type OpenAIResponsesCompactRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

// OpenAIResponseObject represents an OpenAI Responses API response.
type OpenAIResponseObject struct {
	ID        string                 `json:"id"`
	Object    string                 `json:"object"`
	CreatedAt int64                  `json:"created_at"`
	Status    string                 `json:"status"`
	Model     string                 `json:"model,omitempty"`
	Output    []OpenAIResponseItem   `json:"output"`
	Usage     *OpenAIResponsesUsage  `json:"usage,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// OpenAIResponseItem represents one Responses API output item.
type OpenAIResponseItem struct {
	ID        string                  `json:"id,omitempty"`
	Type      string                  `json:"type"`
	Status    string                  `json:"status,omitempty"`
	Role      string                  `json:"role,omitempty"`
	Content   []OpenAIResponseContent `json:"content,omitempty"`
	CallID    string                  `json:"call_id,omitempty"`
	Name      string                  `json:"name,omitempty"`
	Arguments string                  `json:"arguments,omitempty"`
}

// OpenAIResponseContent represents one Responses API content part.
type OpenAIResponseContent struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Annotations []any  `json:"annotations,omitempty"`
}

// OpenAIResponsesUsage represents Responses API token usage.
type OpenAIResponsesUsage struct {
	InputTokens         int                       `json:"input_tokens"`
	InputTokensDetails  OpenAIInputTokensDetails  `json:"input_tokens_details"`
	OutputTokens        int                       `json:"output_tokens"`
	OutputTokensDetails OpenAIOutputTokensDetails `json:"output_tokens_details"`
	TotalTokens         int                       `json:"total_tokens"`
}

// OpenAIInputTokensDetails represents Responses API input token details.
type OpenAIInputTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// OpenAIOutputTokensDetails represents Responses API output token details.
type OpenAIOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

// OpenAIResponsesToCommandCode converts a Responses request to CommandCode format.
func OpenAIResponsesToCommandCode(req *OpenAIResponsesRequest) (*CommandCodeRequest, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	messages, systemText := responsesInputToCommandCode(req.Input)
	system := strings.Join(nonEmptyStrings(req.Instructions, systemText), "\n\n")
	tools := make([]CommandCodeTool, 0, len(req.Tools))
	for _, tool := range req.Tools {
		ccTool := openAIToolToCommandCode(tool)
		if ccTool.Name == "" {
			continue
		}
		tools = append(tools, ccTool)
	}

	ccReq := baseCommandCodeRequest()
	ccReq.Params = CommandCodeParams{
		Model:             req.Model,
		Messages:          messages,
		MaxTokens:         defaultMaxTokens(req.MaxOutputTokens),
		Stream:            true,
		System:            system,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		Metadata:          req.Metadata,
		ReasoningEffort:   req.ReasoningEffort,
		Thinking:          req.Thinking,
		ContextManagement: req.ContextManagement,
		OutputConfig:      req.OutputConfig,
		Tools:             tools,
		ToolChoice:        openAIToolChoiceToCommandCode(req.ToolChoice),
		ParallelToolCalls: req.ParallelToolCalls,
	}
	return ccReq, nil
}

// OpenAIResponsesCompactToCommandCode converts a Responses compact request to CommandCode format.
func OpenAIResponsesCompactToCommandCode(req *OpenAIResponsesCompactRequest) (*CommandCodeRequest, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}
	messages, systemText := responsesInputToCommandCode(req.Input)
	system := strings.Join(nonEmptyStrings(
		"Compact the provided conversation while preserving the information needed to continue it. Return only the compacted context.",
		systemText,
	), "\n\n")
	ccReq := baseCommandCodeRequest()
	ccReq.Params = CommandCodeParams{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: 64000,
		Stream:    true,
		System:    system,
	}
	return ccReq, nil
}

// BuildOpenAIResponseObject builds a non-streaming OpenAI Responses API response.
func BuildOpenAIResponseObject(id string, model string, created int64, text string, toolCalls []ToolCall, usage *Usage) *OpenAIResponseObject {
	output := []OpenAIResponseItem{}
	if text != "" {
		output = append(output, OpenAIResponseItem{
			ID:     "msg_" + randomHex(8),
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []OpenAIResponseContent{{
				Type:        "output_text",
				Text:        text,
				Annotations: []any{},
			}},
		})
	}
	for _, toolCall := range toolCalls {
		output = append(output, OpenAIResponseItem{
			ID:        "fc_" + randomHex(8),
			Type:      "function_call",
			Status:    "completed",
			CallID:    toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: toolCall.Function.Arguments,
		})
	}
	return &OpenAIResponseObject{
		ID:        id,
		Object:    "response",
		CreatedAt: created,
		Status:    "completed",
		Model:     model,
		Output:    output,
		Usage:     responsesUsageFromCommandCode(usage),
	}
}

// BuildOpenAICompactionObject builds a Responses compaction response.
func BuildOpenAICompactionObject(id string, created int64, text string, usage *Usage) *OpenAIResponseObject {
	return &OpenAIResponseObject{
		ID:        id,
		Object:    "response.compaction",
		CreatedAt: created,
		Status:    "completed",
		Output: []OpenAIResponseItem{{
			ID:     "cmp_" + randomHex(8),
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []OpenAIResponseContent{{
				Type:        "output_text",
				Text:        text,
				Annotations: []any{},
			}},
		}},
		Usage: responsesUsageFromCommandCode(usage),
	}
}

// responsesInputToCommandCode converts the Responses input into CommandCode
// messages plus any system/developer text extracted from message items. The
// upstream only accepts user/assistant/tool roles in params.messages, so
// system-role items are lifted into the top-level system prompt instead.
func responsesInputToCommandCode(input any) ([]CommandCodeMessage, string) {
	switch v := input.(type) {
	case string:
		if v == "" {
			return nil, ""
		}
		return []CommandCodeMessage{{Role: "user", Content: []CommandCodeContent{{Type: "text", Text: v}}}}, ""
	case []any:
		toolNameByID := responsesToolNameMap(v)
		systemParts := []string{}
		messages := make([]CommandCodeMessage, 0, len(v))
		for _, item := range v {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if responsesItemIsSystem(itemMap) {
				if text := contentPartToString(itemMap["content"]); text != "" {
					systemParts = append(systemParts, text)
				}
				continue
			}
			if itemType, _ := itemMap["type"].(string); itemType == "reasoning" {
				// Reasoning items are client-side replay of prior thinking;
				// the upstream has no such message role, so drop them instead
				// of leaking their text as user content.
				continue
			}
			if msg, ok := responseInputItemToCommandCode(itemMap, toolNameByID); ok {
				messages = append(messages, msg)
			}
		}
		return messages, strings.Join(systemParts, "\n\n")
	default:
		if text := contentPartToString(v); text != "" {
			return []CommandCodeMessage{{Role: "user", Content: []CommandCodeContent{{Type: "text", Text: text}}}}, ""
		}
	}
	return nil, ""
}

// responsesItemIsSystem reports whether an input item is a message carrying a
// system or developer role, which must not reach params.messages.
func responsesItemIsSystem(itemMap map[string]any) bool {
	itemType, _ := itemMap["type"].(string)
	if itemType != "message" && itemType != "" {
		return false
	}
	role, _ := itemMap["role"].(string)
	return role == "system" || role == "developer"
}

func nonEmptyStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

func responseInputItemToCommandCode(itemMap map[string]any, toolNameByID map[string]string) (CommandCodeMessage, bool) {
	itemType, _ := itemMap["type"].(string)
	switch itemType {
	case "function_call_output":
		callID := firstString(itemMap, "call_id", "tool_call_id", "toolCallId", "id")
		toolName := toolNameByID[callID]
		if toolName == "" {
			toolName = firstString(itemMap, "name", "toolName")
		}
		if toolName == "" {
			toolName = "unknown"
		}
		return CommandCodeMessage{Role: "tool", Content: []CommandCodeContent{{
			Type:       "tool-result",
			ToolCallID: callID,
			ToolName:   toolName,
			Output: &CommandCodeToolOutput{
				Type:  "text",
				Value: contentPartToString(itemMap["output"]),
			},
		}}}, true
	case "function_call":
		callID := firstString(itemMap, "call_id", "tool_call_id", "toolCallId", "id")
		name := firstString(itemMap, "name", "toolName")
		arguments := contentPartToString(itemMap["arguments"])
		return CommandCodeMessage{Role: "assistant", Content: []CommandCodeContent{{
			Type:       "tool-call",
			ToolCallID: callID,
			ToolName:   name,
			Input:      TryParseJSON(arguments),
		}}}, true
	case "message", "":
		role := firstString(itemMap, "role")
		if role == "" {
			role = "user"
		}
		content := openAIContentToCommandCode(itemMap["content"], toolNameByID)
		if len(content) == 0 {
			if text := contentPartToString(itemMap["content"]); text != "" {
				content = []CommandCodeContent{{Type: "text", Text: text}}
			}
		}
		if len(content) == 0 {
			return CommandCodeMessage{}, false
		}
		if commandCodeContentHasType(content, "tool-result") {
			role = "tool"
		}
		return CommandCodeMessage{Role: role, Content: content}, true
	default:
		if text := contentPartToString(itemMap); text != "" {
			return CommandCodeMessage{Role: "user", Content: []CommandCodeContent{{Type: "text", Text: text}}}, true
		}
	}
	return CommandCodeMessage{}, false
}

func responsesToolNameMap(items []any) map[string]string {
	toolNameByID := map[string]string{}
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if itemType, _ := itemMap["type"].(string); itemType != "function_call" {
			continue
		}
		callID := firstString(itemMap, "call_id", "tool_call_id", "toolCallId", "id")
		name := firstString(itemMap, "name", "toolName")
		if callID != "" && name != "" {
			toolNameByID[callID] = name
		}
	}
	return toolNameByID
}

func responsesUsageFromCommandCode(usage *Usage) *OpenAIResponsesUsage {
	if usage == nil {
		return nil
	}
	return &OpenAIResponsesUsage{
		InputTokens: usage.InputTokens,
		InputTokensDetails: OpenAIInputTokensDetails{
			CachedTokens: usage.CachedInputTokens,
		},
		OutputTokens: usage.OutputTokens,
		OutputTokensDetails: OpenAIOutputTokensDetails{
			ReasoningTokens: 0,
		},
		TotalTokens: usage.InputTokens + usage.OutputTokens,
	}
}

func baseCommandCodeRequest() *CommandCodeRequest {
	workingDir, _ := config.GetWorkingDir()
	dateStr := config.GetDateStr()
	env := config.GetEnvironment()
	isGitRepo, currentBranch, mainBranch, gitStatus, recentCommits := config.GetGitInfo()
	return &CommandCodeRequest{
		Config: CommandCodeConfig{
			WorkingDir:    workingDir,
			Date:          dateStr,
			Environment:   env,
			Structure:     []string{},
			IsGitRepo:     isGitRepo,
			CurrentBranch: currentBranch,
			MainBranch:    mainBranch,
			GitStatus:     gitStatus,
			RecentCommits: recentCommits,
		},
		Memory:         nil,
		Taste:          nil,
		Skills:         "",
		PermissionMode: "standard",
	}
}

// ResponseID builds an OpenAI Responses API identifier.
func ResponseID(prefix string) string {
	return prefix + "_" + randomHex(12)
}
