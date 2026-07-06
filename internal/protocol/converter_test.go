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
