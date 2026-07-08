package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
		if msg.Role == "system" {
			continue
		}
		ccMsg, err := openaiMessageToCommandCode(msg, toolNameByID)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message: %w", err)
		}
		if len(ccMsg.Content) == 0 {
			continue
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
	systemParts := []string{}
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if str, ok := msg.Content.(string); ok && str != "" {
				systemParts = append(systemParts, str)
			}
		}
	}
	system := strings.Join(systemParts, "\n\n")

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
			TopP:              req.TopP,
			StopSequences:     req.StopSequences,
			Metadata:          req.Metadata,
			ReasoningEffort:   req.ReasoningEffort,
			Thinking:          req.Thinking,
			ContextManagement: req.ContextManagement,
			OutputConfig:      req.OutputConfig,
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

// anthropicToolChoiceToCommandCode maps Anthropic tool choice values to CommandCode format.
func anthropicToolChoiceToCommandCode(toolChoice any) any {
	choice, ok := toolChoice.(map[string]any)
	if !ok {
		return toolChoice
	}

	switch choice["type"] {
	case nil, "auto", "any", "none":
		return map[string]any{"type": choice["type"]}
	case "tool":
		return map[string]any{"type": "tool", "name": choice["name"]}
	default:
		return toolChoice
	}
}

// AnthropicMessagesToCommandCode converts an Anthropic Messages request to CommandCode format.
func AnthropicMessagesToCommandCode(req *AnthropicRequest) (*CommandCodeRequest, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	ccMessages := make([]CommandCodeMessage, 0, len(req.Messages))
	systemParts := []string{}
	if system := anthropicSystemToString(req.System); system != "" {
		systemParts = append(systemParts, system)
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if system := anthropicSystemToString(msg.Content); system != "" {
				systemParts = append(systemParts, system)
			}
			continue
		}

		ccMsg, err := anthropicMessageToCommandCode(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message: %w", err)
		}
		if len(ccMsg.Content) == 0 {
			continue
		}
		ccMessages = append(ccMessages, ccMsg)
	}

	ccTools := make([]CommandCodeTool, 0, len(req.Tools))
	for _, tool := range req.Tools {
		ccTools = append(ccTools, anthropicToolToCommandCode(tool))
	}

	reasoningEffort := ""
	if req.Thinking != nil {
		reasoningEffort = req.Thinking.Effort
		if reasoningEffort == "" && req.Thinking.BudgetTokens > 0 {
			reasoningEffort = fmt.Sprintf("high-%d", req.Thinking.BudgetTokens)
		}
	}

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
		Params: CommandCodeParams{
			Model:             req.Model,
			Messages:          ccMessages,
			MaxTokens:         defaultMaxTokens(req.MaxTokens),
			Stream:            true,
			System:            strings.Join(systemParts, "\n\n"),
			Temperature:       req.Temperature,
			TopP:              req.TopP,
			StopSequences:     req.StopSequences,
			Metadata:          req.Metadata,
			ReasoningEffort:   reasoningEffort,
			Thinking:          req.Thinking,
			ContextManagement: req.ContextManagement,
			OutputConfig:      req.OutputConfig,
			Tools:             ccTools,
			ToolChoice:        anthropicToolChoiceToCommandCode(req.ToolChoice),
		},
	}, nil
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

// CommandCodeToAnthropicMessages builds an Anthropic Messages response from CommandCode output.
func CommandCodeToAnthropicMessages(messageID string, model string, content []AnthropicContent, finishReason string, usage *Usage) *AnthropicResponse {
	anthropicUsage := &AnthropicUsage{}
	if usage != nil {
		anthropicUsage.InputTokens = usage.InputTokens
		anthropicUsage.OutputTokens = usage.OutputTokens
	}

	return &AnthropicResponse{
		ID:         messageID,
		Type:       "message",
		Role:       "assistant",
		Content:    content,
		Model:      model,
		StopReason: MapFinishReason(finishReason),
		Usage:      anthropicUsage,
	}
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
