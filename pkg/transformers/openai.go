package transformers

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"geminicli2api/pkg/models"
	"geminicli2api/pkg/config"
)

// OpenAIRequestToGemini transforms an OpenAI chat completion request to Gemini format
func OpenAIRequestToGemini(openaiRequest *models.OpenAIChatCompletionRequest) (map[string]interface{}, error) {
	contents := []map[string]interface{}{}

	// Process each message in the conversation
	for _, message := range openaiRequest.Messages {
		role := message.Role

		// Map OpenAI roles to Gemini roles
		if role == "assistant" {
			role = "model"
		} else if role == "system" {
			role = "user" // Gemini treats system messages as user messages
		}

		// Handle different content types
		parts, err := processContent(message.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to process content: %w", err)
		}

		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": parts,
		})
	}

	// Map OpenAI generation parameters to Gemini format
	generationConfig := map[string]interface{}{}

	if openaiRequest.Temperature != nil {
		generationConfig["temperature"] = *openaiRequest.Temperature
	}
	if openaiRequest.TopP != nil {
		generationConfig["topP"] = *openaiRequest.TopP
	}
	if openaiRequest.MaxTokens != nil {
		generationConfig["maxOutputTokens"] = *openaiRequest.MaxTokens
	}
	if openaiRequest.Stop != nil {
		// Gemini supports stop sequences
		switch stop := openaiRequest.Stop.(type) {
		case string:
			generationConfig["stopSequences"] = []string{stop}
		case []string:
			generationConfig["stopSequences"] = stop
		}
	}
	if openaiRequest.FrequencyPenalty != nil {
		generationConfig["frequencyPenalty"] = *openaiRequest.FrequencyPenalty
	}
	if openaiRequest.PresencePenalty != nil {
		generationConfig["presencePenalty"] = *openaiRequest.PresencePenalty
	}
	if openaiRequest.N != nil {
		generationConfig["candidateCount"] = *openaiRequest.N
	}
	if openaiRequest.Seed != nil {
		generationConfig["seed"] = *openaiRequest.Seed
	}
	if openaiRequest.ResponseFormat != nil {
		if formatType, ok := openaiRequest.ResponseFormat["type"].(string); ok && formatType == "json_object" {
			generationConfig["responseMimeType"] = "application/json"
		}
	}

	// Build the request payload
	requestPayload := map[string]interface{}{
		"contents":        contents,
		"generationConfig": generationConfig,
		"safetySettings":  getDefaultSafetySettings(),
		"model":           config.GetBaseModelName(openaiRequest.Model),
	}

	// Add Google Search grounding for search models
	if config.IsSearchModel(openaiRequest.Model) {
		requestPayload["tools"] = []map[string]interface{}{{"googleSearch": map[string]interface{}{}}}
	}

	// Add thinking configuration for thinking models
	if !strings.Contains(openaiRequest.Model, "gemini-2.5-flash-image") {
		thinkingBudget := config.GetThinkingBudget(openaiRequest.Model)
		if thinkingBudget != -1 {
			if generationConfig["thinkingConfig"] == nil {
				generationConfig["thinkingConfig"] = map[string]interface{}{}
			}
			thinkingConfig := generationConfig["thinkingConfig"].(map[string]interface{})
			thinkingConfig["thinkingBudget"] = thinkingBudget
			thinkingConfig["includeThoughts"] = config.ShouldIncludeThoughts(openaiRequest.Model)
		}
	}

	return requestPayload, nil
}

// GeminiResponseToOpenAI transforms a Gemini API response to OpenAI chat completion format
func GeminiResponseToOpenAI(geminiResponse map[string]interface{}, model string) *models.OpenAIChatCompletionResponse {
	choices := []*models.OpenAIChatCompletionChoice{}

	candidates, _ := geminiResponse["candidates"].([]interface{})
	for _, candidate := range candidates {
		candidateMap, ok := candidate.(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := candidateMap["content"].(map[string]interface{})
		role, _ := content["role"].(string)

		// Map Gemini roles back to OpenAI roles
		if role == "model" {
			role = "assistant"
		}

		// Extract and separate thinking tokens from regular content
		parts, _ := content["parts"].([]interface{})
		var contentParts []string
		var reasoningContent string

		for _, part := range parts {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}

			// Text parts (may include thinking tokens)
			if text, ok := partMap["text"].(string); ok {
				if thought, ok := partMap["thought"].(bool); ok && thought {
					reasoningContent += text
				} else {
					contentParts = append(contentParts, text)
				}
				continue
			}

			// Inline image data -> embed as Markdown data URI
			if inlineData, ok := partMap["inlineData"].(map[string]interface{}); ok {
				if data, ok := inlineData["data"].(string); ok && data != "" {
					mimeType := "image/png"
					if mime, ok := inlineData["mimeType"].(string); ok {
						mimeType = mime
					}
					if strings.HasPrefix(mimeType, "image/") {
						contentParts = append(contentParts, fmt.Sprintf("![image](data:%s;base64,%s)", mimeType, data))
					}
				}
			}
		}

		contentText := strings.Join(contentParts, "\n\n")

		// Build message object
		message := models.OpenAIChatMessage{
			Role:    role,
			Content: contentText,
		}

		// Add reasoning_content if there are thinking tokens
		if reasoningContent != "" {
			message.ReasoningContent = &reasoningContent
		}

		finishReason := mapFinishReason(candidateMap["finishReason"])

		choice := models.NewOpenAIChatCompletionChoice(
			getInt(candidateMap["index"], 0),
			message,
			finishReason,
		)

		choices = append(choices, choice)
	}

	return models.NewOpenAIChatCompletionResponse(
		uuid.New().String(),
		model,
		choices,
	)
}

// GeminiStreamChunkToOpenAI transforms a Gemini streaming response chunk to OpenAI streaming format
func GeminiStreamChunkToOpenAI(geminiChunk map[string]interface{}, model string, responseID string) *models.OpenAIChatCompletionStreamResponse {
	choices := []*models.OpenAIChatCompletionStreamChoice{}

	candidates, _ := geminiChunk["candidates"].([]interface{})
	for _, candidate := range candidates {
		candidateMap, ok := candidate.(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := candidateMap["content"].(map[string]interface{})
		role, _ := content["role"].(string)

		// Map Gemini roles back to OpenAI roles
		if role == "model" {
			role = "assistant"
		}

		// Extract and separate thinking tokens from regular content
		parts, _ := content["parts"].([]interface{})
		var contentParts []string
		var reasoningContent string

		for _, part := range parts {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}

			// Text parts (may include thinking tokens)
			if text, ok := partMap["text"].(string); ok {
				if thought, ok := partMap["thought"].(bool); ok && thought {
					reasoningContent += text
				} else {
					contentParts = append(contentParts, text)
				}
				continue
			}

			// Inline image data -> embed as Markdown data URI
			if inlineData, ok := partMap["inlineData"].(map[string]interface{}); ok {
				if data, ok := inlineData["data"].(string); ok && data != "" {
					mimeType := "image/png"
					if mime, ok := inlineData["mimeType"].(string); ok {
						mimeType = mime
					}
					if strings.HasPrefix(mimeType, "image/") {
						contentParts = append(contentParts, fmt.Sprintf("![image](data:%s;base64,%s)", mimeType, data))
					}
				}
			}
		}

		contentText := strings.Join(contentParts, "\n\n")

		// Build delta object
		delta := models.OpenAIDelta{}
		if contentText != "" {
			delta.Content = &contentText
		}
		if reasoningContent != "" {
			delta.ReasoningContent = &reasoningContent
		}

		finishReason := mapFinishReason(candidateMap["finishReason"])

		choice := models.NewOpenAIChatCompletionStreamChoice(
			getInt(candidateMap["index"], 0),
			delta,
			finishReason,
		)

		choices = append(choices, choice)
	}

	return models.NewOpenAIChatCompletionStreamResponse(
		responseID,
		model,
		choices,
	)
}

// processContent processes message content and converts it to Gemini parts
func processContent(content interface{}) ([]map[string]interface{}, error) {
	switch content := content.(type) {
	case string:
		return processTextContent(content), nil
	case []interface{}:
		return processArrayContent(content), nil
	default:
		return nil, fmt.Errorf("unsupported content type: %T", content)
	}
}

// processTextContent processes string content and extracts markdown images
func processTextContent(text string) []map[string]interface{} {
	if text == "" {
		return []map[string]interface{}{{"text": ""}}
	}

	var parts []map[string]interface{}

	// Pattern to match markdown images: ![alt](url)
	pattern := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	matches := pattern.FindAllStringSubmatchIndex(text, -1)

	if len(matches) == 0 {
		return []map[string]interface{}{{"text": text}}
	}

	lastIdx := 0
	for _, match := range matches {
		// Add text before the image
		if match[0] > lastIdx {
			beforeText := text[lastIdx:match[0]]
			if beforeText != "" {
				parts = append(parts, map[string]interface{}{"text": beforeText})
			}
		}

		// Extract URL
		url := text[match[2]:match[3]]
		url = strings.TrimSpace(url)
		url = strings.Trim(url, "\"'")
		url = strings.Trim(url, "'")

		// Process the image URL
		if part, ok := processImageURL(url); ok {
			parts = append(parts, part)
		} else {
			// Keep as markdown if processing fails
			imageText := text[match[0]:match[1]]
			parts = append(parts, map[string]interface{}{"text": imageText})
		}

		lastIdx = match[1]
	}

	// Add remaining text
	if lastIdx < len(text) {
		remainingText := text[lastIdx:]
		if remainingText != "" {
			parts = append(parts, map[string]interface{}{"text": remainingText})
		}
	}

	if len(parts) == 0 {
		return []map[string]interface{}{{"text": text}}
	}

	return parts
}

// processArrayContent processes array content (list of parts)
func processArrayContent(contentArray []interface{}) []map[string]interface{} {
	var parts []map[string]interface{}

	for _, item := range contentArray {
		partMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		partType, ok := partMap["type"].(string)
		if !ok {
			continue
		}

		switch partType {
		case "text":
			if text, ok := partMap["text"].(string); ok {
				textParts := processTextContent(text)
				parts = append(parts, textParts...)
			}

		case "image_url":
			if imageURL, ok := partMap["image_url"].(map[string]interface{}); ok {
				if url, ok := imageURL["url"].(string); ok {
					if part, ok := processImageURL(url); ok {
						parts = append(parts, part)
					}
				}
			}
		}
	}

	if len(parts) == 0 {
		return []map[string]interface{}{{"text": ""}}
	}

	return parts
}

// processImageURL processes an image URL and returns a Gemini inline data part
func processImageURL(url string) (map[string]interface{}, bool) {
	if !strings.HasPrefix(url, "data:") {
		return nil, false // Not a data URI
	}

	// Parse data URI: data:image/png;base64,xxxx
	parts := strings.SplitN(url, ",", 2)
	if len(parts) != 2 {
		return nil, false
	}

	header := parts[0]
	data := parts[1]

	// Extract MIME type
	mimeType := "image/png"
	if strings.Contains(header, ":") {
		mimeTypeParts := strings.SplitN(header, ":", 2)
		if len(mimeTypeParts) == 2 {
			mimeTypeWithParams := mimeTypeParts[1]
			if strings.Contains(mimeTypeWithParams, ";") {
				mimeType = strings.SplitN(mimeTypeWithParams, ";", 2)[0]
			} else {
				mimeType = mimeTypeWithParams
			}
		}
	}

	// Validate base64 data
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil, false
	}

	return map[string]interface{}{
		"inlineData": map[string]interface{}{
			"mimeType": mimeType,
			"data":     data,
		},
	}, true
}

// mapFinishReason maps Gemini finish reasons to OpenAI finish reasons
func mapFinishReason(reason interface{}) *string {
	if reasonStr, ok := reason.(string); ok {
		switch reasonStr {
		case "STOP":
			return stringPtr("stop")
		case "MAX_TOKENS":
			return stringPtr("length")
		case "SAFETY", "RECITATION":
			return stringPtr("content_filter")
		default:
			return nil
		}
	}
	return nil
}

// Helper functions

func getDefaultSafetySettings() []map[string]interface{} {
	return []map[string]interface{}{
		{"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_CIVIC_INTEGRITY", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_IMAGE_DANGEROUS_CONTENT", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_IMAGE_HARASSMENT", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_IMAGE_HATE", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_IMAGE_SEXUALLY_EXPLICIT", "threshold": "BLOCK_NONE"},
		{"category": "HARM_CATEGORY_UNSPECIFIED", "threshold": "BLOCK_NONE"},
	}
}

func stringPtr(s string) *string {
	return &s
}

func getInt(value interface{}, defaultValue int) int {
	if value == nil {
		return defaultValue
	}
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}