package google

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"geminicli2api/pkg/auth"
	"geminicli2api/pkg/config"
)

// Client handles communication with Google's Gemini API
type Client struct {
	authConfig   *auth.AuthConfig
	httpClient   *http.Client
	config       *config.Config
}

// NewClient creates a new Google API client
func NewClient(authConfig *auth.AuthConfig, cfg *config.Config) *Client {
	return &Client{
		authConfig: authConfig,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		config: cfg,
	}
}

// SendGeminiRequest sends a request to Google's Gemini API
func (c *Client) SendGeminiRequest(ctx context.Context, payload map[string]interface{}, isStreaming bool) (*http.Response, error) {
	// Get and validate credentials
	token, err := c.authConfig.GetCredentials(true)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
	if token == nil {
		return nil, fmt.Errorf("no credentials available")
	}

	// Refresh token if needed
	if !token.Valid() && token.RefreshToken != "" {
		if err := c.authConfig.RefreshToken(token); err != nil {
			return nil, fmt.Errorf("token refresh failed: %w", err)
		}
		// Save refreshed credentials
		c.authConfig.SaveCredentials(token, "")
	} else if token.AccessToken == "" {
		return nil, fmt.Errorf("no access token available")
	}

	// Get project ID and onboard user
	projectID, err := c.authConfig.GetUserProjectID(token)
	if err != nil {
		return nil, fmt.Errorf("failed to get user project ID: %w", err)
	}

	if err := c.authConfig.OnboardUser(token, projectID); err != nil {
		return nil, fmt.Errorf("user onboarding failed: %w", err)
	}

	// Build the final payload
	finalPayload := map[string]interface{}{
		"model":   payload["model"],
		"project": projectID,
		"request": payload["request"],
	}

	// Determine the action and URL
	action := "streamGenerateContent"
	if !isStreaming {
		action = "generateContent"
	}
	targetURL := fmt.Sprintf("%s/v1internal:%s", c.config.CodeAssistEndpoint, action)
	if isStreaming {
		targetURL += "?alt=sse"
	}

	// Marshal payload
	payloadData, err := json.Marshal(finalPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(payloadData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", getUserAgent())

	// Send request
	if isStreaming {
		return c.sendStreamingRequest(req)
	}
	return c.sendNonStreamingRequest(req)
}

// sendStreamingRequest sends a streaming request
func (c *Client) sendStreamingRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		log.Printf("Google API returned status %d", resp.StatusCode)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return createErrorResponse(resp.StatusCode, string(body)), nil
	}

	return resp, nil
}

// sendNonStreamingRequest sends a non-streaming request
func (c *Client) sendNonStreamingRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		// Parse response
		var googleAPIResponse map[string]interface{}
		bodyStr := string(body)
		if strings.HasPrefix(bodyStr, "data: ") {
			bodyStr = bodyStr[6:]
		}

		if err := json.Unmarshal([]byte(bodyStr), &googleAPIResponse); err != nil {
			// If parsing fails, return original response
			return createRawResponse(resp.StatusCode, body, resp.Header.Get("Content-Type")), nil
		}

		if response, ok := googleAPIResponse["response"].(map[string]interface{}); ok {
			responseData, _ := json.Marshal(response)
			return createRawResponse(http.StatusOK, responseData, "application/json; charset=utf-8"), nil
		}

		// Fallback to original response
		return createRawResponse(resp.StatusCode, body, resp.Header.Get("Content-Type")), nil
	}

	// Handle error responses
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return createErrorResponse(resp.StatusCode, string(body)), nil
}

// BuildGeminiPayloadFromOpenAI builds a Gemini API payload from an OpenAI-transformed request
func (c *Client) BuildGeminiPayloadFromOpenAI(openaiPayload map[string]interface{}) map[string]interface{} {
	model := openaiPayload["model"]

	// Get safety settings or use defaults
	safetySettings := c.config.SafetySettings
	if ss, ok := openaiPayload["safetySettings"]; ok {
		if ssSlice, ok := ss.([]map[string]interface{}); ok {
			safetySettings = ssSlice
		}
	}

	// Build the request portion
	requestData := map[string]interface{}{
		"contents":         openaiPayload["contents"],
		"systemInstruction": openaiPayload["systemInstruction"],
		"cachedContent":    openaiPayload["cachedContent"],
		"tools":            openaiPayload["tools"],
		"toolConfig":       openaiPayload["toolConfig"],
		"safetySettings":   safetySettings,
		"generationConfig": openaiPayload["generationConfig"],
	}

	// Remove any keys with nil values
	cleanRequestData := make(map[string]interface{})
	for k, v := range requestData {
		if v != nil {
			cleanRequestData[k] = v
		}
	}

	return map[string]interface{}{
		"model":   model,
		"request": cleanRequestData,
	}
}

// BuildGeminiPayloadFromNative builds a Gemini API payload from a native Gemini request
func (c *Client) BuildGeminiPayloadFromNative(nativeRequest map[string]interface{}, modelFromPath string) map[string]interface{} {
	// Create a copy to avoid modifying the original
	request := make(map[string]interface{})
	for k, v := range nativeRequest {
		request[k] = v
	}

	// Set safety settings
	request["safetySettings"] = c.config.SafetySettings

	// Ensure generationConfig exists
	if _, ok := request["generationConfig"]; !ok {
		request["generationConfig"] = make(map[string]interface{})
	}
	genConfig := request["generationConfig"].(map[string]interface{})

	// Ensure thinkingConfig exists
	if _, ok := genConfig["thinkingConfig"]; !ok {
		genConfig["thinkingConfig"] = make(map[string]interface{})
	}
	thinkingConfig := genConfig["thinkingConfig"].(map[string]interface{})

	if !strings.Contains(modelFromPath, "gemini-2.5-flash-image") {
		// Configure thinking based on model variant
		thinkingBudget := config.GetThinkingBudget(modelFromPath)
		includeThoughts := config.ShouldIncludeThoughts(modelFromPath)

		thinkingConfig["includeThoughts"] = includeThoughts

		// Only set thinkingBudget if it's not the default or if not already set
		if _, ok := thinkingConfig["thinkingBudget"]; !ok || thinkingBudget != -1 {
			thinkingConfig["thinkingBudget"] = thinkingBudget
		}
	}

	// Add Google Search grounding for search models
	if config.IsSearchModel(modelFromPath) {
		if _, ok := request["tools"]; !ok {
			request["tools"] = []interface{}{}
		}
		tools := request["tools"].([]interface{})

		// Add googleSearch tool if not already present
		hasGoogleSearch := false
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]interface{}); ok {
				if _, ok := toolMap["googleSearch"]; ok {
					hasGoogleSearch = true
					break
				}
			}
		}

		if !hasGoogleSearch {
			tools = append(tools, map[string]interface{}{"googleSearch": map[string]interface{}{}})
			request["tools"] = tools
		}
	}

	return map[string]interface{}{
		"model":   config.GetBaseModelName(modelFromPath),
		"request": request,
	}
}

// StreamResponse handles streaming response
func (c *Client) StreamResponse(resp *http.Response) <-chan []byte {
	ch := make(chan []byte)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := line[6:] // Remove "data: " prefix
				if data != "" && data != "[DONE]" {
					if obj := c.parseChunk(data); obj != nil {
						if response, ok := obj["response"].(map[string]interface{}); ok {
							if responseJSON, err := json.Marshal(response); err == nil {
								ch <- []byte(fmt.Sprintf("data: %s\n\n", string(responseJSON)))
							}
						} else {
							if objJSON, err := json.Marshal(obj); err == nil {
								ch <- []byte(fmt.Sprintf("data: %s\n\n", string(objJSON)))
							}
						}
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error reading streaming response: %v", err)
			errorResponse := map[string]interface{}{
				"error": map[string]interface{}{
					"message": fmt.Sprintf("Streaming error: %v", err),
					"type":    "api_error",
					"code":    500,
				},
			}
			if errorJSON, err := json.Marshal(errorResponse); err == nil {
				ch <- []byte(fmt.Sprintf("data: %s\n\n", string(errorJSON)))
			}
		}
	}()

	return ch
}

// parseChunk parses a JSON chunk from the streaming response
func (c *Client) parseChunk(chunk string) map[string]interface{} {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(chunk), &obj); err != nil {
		log.Printf("Failed to parse chunk: %v", err)
		return nil
	}
	return obj
}

// Helper functions

func getUserAgent() string {
	return "geminicli2api/1.0.0 (go)"
}

func createErrorResponse(statusCode int, body string) *http.Response {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")

	errorMessage := fmt.Sprintf("Google API error: %d", statusCode)
	if body != "" {
		var errorData map[string]interface{}
		if err := json.Unmarshal([]byte(body), &errorData); err == nil {
			if error, ok := errorData["error"].(map[string]interface{}); ok {
				if msg, ok := error["message"].(string); ok {
					errorMessage = msg
				}
			}
		}
	}

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"message": errorMessage,
			"type":    "invalid_request_error",
			"code":    statusCode,
		},
	}

	errorBody, _ := json.Marshal(errorResponse)

	return &http.Response{
		StatusCode: statusCode,
		Header:     headers,
		Body:       io.NopCloser(bytes.NewReader(errorBody)),
	}
}

func createRawResponse(statusCode int, body []byte, contentType string) *http.Response {
	headers := make(http.Header)
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}

	return &http.Response{
		StatusCode: statusCode,
		Header:     headers,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}