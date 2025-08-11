package models

// Config represents the application configuration
type Config struct {
	General  GeneralConfig  `toml:"general"`
	Database DatabaseConfig `toml:"database"`
	API      APIConfig      `toml:"api"`
	UI       UIConfig       `toml:"ui"`
}

// GeneralConfig contains general application settings
type GeneralConfig struct {
	GoalTimeHours        int      `toml:"goal_time_hours"`
	AutoLogThresholdHours float64 `toml:"auto_log_threshold_hours"`
	TrackDays            []string `toml:"track_days"`
	CheckIntervalSeconds int      `toml:"check_interval_seconds"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path string `toml:"path"`
}

// APIConfig contains API configuration
type APIConfig struct {
	PreferredProvider string           `toml:"preferred_provider"`
	RetryAttempts     int              `toml:"retry_attempts"`
	TimeoutSeconds    int              `toml:"timeout_seconds"`
	Magnetic          MagneticConfig   `toml:"magnetic"`
	Clockify          ClockifyConfig   `toml:"clockify"`
}

// MagneticConfig contains Magnetic API settings
type MagneticConfig struct {
	Enabled     bool   `toml:"enabled"`
	BaseURL     string `toml:"base_url"`
	APIKey      string `toml:"api_key"`
	WorkspaceID string `toml:"workspace_id"`
	ProjectID   string `toml:"project_id"`
}

// ClockifyConfig contains Clockify API settings
type ClockifyConfig struct {
	Enabled     bool   `toml:"enabled"`
	BaseURL     string `toml:"base_url"`
	APIKey      string `toml:"api_key"`
	WorkspaceID string `toml:"workspace_id"`
	ProjectID   string `toml:"project_id"`
}

// UIConfig contains user interface settings
type UIConfig struct {
	ShowMenuBar     bool `toml:"show_menu_bar"`
	ShowSeconds     bool `toml:"show_seconds"`
	Use12HourFormat bool `toml:"use_12_hour_format"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			GoalTimeHours:         8,
			AutoLogThresholdHours: 6.0,
			TrackDays:             []string{"monday", "tuesday", "wednesday", "thursday", "friday"},
			CheckIntervalSeconds:  60,
		},
		Database: DatabaseConfig{
			Path: "~/.timeclip/timeclip.db",
		},
		API: APIConfig{
			PreferredProvider: "magnetic",
			RetryAttempts:     3,
			TimeoutSeconds:    30,
			Magnetic: MagneticConfig{
				Enabled: true,
				BaseURL: "https://app.magnetichq.com/v2/rest/coreAPI",
			},
			Clockify: ClockifyConfig{
				Enabled: false,
				BaseURL: "https://api.clockify.me/api/v1",
			},
		},
		UI: UIConfig{
			ShowMenuBar:     true,
			ShowSeconds:     false,
			Use12HourFormat: true,
		},
	}
}