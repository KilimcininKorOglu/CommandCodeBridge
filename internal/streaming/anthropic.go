package streaming

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// AnthropicTranslator converts CommandCode NDJSON to Anthropic SSE format
type AnthropicTranslator struct {
	model        string
	messageID    string
	ctx          *StreamContext
	inputTokens  int
	outputTokens int
	cachedTokens int
}

// StreamContext tracks streaming state for Anthropic format
type StreamContext struct {
	messageStarted    bool
	blockStarted      bool
	currentBlockType  string
	currentBlockIndex int
	nextBlockIndex    int
	finishReason      string
	hasError          bool
}

// NewAnthropicTranslator creates a new Anthropic streaming translator
func NewAnthropicTranslator(model, messageID string) *AnthropicTranslator {
	return &AnthropicTranslator{
		model:     model,
		messageID: messageID,
		ctx:       &StreamContext{},
	}
}

// Translate translates a CommandCode NDJSON stream to Anthropic SSE format.
func (t *AnthropicTranslator) Translate(reader io.Reader, writer io.Writer) error {
	return t.TranslateWithIdleTimeout(reader, writer, 0)
}

// TranslateWithIdleTimeout translates a stream and fails when no line arrives before idleTimeout.
func (t *AnthropicTranslator) TranslateWithIdleTimeout(reader io.Reader, writer io.Writer, idleTimeout time.Duration) error {
	lines := ScanLines(reader)
	for {
		line, err := NextLine(lines, idleTimeout)
		if err != nil {
			return err
		}
		if line == "" {
			return nil
		}

		events, err := t.ParseLine(line)
		if err != nil {
			return err
		}

		for _, event := range events {
			if err := writeSSEEvent(writer, event); err != nil {
				return err
			}
		}
	}
}

// ParseLine parses a single NDJSON line from CommandCode stream
func (t *AnthropicTranslator) trackUsage(event CommandCodeEvent) {
	usage := event.TotalUsage
	if usage == nil {
		usage = event.Usage
	}
	if usage == nil {
		return
	}
	t.inputTokens = usage.InputTokens
	t.outputTokens = usage.OutputTokens
	t.cachedTokens = usage.CachedInputTokens
}

// OutputTokens returns the tracked output token count.
func (t *AnthropicTranslator) OutputTokens() int {
	return t.outputTokens
}

func (t *AnthropicTranslator) ParseLine(line string) ([]SSEEvent, error) {
	if line == "" {
		return nil, nil
	}

	var event CommandCodeEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return nil, fmt.Errorf("failed to parse NDJSON: %w", err)
	}

	t.trackUsage(event)

	var events []SSEEvent
	var err error
	switch event.Type {
	case "text":
		events, err = t.handleText(event)
	case "delta":
		events, err = t.handleDelta(event)
	case "text-delta":
		events, err = t.handleTextDelta(event)
	case "reasoning-delta", "reasoning-end", "provider-metadata", "tool-input-start", "tool-input-delta", "tool-input-end", "tool-error", "text-end":
		return nil, nil
	case "toolCall", "tool-call":
		events, err = t.handleToolCall(event)
	case "finish", "finish-step":
		events, err = t.handleFinish(event)
	case "error":
		return nil, nil
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t.withMessageStart(events), nil
}

// withMessageStart prepends the Anthropic message_start event once.
func (t *AnthropicTranslator) withMessageStart(events []SSEEvent) []SSEEvent {
	if t.ctx.messageStarted || len(events) == 0 {
		return events
	}

	startEvent := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            t.messageID,
			"type":          "message",
			"role":          "assistant",
			"content":       []any{},
			"model":         t.model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
	data, _ := json.Marshal(startEvent)
	t.ctx.messageStarted = true
	return append([]SSEEvent{{Event: "message_start", Data: string(data)}}, events...)
}

// handleText processes text events
func (t *AnthropicTranslator) handleText(event CommandCodeEvent) ([]SSEEvent, error) {
	return t.handleTextContentDelta(event.Text)
}

// handleDelta processes delta events
func (t *AnthropicTranslator) handleDelta(event CommandCodeEvent) ([]SSEEvent, error) {
	return t.handleTextContentDelta(event.Delta)
}

// handleTextDelta processes text-delta events.
func (t *AnthropicTranslator) handleTextDelta(event CommandCodeEvent) ([]SSEEvent, error) {
	return t.handleTextContentDelta(TextDeltaContent(event))
}

// handleTextContentDelta processes text deltas.
func (t *AnthropicTranslator) handleTextContentDelta(text string) ([]SSEEvent, error) {
	if text == "" {
		return nil, nil
	}

	events := []SSEEvent{}
	if !t.ctx.blockStarted || t.ctx.currentBlockType != "text" {
		close := t.closeTextBlock()
		if close != "" {
			events = append(events, SSEEvent{Event: "content_block_stop", Data: close})
		}
		t.ctx.currentBlockIndex = t.ctx.nextBlockIndex
		t.ctx.nextBlockIndex++
		t.ctx.currentBlockType = "text"
		t.ctx.blockStarted = true

		startEvent := map[string]any{
			"type":  "content_block_start",
			"index": t.ctx.currentBlockIndex,
			"content_block": map[string]any{
				"type": "text",
				"text": "",
			},
		}
		data, err := json.Marshal(startEvent)
		if err != nil {
			return nil, err
		}
		events = append(events, SSEEvent{Event: "content_block_start", Data: string(data)})
	}

	deltaEvent := map[string]any{
		"type":  "content_block_delta",
		"index": t.ctx.currentBlockIndex,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	}
	data, err := json.Marshal(deltaEvent)
	if err != nil {
		return nil, err
	}

	events = append(events, SSEEvent{Event: "content_block_delta", Data: string(data)})
	return events, nil
}

// handleToolCall processes tool call events.
func (t *AnthropicTranslator) handleToolCall(event CommandCodeEvent) ([]SSEEvent, error) {
	events := []SSEEvent{}
	close := t.closeTextBlock()
	if close != "" {
		events = append(events, SSEEvent{Event: "content_block_stop", Data: close})
	}

	index := t.ctx.nextBlockIndex
	t.ctx.nextBlockIndex++
	input := event.Input
	if input == nil {
		input = map[string]any{}
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	startEvent := map[string]any{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]any{
			"type":  "tool_use",
			"id":    event.ToolCallID,
			"name":  event.ToolName,
			"input": map[string]any{},
		},
	}
	startData, err := json.Marshal(startEvent)
	if err != nil {
		return nil, err
	}
	events = append(events, SSEEvent{Event: "content_block_start", Data: string(startData)})

	deltaEvent := map[string]any{
		"type":  "content_block_delta",
		"index": index,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": string(inputBytes),
		},
	}
	deltaData, err := json.Marshal(deltaEvent)
	if err != nil {
		return nil, err
	}
	events = append(events, SSEEvent{Event: "content_block_delta", Data: string(deltaData)})

	stopEvent := map[string]any{
		"type":  "content_block_stop",
		"index": index,
	}
	stopData, err := json.Marshal(stopEvent)
	if err != nil {
		return nil, err
	}
	events = append(events, SSEEvent{Event: "content_block_stop", Data: string(stopData)})
	return events, nil
}

// handleFinish processes finish events
func (t *AnthropicTranslator) handleFinish(event CommandCodeEvent) ([]SSEEvent, error) {
	events := []SSEEvent{}

	// Close current text block
	close := t.closeTextBlock()
	if close != "" {
		events = append(events, SSEEvent{Event: "content_block_stop", Data: close})
	}

	usage := event.TotalUsage
	if usage == nil {
		usage = event.Usage
	}
	anthropicUsage := map[string]any{}
	if usage != nil {
		anthropicUsage["output_tokens"] = usage.OutputTokens
	}

	// Send message delta with stop reason
	deltaEvent := map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": event.FinishReason, "stop_sequence": nil},
		"usage": anthropicUsage,
	}
	data, err := json.Marshal(deltaEvent)
	if err != nil {
		return nil, err
	}
	events = append(events, SSEEvent{Event: "message_delta", Data: string(data)})

	// Send message stop
	stopEvent := map[string]any{
		"type": "message_stop",
	}
	data, _ = json.Marshal(stopEvent)
	events = append(events, SSEEvent{Event: "message_stop", Data: string(data)})

	return events, nil
}

// closeTextBlock closes the current text block if one is active
func (t *AnthropicTranslator) closeTextBlock() string {
	if t.ctx.blockStarted && t.ctx.currentBlockType == "text" {
		t.ctx.blockStarted = false
		t.ctx.currentBlockType = ""
		stopEvent := map[string]any{
			"type":  "content_block_stop",
			"index": t.ctx.currentBlockIndex,
		}
		data, _ := json.Marshal(stopEvent)
		return string(data)
	}
	return ""
}
