package protocol

import "testing"

func TestAnthropicToOpenAIConvertsBase64ImageSource(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-sonnet-5",
		MaxTokens: 128,
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Describe this image.",
					},
					map[string]interface{}{
						"type": "image",
						"source": map[string]interface{}{
							"type":       "base64",
							"media_type": "image/png",
							"data":       "iVBORw0KGgo=",
						},
					},
				},
			},
		},
	}

	openAIReq, err := AnthropicToOpenAI(req)
	if err != nil {
		t.Fatalf("AnthropicToOpenAI() returned error: %v", err)
	}

	content, ok := openAIReq.Messages[0].Content.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected OpenAI content blocks, got %T", openAIReq.Messages[0].Content)
	}

	imageURL, ok := content[1]["image_url"].(map[string]string)
	if !ok {
		t.Fatalf("expected image_url block, got %T", content[1]["image_url"])
	}

	if got, want := imageURL["url"], "data:image/png;base64,iVBORw0KGgo="; got != want {
		t.Fatalf("image_url.url = %q, want %q", got, want)
	}
}

func TestAnthropicToOpenAIKeepsDataURLImageSource(t *testing.T) {
	source := map[string]interface{}{
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
