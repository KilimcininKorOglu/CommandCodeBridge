package http

import (
	"errors"
	"strings"
	"testing"
)

func TestCommandCodeStreamToOpenAIHandlesReasoningAndToolCalls(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`{"type":"reasoning-delta","text":"thinking"}`,
		`{"type":"tool-call","toolCallId":"call_1","toolName":"lookup","input":{"q":"x"}}`,
		`{"type":"finish","finishReason":"tool_calls","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))

	resp, err := commandCodeStreamToOpenAI(stream, "model", "chatcmpl-test", 1, nil)
	if err != nil {
		t.Fatalf("commandCodeStreamToOpenAI() error = %v", err)
	}
	message := resp.Choices[0].Message
	if message.ReasoningContent != "thinking" {
		t.Fatalf("ReasoningContent = %q, want thinking", message.ReasoningContent)
	}
	if len(message.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(message.ToolCalls))
	}
	if message.ToolCalls[0].Function.Arguments != `{"q":"x"}` {
		t.Fatalf("Arguments = %q, want JSON arguments", message.ToolCalls[0].Function.Arguments)
	}
}

func TestCommandCodeStreamToAnthropicMessagesHandlesReasoningAndToolCalls(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`{"type":"reasoning-delta","text":"thinking"}`,
		`{"type":"tool-call","toolCallId":"call_1","toolName":"lookup","input":{"q":"x"}}`,
		`{"type":"finish","finishReason":"tool_calls","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))

	resp, err := commandCodeStreamToAnthropicMessagesWithIdleTimeout(stream, "model", "msg-test", nil, 0, "")
	if err != nil {
		t.Fatalf("commandCodeStreamToAnthropicMessagesWithIdleTimeout() error = %v", err)
	}
	if got, want := len(resp.Content), 2; got != want {
		t.Fatalf("len(Content) = %d, want %d", got, want)
	}
	if got, want := resp.Content[0].Type, "thinking"; got != want {
		t.Fatalf("Content[0].Type = %q, want %q", got, want)
	}
	if got, want := resp.Content[0].Thinking, "thinking"; got != want {
		t.Fatalf("Content[0].Thinking = %q, want %q", got, want)
	}
	if got, want := resp.Content[1].Type, "tool_use"; got != want {
		t.Fatalf("Content[1].Type = %q, want %q", got, want)
	}
	if got, want := resp.Content[1].ID, "call_1"; got != want {
		t.Fatalf("Content[1].ID = %q, want %q", got, want)
	}
	if got, want := resp.Content[1].Name, "lookup"; got != want {
		t.Fatalf("Content[1].Name = %q, want %q", got, want)
	}
	if got, want := resp.Content[1].Input["q"], any("x"); got != want {
		t.Fatalf("Content[1].Input[q] = %v, want %v", got, want)
	}
	if got, want := resp.StopReason, "tool_use"; got != want {
		t.Fatalf("StopReason = %q, want %q", got, want)
	}
}

func TestCommandCodeStreamToOpenAIIgnoresErrorEvents(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`{"type":"error","message":"upstream warning"}`,
		`{"type":"text-delta","text":"hello"}`,
		`{"type":"finish","finishReason":"stop","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))

	resp, err := commandCodeStreamToOpenAI(stream, "model", "chatcmpl-test", 1, nil)
	if err != nil {
		t.Fatalf("commandCodeStreamToOpenAI() error = %v", err)
	}
	if got, want := resp.Choices[0].Message.Content, any("hello"); got != want {
		t.Fatalf("Content = %v, want %v", got, want)
	}
}

func TestCommandCodeStreamToOpenAIUsesDeltaFallbackForTextDelta(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`{"type":"text-delta","delta":"fallback"}`,
		`{"type":"finish","finishReason":"stop","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))

	resp, err := commandCodeStreamToOpenAI(stream, "model", "chatcmpl-test", 1, nil)
	if err != nil {
		t.Fatalf("commandCodeStreamToOpenAI() error = %v", err)
	}
	if got, want := resp.Choices[0].Message.Content, any("fallback"); got != want {
		t.Fatalf("Content = %v, want %v", got, want)
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func TestDecodeRequestBodyMapsMaxBytesError(t *testing.T) {
	apiErr := decodeRequestBody(errorReader{err: errors.New("http: request body too large")}, &struct{}{})
	if apiErr == nil {
		t.Fatal("decodeRequestBody() returned nil, want request body error")
	}
	if got, want := apiErr.Code, 413; got != want {
		t.Fatalf("Code = %d, want %d", got, want)
	}
	if got, want := apiErr.Message, "Request body exceeds 10MB limit"; got != want {
		t.Fatalf("Message = %q, want %q", got, want)
	}
}

func TestCommandCodeStreamToOpenAIUsesFinishStepUsage(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`{"type":"text-delta","text":"hello"}`,
		`{"type":"finish-step","finishReason":"length","usage":{"inputTokens":3,"outputTokens":4,"cachedInputTokens":1}}`,
		`{"type":"finish"}`,
	}, "\n"))

	resp, err := commandCodeStreamToOpenAI(stream, "model", "chatcmpl-test", 1, nil)
	if err != nil {
		t.Fatalf("commandCodeStreamToOpenAI() error = %v", err)
	}
	if got, want := resp.Choices[0].FinishReason, "length"; got != want {
		t.Fatalf("FinishReason = %q, want %q", got, want)
	}
	if got, want := resp.Usage.OutputTokens, 4; got != want {
		t.Fatalf("OutputTokens = %d, want %d", got, want)
	}
	if got, want := resp.Usage.CachedInputTokens, 1; got != want {
		t.Fatalf("CachedInputTokens = %d, want %d", got, want)
	}
}
