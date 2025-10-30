package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"geminicli2api/pkg/config"
)

var (
	credentials     *oauth2.Token
	userProjectID   string
	onboardingDone  bool
	credsFromEnv    bool
	credentialsMux  sync.RWMutex
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Config         *config.Config
	OAuth2Config   *oauth2.Config
	HTTPClient     *http.Client
}

// NewAuthConfig creates a new authentication configuration
func NewAuthConfig(cfg *config.Config) *AuthConfig {
	oauth2Config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8080",
	}

	return &AuthConfig{
		Config:       cfg,
		OAuth2Config: oauth2Config,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// AuthenticateUser authenticates the user with multiple methods
func (ac *AuthConfig) AuthenticateUser(r *http.Request) (string, error) {
	// Check for API key in query parameters first (for Gemini client compatibility)
	apiKey := r.URL.Query().Get("key")
	if apiKey == ac.Config.GeminiAuthPassword {
		return "api_key_user", nil
	}

	// Check for API key in x-goog-api-key header (Google SDK format)
	googAPIKey := r.Header.Get("x-goog-api-key")
	if googAPIKey == ac.Config.GeminiAuthPassword {
		return "goog_api_key_user", nil
	}

	// Check for API key in Authorization header (Bearer token format)
	authHeader := r.Header.Get("authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
		if bearerToken == ac.Config.GeminiAuthPassword {
			return "bearer_user", nil
		}
	}

	// Check for HTTP Basic Authentication
	if strings.HasPrefix(authHeader, "Basic ") {
		encodedCreds := strings.TrimPrefix(authHeader, "Basic ")
		decodedCreds, err := base64.StdEncoding.DecodeString(encodedCreds)
		if err == nil {
			creds := string(decodedCreds)
			parts := strings.SplitN(creds, ":", 2)
			if len(parts) == 2 && parts[1] == ac.Config.GeminiAuthPassword {
				return parts[0], nil
			}
		}
	}

	return "", fmt.Errorf("invalid authentication credentials. Use HTTP Basic Auth, Bearer token, 'key' query parameter, or 'x-goog-api-key' header")
}

// GetCredentials loads OAuth2 credentials
func (ac *AuthConfig) GetCredentials(allowOAuthFlow bool) (*oauth2.Token, error) {
	credentialsMux.RLock()
	if credentials != nil && credentials.Valid() {
		creds := credentials
		credentialsMux.RUnlock()
		return creds, nil
	}
	credentialsMux.RUnlock()

	// Check environment variable first
	if envCredsJSON := os.Getenv("GEMINI_CREDENTIALS"); envCredsJSON != "" {
		token, err := ac.parseEnvCredentials(envCredsJSON)
		if err == nil {
			credentialsMux.Lock()
			credentials = token
			credsFromEnv = true
			credentialsMux.Unlock()
			return token, nil
		}
		log.Printf("Failed to parse environment credentials: %v", err)
	}

	// Check credential file
	if _, err := os.Stat(ac.Config.CredentialFile); err == nil {
		token, err := ac.loadFileCredentials()
		if err == nil {
			credentialsMux.Lock()
			credentials = token
			credsFromEnv = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != ""
			credentialsMux.Unlock()
			return token, nil
		}
		log.Printf("Failed to load file credentials: %v", err)
	}

	if !allowOAuthFlow {
		return nil, nil
	}

	// Start OAuth flow
	return ac.startOAuthFlow()
}

// parseEnvCredentials parses credentials from environment variable
func (ac *AuthConfig) parseEnvCredentials(envCredsJSON string) (*oauth2.Token, error) {
	var credsData map[string]interface{}
	if err := json.Unmarshal([]byte(envCredsJSON), &credsData); err != nil {
		return nil, fmt.Errorf("failed to parse environment credentials JSON: %w", err)
	}

	// Check for refresh token
	if refreshToken, ok := credsData["refresh_token"].(string); ok && refreshToken != "" {
		log.Println("Environment refresh token found - creating credentials")

		token := &oauth2.Token{
			RefreshToken: refreshToken,
			TokenType:    "Bearer",
		}

		if accessToken, ok := credsData["access_token"].(string); ok {
			token.AccessToken = accessToken
		}
		if tokenStr, ok := credsData["token"].(string); ok {
			token.AccessToken = tokenStr
		}

		// Handle expiry
		if expiryStr, ok := credsData["expiry"].(string); ok {
			if expiry, err := time.Parse(time.RFC3339, expiryStr); err == nil {
				token.Expiry = expiry
			}
		}

		// Extract project ID if available
		if projectID, ok := credsData["project_id"].(string); ok {
			credentialsMux.Lock()
			userProjectID = projectID
			credentialsMux.Unlock()
			log.Printf("Extracted project_id from environment credentials: %s", projectID)
		}

		// Try to refresh if needed
		if !token.Valid() && token.RefreshToken != "" {
			if err := ac.RefreshToken(token); err != nil {
				log.Printf("Failed to refresh environment credentials: %v", err)
			}
		}

		return token, nil
	}

	return nil, fmt.Errorf("no refresh token found in environment credentials")
}

// loadFileCredentials loads credentials from file
func (ac *AuthConfig) loadFileCredentials() (*oauth2.Token, error) {
	data, err := os.ReadFile(ac.Config.CredentialFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read credential file: %w", err)
	}

	var credsData map[string]interface{}
	if err := json.Unmarshal(data, &credsData); err != nil {
		return nil, fmt.Errorf("failed to parse credential file JSON: %w", err)
	}

	// Check for refresh token
	if refreshToken, ok := credsData["refresh_token"].(string); ok && refreshToken != "" {
		log.Println("File refresh token found - creating credentials")

		token := &oauth2.Token{
			RefreshToken: refreshToken,
			TokenType:    "Bearer",
		}

		if accessToken, ok := credsData["access_token"].(string); ok {
			token.AccessToken = accessToken
		}
		if tokenStr, ok := credsData["token"].(string); ok {
			token.AccessToken = tokenStr
		}

		// Handle expiry
		if expiryStr, ok := credsData["expiry"].(string); ok {
			if expiry, err := time.Parse(time.RFC3339, expiryStr); err == nil {
				token.Expiry = expiry
			}
		}

		// Extract project ID if available
		if projectID, ok := credsData["project_id"].(string); ok {
			credentialsMux.Lock()
			userProjectID = projectID
			credentialsMux.Unlock()
		}

		// Try to refresh if needed
		if !token.Valid() && token.RefreshToken != "" {
			if err := ac.RefreshToken(token); err == nil {
				ac.SaveCredentials(token, "")
			} else {
				log.Printf("Failed to refresh file credentials: %v", err)
			}
		}

		return token, nil
	}

	return nil, fmt.Errorf("no refresh token found in credential file")
}

// RefreshToken refreshes the OAuth2 token
func (ac *AuthConfig) RefreshToken(token *oauth2.Token) error {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, ac.HTTPClient)
	// Use the OAuth2 config to refresh the token
	newToken, err := ac.OAuth2Config.TokenSource(ctx, token).Token()
	if err != nil {
		return err
	}
	// Update the existing token
	token.AccessToken = newToken.AccessToken
	token.RefreshToken = newToken.RefreshToken
	token.TokenType = newToken.TokenType
	token.Expiry = newToken.Expiry
	return nil
}

// startOAuthFlow starts the OAuth2 flow
func (ac *AuthConfig) startOAuthFlow() (*oauth2.Token, error) {
	authURL := ac.OAuth2Config.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("AUTHENTICATION REQUIRED\n")
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("Please open this URL in your browser to log in:\n")
	fmt.Printf("%s\n", authURL)
	fmt.Printf("%s\n\n", strings.Repeat("=", 80))

	// Start callback server
	authCode, err := ac.startCallbackServer()
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}

	if authCode == "" {
		return nil, fmt.Errorf("no authorization code received")
	}

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, ac.HTTPClient)
	token, err := ac.OAuth2Config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	credentialsMux.Lock()
	credentials = token
	credsFromEnv = false
	credentialsMux.Unlock()

	ac.SaveCredentials(token, "")
	log.Println("Authentication successful! Credentials saved.")

	return token, nil
}

// startCallbackServer starts a local HTTP server to handle OAuth callback
func (ac *AuthConfig) startCallbackServer() (string, error) {
	var authCode string
	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		code := query.Get("code")
		if code != "" {
			authCode = code
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<h1>OAuth authentication successful!</h1><p>You can close this window. Please check the proxy server logs to verify that onboarding completed successfully. No need to restart the proxy.</p>")
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<h1>Authentication failed.</h1><p>Please try again.</p>")
		}

		go func() {
			time.Sleep(100 * time.Millisecond)
			server.Shutdown(context.Background())
		}()
	})

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return "", fmt.Errorf("callback server error: %w", err)
	}

	return authCode, nil
}

// SaveCredentials saves credentials to file
func (ac *AuthConfig) SaveCredentials(token *oauth2.Token, projectID string) {
	if credsFromEnv {
		// Don't overwrite environment credentials, but update project ID if needed
		if projectID != "" {
			ac.updateProjectIDInFile(projectID)
		}
		return
	}

	credsData := map[string]interface{}{
		"client_id":     ac.Config.ClientID,
		"client_secret": ac.Config.ClientSecret,
		"token":         token.AccessToken,
		"refresh_token": token.RefreshToken,
		"token_uri":     "https://oauth2.googleapis.com/token",
		"scopes":        ac.Config.Scopes,
	}

	if !token.Expiry.IsZero() {
		credsData["expiry"] = token.Expiry.Format(time.RFC3339)
	}

	if projectID != "" {
		credsData["project_id"] = projectID
	} else {
		// Try to preserve existing project ID
		if existingProjectID := ac.getProjectIDFromFile(); existingProjectID != "" {
			credsData["project_id"] = existingProjectID
		}
	}

	data, _ := json.MarshalIndent(credsData, "", "  ")
	os.WriteFile(ac.Config.CredentialFile, data, 0600)
}

// updateProjectIDInFile updates project ID in existing credential file
func (ac *AuthConfig) updateProjectIDInFile(projectID string) {
	if data, err := os.ReadFile(ac.Config.CredentialFile); err == nil {
		var existingData map[string]interface{}
		if json.Unmarshal(data, &existingData) == nil {
			if _, hasProjectID := existingData["project_id"]; !hasProjectID {
				existingData["project_id"] = projectID
				if newData, err := json.MarshalIndent(existingData, "", "  "); err == nil {
					os.WriteFile(ac.Config.CredentialFile, newData, 0600)
					log.Printf("Added project_id %s to existing credential file", projectID)
				}
			}
		}
	}
}

// getProjectIDFromFile gets project ID from credential file
func (ac *AuthConfig) getProjectIDFromFile() string {
	if data, err := os.ReadFile(ac.Config.CredentialFile); err == nil {
		var credsData map[string]interface{}
		if json.Unmarshal(data, &credsData) == nil {
			if projectID, ok := credsData["project_id"].(string); ok {
				return projectID
			}
		}
	}
	return ""
}

// GetUserProjectID gets the user's project ID
func (ac *AuthConfig) GetUserProjectID(token *oauth2.Token) (string, error) {
	credentialsMux.RLock()
	defer credentialsMux.RUnlock()

	// Priority 1: Check environment variable
	if envProjectID := os.Getenv("GOOGLE_CLOUD_PROJECT"); envProjectID != "" {
		log.Printf("Using project ID from GOOGLE_CLOUD_PROJECT environment variable: %s", envProjectID)
		userProjectID = envProjectID
		ac.SaveCredentials(token, envProjectID)
		return envProjectID, nil
	}

	// Priority 2: Use cached project ID
	if userProjectID != "" {
		log.Printf("Using cached project ID: %s", userProjectID)
		return userProjectID, nil
	}

	// Priority 3: Check credential file
	if projectID := ac.getProjectIDFromFile(); projectID != "" {
		log.Printf("Using cached project ID from credential file: %s", projectID)
		userProjectID = projectID
		return projectID, nil
	}

	// Priority 4: Discover via API call
	return ac.discoverProjectID(token)
}

// discoverProjectID discovers project ID via API call
func (ac *AuthConfig) discoverProjectID(token *oauth2.Token) (string, error) {
	// Ensure token is valid
	if !token.Valid() && token.RefreshToken != "" {
		if err := ac.RefreshToken(token); err != nil {
			log.Printf("Failed to refresh credentials while getting project ID: %v", err)
		}
	}

	if token.AccessToken == "" {
		return "", fmt.Errorf("no valid access token available for project ID discovery")
	}

	probePayload := map[string]interface{}{
		"metadata": ac.getClientMetadata(),
	}

	payload, _ := json.Marshal(probePayload)

	req, err := http.NewRequest("POST", ac.Config.CodeAssistEndpoint+"/v1internal:loadCodeAssist", strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", getUserAgent())

	resp, err := ac.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	projectID, ok := data["cloudaicompanionProject"].(string)
	if !ok || projectID == "" {
		return "", fmt.Errorf("could not find 'cloudaicompanionProject' in loadCodeAssist response")
	}

	log.Printf("Discovered project ID via API: %s", projectID)
	userProjectID = projectID
	ac.SaveCredentials(token, projectID)

	return projectID, nil
}

// OnboardUser ensures the user is onboarded
func (ac *AuthConfig) OnboardUser(token *oauth2.Token, projectID string) error {
	credentialsMux.Lock()
	if onboardingDone {
		credentialsMux.Unlock()
		return nil
	}
	credentialsMux.Unlock()

	// Ensure token is valid
	if !token.Valid() && token.RefreshToken != "" {
		if err := ac.RefreshToken(token); err != nil {
			return fmt.Errorf("failed to refresh credentials during onboarding: %w", err)
		}
		ac.SaveCredentials(token, "")
	}

	// Load assist to check tier
	if err := ac.loadCodeAssist(token, projectID); err != nil {
		return fmt.Errorf("loadCodeAssist failed: %w", err)
	}

	// If not already onboarded, start onboarding process
	if err := ac.startOnboarding(token, projectID); err != nil {
		return fmt.Errorf("onboarding failed: %w", err)
	}

	credentialsMux.Lock()
	onboardingDone = true
	credentialsMux.Unlock()

	return nil
}

// loadCodeAssist loads code assist to check current status
func (ac *AuthConfig) loadCodeAssist(token *oauth2.Token, projectID string) error {
	payload := map[string]interface{}{
		"cloudaicompanionProject": projectID,
		"metadata":                ac.getClientMetadata(),
	}

	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", ac.Config.CodeAssistEndpoint+"/v1internal:loadCodeAssist", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", getUserAgent())

	resp, err := ac.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("loadCodeAssist failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loadData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&loadData); err != nil {
		return fmt.Errorf("failed to decode loadCodeAssist response: %w", err)
	}

	// Check if already onboarded
	if _, ok := loadData["currentTier"]; ok {
		credentialsMux.Lock()
		onboardingDone = true
		credentialsMux.Unlock()
		return nil
	}

	return nil
}

// startOnboarding starts the onboarding process
func (ac *AuthConfig) startOnboarding(token *oauth2.Token, projectID string) error {
	tierID := "legacy-tier" // Default tier

	payload := map[string]interface{}{
		"tierId":                  tierID,
		"cloudaicompanionProject": projectID,
		"metadata":                ac.getClientMetadata(),
	}

	data, _ := json.Marshal(payload)

	for {
		req, err := http.NewRequest("POST", ac.Config.CodeAssistEndpoint+"/v1internal:onboardUser", strings.NewReader(string(data)))
		if err != nil {
			return fmt.Errorf("failed to create onboarding request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", getUserAgent())

		resp, err := ac.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to make onboarding request: %w", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("onboarding request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var lroData map[string]interface{}
		if err := json.Unmarshal(body, &lroData); err != nil {
			return fmt.Errorf("failed to decode onboarding response: %w", err)
		}

		if done, ok := lroData["done"].(bool); ok && done {
			break
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}

// getClientMetadata returns client metadata for API calls
func (ac *AuthConfig) getClientMetadata() map[string]interface{} {
	return map[string]interface{}{
		"clientName":    "geminicli2api",
		"clientVersion": "1.0.0",
		"platform":      "go",
	}
}

// getUserAgent returns the user agent string
func getUserAgent() string {
	return fmt.Sprintf("geminicli2api/1.0.0 (go)")
}