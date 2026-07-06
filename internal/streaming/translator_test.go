package streaming

import (
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
