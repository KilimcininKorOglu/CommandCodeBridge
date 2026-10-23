package streaming

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// ErrStreamIdleTimeout indicates that no upstream stream data arrived before the idle timeout.
var ErrStreamIdleTimeout = errors.New("stream idle timeout")

const maxStreamLineSize = 4 * 1024 * 1024

// OpenAITranslator converts CommandCode NDJSON to OpenAI SSE format
type OpenAITranslator struct {
	model           string
	completionID    string
	created         int64
	inputTokens     int
	outputTokens    int
	cachedTokens    int
	buffer          strings.Builder
	roleSent        bool
	finished        bool
	toolCallCount   int
	toolCallsEmitted bool
}

// NewOpenAITranslator creates a new OpenAI streaming translator
func NewOpenAITranslator(model, completionID string, created int64) *OpenAITranslator {
	return &OpenAITranslator{
		model:        model,
		completionID: completionID,
		created:      created,
	}
}

// ParseLine parses a single NDJSON line from CommandCode stream
func (t *OpenAITranslator) ParseLine(line string) ([]SSEEvent, error) {
	if line == "" {
		return nil, nil
	}

	var event CommandCodeEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return nil, fmt.Errorf("failed to parse NDJSON: %w", err)
	}

	// Track usage
	if event.TotalUsage != nil {
		t.inputTokens = event.TotalUsage.InputTokens
		t.outputTokens = event.TotalUsage.OutputTokens
		t.cachedTokens = event.TotalUsage.CachedInputTokens
	} else if event.Usage != nil {
		t.inputTokens = event.Usage.InputTokens
		t.outputTokens = event.Usage.OutputTokens
		t.cachedTokens = event.Usage.CachedInputTokens
	}

	switch event.Type {
	case "text":
		return t.handleText(event)
	case "delta":
		return t.handleDelta(event)
	case "text-delta":
		return t.handleTextDelta(event)
	case "reasoning-delta":
		return t.handleReasoningDelta(event)
	case "reasoning-end", "provider-metadata", "tool-input-start", "tool-input-delta", "tool-input-end", "tool-error", "text-end":
		return nil, nil
	case "toolCall", "tool-call":
		return t.handleToolCall(event)
	case "toolResult", "tool-result":
		// Tool results are request-side messages; emitting them as completion
		// chunks would violate the chat completions stream shape.
		return nil, nil
	case "finish", "finish-step":
		return t.handleFinish(event)
	case "error":
		return nil, nil
	default:
		return nil, nil
	}
}

// roleChunk returns the leading chunk carrying the assistant role, which the
// OpenAI stream shape sends before any content delta.
func (t *OpenAITranslator) roleChunk() []SSEEvent {
	if t.roleSent {
		return nil
	}
	t.roleSent = true
	chunk := OpenAIChunk{
		ID:      t.completionID,
		Object:  "chat.completion.chunk",
		Created: t.created,
		Model:   t.model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Delta: &OpenAIMessage{
					Role:    "assistant",
					Content: "",
				},
			},
		},
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return nil
	}
	return []SSEEvent{{Event: "", Data: string(data)}}
}

// handleText processes text events
func (t *OpenAITranslator) handleText(event CommandCodeEvent) ([]SSEEvent, error) {
	t.roleSent = true
	chunk := OpenAIChunk{
		ID:      t.completionID,
		Object:  "chat.completion.chunk",
		Created: t.created,
		Model:   t.model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Delta: &OpenAIMessage{
					Role:    "assistant",
					Content: event.Text,
				},
			},
		},
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		return nil, err
	}

	return []SSEEvent{
		{Event: "", Data: string(data)},
	}, nil
}

// handleDelta processes delta events
func (t *OpenAITranslator) handleDelta(event CommandCodeEvent) ([]SSEEvent, error) {
	return t.handleContentDelta(event.Delta)
}

// handleTextDelta processes text-delta events.
func (t *OpenAITranslator) handleTextDelta(event CommandCodeEvent) ([]SSEEvent, error) {
	return t.handleContentDelta(TextDeltaContent(event))
}

// TextDeltaContent returns the text-delta payload with delta as a compatibility fallback.
func TextDeltaContent(event CommandCodeEvent) string {
	if event.Text != "" {
		return event.Text
	}
	return event.Delta
}

// handleContentDelta processes assistant content deltas.
func (t *OpenAITranslator) handleContentDelta(content string) ([]SSEEvent, error) {
	if content == "" {
		return nil, nil
	}

	events := t.roleChunk()

	chunk := OpenAIChunk{
		ID:      t.completionID,
		Object:  "chat.completion.chunk",
		Created: t.created,
		Model:   t.model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Delta: &OpenAIMessage{
					Content: content,
				},
			},
		},
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		return nil, err
	}

	return append(events, SSEEvent{Event: "", Data: string(data)}), nil
}

// handleReasoningDelta processes assistant reasoning deltas.
func (t *OpenAITranslator) handleReasoningDelta(event CommandCodeEvent) ([]SSEEvent, error) {
	if event.Text == "" {
		return nil, nil
	}

	events := t.roleChunk()

	chunk := OpenAIChunk{
		ID:      t.completionID,
		Object:  "chat.completion.chunk",
		Created: t.created,
		Model:   t.model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Delta: &OpenAIMessage{
					ReasoningContent: event.Text,
				},
			},
		},
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		return nil, err
	}

	return append(events, SSEEvent{Event: "", Data: string(data)}), nil
}

// handleToolCall processes tool call events.
func (t *OpenAITranslator) handleToolCall(event CommandCodeEvent) ([]SSEEvent, error) {
	arguments := ""
	if inputString, ok := event.Input.(string); ok {
		arguments = inputString
	} else if inputBytes, err := json.Marshal(event.Input); err == nil {
		arguments = string(inputBytes)
	}

	events := t.roleChunk()

	index := t.toolCallCount
	t.toolCallCount++
	t.toolCallsEmitted = true

	toolCallStart := ToolCall{
		Index: index,
		ID:    event.ToolCallID,
		Type:  "function",
		Function: &FunctionCall{
			Name: event.ToolName,
		},
	}
	toolCallArguments := ToolCall{
		Index: index,
		Function: &FunctionCall{
			Arguments: arguments,
		},
	}

	toolEvents, err := t.openAIToolCallEvents(toolCallStart, toolCallArguments)
	if err != nil {
		return nil, err
	}
	return append(events, toolEvents...), nil
}

func (t *OpenAITranslator) openAIToolCallEvents(toolCalls ...ToolCall) ([]SSEEvent, error) {
	events := make([]SSEEvent, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		chunk := OpenAIChunk{
			ID:      t.completionID,
			Object:  "chat.completion.chunk",
			Created: t.created,
			Model:   t.model,
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Delta: &OpenAIMessage{
						Content:   nil,
						ToolCalls: []ToolCall{toolCall},
					},
				},
			},
		}

		data, err := json.Marshal(chunk)
		if err != nil {
			return nil, err
		}
		events = append(events, SSEEvent{Event: "", Data: string(data)})
	}
	return events, nil
}

// handleFinish processes finish events
func (t *OpenAITranslator) handleFinish(event CommandCodeEvent) ([]SSEEvent, error) {
	events, err := t.finishEvents(event.FinishReason, event)
	if err != nil {
		return nil, err
	}
	return events, nil
}

// MapOpenAIFinishReason normalizes upstream finish reasons to the OpenAI
// vocabulary and derives tool_calls when tool calls were emitted.
func MapOpenAIFinishReason(reason string, toolCallsEmitted bool) string {
	switch reason {
	case "tool_calls", "tool-calls", "toolUse", "tool_use":
		return "tool_calls"
	case "length", "max_tokens":
		return "length"
	case "content_filter":
		return "content_filter"
	case "stop", "end_turn", "":
		if toolCallsEmitted {
			return "tool_calls"
		}
		return "stop"
	default:
		return reason
	}
}

// finishEvents builds the terminal chunk with finish_reason and usage.
func (t *OpenAITranslator) finishEvents(reason string, event CommandCodeEvent) ([]SSEEvent, error) {
	if t.finished {
		return nil, nil
	}
	t.finished = true
	events := t.roleChunk()

	usage := event.TotalUsage
	if usage == nil {
		usage = event.Usage
	}
	inputTokens, outputTokens, cachedTokens := t.inputTokens, t.outputTokens, t.cachedTokens
	if usage != nil {
		inputTokens, outputTokens, cachedTokens = usage.InputTokens, usage.OutputTokens, usage.CachedInputTokens
	}

	chunk := OpenAIChunk{
		ID:      t.completionID,
		Object:  "chat.completion.chunk",
		Created: t.created,
		Model:   t.model,
		Choices: []OpenAIChoice{
			{
				Index:        0,
				Delta:        &OpenAIMessage{},
				FinishReason: MapOpenAIFinishReason(reason, t.toolCallsEmitted),
			},
		},
	}
	if outputTokens > 0 {
		chunk.Usage = &Usage{
			InputTokens:       inputTokens,
			OutputTokens:      outputTokens,
			CachedInputTokens: cachedTokens,
		}
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		return nil, err
	}

	return append(events, SSEEvent{Event: "", Data: string(data)}), nil
}

// WriteFinish emits a synthesized terminal chunk when the upstream stream
// ended without an explicit finish event; without it clients keep waiting for
// the finish_reason chunk.
func (t *OpenAITranslator) WriteFinish(writer io.Writer) error {
	events, err := t.finishEvents("", CommandCodeEvent{})
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

// GetDoneEvent returns the SSE done event
func (t *OpenAITranslator) GetDoneEvent() SSEEvent {
	return SSEEvent{Event: "", Data: "[DONE]"}
}

// GetUsage returns the tracked usage statistics
func (t *OpenAITranslator) GetUsage() (int, int, int) {
	return t.inputTokens, t.outputTokens, t.cachedTokens
}

// TranslateStream translates a CommandCode NDJSON stream to OpenAI SSE format.
func (t *OpenAITranslator) TranslateStream(reader io.Reader, writer io.Writer) error {
	return t.TranslateStreamWithIdleTimeout(reader, writer, 0)
}

// TranslateStreamWithIdleTimeout translates a stream and fails when no line arrives before idleTimeout.
func (t *OpenAITranslator) TranslateStreamWithIdleTimeout(reader io.Reader, writer io.Writer, idleTimeout time.Duration) error {
	done := make(chan struct{})
	defer close(done)
	lines := ScanLines(reader, done)
	for {
		line, err := NextLine(lines, idleTimeout)
		if err != nil {
			return err
		}
		if line == "" {
			// Upstream ended without a finish event: synthesize the terminal
			// chunk so clients see finish_reason instead of waiting forever.
			return t.WriteFinish(writer)
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

// StreamLine is one line or terminal read error from an upstream stream.
type StreamLine struct {
	Line string
	Err  error
}

// ScanLines reads lines asynchronously so callers can apply idle timeouts.
func ScanLines(reader io.Reader, done <-chan struct{}) <-chan StreamLine {
	lines := make(chan StreamLine)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), maxStreamLineSize)
		for scanner.Scan() {
			select {
			case lines <- StreamLine{Line: scanner.Text()}:
			case <-done:
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case lines <- StreamLine{Err: err}:
			case <-done:
				return
			}
		}
	}()
	return lines
}

// NextLine reads one stream line and applies the optional idle timeout.
func NextLine(lines <-chan StreamLine, idleTimeout time.Duration) (string, error) {
	if idleTimeout <= 0 {
		item, ok := <-lines
		if !ok {
			return "", nil
		}
		return item.Line, item.Err
	}

	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()
	select {
	case item, ok := <-lines:
		if !ok {
			return "", nil
		}
		return item.Line, item.Err
	case <-timer.C:
		return "", ErrStreamIdleTimeout
	}
}

// writeSSEEvent writes an SSE event to the writer
func writeSSEEvent(writer io.Writer, event SSEEvent) error {
	if event.Event != "" {
		if _, err := fmt.Fprintf(writer, "event: %s\n", event.Event); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(writer, "data: %s\n\n", event.Data); err != nil {
		return err
	}
	return nil
}
