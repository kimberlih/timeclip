package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"timeclip/internal/models"
)

// Manager handles configuration loading, validation, and generation
type Manager struct {
	config     *models.Config
	configPath string
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{}
}

// Load loads the configuration from the default or specified path
func (m *Manager) Load(configPath ...string) (*models.Config, error) {
	var path string
	if len(configPath) > 0 && configPath[0] != "" {
		path = configPath[0]
	} else {
		var err error
		path, err = m.getDefaultConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to get default config path: %w", err)
		}
	}

	m.configPath = path

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Config doesn't exist, generate it
		return m.generateDefaultConfig(path)
	}

	// Load existing config
	return m.loadFromFile(path)
}

// loadFromFile loads configuration from the specified file
func (m *Manager) loadFromFile(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	config := models.DefaultConfig()
	if err := toml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Validate the loaded configuration
	if err := m.validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	m.config = config
	return config, nil
}

// generateDefaultConfig creates a default configuration file and prompts user
func (m *Manager) generateDefaultConfig(path string) (*models.Config, error) {
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory %s: %w", configDir, err)
	}

	// Generate default config
	config := models.DefaultConfig()

	// Save to file
	if err := m.saveToFile(config, path); err != nil {
		return nil, fmt.Errorf("failed to save default config: %w", err)
	}

	// Inform user about config creation
	fmt.Printf("🎉 First run detected!\n")
	fmt.Printf("📁 Created default configuration file: %s\n\n", path)
	fmt.Printf("⚠️  IMPORTANT: Please edit the configuration file to add your API credentials:\n")
	fmt.Printf("   - Set your Magnetic or Clockify API key\n")
	fmt.Printf("   - Set your workspace and project IDs\n")
	fmt.Printf("   - Customize your daily goal and tracking days\n\n")
	fmt.Printf("📖 Run 'timeclip' again after editing the config file.\n")

	// Exit after generating config to force user to edit it
	os.Exit(0)
	return nil, nil // This will never be reached
}

// saveToFile saves configuration to the specified file
func (m *Manager) saveToFile(config *models.Config, path string) error {
	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validateConfig validates the configuration for common issues
func (m *Manager) validateConfig(config *models.Config) error {
	var errors []string

	// Validate general settings
	if config.General.GoalTimeHours <= 0 {
		errors = append(errors, "goal_time_hours must be greater than 0")
	}
	if config.General.AutoLogThresholdHours <= 0 {
		errors = append(errors, "auto_log_threshold_hours must be greater than 0")
	}
	if config.General.CheckIntervalSeconds < 10 {
		errors = append(errors, "check_interval_seconds must be at least 10 seconds")
	}

	// Validate track days
	validDays := map[string]bool{
		"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
		"friday": true, "saturday": true, "sunday": true,
	}
	for _, day := range config.General.TrackDays {
		if !validDays[strings.ToLower(day)] {
			errors = append(errors, fmt.Sprintf("invalid track day: %s", day))
		}
	}

	// Validate API configuration
	if config.API.PreferredProvider != "magnetic" && config.API.PreferredProvider != "clockify" {
		errors = append(errors, "preferred_provider must be either 'magnetic' or 'clockify'")
	}

	// Check that at least one API is enabled and configured
	magneticEnabled := config.API.Magnetic.Enabled && config.API.Magnetic.APIKey != ""
	clockifyEnabled := config.API.Clockify.Enabled && config.API.Clockify.APIKey != ""

	if !magneticEnabled && !clockifyEnabled {
		errors = append(errors, "at least one API must be enabled with a valid API key")
	}

	// Validate preferred provider is actually enabled
	if config.API.PreferredProvider == "magnetic" && !magneticEnabled {
		errors = append(errors, "magnetic is set as preferred provider but is not properly configured")
	}
	if config.API.PreferredProvider == "clockify" && !clockifyEnabled {
		errors = append(errors, "clockify is set as preferred provider but is not properly configured")
	}

	// Validate database path
	if config.Database.Path == "" {
		errors = append(errors, "database path cannot be empty")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// getDefaultConfigPath returns the default configuration file path
func (m *Manager) getDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".timeclip", "config.toml"), nil
}

// GetConfig returns the loaded configuration
func (m *Manager) GetConfig() *models.Config {
	return m.config
}

// GetConfigPath returns the path to the configuration file
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// SaveConfig saves a configuration to the file
func (m *Manager) SaveConfig(config *models.Config) error {
	if m.configPath == "" {
		return fmt.Errorf("no config path set")
	}

	// Validate the configuration before saving
	if err := m.validateConfig(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Save to file
	if err := m.saveToFile(config, m.configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Update internal config
	m.config = config

	return nil
}

// ExpandPath expands ~ in file paths to the user's home directory
func (m *Manager) ExpandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, path[2:]), nil
}