package napcat

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadConfig loads configuration from file or returns default
func LoadConfig() Config {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return DefaultConfig()
	}

	configPath := filepath.Join(configDir, "LagrangeQQ", "napcat.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Create default config file
		defaultCfg := DefaultConfig()
		_ = SaveConfig(defaultCfg)
		return defaultCfg
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig()
	}
	return cfg
}

// SaveConfig saves configuration to file
func SaveConfig(cfg Config) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	appDir := filepath.Join(configDir, "LagrangeQQ")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(appDir, "napcat.json")
	return os.WriteFile(configPath, data, 0644)
}
