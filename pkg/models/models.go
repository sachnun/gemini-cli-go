package models

import (
	"time"
)

// OpenAI Models

// OpenAIChatMessage represents a message in OpenAI chat format
type OpenAIChatMessage struct {
	Role             string      `json:"role"`
	Content          interface{} `json:"content"` // Can be string or []interface{}
	ReasoningContent *string     `json:"reasoning_content,omitempty"`
}

// OpenAIChatCompletionRequest represents an OpenAI chat completion request
type OpenAIChatCompletionRequest struct {
	Model            string                 `json:"model"`
	Messages         []OpenAIChatMessage    `json:"messages"`
	Stream           bool                   `json:"stream"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	Stop             interface{}            `json:"stop,omitempty"` // Can be string or []string
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	N                *int                   `json:"n,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`
	ResponseFormat   map[string]interface{} `json:"response_format,omitempty"`
}

// OpenAIChatCompletionChoice represents a choice in OpenAI chat completion response
type OpenAIChatCompletionChoice struct {
	Index        int                  `json:"index"`
	Message      OpenAIChatMessage    `json:"message"`
	FinishReason *string              `json:"finish_reason,omitempty"`
}

// OpenAIChatCompletionResponse represents an OpenAI chat completion response
type OpenAIChatCompletionResponse struct {
	ID      string                          `json:"id"`
	Object  string                          `json:"object"`
	Created int64                           `json:"created"`
	Model   string                          `json:"model"`
	Choices []*OpenAIChatCompletionChoice    `json:"choices"`
}

// OpenAIDelta represents a delta in streaming OpenAI response
type OpenAIDelta struct {
	Content          *string `json:"content,omitempty"`
	ReasoningContent *string `json:"reasoning_content,omitempty"`
}

// OpenAIChatCompletionStreamChoice represents a streaming choice in OpenAI response
type OpenAIChatCompletionStreamChoice struct {
	Index        int       `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	FinishReason *string   `json:"finish_reason,omitempty"`
}

// OpenAIChatCompletionStreamResponse represents a streaming OpenAI chat completion response
type OpenAIChatCompletionStreamResponse struct {
	ID      string                               `json:"id"`
	Object  string                               `json:"object"`
	Created int64                                `json:"created"`
	Model   string                               `json:"model"`
	Choices []*OpenAIChatCompletionStreamChoice   `json:"choices"`
}

// Gemini Models

// GeminiPart represents a part of Gemini content
type GeminiPart struct {
	Text string `json:"text"`
	// Can be extended with other part types like inlineData, functionCall, etc.
}

// GeminiContent represents content in Gemini format
type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiRequest represents a Gemini API request
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
	// Can be extended with other fields like systemInstruction, tools, etc.
}

// GeminiCandidate represents a candidate in Gemini response
type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason *string       `json:"finish_reason,omitempty"`
	Index        int           `json:"index"`
}

// GeminiResponse represents a Gemini API response
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

// Helper functions

// NewOpenAIChatCompletionResponse creates a new OpenAI chat completion response
func NewOpenAIChatCompletionResponse(id, model string, choices []*OpenAIChatCompletionChoice) *OpenAIChatCompletionResponse {
	return &OpenAIChatCompletionResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
	}
}

// NewOpenAIChatCompletionStreamResponse creates a new OpenAI chat completion stream response
func NewOpenAIChatCompletionStreamResponse(id, model string, choices []*OpenAIChatCompletionStreamChoice) *OpenAIChatCompletionStreamResponse {
	return &OpenAIChatCompletionStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
	}
}

// NewOpenAIChatCompletionChoice creates a new OpenAI chat completion choice
func NewOpenAIChatCompletionChoice(index int, message OpenAIChatMessage, finishReason *string) *OpenAIChatCompletionChoice {
	return &OpenAIChatCompletionChoice{
		Index:        index,
		Message:      message,
		FinishReason: finishReason,
	}
}

// NewOpenAIChatCompletionStreamChoice creates a new OpenAI chat completion stream choice
func NewOpenAIChatCompletionStreamChoice(index int, delta OpenAIDelta, finishReason *string) *OpenAIChatCompletionStreamChoice {
	return &OpenAIChatCompletionStreamChoice{
		Index:        index,
		Delta:        delta,
		FinishReason: finishReason,
	}
}

// NewGeminiContent creates new Gemini content
func NewGeminiContent(role string, parts []GeminiPart) *GeminiContent {
	return &GeminiContent{
		Role:  role,
		Parts: parts,
	}
}

// NewGeminiPart creates new Gemini part
func NewGeminiPart(text string) *GeminiPart {
	return &GeminiPart{
		Text: text,
	}
}

// NewGeminiCandidate creates new Gemini candidate
func NewGeminiCandidate(index int, content GeminiContent, finishReason *string) *GeminiCandidate {
	return &GeminiCandidate{
		Index:        index,
		Content:      content,
		FinishReason: finishReason,
	}
}

// NewGeminiResponse creates new Gemini response
func NewGeminiResponse(candidates []GeminiCandidate) *GeminiResponse {
	return &GeminiResponse{
		Candidates: candidates,
	}
}