package config

import "timeclip/internal/models"

// GetDefaultConfig returns the default configuration with all standard values
func GetDefaultConfig() *models.Config {
	return &models.Config{
		General: models.GeneralConfig{
			GoalTimeHours:         8,
			AutoLogThresholdHours: 6.0,
			TrackDays:             []string{"monday", "tuesday", "wednesday", "thursday", "friday"},
			CheckIntervalSeconds:  60,
		},
		Database: models.DatabaseConfig{
			Path: "~/.timeclip/timeclip.db",
		},
		API: models.APIConfig{
			PreferredProvider: "magnetic",
			RetryAttempts:     3,
			TimeoutSeconds:    30,
			Magnetic: models.MagneticConfig{
				Enabled: true,
				BaseURL: "https://app.magnetichq.com/v2/rest/coreAPI",
				APIKey:  "", // User must fill this in
			},
			Clockify: models.ClockifyConfig{
				Enabled: false,
				BaseURL: "https://api.clockify.me/api/v1",
				APIKey:  "", // User must fill this in
			},
		},
		UI: models.UIConfig{
			ShowSeconds:     false,
			Use12HourFormat: true,
		},
	}
}

// ValidTrackDays returns the list of valid day names
func ValidTrackDays() []string {
	return []string{
		"monday", "tuesday", "wednesday", "thursday",
		"friday", "saturday", "sunday",
	}
}

// DefaultTrackDays returns the default tracking days (weekdays)
func DefaultTrackDays() []string {
	return []string{"monday", "tuesday", "wednesday", "thursday", "friday"}
}