package clockify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"timeclip/internal/models"
)

// Client represents a Clockify time tracking API client
type Client struct {
	config     *Config
	httpClient *http.Client
}

// Config contains Clockify-specific configuration
type Config struct {
	BaseURL     string
	APIKey      string
	WorkspaceID string
	ProjectID   string
	Timeout     int
	Retries     int
}

// ClockifyTimeEntry represents a time entry in Clockify's format
type ClockifyTimeEntry struct {
	Start       string `json:"start"`
	End         string `json:"end"`
	Description string `json:"description"`
	ProjectID   string `json:"projectId,omitempty"`
	TaskID      string `json:"taskId,omitempty"`
	TagIDs      []string `json:"tagIds,omitempty"`
}

// ClockifyProject represents a project in Clockify
type ClockifyProject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WorkspaceID string `json:"workspaceId"`
	Color       string `json:"color,omitempty"`
	Archived    bool   `json:"archived"`
}

// ClockifyWorkspace represents a workspace in Clockify
type ClockifyWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ClockifyUser represents the current user information
type ClockifyUser struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	ActiveWorkspace string `json:"activeWorkspace"`
}

// NewClient creates a new Clockify API client
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.clockify.me/api/v1"
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30
	}
	if config.Retries == 0 {
		config.Retries = 3
	}

	httpClient := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
	}, nil
}

// Name returns the name of the time tracking service
func (c *Client) Name() string {
	return "Clockify"
}

// IsConfigured returns true if the client is properly configured
func (c *Client) IsConfigured() bool {
	return c.config != nil &&
		c.config.BaseURL != "" &&
		c.config.APIKey != ""
}

// ValidateConfig validates the client configuration
func (c *Client) ValidateConfig() error {
	if c.config == nil {
		return fmt.Errorf("configuration is nil")
	}

	if c.config.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	if c.config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	return nil
}

// Authenticate validates the API credentials by fetching user info
func (c *Client) Authenticate() error {
	req, err := c.createRequest("GET", "/user", nil)
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed: invalid API credentials (status %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// TimeEntry represents a time entry for the Clockify API
type TimeEntry struct {
	Date        time.Time `json:"date"`
	Hours       float64   `json:"hours"`
	Minutes     int       `json:"minutes"`
	Description string    `json:"description"`
	ProjectID   string    `json:"project_id,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
}

// CreateTimeEntry creates a new time entry in Clockify
func (c *Client) CreateTimeEntry(entry interface{}) (*models.APIResponse, error) {
	// Convert interface{} to our TimeEntry type
	timeEntry, ok := entry.(*TimeEntry)
	if !ok {
		return nil, fmt.Errorf("invalid entry type for Clockify API")
	}
	workspaceID := c.getWorkspaceID(timeEntry)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	// Calculate start and end times
	// For simplicity, we'll create an entry that spans the entire duration
	startTime := timeEntry.Date
	endTime := startTime.Add(time.Duration(timeEntry.Minutes) * time.Minute)

	clockifyEntry := &ClockifyTimeEntry{
		Start:       startTime.Format("2006-01-02T15:04:05.000Z"),
		End:         endTime.Format("2006-01-02T15:04:05.000Z"),
		Description: timeEntry.Description,
		ProjectID:   c.getProjectID(timeEntry),
	}

	jsonData, err := json.Marshal(clockifyEntry)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal time entry: %w", err)
	}

	endpoint := fmt.Sprintf("/workspaces/%s/time-entries", workspaceID)
	req, err := c.createRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return models.NewAPIResponse(false, "Failed to create time entry"), fmt.Errorf("API request failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		return models.NewAPIResponse(true, "Time entry created successfully").WithData(result), nil
	}

	return models.NewAPIResponse(true, "Time entry created successfully").WithData(string(body)), nil
}

// GetWorkspaces retrieves available workspaces
func (c *Client) GetWorkspaces() ([]*models.Workspace, error) {
	req, err := c.createRequest("GET", "/workspaces", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to retrieve workspaces (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var clockifyWorkspaces []ClockifyWorkspace
	if err := json.Unmarshal(body, &clockifyWorkspaces); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	workspaces := make([]*models.Workspace, len(clockifyWorkspaces))
	for i, cw := range clockifyWorkspaces {
		workspaces[i] = &models.Workspace{
			ID:   cw.ID,
			Name: cw.Name,
		}
	}

	return workspaces, nil
}

// GetProjects retrieves available projects for a workspace
func (c *Client) GetProjects(workspaceID string) ([]*models.Project, error) {
	endpoint := fmt.Sprintf("/workspaces/%s/projects", workspaceID)
	req, err := c.createRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to retrieve projects (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var clockifyProjects []ClockifyProject
	if err := json.Unmarshal(body, &clockifyProjects); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	projects := make([]*models.Project, 0, len(clockifyProjects))
	for _, cp := range clockifyProjects {
		// Skip archived projects
		if !cp.Archived {
			projects = append(projects, &models.Project{
				ID:          cp.ID,
				Name:        cp.Name,
				WorkspaceID: cp.WorkspaceID,
			})
		}
	}

	return projects, nil
}

// createRequest creates an HTTP request with proper authentication
func (c *Client) createRequest(method, endpoint string, body io.Reader) (*http.Request, error) {
	url := c.config.BaseURL + endpoint
	
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Add authentication header (Clockify uses X-Api-Key)
	req.Header.Set("X-Api-Key", c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Timeclip/1.0")

	return req, nil
}

// getProjectID returns the project ID to use for the time entry
func (c *Client) getProjectID(entry *TimeEntry) string {
	if entry.ProjectID != "" {
		return entry.ProjectID
	}
	return c.config.ProjectID
}

// getWorkspaceID returns the workspace ID to use for the time entry
func (c *Client) getWorkspaceID(entry *TimeEntry) string {
	if entry.WorkspaceID != "" {
		return entry.WorkspaceID
	}
	return c.config.WorkspaceID
}