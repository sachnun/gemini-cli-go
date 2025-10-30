package routes

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"geminicli2api/pkg/auth"
	"geminicli2api/pkg/config"
	"geminicli2api/pkg/google"
)

// GeminiHandler handles native Gemini API endpoints
type GeminiHandler struct {
	authConfig   *auth.AuthConfig
	googleClient *google.Client
	config       *config.Config
}

// NewGeminiHandler creates a new Gemini handler
func NewGeminiHandler(authConfig *auth.AuthConfig, googleClient *google.Client, cfg *config.Config) *GeminiHandler {
	return &GeminiHandler{
		authConfig:   authConfig,
		googleClient: googleClient,
		config:       cfg,
	}
}

// RegisterRoutes registers native Gemini API routes
func (h *GeminiHandler) RegisterRoutes(router *gin.Engine) {
	// Native Gemini endpoints
	router.GET("/v1beta/models", h.AuthMiddleware(), h.ListModels)
	// Specific generateContent endpoints
	router.POST("/v1beta/models/:model/generateContent", h.AuthMiddleware(), h.GeminiProxy)
	router.POST("/v1beta/models/:model/streamGenerateContent", h.AuthMiddleware(), h.GeminiProxy)
}

// AuthMiddleware handles authentication for Gemini routes
func (h *GeminiHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		username, err := h.authConfig.AuthenticateUser(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": err.Error(),
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

// ListModels handles native Gemini models list
func (h *GeminiHandler) ListModels(c *gin.Context) {
	log.Printf("Gemini models list requested")

	modelsResponse := gin.H{
		"models": h.config.SupportedModels,
	}

	log.Printf("Returning %d Gemini models", len(h.config.SupportedModels))

	c.JSON(http.StatusOK, modelsResponse)
}

// GeminiProxy handles native Gemini API proxy requests
func (h *GeminiHandler) GeminiProxy(c *gin.Context) {
	fullPath := c.Param("full_path")
	if fullPath == "" {
		fullPath = c.Request.URL.Path
	}

	// Determine if this is a streaming request
	isStreaming := strings.Contains(strings.ToLower(fullPath), "stream")

	// Extract model name from the path
	modelName := extractModelFromPath(fullPath)

	log.Printf("Gemini proxy request: path=%s, model=%s, stream=%v", fullPath, modelName, isStreaming)

	if modelName == "" {
		log.Printf("Could not extract model name from path: %s", fullPath)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Could not extract model name from path: " + fullPath,
				"code":    http.StatusBadRequest,
			},
		})
		return
	}

	// Read the request body
	var requestData map[string]interface{}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&requestData); err != nil {
			log.Printf("Invalid JSON in request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "Invalid JSON in request body",
					"code":    http.StatusBadRequest,
				},
			})
			return
		}
	}

	// Build the payload for Google API
	geminiPayload := h.googleClient.BuildGeminiPayloadFromNative(requestData, modelName)

	// Send the request to Google API
	resp, err := h.googleClient.SendGeminiRequest(c.Request.Context(), geminiPayload, isStreaming)
	if err != nil {
		log.Printf("Gemini proxy error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Proxy error: " + err.Error(),
				"code":    http.StatusInternalServerError,
			},
		})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// Set status code
	c.Status(resp.StatusCode)

	// Stream response body
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Printf("Error streaming response: %v", err)
		return
	}

	if resp.StatusCode == http.StatusOK {
		log.Printf("Successfully processed Gemini request for model: %s", modelName)
	} else {
		log.Printf("Gemini API returned error: status=%d", resp.StatusCode)
	}
}

// HealthCheck handles health check endpoint
func (h *GeminiHandler) HealthCheckDisabled(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "geminicli2api",
	})
}

// extractModelFromPath extracts the model name from a Gemini API path
//
// Examples:
// - "v1beta/models/gemini-1.5-pro/generateContent" -> "gemini-1.5-pro"
// - "v1/models/gemini-2.0-flash/streamGenerateContent" -> "gemini-2.0-flash"
//
// Args:
//   path: The API path
//
// Returns:
//   Model name (just the model name, not prefixed with "models/") or empty string if not found
func extractModelFromPath(path string) string {
	parts := strings.Split(path, "/")

	// Look for the pattern: .../models/{model_name}/...
	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			modelName := parts[i+1]
			// Remove any action suffix like ":streamGenerateContent" or ":generateContent"
			if idx := strings.Index(modelName, ":"); idx != -1 {
				modelName = modelName[:idx]
			}
			// Return just the model name without "models/" prefix
			return modelName
		}
	}

	// If we can't find the pattern, return empty string
	return ""
}