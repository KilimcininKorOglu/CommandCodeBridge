package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kilimcininkoroglu/commandcode-bridge/internal/config"
)

// OpenAIToCommandCode converts an OpenAI request to CommandCode format
func OpenAIToCommandCode(req *OpenAIRequest) (*CommandCodeRequest, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	// Convert messages
	toolNameByID := buildOpenAIToolNameMap(req.Messages)
	ccMessages := make([]CommandCodeMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		ccMsg, err := openaiMessageToCommandCode(msg, toolNameByID)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message: %w", err)
		}
		ccMessages = append(ccMessages, ccMsg)
	}

	// Convert tools
	ccTools := make([]CommandCodeTool, 0, len(req.Tools))
	for _, tool := range req.Tools {
		ccTool := openAIToolToCommandCode(tool)
		ccTools = append(ccTools, ccTool)
	}

	// Build system message from OpenAI messages if present
	system := ""
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if str, ok := msg.Content.(string); ok {
				system = str
			}
		}
	}

	// Get environment info
	workingDir, _ := config.GetWorkingDir()
	dateStr := config.GetDateStr()
	env := config.GetEnvironment()
	isGitRepo, currentBranch, mainBranch, gitStatus, recentCommits := config.GetGitInfo()

	ccReq := &CommandCodeRequest{
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
		Params: CommandCodeParams{
			Model:             req.Model,
			Messages:          ccMessages,
			MaxTokens:         defaultMaxTokens(req.MaxTokens),
			Stream:            true,
			System:            system,
			Temperature:       req.Temperature,
			ReasoningEffort:   req.ReasoningEffort,
			Tools:             ccTools,
			ToolChoice:        openAIToolChoiceToCommandCode(req.ToolChoice),
			ParallelToolCalls: req.ParallelToolCalls,
		},
	}

	return ccReq, nil
}

// defaultMaxTokens returns the upstream default when the OpenAI request omits max_tokens.
func defaultMaxTokens(maxTokens int) int {
	if maxTokens <= 0 {
		return 64000
	}
	if maxTokens > 200000 {
		return 200000
	}
	return maxTokens
}

// openAIToolChoiceToCommandCode maps OpenAI tool choice values to CommandCode format.
func openAIToolChoiceToCommandCode(toolChoice any) any {
	switch choice := toolChoice.(type) {
	case string:
		switch choice {
		case "auto", "none":
			return map[string]any{"type": choice}
		case "required":
			return map[string]any{"type": "any"}
		default:
			return map[string]any{"type": "auto"}
		}
	case map[string]any:
		if choice["type"] == "function" {
			if fn, ok := choice["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok {
					return map[string]any{"type": "tool", "name": name}
				}
			}
		}
	}
	return toolChoice
}

// anthropicToolChoiceToOpenAI maps Anthropic tool choice values to OpenAI format.
func anthropicToolChoiceToOpenAI(toolChoice any) any {
	choice, ok := toolChoice.(map[string]any)
	if !ok {
		return toolChoice
	}

	switch choice["type"] {
	case nil, "auto":
		return "auto"
	case "any":
		return "required"
	case "none":
		return "none"
	case "tool":
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": choice["name"],
			},
		}
	default:
		return toolChoice
	}
}

// CommandCodeToOpenAI converts a CommandCode response to OpenAI format
func CommandCodeToOpenAI(ccResp *CommandCodeResponse) (*OpenAIResponse, error) {
	if ccResp == nil {
		return nil, errors.New("nil response")
	}

	choices := []OpenAIChoice{
		{
			Index:        0,
			Message:      &OpenAIMessage{Role: "assistant", Content: ccResp.Content},
			FinishReason: ccResp.FinishReason,
		},
	}

	return &OpenAIResponse{
		ID:      ccResp.ID,
		Object:  "chat.completion",
		Created: ccResp.Created,
		Model:   ccResp.Model,
		Choices: choices,
		Usage:   ccResp.Usage,
	}, nil
}

// AnthropicToOpenAI converts an Anthropic request to OpenAI format
func AnthropicToOpenAI(req *AnthropicRequest) (*OpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	// Convert messages
	openaiMessages := make([]OpenAIMessage, 0, len(req.Messages)+1)

	// Add system message if present
	if req.System != nil {
		if str, ok := req.System.(string); ok && str != "" {
			openaiMessages = append(openaiMessages, OpenAIMessage{
				Role:    "system",
				Content: str,
			})
		}
	}

	// Convert Anthropic messages to OpenAI format
	for _, msg := range req.Messages {
		openaiMsg, err := anthropicMessageToOpenAI(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message: %w", err)
		}
		openaiMessages = append(openaiMessages, openaiMsg)
	}

	// Convert tools
	openaiTools := make([]OpenAITool, 0, len(req.Tools))
	for _, tool := range req.Tools {
		openaiTool := anthropicToolToOpenAI(tool)
		openaiTools = append(openaiTools, openaiTool)
	}

	// Map thinking budget_tokens to reasoning_effort
	reasoningEffort := ""
	if req.Thinking != nil && req.Thinking.BudgetTokens > 0 {
		reasoningEffort = fmt.Sprintf("high-%d", req.Thinking.BudgetTokens)
	}

	return &OpenAIRequest{
		Model:           req.Model,
		Messages:        openaiMessages,
		MaxTokens:       req.MaxTokens,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		Tools:           openaiTools,
		ToolChoice:      anthropicToolChoiceToOpenAI(req.ToolChoice),
		ReasoningEffort: reasoningEffort,
	}, nil
}

// OpenAIToAnthropic converts an OpenAI response to Anthropic format
func OpenAIToAnthropic(openaiResp *OpenAIResponse) (*AnthropicResponse, error) {
	if openaiResp == nil {
		return nil, errors.New("nil response")
	}

	if len(openaiResp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	choice := openaiResp.Choices[0]
	if choice.Message == nil {
		return nil, errors.New("missing message in response choice")
	}

	content := []AnthropicContent{}
	if text, ok := choice.Message.Content.(string); ok && text != "" {
		content = append(content, AnthropicContent{Type: "text", Text: text})
	}
	content = append(content, toolCallsToAnthropicContent(choice.Message.ToolCalls)...)

	usage := &AnthropicUsage{}
	if openaiResp.Usage != nil {
		usage.InputTokens = openaiResp.Usage.InputTokens
		usage.OutputTokens = openaiResp.Usage.OutputTokens
	}

	return &AnthropicResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    content,
		Model:      openaiResp.Model,
		StopReason: MapFinishReason(choice.FinishReason),
		Usage:      usage,
	}, nil
}

func toolCallsToAnthropicContent(toolCalls []ToolCall) []AnthropicContent {
	content := make([]AnthropicContent, 0, len(toolCalls))
	for _, tc := range toolCalls {
		input := map[string]any{}
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		content = append(content, AnthropicContent{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	return content
}

// MapFinishReason maps OpenAI finish reason to Anthropic stop reason
func MapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "tool-calls":
		return "tool_use"
	case "content_filter":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}

// MapAnthropicStopReason maps Anthropic stop reason to OpenAI finish reason
func MapAnthropicStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

// BuildAnthropicResponse builds Anthropic format response from components
func BuildAnthropicResponse(model string, fullText string, toolCalls []ToolCall, finishReason string, usage *Usage) *AnthropicResponse {
	content := []AnthropicContent{}

	if fullText != "" {
		content = append(content, AnthropicContent{
			Type: "text",
			Text: fullText,
		})
	}

	content = append(content, toolCallsToAnthropicContent(toolCalls)...)

	anthropicUsage := &AnthropicUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}

	return &AnthropicResponse{
		ID:         "msg_" + randomHex(8),
		Type:       "message",
		Role:       "assistant",
		Content:    content,
		Model:      model,
		StopReason: MapFinishReason(finishReason),
		Usage:      anthropicUsage,
	}
}

// randomHex generates random hex string
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
