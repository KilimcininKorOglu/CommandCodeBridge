package streaming

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenAITranslatorUsesTextForTextDelta(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"text-delta","text":"hello"}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if !strings.Contains(events[1].Data, `"content":"hello"`) {
		t.Fatalf("event data = %s, want text delta content", events[1].Data)
	}
}

func TestOpenAITranslatorUsesDeltaFallbackForTextDelta(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"text-delta","delta":"fallback"}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if !strings.Contains(events[1].Data, `"content":"fallback"`) {
		t.Fatalf("event data = %s, want delta fallback content", events[1].Data)
	}
}

func TestOpenAITranslatorIgnoresCommandCodeErrorEvents(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"error","message":"upstream warning"}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}

func TestOpenAITranslatorStreamsToolCallAsIncrementalDeltas(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"tool-call","toolCallId":"call_1","toolName":"lookup","input":{"q":"x"}}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	if !strings.Contains(events[0].Data, `"role":"assistant"`) {
		t.Fatalf("first event = %s, want leading role chunk", events[0].Data)
	}

	var startChunk OpenAIChunk
	if err := json.Unmarshal([]byte(events[1].Data), &startChunk); err != nil {
		t.Fatalf("start chunk JSON error = %v", err)
	}
	startToolCall := startChunk.Choices[0].Delta.ToolCalls[0]
	if got, want := startToolCall.Index, 0; got != want {
		t.Fatalf("start tool call index = %d, want %d", got, want)
	}
	if got, want := startToolCall.ID, "call_1"; got != want {
		t.Fatalf("start tool call ID = %q, want %q", got, want)
	}
	if got, want := startToolCall.Type, "function"; got != want {
		t.Fatalf("start tool call type = %q, want %q", got, want)
	}
	if startToolCall.Function == nil {
		t.Fatal("start tool call function = nil, want function")
	}
	if got, want := startToolCall.Function.Name, "lookup"; got != want {
		t.Fatalf("start tool call function name = %q, want %q", got, want)
	}
	if startToolCall.Function.Arguments != "" {
		t.Fatalf("start tool call arguments = %q, want empty", startToolCall.Function.Arguments)
	}

	var argumentsChunk OpenAIChunk
	if err := json.Unmarshal([]byte(events[2].Data), &argumentsChunk); err != nil {
		t.Fatalf("arguments chunk JSON error = %v", err)
	}
	argumentsToolCall := argumentsChunk.Choices[0].Delta.ToolCalls[0]
	if got, want := argumentsToolCall.Index, 0; got != want {
		t.Fatalf("arguments tool call index = %d, want %d", got, want)
	}
	if argumentsToolCall.ID != "" || argumentsToolCall.Type != "" {
		t.Fatalf("arguments tool call = %#v, want only index and function arguments", argumentsToolCall)
	}
	if argumentsToolCall.Function == nil {
		t.Fatal("arguments tool call function = nil, want function")
	}
	if got, want := argumentsToolCall.Function.Arguments, `{"q":"x"}`; got != want {
		t.Fatalf("arguments tool call function arguments = %q, want %q", got, want)
	}
}

func TestOpenAITranslatorHandlesReasoningDelta(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"reasoning-delta","text":"thinking"}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if !strings.Contains(events[0].Data, `"role":"assistant"`) {
		t.Fatalf("first event = %s, want leading role chunk", events[0].Data)
	}
	if !strings.Contains(events[1].Data, `"reasoning_content":"thinking"`) {
		t.Fatalf("event data = %s, want reasoning delta content", events[1].Data)
	}
}

func TestOpenAITranslatorUsesTotalUsageForFinish(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"finish","finishReason":"stop","usage":{"inputTokens":1,"outputTokens":2},"totalUsage":{"inputTokens":3,"outputTokens":4,"cachedInputTokens":1}}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if !strings.Contains(events[1].Data, `"inputTokens":3`) || !strings.Contains(events[1].Data, `"outputTokens":4`) {
		t.Fatalf("event data = %s, want total usage", events[1].Data)
	}
}

func TestOpenAITranslatorScansLargeNDJSONLines(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	largeText := strings.Repeat("x", 128*1024)
	line, err := json.Marshal(map[string]any{"type": "text-delta", "text": largeText})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	reader := strings.NewReader(string(line) + "\n")
	var output bytes.Buffer

	if err := translator.TranslateStream(reader, &output); err != nil {
		t.Fatalf("TranslateStream() error = %v", err)
	}
	if !strings.Contains(output.String(), largeText) {
		t.Fatal("TranslateStream() output does not contain the large content")
	}
}

func TestAnthropicTranslatorHandlesTextDelta(t *testing.T) {
	translator := NewAnthropicTranslator("model", "msg-test")
	events, err := translator.ParseLine(`{"type":"text-delta","text":"hello"}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("len(events) = %d, want 4", len(events))
	}
	if events[0].Event != "message_start" {
		t.Fatalf("first event = %q, want message_start", events[0].Event)
	}
	if events[1].Event != "ping" {
		t.Fatalf("second event = %q, want ping", events[1].Event)
	}
	if events[2].Event != "content_block_start" {
		t.Fatalf("third event = %q, want content_block_start", events[2].Event)
	}
	if events[3].Event != "content_block_delta" {
		t.Fatalf("fourth event = %q, want content_block_delta", events[3].Event)
	}
	if !strings.Contains(events[3].Data, `"text":"hello"`) {
		t.Fatalf("event data = %s, want text delta content", events[3].Data)
	}
}

func TestAnthropicTranslatorMapsStopReasonAndIgnoresDuplicateFinish(t *testing.T) {
	translator := NewAnthropicTranslator("model", "msg-test")
	reader := strings.NewReader(strings.Join([]string{
		`{"type":"text-delta","text":"hello"}`,
		`{"type":"finish","finishReason":"stop","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
		`{"type":"finish-step","finishReason":"stop","usage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))
	var output bytes.Buffer

	if err := translator.Translate(reader, &output); err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if got := strings.Count(output.String(), `"type":"message_stop"`); got != 1 {
		t.Fatalf("message_stop count = %d, want 1", got)
	}
	if !strings.Contains(output.String(), `"stop_reason":"end_turn"`) {
		t.Fatalf("output = %s, want end_turn stop reason", output.String())
	}
}

func TestAnthropicTranslatorStreamsThinkingBlocks(t *testing.T) {
	translator := NewAnthropicTranslator("model", "msg-test")
	reader := strings.NewReader(strings.Join([]string{
		`{"type":"reasoning-delta","text":"think"}`,
		`{"type":"text-delta","text":"answer"}`,
		`{"type":"finish","finishReason":"stop","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))
	var output bytes.Buffer

	if err := translator.Translate(reader, &output); err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if !strings.Contains(output.String(), `"type":"thinking"`) {
		t.Fatalf("output = %s, want thinking content block", output.String())
	}
	if !strings.Contains(output.String(), `"type":"thinking_delta"`) {
		t.Fatalf("output = %s, want thinking delta", output.String())
	}
	if !strings.Contains(output.String(), `"type":"text"`) {
		t.Fatalf("output = %s, want text content block after thinking", output.String())
	}
}

func TestAnthropicTranslatorHandlesHyphenToolCall(t *testing.T) {
	translator := NewAnthropicTranslator("model", "msg-test")
	events, err := translator.ParseLine(`{"type":"tool-call","toolCallId":"toolu_1","toolName":"lookup","input":{"q":"x"}}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("len(events) = %d, want 5", len(events))
	}
	if events[0].Event != "message_start" || events[1].Event != "ping" || events[2].Event != "content_block_start" || events[3].Event != "content_block_delta" || events[4].Event != "content_block_stop" {
		t.Fatalf("events = %#v, want message start and tool use block lifecycle", events)
	}
	var delta map[string]any
	if err := json.Unmarshal([]byte(events[3].Data), &delta); err != nil {
		t.Fatalf("tool delta JSON error = %v", err)
	}
	deltaBody := delta["delta"].(map[string]any)
	if got, want := deltaBody["type"], "input_json_delta"; got != want {
		t.Fatalf("delta.type = %v, want %v", got, want)
	}
}

func TestAnthropicTranslatorSynthesizesFinishOnStreamEnd(t *testing.T) {
	translator := NewAnthropicTranslator("model", "msg-test")
	reader := strings.NewReader(`{"type":"text-delta","text":"hi"}` + "\n")
	var output bytes.Buffer

	if err := translator.Translate(reader, &output); err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	out := output.String()
	if !strings.Contains(out, `"type":"message_delta"`) {
		t.Fatalf("output = %s, want synthesized message_delta", out)
	}
	if !strings.Contains(out, `"type":"message_stop"`) {
		t.Fatalf("output = %s, want synthesized message_stop", out)
	}
	if !strings.Contains(out, `"stop_reason":"end_turn"`) {
		t.Fatalf("output = %s, want end_turn stop reason", out)
	}
}

func TestAnthropicTranslatorDerivesToolUseStopReason(t *testing.T) {
	translator := NewAnthropicTranslator("model", "msg-test")
	reader := strings.NewReader(strings.Join([]string{
		`{"type":"tool-call","toolCallId":"toolu_1","toolName":"lookup","input":{"q":"x"}}`,
		`{"type":"finish","finishReason":"stop","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))
	var output bytes.Buffer

	if err := translator.Translate(reader, &output); err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if !strings.Contains(output.String(), `"stop_reason":"tool_use"`) {
		t.Fatalf("output = %s, want tool_use stop reason", output.String())
	}
}

func TestAnthropicTranslatorEmitsSignatureDeltaForThinking(t *testing.T) {
	translator := NewAnthropicTranslator("model", "msg-test")
	reader := strings.NewReader(strings.Join([]string{
		`{"type":"reasoning-delta","text":"think"}`,
		`{"type":"finish","finishReason":"stop","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))
	var output bytes.Buffer

	if err := translator.Translate(reader, &output); err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if !strings.Contains(output.String(), `"type":"signature_delta"`) {
		t.Fatalf("output = %s, want signature_delta before thinking block closes", output.String())
	}
}

func TestOpenAITranslatorSynthesizesFinishOnStreamEnd(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	reader := strings.NewReader(`{"type":"text-delta","text":"hi"}` + "\n")
	var output bytes.Buffer

	if err := translator.TranslateStream(reader, &output); err != nil {
		t.Fatalf("TranslateStream() error = %v", err)
	}
	out := output.String()
	if !strings.Contains(out, `"finish_reason":"stop"`) {
		t.Fatalf("output = %s, want synthesized finish_reason", out)
	}
	if !strings.Contains(out, `"role":"assistant"`) {
		t.Fatalf("output = %s, want leading role chunk", out)
	}
}

func TestOpenAITranslatorDerivesToolCallsFinishReason(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	reader := strings.NewReader(strings.Join([]string{
		`{"type":"tool-call","toolCallId":"call_1","toolName":"calc","input":{"expr":"1+1"}}`,
		`{"type":"finish","finishReason":"stop","totalUsage":{"inputTokens":3,"outputTokens":4}}`,
	}, "\n"))
	var output bytes.Buffer

	if err := translator.TranslateStream(reader, &output); err != nil {
		t.Fatalf("TranslateStream() error = %v", err)
	}
	if !strings.Contains(output.String(), `"finish_reason":"tool_calls"`) {
		t.Fatalf("output = %s, want tool_calls finish reason", output.String())
	}
	if !strings.Contains(output.String(), `"index":0`) {
		t.Fatalf("output = %s, want tool call index 0", output.String())
	}
}
