package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	// API Endpoints
	CodeAssistEndpoint = "https://cloudcode-pa.googleapis.com"

	// Client Configuration
	CLIVersion = "0.1.5" // Match current gemini-cli version
)

// OAuth Configuration - use environment variables
func GetClientID() string {
	return os.Getenv("GOOGLE_CLIENT_ID")
}

func GetClientSecret() string {
	return os.Getenv("GOOGLE_CLIENT_SECRET")
}

var (
	Scopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
)

// Config holds application configuration
type Config struct {
	CredentialFile      string
	GeminiAuthPassword  string
	CodeAssistEndpoint  string
	CLIVersion          string
	ClientID            string
	ClientSecret        string
	Scopes              []string
	SafetySettings      []map[string]interface{}
	SupportedModels     []Model
}

// Model represents a Gemini model configuration
type Model struct {
	Name                     string   `json:"name"`
	Version                  string   `json:"version"`
	DisplayName              string   `json:"displayName"`
	Description              string   `json:"description"`
	InputTokenLimit          int      `json:"inputTokenLimit"`
	OutputTokenLimit         int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	Temperature              float64  `json:"temperature"`
	MaxTemperature           float64  `json:"maxTemperature"`
	TopP                     float64  `json:"topP"`
	TopK                     int      `json:"topK"`
}

// NewConfig creates a new configuration instance
func NewConfig() *Config {
	scriptDir, _ := os.Getwd()
	credFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credFile == "" {
		credFile = "oauth_creds.json"
	}

	return &Config{
		CredentialFile:     fmt.Sprintf("%s/%s", scriptDir, credFile),
		GeminiAuthPassword: getEnvOrDefault("GEMINI_AUTH_PASSWORD", "123456"),
		CodeAssistEndpoint: CodeAssistEndpoint,
		CLIVersion:         CLIVersion,
		ClientID:           GetClientID(),
		ClientSecret:       GetClientSecret(),
		Scopes:             Scopes,
		SafetySettings:     getDefaultSafetySettings(),
		SupportedModels:    generateSupportedModels(),
	}
}

// getDefaultSafetySettings returns the default safety settings for Google API
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

// Base models configuration
func getBaseModels() []Model {
	return []Model{
		{
			Name:                      "models/gemini-2.5-pro-preview-03-25",
			Version:                   "001",
			DisplayName:               "Gemini 2.5 Pro Preview 03-25",
			Description:               "Preview version of Gemini 2.5 Pro from May 6th",
			InputTokenLimit:           1048576,
			OutputTokenLimit:          65535,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			Temperature:               1.0,
			MaxTemperature:            2.0,
			TopP:                      0.95,
			TopK:                      64,
		},
		{
			Name:                      "models/gemini-2.5-pro-preview-05-06",
			Version:                   "001",
			DisplayName:               "Gemini 2.5 Pro Preview 05-06",
			Description:               "Preview version of Gemini 2.5 Pro from May 6th",
			InputTokenLimit:           1048576,
			OutputTokenLimit:          65535,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			Temperature:               1.0,
			MaxTemperature:            2.0,
			TopP:                      0.95,
			TopK:                      64,
		},
		{
			Name:                      "models/gemini-2.5-pro-preview-06-05",
			Version:                   "001",
			DisplayName:               "Gemini 2.5 Pro Preview 06-05",
			Description:               "Preview version of Gemini 2.5 Pro from June 5th",
			InputTokenLimit:           1048576,
			OutputTokenLimit:          65535,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			Temperature:               1.0,
			MaxTemperature:            2.0,
			TopP:                      0.95,
			TopK:                      64,
		},
		{
			Name:                      "models/gemini-2.5-pro",
			Version:                   "001",
			DisplayName:               "Gemini 2.5 Pro",
			Description:               "Advanced multimodal model with enhanced capabilities",
			InputTokenLimit:           1048576,
			OutputTokenLimit:          65535,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			Temperature:               1.0,
			MaxTemperature:            2.0,
			TopP:                      0.95,
			TopK:                      64,
		},
		{
			Name:                      "models/gemini-2.5-flash-preview-05-20",
			Version:                   "001",
			DisplayName:               "Gemini 2.5 Flash Preview 05-20",
			Description:               "Preview version of Gemini 2.5 Flash from May 20th",
			InputTokenLimit:           1048576,
			OutputTokenLimit:          65535,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			Temperature:               1.0,
			MaxTemperature:            2.0,
			TopP:                      0.95,
			TopK:                      64,
		},
		{
			Name:                      "models/gemini-2.5-flash-preview-04-17",
			Version:                   "001",
			DisplayName:               "Gemini 2.5 Flash Preview 04-17",
			Description:               "Preview version of Gemini 2.5 Flash from April 17th",
			InputTokenLimit:           1048576,
			OutputTokenLimit:          65535,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			Temperature:               1.0,
			MaxTemperature:            2.0,
			TopP:                      0.95,
			TopK:                      64,
		},
		{
			Name:                      "models/gemini-2.5-flash",
			Version:                   "001",
			DisplayName:               "Gemini 2.5 Flash",
			Description:               "Fast and efficient multimodal model with latest improvements",
			InputTokenLimit:           1048576,
			OutputTokenLimit:          65535,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			Temperature:               1.0,
			MaxTemperature:            2.0,
			TopP:                      0.95,
			TopK:                      64,
		},
		{
			Name:                      "models/gemini-2.5-flash-image-preview",
			Version:                   "001",
			DisplayName:               "Gemini 2.5 Flash Image Preview",
			Description:               "Gemini 2.5 Flash Image Preview",
			InputTokenLimit:           32768,
			OutputTokenLimit:          32768,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			Temperature:               1.0,
			MaxTemperature:            2.0,
			TopP:                      0.95,
			TopK:                      64,
		},
	}
}

// generateSupportedModels generates all supported models including variants
func generateSupportedModels() []Model {
	baseModels := getBaseModels()
	var allModels []Model

	// Add base models
	allModels = append(allModels, baseModels...)

	// Add search variants
	for _, model := range baseModels {
		if !strings.Contains(model.Name, "gemini-2.5-flash-image") && contains(model.SupportedGenerationMethods, "generateContent") {
			searchVariant := model
			searchVariant.Name = model.Name + "-search"
			searchVariant.DisplayName = model.DisplayName + " with Google Search"
			searchVariant.Description = model.Description + " (includes Google Search grounding)"
			allModels = append(allModels, searchVariant)
		}
	}

	// Add thinking variants
	for _, model := range baseModels {
		if !strings.Contains(model.Name, "gemini-2.5-flash-image") &&
			contains(model.SupportedGenerationMethods, "generateContent") &&
			(strings.Contains(model.Name, "gemini-2.5-flash") || strings.Contains(model.Name, "gemini-2.5-pro")) {

			// Add -nothinking variant
			nothinkingVariant := model
			nothinkingVariant.Name = model.Name + "-nothinking"
			nothinkingVariant.DisplayName = model.DisplayName + " (No Thinking)"
			nothinkingVariant.Description = model.Description + " (thinking disabled)"
			allModels = append(allModels, nothinkingVariant)

			// Add -maxthinking variant
			maxthinkingVariant := model
			maxthinkingVariant.Name = model.Name + "-maxthinking"
			maxthinkingVariant.DisplayName = model.DisplayName + " (Max Thinking)"
			maxthinkingVariant.Description = model.Description + " (maximum thinking budget)"
			allModels = append(allModels, maxthinkingVariant)
		}
	}

	// Add combined variants (search + thinking)
	for _, model := range baseModels {
		if contains(model.SupportedGenerationMethods, "generateContent") &&
			(strings.Contains(model.Name, "gemini-2.5-flash") || strings.Contains(model.Name, "gemini-2.5-pro")) {

			// search + nothinking
			searchNothinking := model
			searchNothinking.Name = model.Name + "-search-nothinking"
			searchNothinking.DisplayName = model.DisplayName + " with Google Search (No Thinking)"
			searchNothinking.Description = model.Description + " (includes Google Search grounding, thinking disabled)"
			allModels = append(allModels, searchNothinking)

			// search + maxthinking
			searchMaxthinking := model
			searchMaxthinking.Name = model.Name + "-search-maxthinking"
			searchMaxthinking.DisplayName = model.DisplayName + " with Google Search (Max Thinking)"
			searchMaxthinking.Description = model.Description + " (includes Google Search grounding, maximum thinking budget)"
			allModels = append(allModels, searchMaxthinking)
		}
	}

	return allModels
}

// Helper functions for model variants
func GetBaseModelName(modelName string) string {
	suffixes := []string{"-maxthinking", "-nothinking", "-search"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(modelName, suffix) {
			return modelName[:len(modelName)-len(suffix)]
		}
	}
	return modelName
}

func IsSearchModel(modelName string) bool {
	return strings.Contains(modelName, "-search")
}

func IsNothinkingModel(modelName string) bool {
	return strings.Contains(modelName, "-nothinking")
}

func IsMaxthinkingModel(modelName string) bool {
	return strings.Contains(modelName, "-maxthinking")
}

func GetThinkingBudget(modelName string) int {
	baseModel := GetBaseModelName(modelName)

	if IsNothinkingModel(modelName) {
		if strings.Contains(baseModel, "gemini-2.5-flash") {
			return 0
		} else if strings.Contains(baseModel, "gemini-2.5-pro") {
			return 128
		}
	} else if IsMaxthinkingModel(modelName) {
		if strings.Contains(baseModel, "gemini-2.5-flash") {
			return 24576
		} else if strings.Contains(baseModel, "gemini-2.5-pro") {
			return 32768
		}
	} else {
		return -1 // Default for all models
	}
	return -1
}

func ShouldIncludeThoughts(modelName string) bool {
	if IsNothinkingModel(modelName) {
		baseModel := GetBaseModelName(modelName)
		return strings.Contains(baseModel, "gemini-2.5-pro")
	}
	return true
}

// Utility functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}