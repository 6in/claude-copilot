package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"copilot-proxy/config"
)

const (
	clientID      = "Iv1.b507a08c87ecfe98" // Well-known Client ID for GitHub Copilot
	deviceCodeURL = "https://github.com/login/device/code"
	tokenURL      = "https://github.com/login/oauth/access_token"
)

// DeviceCodeRequest holds the parameters for requesting a device code
type DeviceCodeRequest struct {
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
}

// DeviceCodeResponse maps to GitHub's response for a device code
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenRequest holds the parameters for polling the token endpoint
type TokenRequest struct {
	ClientID   string `json:"client_id"`
	DeviceCode string `json:"device_code"`
	GrantType  string `json:"grant_type"`
}

// TokenResponse maps to the final access token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

// EnsureToken checks if a valid token exists; if not, performs the Device Auth flow
func EnsureToken(cfg *config.AppConfig) error {
	if cfg.GitHubToken != "" {
		fmt.Println("✅ Found existing GitHub Copilot token.")
		// Optionally: Add logic here to validate the token if needed
		return nil
	}

	fmt.Println("🚀 Commencing GitHub Device Authentication...")

	// 1. Request Device Code
	reqBody, _ := json.Marshal(DeviceCodeRequest{
		ClientID: clientID,
		Scope:    "read:user", // Standard scope sufficient for Copilot API access via this ClientID
	})

	req, _ := http.NewRequest("POST", deviceCodeURL, bytes.NewBuffer(reqBody))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("device code request failed with status: %s", resp.Status)
	}

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return fmt.Errorf("failed to decode device response: %w", err)
	}

	// Print instructions for the user
	fmt.Println("\n========================================================")
	fmt.Printf("1. Open your browser to: %s\n", deviceResp.VerificationURI)
	fmt.Printf("2. Enter the following code: %s\n", deviceResp.UserCode)
	fmt.Println("Waiting for authorization...")
	fmt.Println("========================================================")

	// 2. Poll for Token
	token, err := pollForToken(deviceResp.DeviceCode, deviceResp.Interval, deviceResp.ExpiresIn)
	if err != nil {
		return err
	}

	fmt.Println("✅ Successfully authenticated!")

	// Save token
	cfg.GitHubToken = token
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	return nil
}

func pollForToken(deviceCode string, interval int, expiresIn int) (string, error) {
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	pollInterval := time.Duration(interval) * time.Second

	// Add 1s padding to avoid rate limiting
	if pollInterval < 5*time.Second {
		pollInterval = 5 * time.Second
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for time.Now().Before(deadline) {
		reqBody, _ := json.Marshal(TokenRequest{
			ClientID:   clientID,
			DeviceCode: deviceCode,
			GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		})

		req, _ := http.NewRequest("POST", tokenURL, bytes.NewBuffer(reqBody))
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			// Network error, just retry after interval
			time.Sleep(pollInterval)
			continue
		}

		var tokenResp TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			resp.Body.Close()
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		if tokenResp.AccessToken != "" {
			return tokenResp.AccessToken, nil
		}

		// Handle expected OAuth errors (e.g., authorization_pending)
		if tokenResp.Error != "authorization_pending" && tokenResp.Error != "slow_down" {
			return "", fmt.Errorf("auth error: %s", tokenResp.Error)
		}

		// If slow_down, increase interval
		if tokenResp.Error == "slow_down" {
			pollInterval += 5 * time.Second
		}

		time.Sleep(pollInterval)
	}

	return "", fmt.Errorf("authentication timed out")
}
