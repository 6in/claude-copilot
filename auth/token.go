package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const copilotInternalTokenURL = "https://api.github.com/copilot_internal/v2/token"

// CopilotTokenResponse represents the token received to talk to the Copilot Chat API
type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	// ignoring other telemetry/tracking fields for now
}

// Cached session token
var (
	cachedCopilotToken string
	tokenExpiresAt     time.Time
)

// GetCopilotToken exchanges the GitHub OAuth token for a Copilot Session Token.
// It caches the token in memory until it expires.
func GetCopilotToken(githubOauthToken string) (string, error) {
	// Return cached token if still valid (adding 30 sec buffer)
	if cachedCopilotToken != "" && time.Now().Add(30*time.Second).Before(tokenExpiresAt) {
		return cachedCopilotToken, nil
	}

	req, err := http.NewRequest("GET", copilotInternalTokenURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "token "+githubOauthToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Version", "vscode/1.90.0")
	req.Header.Set("Editor-Plugin-Version", "copilot-chat/0.15.0")
	req.Header.Set("User-Agent", "GitHubCopilotChat/0.15.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch copilot token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch copilot token, status %d", resp.StatusCode)
	}

	var tokenResp CopilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode copilot token response: %w", err)
	}

	// Cache the properties
	cachedCopilotToken = tokenResp.Token
	tokenExpiresAt = time.Unix(tokenResp.ExpiresAt, 0)

	fmt.Println("🔄 Refreshed GitHub Copilot API Session Token")

	return cachedCopilotToken, nil
}
