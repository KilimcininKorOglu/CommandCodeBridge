package protocol

import "testing"

func TestAnthropicMessagesToCommandCodeConvertsBase64ImageSource(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-sonnet-5",
		MaxTokens: 128,
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type": "text",
						"text": "Describe this image.",
					},
					map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/png",
							"data":       "iVBORw0KGgo=",
						},
					},
				},
			},
		},
	}

	ccReq, err := AnthropicMessagesToCommandCode(req)
	if err != nil {
		t.Fatalf("AnthropicMessagesToCommandCode() returned error: %v", err)
	}

	content := ccReq.Params.Messages[0].Content
	if got, want := len(content), 2; got != want {
		t.Fatalf("len(Content) = %d, want %d", got, want)
	}
	if got, want := content[1].Type, "image"; got != want {
		t.Fatalf("Content[1].Type = %q, want %q", got, want)
	}
	if got, want := content[1].Image, "data:image/png;base64,iVBORw0KGgo="; got != want {
		t.Fatalf("Content[1].Image = %q, want %q", got, want)
	}
}

func TestAnthropicImageSourceToURLKeepsDataURL(t *testing.T) {
	source := map[string]any{
		"type":       "base64",
		"media_type": "image/jpeg",
		"data":       "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
	}

	url, ok := anthropicImageSourceToURL(source)
	if !ok {
		t.Fatal("expected Anthropic image source to convert")
	}

	if got, want := url, "data:image/jpeg;base64,/9j/4AAQSkZJRg=="; got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}
