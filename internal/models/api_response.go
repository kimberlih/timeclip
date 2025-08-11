package models

import "time"

// APIResponse represents a standardized response from time tracking APIs
type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// TimeEntry represents a time entry for API submission
type TimeEntry struct {
	Date        string  `json:"date"`         // YYYY-MM-DD format
	Hours       float64 `json:"hours"`        // Total hours for the day
	Description string  `json:"description"`  // Entry description
	ProjectID   string  `json:"project_id"`   // Project identifier
	WorkspaceID string  `json:"workspace_id"` // Workspace identifier
}

// Workspace represents a workspace/organization in time tracking systems
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Project represents a project within a workspace
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id"`
}

// NewAPIResponse creates a new API response with current timestamp
func NewAPIResponse(success bool, message string) *APIResponse {
	return &APIResponse{
		Success:   success,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// WithError adds error information to the API response
func (r *APIResponse) WithError(err error) *APIResponse {
	r.Success = false
	r.Error = err.Error()
	return r
}

// WithData adds data to the API response
func (r *APIResponse) WithData(data interface{}) *APIResponse {
	r.Data = data
	return r
}