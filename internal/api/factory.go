package api

import (
	"fmt"

	"timeclip/internal/api/clockify"
	"timeclip/internal/api/magnetic"
	"timeclip/internal/models"
)

// Factory creates time tracking API clients
type Factory struct{}

// NewFactory creates a new API factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateAPI creates a time tracking API client based on configuration
func (f *Factory) CreateAPI(provider string, config *models.Config) (TimeTrackingAPI, error) {
	switch provider {
	case "magnetic":
		if !config.API.Magnetic.Enabled {
			return nil, fmt.Errorf("magnetic API is disabled in configuration")
		}
		return magnetic.NewClient(&magnetic.Config{
			BaseURL:     config.API.Magnetic.BaseURL,
			APIKey:      config.API.Magnetic.APIKey,
			WorkspaceID: config.API.Magnetic.WorkspaceID,
			ProjectID:   config.API.Magnetic.ProjectID,
			Timeout:     config.API.TimeoutSeconds,
			Retries:     config.API.RetryAttempts,
		})

	case "clockify":
		if !config.API.Clockify.Enabled {
			return nil, fmt.Errorf("clockify API is disabled in configuration")
		}
		return clockify.NewClient(&clockify.Config{
			BaseURL:     config.API.Clockify.BaseURL,
			APIKey:      config.API.Clockify.APIKey,
			WorkspaceID: config.API.Clockify.WorkspaceID,
			ProjectID:   config.API.Clockify.ProjectID,
			Timeout:     config.API.TimeoutSeconds,
			Retries:     config.API.RetryAttempts,
		})

	default:
		return nil, fmt.Errorf("unknown time tracking provider: %s", provider)
	}
}

// CreatePreferredAPI creates the preferred API client from configuration
func (f *Factory) CreatePreferredAPI(config *models.Config) (TimeTrackingAPI, error) {
	return f.CreateAPI(config.API.PreferredProvider, config)
}

// CreateAllEnabledAPIs creates all enabled API clients
func (f *Factory) CreateAllEnabledAPIs(config *models.Config) (map[string]TimeTrackingAPI, error) {
	clients := make(map[string]TimeTrackingAPI)
	var errors []string

	// Try to create Magnetic client if enabled
	if config.API.Magnetic.Enabled && config.API.Magnetic.APIKey != "" {
		if client, err := f.CreateAPI("magnetic", config); err == nil {
			clients["magnetic"] = client
		} else {
			errors = append(errors, fmt.Sprintf("magnetic: %v", err))
		}
	}

	// Try to create Clockify client if enabled
	if config.API.Clockify.Enabled && config.API.Clockify.APIKey != "" {
		if client, err := f.CreateAPI("clockify", config); err == nil {
			clients["clockify"] = client
		} else {
			errors = append(errors, fmt.Sprintf("clockify: %v", err))
		}
	}

	if len(clients) == 0 {
		if len(errors) > 0 {
			return nil, fmt.Errorf("failed to create any API clients: %v", errors)
		}
		return nil, fmt.Errorf("no API providers are enabled and configured")
	}

	return clients, nil
}

// ValidateAPI validates an API client's configuration and connectivity
func (f *Factory) ValidateAPI(api TimeTrackingAPI) error {
	// Check if configured
	if !api.IsConfigured() {
		return fmt.Errorf("%s API is not properly configured", api.Name())
	}

	// Validate configuration
	if err := api.ValidateConfig(); err != nil {
		return fmt.Errorf("%s API configuration error: %w", api.Name(), err)
	}

	// Test authentication
	if err := api.Authenticate(); err != nil {
		return fmt.Errorf("%s API authentication failed: %w", api.Name(), err)
	}

	return nil
}

// ValidateAllAPIs validates all provided API clients
func (f *Factory) ValidateAllAPIs(apis map[string]TimeTrackingAPI) map[string]error {
	results := make(map[string]error)

	for name, api := range apis {
		results[name] = f.ValidateAPI(api)
	}

	return results
}

// GetAvailableProviders returns a list of all available time tracking providers
func (f *Factory) GetAvailableProviders() []string {
	return []string{"magnetic", "clockify"}
}

// IsProviderSupported checks if a provider is supported
func (f *Factory) IsProviderSupported(provider string) bool {
	for _, p := range f.GetAvailableProviders() {
		if p == provider {
			return true
		}
	}
	return false
}