package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

const toolOutputErrorPrefix = "Error:"

// TryParseJSON attempts to parse a JSON string, returns empty object on failure
func TryParseJSON(str string) any {
	var result any
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		return map[string]any{}
	}
	return result
}

// MakeChunk creates an OpenAI streaming chunk
func MakeChunk(id string, created int64, model string, delta any, finishReason string, usage *Usage) map[string]any {
	chunk := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"delta":         delta,
				"finish_reason": finishReason,
			},
		},
	}

	if usage != nil {
		chunk["usage"] = usage
	}

	return chunk
}

// NormalizeUsage normalizes usage stats to prevent false billing when output tokens are zero
func NormalizeUsage(u *Usage) {
	if u == nil {
		return
	}
	if u.OutputTokens == 0 {
		u.InputTokens = 0
		u.CachedInputTokens = 0
	}
}

// MapCommandCodeError maps CommandCode status to OpenAI error format
func MapCommandCodeError(status int, body string) map[string]any {
	errorType := "upstream_error"
	switch status {
	case 400:
		errorType = "invalid_request_error"
	case 401, 403:
		errorType = "authentication_error"
	case 402, 429:
		errorType = "rate_limit_error"
	case 404:
		errorType = "not_found"
	case 422:
		errorType = "invalid_request_error"
	case 500, 502, 503:
		errorType = "upstream_error"
	}

	message := fmt.Sprintf("CommandCode API error (%d)", status)
	if body != "" && len(body) < 200 {
		message = body
	}

	result := map[string]any{
		"error": map[string]any{
			"type":    errorType,
			"message": message,
		},
	}

	// Add retry_after for rate limit errors
	if status == 429 {
		result["retry_after"] = 30
	}

	return result
}

// buildOpenAIToolNameMap maps tool call IDs to tool names from assistant messages.
func buildOpenAIToolNameMap(messages []OpenAIMessage) map[string]string {
	toolNameByID := map[string]string{}
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, toolCall := range msg.ToolCalls {
			if toolCall.ID != "" {
				toolNameByID[toolCall.ID] = toolCall.Function.Name
			}
		}
	}
	return toolNameByID
}

// openaiMessageToCommandCode converts an OpenAI message to CommandCode format
func openaiMessageToCommandCode(msg OpenAIMessage, toolNameByID map[string]string) (CommandCodeMessage, error) {
	if msg.Role == "tool" {
		return openaiToolMessageToCommandCode(msg, toolNameByID)
	}

	ccMsg := CommandCodeMessage{
		Role:    msg.Role,
		Content: openAIContentToCommandCode(msg.Content),
	}

	for _, tc := range msg.ToolCalls {
		ccMsg.Content = append(ccMsg.Content, CommandCodeContent{
			Type:       "tool-call",
			ToolCallID: tc.ID,
			ToolName:   tc.Function.Name,
			Input:      TryParseJSON(tc.Function.Arguments),
		})
	}

	return ccMsg, nil
}

func openaiToolMessageToCommandCode(msg OpenAIMessage, toolNameByID map[string]string) (CommandCodeMessage, error) {
	toolName := msg.Name
	if toolName == "" {
		toolName = toolNameByID[msg.ToolCallID]
	}
	if toolName == "" {
		toolName = "unknown"
	}

	output, err := openAIContentToString(msg.Content)
	if err != nil {
		return CommandCodeMessage{}, err
	}
	outputType := "text"
	if strings.HasPrefix(output, toolOutputErrorPrefix) {
		outputType = "error-text"
	}

	return CommandCodeMessage{
		Role: "tool",
		Content: []CommandCodeContent{
			{
				Type:       "tool-result",
				ToolCallID: msg.ToolCallID,
				ToolName:   toolName,
				Output: &CommandCodeToolOutput{
					Type:  outputType,
					Value: output,
				},
			},
		},
	}, nil
}

func openAIContentToCommandCode(content any) []CommandCodeContent {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []CommandCodeContent{{Type: "text", Text: v}}
	case []any:
		ccContent := []CommandCodeContent{}
		for _, item := range v {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ccContent = append(ccContent, openAIContentPartToCommandCode(itemMap)...)
		}
		return ccContent
	default:
		return nil
	}
}

func openAIContentPartToCommandCode(itemMap map[string]any) []CommandCodeContent {
	itemType, _ := itemMap["type"].(string)
	switch itemType {
	case "text", "input_text", "output_text", "refusal", "thinking", "redacted_thinking", "reasoning", "document", "search_result":
		if text := contentPartToString(itemMap); text != "" {
			return []CommandCodeContent{{Type: "text", Text: text}}
		}
	case "image_url":
		if imageURL, ok := itemMap["image_url"].(map[string]any); ok {
			if url, ok := imageURL["url"].(string); ok && url != "" {
				return []CommandCodeContent{{Type: "image", Image: url}}
			}
		}
	case "input_image", "image":
		if text := contentPartToString(itemMap); text != "" {
			return []CommandCodeContent{{Type: "text", Text: text}}
		}
	case "tool_use", "tool-call":
		id := firstString(itemMap, "id", "toolCallId", "tool_use_id")
		name := firstString(itemMap, "name", "toolName")
		input := itemMap["input"]
		if input == nil {
			input = itemMap["arguments"]
		}
		return []CommandCodeContent{{Type: "tool-call", ToolCallID: id, ToolName: name, Input: input}}
	case "tool_result", "tool-result":
		if text := contentPartToString(itemMap["content"]); text != "" {
			return []CommandCodeContent{{Type: "text", Text: text}}
		}
		if text := contentPartToString(itemMap["output"]); text != "" {
			return []CommandCodeContent{{Type: "text", Text: text}}
		}
	default:
		if text := contentPartToString(itemMap); text != "" {
			return []CommandCodeContent{{Type: "text", Text: text}}
		}
	}
	return nil
}

func openAIContentToString(content any) (string, error) {
	switch v := content.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []any:
		var b strings.Builder
		for _, item := range v {
			if itemMap, ok := item.(map[string]any); ok {
				b.WriteString(contentPartToString(itemMap))
			}
		}
		return b.String(), nil
	default:
		contentBytes, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(contentBytes), nil
	}
}

func contentPartToString(content any) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			b.WriteString(contentPartToString(item))
		}
		return b.String()
	case map[string]any:
		for _, key := range []string{"text", "content", "output_text", "input_text", "refusal", "thinking", "redacted_thinking"} {
			if text, ok := v[key].(string); ok {
				return text
			}
		}
		if imageURL, ok := v["image_url"].(map[string]any); ok {
			if url, ok := imageURL["url"].(string); ok {
				return "[Image URL: " + url + "]"
			}
		}
		if url, ok := v["image_url"].(string); ok {
			return "[Image URL: " + url + "]"
		}
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	default:
		return fmt.Sprint(v)
	}
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok {
			return value
		}
	}
	return ""
}

// openAIToolToCommandCode converts an OpenAI tool to CommandCode format
func openAIToolToCommandCode(tool OpenAITool) CommandCodeTool {
	return CommandCodeTool{
		Type:        tool.Type,
		Name:        tool.Function.Name,
		Description: tool.Function.Description,
		InputSchema: tool.Function.Parameters,
	}
}

// anthropicMessageToOpenAI converts an Anthropic message to OpenAI format
func anthropicMessageToOpenAI(msg AnthropicMessage) (OpenAIMessage, error) {
	openaiMsg := OpenAIMessage{
		Role:    msg.Role,
		Content: nil,
	}

	// Handle content based on type
	switch v := msg.Content.(type) {
	case string:
		openaiMsg.Content = v
	case []any:
		content := make([]any, 0)
		for _, item := range v {
			if itemMap, ok := item.(map[string]any); ok {
				if itemType, ok := itemMap["type"].(string); ok {
					switch itemType {
					case "text":
						if text, ok := itemMap["text"].(string); ok {
							content = append(content, map[string]any{
								"type": "text",
								"text": text,
							})
						}
					case "image":
						if source, ok := itemMap["source"].(map[string]any); ok {
							if url, ok := anthropicImageSourceToURL(source); ok {
								content = append(content, map[string]any{
									"type": "image_url",
									"image_url": map[string]string{
										"url": url,
									},
								})
							}
						}
					case "tool_use":
						if id, ok := itemMap["id"].(string); ok {
							if name, ok := itemMap["name"].(string); ok {
								if input, ok := itemMap["input"]; ok {
									toolCall := ToolCall{
										ID:   id,
										Type: "function",
										Function: FunctionCall{
											Name: name,
										},
									}
									if inputBytes, err := json.Marshal(input); err == nil {
										toolCall.Function.Arguments = string(inputBytes)
									}
									openaiMsg.ToolCalls = append(openaiMsg.ToolCalls, toolCall)
								}
							}
						}
					case "tool_result":
						if toolUseID, ok := itemMap["tool_use_id"].(string); ok {
							openaiMsg.Role = "tool"
							openaiMsg.ToolCallID = toolUseID
							switch contentVal := itemMap["content"].(type) {
							case string:
								openaiMsg.Content = contentVal
							case []any:
								var parts []string
								for _, part := range contentVal {
									if partMap, ok := part.(map[string]any); ok {
										if text, ok := partMap["text"].(string); ok {
											parts = append(parts, text)
										}
									}
								}
								openaiMsg.Content = strings.Join(parts, "\n")
							}
						}
					}
				}
			}
		}
		if len(content) > 0 {
			openaiMsg.Content = content
		}
	}

	return openaiMsg, nil
}

// anthropicImageSourceToURL converts an Anthropic image source into an OpenAI image URL.
func anthropicImageSourceToURL(source map[string]any) (string, bool) {
	if url, ok := source["url"].(string); ok && url != "" {
		return url, true
	}

	data, ok := source["data"].(string)
	if !ok || data == "" {
		return "", false
	}

	mediaType, ok := source["media_type"].(string)
	if !ok || mediaType == "" {
		return "", false
	}

	if strings.HasPrefix(data, "data:") {
		return data, true
	}

	return fmt.Sprintf("data:%s;base64,%s", mediaType, data), true
}

// anthropicToolToOpenAI converts an Anthropic tool to OpenAI format
func anthropicToolToOpenAI(tool AnthropicTool) OpenAITool {
	return OpenAITool{
		Type: "function",
		Function: OpenAIToolFunction{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.InputSchema,
		},
	}
}
