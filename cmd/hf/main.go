package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"geminicli2api/pkg/auth"
	"geminicli2api/pkg/config"
	"geminicli2api/pkg/google"
	"geminicli2api/pkg/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading .env file")
	}

	// Set port for Hugging Face Spaces
	os.Setenv("PORT", "7860")
	log.Println("Starting in Hugging Face Spaces mode on port 7860")

	// Initialize configuration
	cfg := config.NewConfig()

	// Initialize authentication
	authConfig := auth.NewAuthConfig(cfg)

	// Initialize Google API client
	googleClient := google.NewClient(authConfig, cfg)

	// Initialize handlers
	openaiHandler := routes.NewOpenAIHandler(authConfig, googleClient, cfg)
	geminiHandler := routes.NewGeminiHandler(authConfig, googleClient, cfg)

	// Initialize Gin router
	router := gin.Default()

	// Add CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Handle CORS preflight requests
	router.OPTIONS("/*path", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Status(http.StatusOK)
	})

	// Root endpoint - no authentication required
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":        "geminicli2api",
			"description": "OpenAI-compatible API proxy for Google's Gemini models via gemini-cli",
			"purpose":     "Provides both OpenAI-compatible endpoints (/v1/chat/completions) and native Gemini API endpoints for accessing Google's Gemini models",
			"version":     "1.0.0",
			"endpoints": gin.H{
				"openai_compatible": gin.H{
					"chat_completions": "/v1/chat/completions",
					"models":           "/v1/models",
				},
				"native_gemini": gin.H{
					"models":    "/v1beta/models",
					"generate":  "/v1beta/models/{model}/generateContent",
					"stream":    "/v1beta/models/{model}/streamGenerateContent",
				},
				"health": "/health",
			},
			"authentication": "Required for all endpoints except root and health",
			"repository":     "https://github.com/user/geminicli2api",
		})
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "geminicli2api",
		})
	})

	// Register OpenAI routes
	openaiHandler.RegisterRoutes(router)

	// Register Gemini routes
	geminiHandler.RegisterRoutes(router)

	// Perform startup authentication and onboarding
	if err := performStartupSetup(authConfig); err != nil {
		log.Printf("Startup setup warning: %v", err)
	}

	log.Printf("Starting Gemini proxy server on port 7860")
	log.Printf("Authentication required - Password: see .env file")

	// Start server
	if err := router.Run(":7860"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// performStartupSetup handles startup authentication and onboarding
func performStartupSetup(authConfig *auth.AuthConfig) error {
	log.Println("Starting Gemini proxy server...")

	// Check if credentials exist
	envCredsJSON := os.Getenv("GEMINI_CREDENTIALS")
	credsFileExists := false
	if _, err := os.Stat(authConfig.Config.CredentialFile); err == nil {
		credsFileExists = true
	}

	if envCredsJSON != "" || credsFileExists {
		// Try to load existing credentials without OAuth flow first
		if creds, err := authConfig.GetCredentials(false); err == nil && creds != nil {
			if projID, err := authConfig.GetUserProjectID(creds); err == nil && projID != "" {
				if err := authConfig.OnboardUser(creds, projID); err == nil {
					log.Printf("Successfully onboarded with project ID: %s", projID)
					log.Println("Gemini proxy server started successfully")
					log.Println("Authentication required - Password: see .env file")
					return nil
				} else {
					log.Printf("Setup failed: %v", err)
					return fmt.Errorf("setup failed: %w", err)
				}
			}
		} else {
			log.Println("Credentials file exists but could not be loaded. Server started - authentication will be required on first request.")
			return nil
		}
	} else {
		// No credentials found - prompt user to authenticate
		log.Println("No credentials found. Starting OAuth authentication flow...")
		if creds, err := authConfig.GetCredentials(true); err == nil && creds != nil {
			if projID, err := authConfig.GetUserProjectID(creds); err == nil && projID != "" {
				if err := authConfig.OnboardUser(creds, projID); err == nil {
					log.Printf("Successfully onboarded with project ID: %s", projID)
					log.Println("Gemini proxy server started successfully")
				} else {
					log.Printf("Setup failed: %v", err)
					return fmt.Errorf("setup failed: %w", err)
				}
			}
		} else {
			log.Println("Authentication failed. Server started but will not function until credentials are provided.")
			return fmt.Errorf("authentication failed")
		}
	}

	log.Println("Authentication required - Password: see .env file")
	return nil
}