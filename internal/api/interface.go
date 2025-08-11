package api

import (
	"fmt"
	"time"

	"timeclip/internal/models"
)

// TimeTrackingAPI defines the interface that all time tracking service clients must implement
type TimeTrackingAPI interface {
	// Name returns the name of the time tracking service
	Name() string

	// Authenticate validates the API credentials
	Authenticate() error

	// CreateTimeEntry creates a new time entry for the specified date
	CreateTimeEntry(entry interface{}) (*models.APIResponse, error)

	// GetWorkspaces retrieves available workspaces for the authenticated user
	GetWorkspaces() ([]*models.Workspace, error)

	// GetProjects retrieves available projects for a workspace
	GetProjects(workspaceID string) ([]*models.Project, error)

	// IsConfigured returns true if the API client is properly configured
	IsConfigured() bool

	// ValidateConfig validates the current configuration
	ValidateConfig() error
}

// TimeEntry represents a time entry to be submitted to a time tracking API
type TimeEntry struct {
	Date        time.Time `json:"date"`
	Hours       float64   `json:"hours"`
	Minutes     int       `json:"minutes"`
	Description string    `json:"description"`
	ProjectID   string    `json:"project_id,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

// NewTimeEntry creates a new time entry from a daily time entry
func NewTimeEntry(dailyEntry *models.DailyTimeEntry, description string) *TimeEntry {
	date, _ := time.Parse("2006-01-02", dailyEntry.Date)
	
	return &TimeEntry{
		Date:        date,
		Hours:       float64(dailyEntry.ActiveMinutes) / 60.0,
		Minutes:     dailyEntry.ActiveMinutes,
		Description: description,
	}
}

// WithProject adds project information to the time entry
func (te *TimeEntry) WithProject(workspaceID, projectID string) *TimeEntry {
	te.WorkspaceID = workspaceID
	te.ProjectID = projectID
	return te
}

// WithTags adds tags to the time entry
func (te *TimeEntry) WithTags(tags ...string) *TimeEntry {
	te.Tags = append(te.Tags, tags...)
	return te
}

// APIError represents an error from a time tracking API
type APIError struct {
	Service    string `json:"service"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

// Error implements the error interface
func (ae *APIError) Error() string {
	if ae.Details != "" {
		return fmt.Sprintf("%s API error (%d): %s - %s", ae.Service, ae.StatusCode, ae.Message, ae.Details)
	}
	return fmt.Sprintf("%s API error (%d): %s", ae.Service, ae.StatusCode, ae.Message)
}

// IsAuthenticationError returns true if the error is related to authentication
func (ae *APIError) IsAuthenticationError() bool {
	return ae.StatusCode == 401 || ae.StatusCode == 403
}

// IsRateLimitError returns true if the error is due to rate limiting
func (ae *APIError) IsRateLimitError() bool {
	return ae.StatusCode == 429
}

// APIConfig contains common API configuration
type APIConfig struct {
	BaseURL         string
	APIKey          string
	WorkspaceID     string
	ProjectID       string
	TimeoutSeconds  int
	RetryAttempts   int
	UserAgent       string
}

// NewAPIConfig creates a new API configuration
func NewAPIConfig(baseURL, apiKey string) *APIConfig {
	return &APIConfig{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		TimeoutSeconds: 30,
		RetryAttempts:  3,
		UserAgent:      "Timeclip/1.0",
	}
}