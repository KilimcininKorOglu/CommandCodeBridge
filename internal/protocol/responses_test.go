package protocol

import "testing"

func TestOpenAIResponsesToCommandCodeConvertsStringInput(t *testing.T) {
	req := &OpenAIResponsesRequest{Model: "model", Input: "hello", Instructions: "be concise", MaxOutputTokens: 10}

	ccReq, err := OpenAIResponsesToCommandCode(req)
	if err != nil {
		t.Fatalf("OpenAIResponsesToCommandCode() error = %v", err)
	}
	if got, want := ccReq.Params.System, "be concise"; got != want {
		t.Fatalf("System = %q, want %q", got, want)
	}
	if got, want := ccReq.Params.MaxTokens, 10; got != want {
		t.Fatalf("MaxTokens = %d, want %d", got, want)
	}
	if got, want := ccReq.Params.Messages[0].Role, "user"; got != want {
		t.Fatalf("Role = %q, want %q", got, want)
	}
	if got, want := ccReq.Params.Messages[0].Content[0].Text, "hello"; got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
}

func TestOpenAIResponsesToCommandCodeConvertsTopLevelFunctionTool(t *testing.T) {
	req := &OpenAIResponsesRequest{
		Model: "model",
		Input: "hello",
		Tools: []OpenAITool{{
			Type:        "function",
			Name:        "Read",
			Description: "Read a file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
			},
		}},
	}

	ccReq, err := OpenAIResponsesToCommandCode(req)
	if err != nil {
		t.Fatalf("OpenAIResponsesToCommandCode() error = %v", err)
	}
	if got, want := ccReq.Params.Tools[0].Type, "function"; got != want {
		t.Fatalf("Tool.Type = %q, want %q", got, want)
	}
	if got, want := ccReq.Params.Tools[0].Name, "Read"; got != want {
		t.Fatalf("Tool.Name = %q, want %q", got, want)
	}
	if got, want := ccReq.Params.Tools[0].Description, "Read a file"; got != want {
		t.Fatalf("Tool.Description = %q, want %q", got, want)
	}
	if ccReq.Params.Tools[0].InputSchema == nil {
		t.Fatal("Tool.InputSchema = nil, want schema")
	}
}

func TestOpenAIResponsesToCommandCodeSkipsUnsupportedCustomTools(t *testing.T) {
	req := &OpenAIResponsesRequest{
		Model: "model",
		Input: "hello",
		Tools: []OpenAITool{{
			Type:        "custom",
			Name:        "ApplyPatch",
			Description: "Apply a patch",
		}},
	}

	ccReq, err := OpenAIResponsesToCommandCode(req)
	if err != nil {
		t.Fatalf("OpenAIResponsesToCommandCode() error = %v", err)
	}
	if got := len(ccReq.Params.Tools); got != 0 {
		t.Fatalf("len(Tools) = %d, want 0", got)
	}
}

func TestOpenAIResponsesToCommandCodeConvertsFunctionCallOutput(t *testing.T) {
	req := &OpenAIResponsesRequest{
		Model: "model",
		Input: []any{
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "lookup", "arguments": `{"q":"x"}`},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "result"},
		},
	}

	ccReq, err := OpenAIResponsesToCommandCode(req)
	if err != nil {
		t.Fatalf("OpenAIResponsesToCommandCode() error = %v", err)
	}
	if got, want := ccReq.Params.Messages[1].Role, "tool"; got != want {
		t.Fatalf("Role = %q, want %q", got, want)
	}
	toolResult := ccReq.Params.Messages[1].Content[0]
	if got, want := toolResult.Type, "tool-result"; got != want {
		t.Fatalf("Type = %q, want %q", got, want)
	}
	if got, want := toolResult.ToolName, "lookup"; got != want {
		t.Fatalf("ToolName = %q, want %q", got, want)
	}
	if toolResult.Output == nil {
		t.Fatal("Output = nil, want output")
	}
	if got, want := toolResult.Output.Value, "result"; got != want {
		t.Fatalf("Output.Value = %q, want %q", got, want)
	}
}

func TestBuildOpenAIResponseObjectIncludesTextAndToolCalls(t *testing.T) {
	resp := BuildOpenAIResponseObject("resp_1", "model", 1, "hello", []ToolCall{{ID: "call_1", Type: "function", Function: FunctionCall{Name: "lookup", Arguments: `{"q":"x"}`}}}, &Usage{InputTokens: 2, OutputTokens: 3})

	if got, want := resp.Object, "response"; got != want {
		t.Fatalf("Object = %q, want %q", got, want)
	}
	if got, want := len(resp.Output), 2; got != want {
		t.Fatalf("len(Output) = %d, want %d", got, want)
	}
	if got, want := resp.Output[0].Content[0].Text, "hello"; got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
	if got, want := resp.Output[1].Type, "function_call"; got != want {
		t.Fatalf("Type = %q, want %q", got, want)
	}
	if got, want := resp.Usage.TotalTokens, 5; got != want {
		t.Fatalf("TotalTokens = %d, want %d", got, want)
	}
}
