package protocol

import "testing"

func TestOpenAIToCommandCodeMapsToolChoiceRequiredToAny(t *testing.T) {
	req := &OpenAIRequest{
		Model: "model",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "run a tool"},
		},
		ToolChoice: "required",
	}

	ccReq, err := OpenAIToCommandCode(req)
	if err != nil {
		t.Fatalf("OpenAIToCommandCode() error = %v", err)
	}

	choice, ok := ccReq.Params.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("ToolChoice = %T, want map", ccReq.Params.ToolChoice)
	}
	if got, want := choice["type"], "any"; got != want {
		t.Fatalf("ToolChoice.type = %v, want %v", got, want)
	}
}

func TestOpenAIToCommandCodeMapsFunctionToolChoiceToTool(t *testing.T) {
	req := &OpenAIRequest{
		Model: "model",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "run a tool"},
		},
		ToolChoice: map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": "lookup",
			},
		},
	}

	ccReq, err := OpenAIToCommandCode(req)
	if err != nil {
		t.Fatalf("OpenAIToCommandCode() error = %v", err)
	}

	choice, ok := ccReq.Params.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("ToolChoice = %T, want map", ccReq.Params.ToolChoice)
	}
	if got, want := choice["type"], "tool"; got != want {
		t.Fatalf("ToolChoice.type = %v, want %v", got, want)
	}
	if got, want := choice["name"], "lookup"; got != want {
		t.Fatalf("ToolChoice.name = %v, want %v", got, want)
	}
}

func TestOpenAIToCommandCodeMovesSystemMessagesOutOfMessageList(t *testing.T) {
	req := &OpenAIRequest{
		Model: "model",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "first"},
			{Role: "user", Content: "hello"},
			{Role: "system", Content: "second"},
		},
	}

	ccReq, err := OpenAIToCommandCode(req)
	if err != nil {
		t.Fatalf("OpenAIToCommandCode() error = %v", err)
	}
	if got, want := ccReq.Params.System, "first\n\nsecond"; got != want {
		t.Fatalf("System = %q, want %q", got, want)
	}
	if got, want := len(ccReq.Params.Messages), 1; got != want {
		t.Fatalf("len(Messages) = %d, want %d", got, want)
	}
	if got, want := ccReq.Params.Messages[0].Role, "user"; got != want {
		t.Fatalf("Messages[0].Role = %q, want %q", got, want)
	}
}

func TestOpenAIToCommandCodeConvertsAssistantToolCallsAndToolResponses(t *testing.T) {
	req := &OpenAIRequest{
		Model: "model",
		Messages: []OpenAIMessage{
			{
				Role:    "assistant",
				Content: nil,
				ToolCalls: []ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: FunctionCall{
							Name:      "lookup",
							Arguments: `{"q":"x"}`,
						},
					},
				},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "result"},
		},
	}

	ccReq, err := OpenAIToCommandCode(req)
	if err != nil {
		t.Fatalf("OpenAIToCommandCode() error = %v", err)
	}
	if got, want := len(ccReq.Params.Messages), 2; got != want {
		t.Fatalf("len(Messages) = %d, want %d", got, want)
	}
	toolCall := ccReq.Params.Messages[0].Content[0]
	if got, want := toolCall.Type, "tool-call"; got != want {
		t.Fatalf("tool call Type = %q, want %q", got, want)
	}
	if got, want := toolCall.ToolCallID, "call_1"; got != want {
		t.Fatalf("tool call ID = %q, want %q", got, want)
	}
	if got, want := toolCall.ToolName, "lookup"; got != want {
		t.Fatalf("tool call name = %q, want %q", got, want)
	}
	input, ok := toolCall.Input.(map[string]any)
	if !ok {
		t.Fatalf("tool call input = %T, want map", toolCall.Input)
	}
	if got, want := input["q"], any("x"); got != want {
		t.Fatalf("tool call input q = %v, want %v", got, want)
	}
	toolResult := ccReq.Params.Messages[1]
	if got, want := toolResult.Role, "tool"; got != want {
		t.Fatalf("tool result role = %q, want %q", got, want)
	}
	if got, want := toolResult.Content[0].Type, "tool-result"; got != want {
		t.Fatalf("tool result Type = %q, want %q", got, want)
	}
	if got, want := toolResult.Content[0].ToolName, "lookup"; got != want {
		t.Fatalf("tool result name = %q, want %q", got, want)
	}
	if toolResult.Content[0].Output == nil {
		t.Fatal("tool result Output = nil, want output")
	}
	if got, want := toolResult.Content[0].Output.Value, "result"; got != want {
		t.Fatalf("tool result output = %q, want %q", got, want)
	}
}

func TestAnthropicToOpenAIConvertsSystemArrayAndMidConversationSystem(t *testing.T) {
	req := &AnthropicRequest{
		Model: "model",
		System: []any{
			map[string]any{"type": "text", "text": "first"},
			map[string]any{"type": "text", "text": "second"},
		},
		Messages: []AnthropicMessage{
			{Role: "user", Content: "hello"},
			{Role: "system", Content: "mid"},
		},
	}

	openAIReq, err := AnthropicToOpenAI(req)
	if err != nil {
		t.Fatalf("AnthropicToOpenAI() error = %v", err)
	}
	ccReq, err := OpenAIToCommandCode(openAIReq)
	if err != nil {
		t.Fatalf("OpenAIToCommandCode() error = %v", err)
	}
	if got, want := ccReq.Params.System, "first\n\nsecond\n\nmid"; got != want {
		t.Fatalf("System = %q, want %q", got, want)
	}
	if got, want := len(ccReq.Params.Messages), 1; got != want {
		t.Fatalf("len(Messages) = %d, want %d", got, want)
	}
	if got, want := ccReq.Params.Messages[0].Role, "user"; got != want {
		t.Fatalf("Messages[0].Role = %q, want %q", got, want)
	}
}

func TestAnthropicToOpenAIPreservesClaudeCodeRequestFields(t *testing.T) {
	req := &AnthropicRequest{
		Model:         "model",
		MaxTokens:     10,
		TopP:          0.7,
		StopSequences: []string{"stop-here"},
		Metadata:      &AnthropicMetadata{UserID: "user-id"},
		Thinking:      &AnthropicThinking{Type: "enabled", Effort: "high"},
		ContextManagement: map[string]any{
			"edits": true,
		},
		OutputConfig: map[string]any{
			"format": map[string]any{"type": "json_object"},
		},
		Messages: []AnthropicMessage{{Role: "user", Content: "hello"}},
	}

	openAIReq, err := AnthropicToOpenAI(req)
	if err != nil {
		t.Fatalf("AnthropicToOpenAI() error = %v", err)
	}
	ccReq, err := OpenAIToCommandCode(openAIReq)
	if err != nil {
		t.Fatalf("OpenAIToCommandCode() error = %v", err)
	}
	if got, want := ccReq.Params.TopP, 0.7; got != want {
		t.Fatalf("TopP = %v, want %v", got, want)
	}
	if got, want := ccReq.Params.StopSequences[0], "stop-here"; got != want {
		t.Fatalf("StopSequences[0] = %q, want %q", got, want)
	}
	metadata, ok := ccReq.Params.Metadata.(*AnthropicMetadata)
	if !ok {
		t.Fatalf("Metadata = %T, want *AnthropicMetadata", ccReq.Params.Metadata)
	}
	if got, want := metadata.UserID, "user-id"; got != want {
		t.Fatalf("Metadata.UserID = %q, want %q", got, want)
	}
	if got, want := ccReq.Params.ReasoningEffort, "high"; got != want {
		t.Fatalf("ReasoningEffort = %q, want %q", got, want)
	}
	if ccReq.Params.Thinking == nil {
		t.Fatal("Thinking = nil, want preserved thinking config")
	}
	if ccReq.Params.ContextManagement == nil {
		t.Fatal("ContextManagement = nil, want preserved context management")
	}
	if ccReq.Params.OutputConfig == nil {
		t.Fatal("OutputConfig = nil, want preserved output config")
	}
}

func TestAnthropicToOpenAIMapsToolChoiceAnyToRequired(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "model",
		MaxTokens: 10,
		Messages: []AnthropicMessage{
			{Role: "user", Content: "run a tool"},
		},
		ToolChoice: map[string]any{"type": "any"},
	}

	openAIReq, err := AnthropicToOpenAI(req)
	if err != nil {
		t.Fatalf("AnthropicToOpenAI() error = %v", err)
	}
	if got, want := openAIReq.ToolChoice, "required"; got != want {
		t.Fatalf("ToolChoice = %v, want %v", got, want)
	}
}

func TestAnthropicToOpenAIMapsToolChoiceToolToFunction(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "model",
		MaxTokens: 10,
		Messages: []AnthropicMessage{
			{Role: "user", Content: "run a tool"},
		},
		ToolChoice: map[string]any{"type": "tool", "name": "lookup"},
	}

	openAIReq, err := AnthropicToOpenAI(req)
	if err != nil {
		t.Fatalf("AnthropicToOpenAI() error = %v", err)
	}
	choice, ok := openAIReq.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("ToolChoice = %T, want map", openAIReq.ToolChoice)
	}
	if got, want := choice["type"], "function"; got != want {
		t.Fatalf("ToolChoice.type = %v, want %v", got, want)
	}
	function, ok := choice["function"].(map[string]any)
	if !ok {
		t.Fatalf("ToolChoice.function = %T, want map", choice["function"])
	}
	if got, want := function["name"], "lookup"; got != want {
		t.Fatalf("ToolChoice.function.name = %v, want %v", got, want)
	}
}

func TestOpenAIToAnthropicConvertsReasoningContentToThinkingBlock(t *testing.T) {
	resp := &OpenAIResponse{
		ID:    "chatcmpl-test",
		Model: "model",
		Choices: []OpenAIChoice{
			{
				Message:      &OpenAIMessage{Role: "assistant", Content: "answer", ReasoningContent: "thinking"},
				FinishReason: "stop",
			},
		},
		Usage: &Usage{InputTokens: 3, OutputTokens: 4},
	}

	anthropicResp, err := OpenAIToAnthropic(resp)
	if err != nil {
		t.Fatalf("OpenAIToAnthropic() error = %v", err)
	}
	if got, want := len(anthropicResp.Content), 2; got != want {
		t.Fatalf("len(Content) = %d, want %d", got, want)
	}
	if got, want := anthropicResp.Content[0].Type, "thinking"; got != want {
		t.Fatalf("Content[0].Type = %q, want %q", got, want)
	}
	if got, want := anthropicResp.Content[0].Thinking, "thinking"; got != want {
		t.Fatalf("Content[0].Thinking = %q, want %q", got, want)
	}
}

func TestOpenAIToAnthropicConvertsToolCallsWithoutTextContent(t *testing.T) {
	resp := &OpenAIResponse{
		ID:    "chatcmpl-test",
		Model: "model",
		Choices: []OpenAIChoice{
			{
				Message: &OpenAIMessage{
					Role:    "assistant",
					Content: nil,
					ToolCalls: []ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: FunctionCall{
								Name:      "lookup",
								Arguments: `{"q":"x"}`,
							},
						},
					},
				},
				FinishReason: "tool-calls",
			},
		},
		Usage: &Usage{InputTokens: 3, OutputTokens: 4},
	}

	anthropicResp, err := OpenAIToAnthropic(resp)
	if err != nil {
		t.Fatalf("OpenAIToAnthropic() error = %v", err)
	}
	if got, want := len(anthropicResp.Content), 1; got != want {
		t.Fatalf("len(Content) = %d, want %d", got, want)
	}
	content := anthropicResp.Content[0]
	if got, want := content.Type, "tool_use"; got != want {
		t.Fatalf("Content[0].Type = %q, want %q", got, want)
	}
	if got, want := content.ID, "call_1"; got != want {
		t.Fatalf("Content[0].ID = %q, want %q", got, want)
	}
	if got, want := content.Name, "lookup"; got != want {
		t.Fatalf("Content[0].Name = %q, want %q", got, want)
	}
	if got, want := content.Input["q"], any("x"); got != want {
		t.Fatalf("Content[0].Input[q] = %v, want %v", got, want)
	}
	if got, want := anthropicResp.StopReason, "tool_use"; got != want {
		t.Fatalf("StopReason = %q, want %q", got, want)
	}
}
