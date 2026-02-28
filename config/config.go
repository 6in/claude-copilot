package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AppConfig holds the necessary configurations for the proxy
type AppConfig struct {
	Port        string `json:"port"`
	GitHubToken string `json:"github_token"`
}

// LoadConfig reads the config or creates a default one
func LoadConfig() (*AppConfig, error) {
	configPath := GetConfigPath()

	// Create default config if not exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultCfg := &AppConfig{
			Port: "8080", // Default proxy port
		}
		if err := SaveConfig(defaultCfg); err != nil {
			return nil, err
		}
		return defaultCfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with env var if available
	if port := os.Getenv("PROXY_PORT"); port != "" {
		cfg.Port = port
	}

	return &cfg, nil
}

// SaveConfig writes the given config to the file system
func SaveConfig(cfg *AppConfig) error {
	configPath := GetConfigPath()

	// ensure dir exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil { // 0600 for sensitive token
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// DeleteConfig removes the config file (used by -logoff)
func DeleteConfig() error {
	configPath := GetConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // already gone
	}
	return os.Remove(configPath)
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "." // Fallback
	}
	return filepath.Join(homeDir, ".claude_copilot_proxy.json")
}
