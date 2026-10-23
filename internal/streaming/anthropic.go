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
	pingSent          bool
	blockStarted      bool
	currentBlockType  string
	currentBlockIndex int
	nextBlockIndex    int
	finishReason      string
	hasError          bool
	finished          bool
	toolUseEmitted    bool
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
	done := make(chan struct{})
	defer close(done)
	lines := ScanLines(reader, done)
	for {
		line, err := NextLine(lines, idleTimeout)
		if err != nil {
			return err
		}
		if line == "" {
			// Upstream closed the stream without a finish event: synthesize the
			// terminal sequence, otherwise Anthropic clients wait for
			// message_stop forever ("still thinking" hang).
			return t.WriteFinish(writer, "")
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

// WriteFinish emits the terminal Anthropic events (content_block_stop for any
// open block, message_delta with stop reason and usage, message_stop). It is
// idempotent and used both for real finish events and for synthesized ends.
func (t *AnthropicTranslator) WriteFinish(writer io.Writer, finishReason string) error {
	events, err := t.finishEvents(finishReason)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := writeSSEEvent(writer, event); err != nil {
			return err
		}
	}
	return nil
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

// GetUsage returns the tracked usage statistics
func (t *AnthropicTranslator) GetUsage() (int, int, int) {
	return t.inputTokens, t.outputTokens, t.cachedTokens
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
	case "reasoning-delta":
		events, err = t.handleReasoningDelta(event)
	case "reasoning-end", "provider-metadata", "tool-input-start", "tool-input-delta", "tool-input-end", "tool-error", "text-end":
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

// withMessageStart prepends the Anthropic message_start event once, followed
// by the ping event the real API sends after message_start.
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
	prepend := []SSEEvent{{Event: "message_start", Data: string(data)}}
	if !t.ctx.pingSent {
		ping, _ := json.Marshal(map[string]any{"type": "ping"})
		prepend = append(prepend, SSEEvent{Event: "ping", Data: string(ping)})
		t.ctx.pingSent = true
	}
	return append(prepend, events...)
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
		events = append(events, t.closeBlockEvents()...)
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

// handleReasoningDelta processes thinking deltas.
func (t *AnthropicTranslator) handleReasoningDelta(event CommandCodeEvent) ([]SSEEvent, error) {
	text := TextDeltaContent(event)
	if text == "" {
		return nil, nil
	}

	events := []SSEEvent{}
	// Reuse the open thinking block across consecutive reasoning deltas; only
	// close when a block of a different type is open. Closing unconditionally
	// would fragment every delta into its own single-delta thinking block.
	if t.ctx.blockStarted && t.ctx.currentBlockType != "thinking" {
		events = append(events, t.closeBlockEvents()...)
	}

	if !t.ctx.blockStarted || t.ctx.currentBlockType != "thinking" {
		t.ctx.currentBlockIndex = t.ctx.nextBlockIndex
		t.ctx.nextBlockIndex++
		t.ctx.currentBlockType = "thinking"
		t.ctx.blockStarted = true

		startEvent := map[string]any{
			"type":  "content_block_start",
			"index": t.ctx.currentBlockIndex,
			"content_block": map[string]any{
				"type":     "thinking",
				"thinking": "",
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
			"type":     "thinking_delta",
			"thinking": text,
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
	events = append(events, t.closeBlockEvents()...)

	index := t.ctx.nextBlockIndex
	t.ctx.nextBlockIndex++
	t.ctx.toolUseEmitted = true
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
	return t.finishEvents(event.FinishReason)
}

// finishEvents builds the terminal Anthropic event sequence. When the upstream
// finish reason is missing or a plain stop, tool_use blocks already emitted
// still require stop_reason "tool_use" per the Messages spec.
func (t *AnthropicTranslator) finishEvents(finishReason string) ([]SSEEvent, error) {
	if t.ctx.finished {
		return nil, nil
	}
	t.ctx.finished = true
	events := []SSEEvent{}

	events = append(events, t.closeBlockEvents()...)

	stopReason := mapAnthropicStreamStopReason(finishReason)
	if stopReason == "end_turn" && t.ctx.toolUseEmitted {
		stopReason = "tool_use"
	}

	usage := map[string]any{"output_tokens": t.outputTokens}
	if t.inputTokens > 0 {
		usage["input_tokens"] = t.inputTokens
	}

	// Send message delta with stop reason
	deltaEvent := map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": usage,
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

	return t.withMessageStart(events), nil
}

func mapAnthropicStreamStopReason(reason string) string {
	switch reason {
	case "tool_calls", "tool-calls", "toolUse":
		return "tool_use"
	case "length", "max_tokens":
		return "max_tokens"
	case "content_filter":
		return "stop_sequence"
	case "stop", "end_turn", "":
		return "end_turn"
	default:
		return reason
	}
}

// closeBlockEvents emits the events that close the currently open text or
// thinking block. Thinking blocks get a signature_delta before their
// content_block_stop, mirroring the real Anthropic event shape.
func (t *AnthropicTranslator) closeBlockEvents() []SSEEvent {
	if !t.ctx.blockStarted {
		return nil
	}
	blockType := t.ctx.currentBlockType
	index := t.ctx.currentBlockIndex
	t.ctx.blockStarted = false
	t.ctx.currentBlockType = ""

	events := []SSEEvent{}
	if blockType == "thinking" {
		signatureEvent := map[string]any{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]any{
				"type":      "signature_delta",
				"signature": "bridge",
			},
		}
		if data, err := json.Marshal(signatureEvent); err == nil {
			events = append(events, SSEEvent{Event: "content_block_delta", Data: string(data)})
		}
	}

	stopEvent := map[string]any{
		"type":  "content_block_stop",
		"index": index,
	}
	data, _ := json.Marshal(stopEvent)
	return append(events, SSEEvent{Event: "content_block_stop", Data: string(data)})
}
