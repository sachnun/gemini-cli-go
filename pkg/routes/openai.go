package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"geminicli2api/pkg/auth"
	"geminicli2api/pkg/config"
	"geminicli2api/pkg/google"
	"geminicli2api/pkg/models"
	"geminicli2api/pkg/transformers"
)

// OpenAIHandler handles OpenAI-compatible endpoints
type OpenAIHandler struct {
	authConfig *auth.AuthConfig
	googleClient *google.Client
	config      *config.Config
}

// NewOpenAIHandler creates a new OpenAI handler
func NewOpenAIHandler(authConfig *auth.AuthConfig, googleClient *google.Client, cfg *config.Config) *OpenAIHandler {
	return &OpenAIHandler{
		authConfig:  authConfig,
		googleClient: googleClient,
		config:      cfg,
	}
}

// RegisterRoutes registers OpenAI-compatible routes
func (h *OpenAIHandler) RegisterRoutes(router *gin.Engine) {
	openai := router.Group("/v1")
	{
		openai.POST("/chat/completions", h.AuthMiddleware(), h.ChatCompletions)
		openai.GET("/models", h.AuthMiddleware(), h.ListModels)
	}
}

// AuthMiddleware handles authentication for OpenAI routes
func (h *OpenAIHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		username, err := h.authConfig.AuthenticateUser(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":    "authentication_error",
					"code":    http.StatusUnauthorized,
				},
			})
			c.Abort()
			return
		}
		c.Set("username", username)
		c.Next()
	}
}

// ChatCompletions handles OpenAI chat completions
func (h *OpenAIHandler) ChatCompletions(c *gin.Context) {
	var request models.OpenAIChatCompletionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Invalid request format: " + err.Error(),
				"type":    "invalid_request_error",
				"code":    http.StatusBadRequest,
			},
		})
		return
	}

	log.Printf("OpenAI chat completion request: model=%s, stream=%v", request.Model, request.Stream)

	// Transform OpenAI request to Gemini format
	geminiRequestData, err := transformers.OpenAIRequestToGemini(&request)
	if err != nil {
		log.Printf("Error processing OpenAI request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Request processing failed: " + err.Error(),
				"type":    "invalid_request_error",
				"code":    http.StatusBadRequest,
			},
		})
		return
	}

	// Build the payload for Google API
	geminiPayload := h.googleClient.BuildGeminiPayloadFromOpenAI(geminiRequestData)

	if request.Stream {
		h.handleStreamingResponse(c, &request, geminiPayload)
	} else {
		h.handleNonStreamingResponse(c, &request, geminiPayload)
	}
}

// handleStreamingResponse handles streaming responses
func (h *OpenAIHandler) handleStreamingResponse(c *gin.Context, request *models.OpenAIChatCompletionRequest, geminiPayload map[string]interface{}) {
	responseID := fmt.Sprintf("chatcmpl-%s", uuid.New().String())
	log.Printf("Starting streaming response: %s", responseID)

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Send response
	resp, err := h.googleClient.SendGeminiRequest(c.Request.Context(), geminiPayload, true)
	if err != nil {
		log.Printf("Streaming request failed: %v", err)
		h.sendStreamingError(c, "Streaming request failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Google API returned status %d", resp.StatusCode)
		h.handleStreamingErrorResponse(c, resp)
		return
	}

	// Stream the response
	ch := h.googleClient.StreamResponse(resp)
	for chunk := range ch {
		_, err := c.Writer.Write(chunk)
		if err != nil {
			log.Printf("Error writing chunk: %v", err)
			return
		}
		c.Writer.Flush()
	}

	// Send final marker
	finalChunk := []byte("data: [DONE]\n\n")
	_, err = c.Writer.Write(finalChunk)
	if err != nil {
		log.Printf("Error writing final chunk: %v", err)
		return
	}
	c.Writer.Flush()

	log.Printf("Completed streaming response: %s", responseID)
}

// handleNonStreamingResponse handles non-streaming responses
func (h *OpenAIHandler) handleNonStreamingResponse(c *gin.Context, request *models.OpenAIChatCompletionRequest, geminiPayload map[string]interface{}) {
	resp, err := h.googleClient.SendGeminiRequest(c.Request.Context(), geminiPayload, false)
	if err != nil {
		log.Printf("Non-streaming request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Request failed: " + err.Error(),
				"type":    "api_error",
				"code":    http.StatusInternalServerError,
			},
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Google API returned status %d", resp.StatusCode)
		h.handleNonStreamingErrorResponse(c, resp)
		return
	}

	// Parse Gemini response and transform to OpenAI format
	var geminiResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&geminiResponse); err != nil {
		log.Printf("Failed to parse Gemini response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to process response: " + err.Error(),
				"type":    "api_error",
				"code":    http.StatusInternalServerError,
			},
		})
		return
	}

	openaiResponse := transformers.GeminiResponseToOpenAI(geminiResponse, request.Model)
	log.Printf("Successfully processed non-streaming response for model: %s", request.Model)

	c.JSON(http.StatusOK, openaiResponse)
}

// handleStreamingErrorResponse handles error responses in streaming mode
func (h *OpenAIHandler) handleStreamingErrorResponse(c *gin.Context, resp *http.Response) {
	// Try to parse error response
	var errorData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errorData); err == nil {
		if error, ok := errorData["error"].(map[string]interface{}); ok {
			h.sendStreamingError(c, error["message"].(string), resp.StatusCode)
			return
		}
	}

	h.sendStreamingError(c, fmt.Sprintf("API error: %d", resp.StatusCode), resp.StatusCode)
}

// handleNonStreamingErrorResponse handles error responses in non-streaming mode
func (h *OpenAIHandler) handleNonStreamingErrorResponse(c *gin.Context, resp *http.Response) {
	// Try to parse error response
	var errorData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errorData); err == nil {
		if error, ok := errorData["error"].(map[string]interface{}); ok {
			c.JSON(resp.StatusCode, gin.H{
				"error": gin.H{
					"message": error["message"],
					"type":    error["type"],
					"code":    error["code"],
				},
			})
			return
		}
	}

	// Fallback error response
	errorType := "invalid_request_error"
	if resp.StatusCode != http.StatusNotFound {
		errorType = "api_error"
	}

	c.JSON(resp.StatusCode, gin.H{
		"error": gin.H{
			"message": fmt.Sprintf("API error: %d", resp.StatusCode),
			"type":    errorType,
			"code":    resp.StatusCode,
		},
	})
}

// sendStreamingError sends an error in streaming format
func (h *OpenAIHandler) sendStreamingError(c *gin.Context, message string, code int) {
	errorType := "invalid_request_error"
	if code != http.StatusNotFound {
		errorType = "api_error"
	}

	errorData := gin.H{
		"error": gin.H{
			"message": message,
			"type":    errorType,
			"code":    code,
		},
	}

	if errorJSON, err := json.Marshal(errorData); err == nil {
		c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", string(errorJSON))))
		c.Writer.Write([]byte("data: [DONE]\n\n"))
		c.Writer.Flush()
	}
}

// ListModels handles OpenAI models list
func (h *OpenAIHandler) ListModels(c *gin.Context) {
	log.Printf("OpenAI models list requested")

	openaiModels := []gin.H{}
	for _, model := range h.config.SupportedModels {
		// Remove "models/" prefix for OpenAI compatibility
		modelID := model.Name
		if strings.HasPrefix(modelID, "models/") {
			modelID = modelID[7:]
		}

		modelPermission := gin.H{
			"id":                     "modelperm-" + strings.ReplaceAll(modelID, "/", "-"),
			"object":                 "model_permission",
			"created":                1677610602,
			"allow_create_engine":    false,
			"allow_sampling":         true,
			"allow_logprobs":         false,
			"allow_search_indices":   false,
			"allow_view":             true,
			"allow_fine_tuning":      false,
			"organization":           "*",
			"group":                  nil,
			"is_blocking":            false,
		}

		openaiModels = append(openaiModels, gin.H{
			"id":         modelID,
			"object":     "model",
			"created":    1677610602, // Static timestamp
			"owned_by":   "google",
			"permission": []gin.H{modelPermission},
			"root":       modelID,
			"parent":     nil,
		})
	}

	log.Printf("Returning %d models", len(openaiModels))

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   openaiModels,
	})
}