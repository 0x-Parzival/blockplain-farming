// blockplain/config/config.go

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// Config holds the application configuration
type Config struct {
	DataDir      string        `json:"data_dir"`
	BlockSize    int           `json:"block_size"`
	SyncInterval time.Duration `json:"sync_interval"`
	LogLevel     string        `json:"log_level"`
	NodeID       string        `json:"node_id"`
	PeerList     []string      `json:"peer_list"`
	APIPort      int           `json:"api_port"`
	P2PPort      int           `json:"p2p_port"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	
	return &Config{
		DataDir:      filepath.Join(homeDir, ".blockplain"),
		BlockSize:    1024 * 1024, // 1MB
		SyncInterval: 30 * time.Second,
		LogLevel:     "info",
		NodeID:       "",
		PeerList:     []string{},
		APIPort:      8080,
		P2PPort:      9090,
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return DefaultConfig(), nil
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to a file
func SaveConfig(config *Config, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %v", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// EnsureDataDir creates the data directory if it doesn't exist
func (c *Config) EnsureDataDir() error {
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}
	
	if c.BlockSize <= 0 {
		return fmt.Errorf("block size must be positive")
	}
	
	if c.SyncInterval <= 0 {
		return fmt.Errorf("sync interval must be positive")
	}
	
	if c.APIPort <= 0 || c.APIPort > 65535 {
		return fmt.Errorf("API port must be between 1 and 65535")
	}
	
	if c.P2PPort <= 0 || c.P2PPort > 65535 {
		return fmt.Errorf("P2P port must be between 1 and 65535")
	}
	
	return nil
}