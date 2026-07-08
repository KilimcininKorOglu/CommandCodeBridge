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
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if !strings.Contains(events[0].Data, `"content":"hello"`) {
		t.Fatalf("event data = %s, want text delta content", events[0].Data)
	}
}

func TestOpenAITranslatorUsesDeltaFallbackForTextDelta(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"text-delta","delta":"fallback"}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if !strings.Contains(events[0].Data, `"content":"fallback"`) {
		t.Fatalf("event data = %s, want delta fallback content", events[0].Data)
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

func TestOpenAITranslatorHandlesReasoningDelta(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"reasoning-delta","text":"thinking"}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if !strings.Contains(events[0].Data, `"reasoning_content":"thinking"`) {
		t.Fatalf("event data = %s, want reasoning delta content", events[0].Data)
	}
}

func TestOpenAITranslatorUsesTotalUsageForFinish(t *testing.T) {
	translator := NewOpenAITranslator("model", "chatcmpl-test", 1)
	events, err := translator.ParseLine(`{"type":"finish","finishReason":"stop","usage":{"inputTokens":1,"outputTokens":2},"totalUsage":{"inputTokens":3,"outputTokens":4,"cachedInputTokens":1}}`)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if !strings.Contains(events[0].Data, `"inputTokens":3`) || !strings.Contains(events[0].Data, `"outputTokens":4`) {
		t.Fatalf("event data = %s, want total usage", events[0].Data)
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
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	if events[0].Event != "message_start" {
		t.Fatalf("first event = %q, want message_start", events[0].Event)
	}
	if events[1].Event != "content_block_start" {
		t.Fatalf("second event = %q, want content_block_start", events[1].Event)
	}
	if events[2].Event != "content_block_delta" {
		t.Fatalf("third event = %q, want content_block_delta", events[2].Event)
	}
	if !strings.Contains(events[2].Data, `"text":"hello"`) {
		t.Fatalf("event data = %s, want text delta content", events[2].Data)
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
	if len(events) != 4 {
		t.Fatalf("len(events) = %d, want 4", len(events))
	}
	if events[0].Event != "message_start" || events[1].Event != "content_block_start" || events[2].Event != "content_block_delta" || events[3].Event != "content_block_stop" {
		t.Fatalf("events = %#v, want message start and tool use block lifecycle", events)
	}
	var delta map[string]any
	if err := json.Unmarshal([]byte(events[2].Data), &delta); err != nil {
		t.Fatalf("tool delta JSON error = %v", err)
	}
	deltaBody := delta["delta"].(map[string]any)
	if got, want := deltaBody["type"], "input_json_delta"; got != want {
		t.Fatalf("delta.type = %v, want %v", got, want)
	}
}
