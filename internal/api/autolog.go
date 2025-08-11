package api

import (
	"fmt"
	"log"
	"sync"

	"timeclip/internal/database"
	"timeclip/internal/models"
)

// AutoLogger handles automatic time logging to time tracking APIs
type AutoLogger struct {
	mu            sync.RWMutex
	db            *database.DB
	factory       *Factory
	config        *models.Config
	apis          map[string]TimeTrackingAPI
	isRunning     bool
	stopChan      chan bool
	logChan       chan *LogRequest
	thresholdHours float64
}

// LogRequest represents a request to log time
type LogRequest struct {
	Entry       *models.DailyTimeEntry
	Description string
	Force       bool // Force logging even if already logged
}

// NewAutoLogger creates a new auto-logger
func NewAutoLogger(db *database.DB, config *models.Config) *AutoLogger {
	return &AutoLogger{
		db:             db,
		factory:        NewFactory(),
		config:         config,
		apis:           make(map[string]TimeTrackingAPI),
		stopChan:       make(chan bool),
		logChan:        make(chan *LogRequest, 100),
		thresholdHours: config.General.AutoLogThresholdHours,
	}
}

// Start begins the auto-logging service
func (al *AutoLogger) Start() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.isRunning {
		return fmt.Errorf("auto-logger is already running")
	}

	// Initialize API clients
	if err := al.initializeAPIs(); err != nil {
		return fmt.Errorf("failed to initialize APIs: %w", err)
	}

	al.isRunning = true
	
	// Start processing goroutine
	go al.processLoop()

	log.Printf("Auto-logger started with threshold: %.1f hours", al.thresholdHours)
	return nil
}

// Stop stops the auto-logging service
func (al *AutoLogger) Stop() {
	al.mu.Lock()
	defer al.mu.Unlock()

	if !al.isRunning {
		return
	}

	al.isRunning = false
	close(al.stopChan)
	log.Println("Auto-logger stopped")
}

// CheckAndLog checks if an entry should be auto-logged and logs it
func (al *AutoLogger) CheckAndLog(entry *models.DailyTimeEntry) {
	if !al.ShouldAutoLog(entry) {
		return
	}

	description := fmt.Sprintf("Timeclip auto-log for %s", entry.Date)
	
	select {
	case al.logChan <- &LogRequest{
		Entry:       entry,
		Description: description,
		Force:       false,
	}:
		log.Printf("Queued auto-log for %s (%.1f hours)", entry.Date, float64(entry.ActiveMinutes)/60.0)
	default:
		log.Printf("Warning: Auto-log queue is full, skipping entry for %s", entry.Date)
	}
}

// ForceLog forces logging of an entry regardless of threshold
func (al *AutoLogger) ForceLog(entry *models.DailyTimeEntry, description string) {
	if description == "" {
		description = fmt.Sprintf("Manual log for %s", entry.Date)
	}

	select {
	case al.logChan <- &LogRequest{
		Entry:       entry,
		Description: description,
		Force:       true,
	}:
		log.Printf("Queued manual log for %s", entry.Date)
	default:
		log.Printf("Warning: Auto-log queue is full, unable to queue manual log for %s", entry.Date)
	}
}

// ShouldAutoLog returns true if an entry should be auto-logged
func (al *AutoLogger) ShouldAutoLog(entry *models.DailyTimeEntry) bool {
	if entry == nil {
		return false
	}

	// Already auto-logged
	if entry.AutoLogged {
		return false
	}

	// Check threshold
	actualHours := float64(entry.ActiveMinutes) / 60.0
	return actualHours >= al.thresholdHours
}

// GetEnabledAPIs returns the currently enabled API clients
func (al *AutoLogger) GetEnabledAPIs() map[string]TimeTrackingAPI {
	al.mu.RLock()
	defer al.mu.RUnlock()

	result := make(map[string]TimeTrackingAPI)
	for name, api := range al.apis {
		result[name] = api
	}
	return result
}

// IsRunning returns true if the auto-logger is currently running
func (al *AutoLogger) IsRunning() bool {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return al.isRunning
}

// UpdateConfig updates the configuration and reinitializes APIs if needed
func (al *AutoLogger) UpdateConfig(newConfig *models.Config) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.config = newConfig
	al.thresholdHours = newConfig.General.AutoLogThresholdHours

	// Reinitialize APIs with new config
	return al.initializeAPIs()
}

// initializeAPIs initializes all enabled API clients
func (al *AutoLogger) initializeAPIs() error {
	// Clear existing APIs
	al.apis = make(map[string]TimeTrackingAPI)

	// Create all enabled APIs
	enabledAPIs, err := al.factory.CreateAllEnabledAPIs(al.config)
	if err != nil {
		return fmt.Errorf("failed to create API clients: %w", err)
	}

	// Validate APIs
	validationResults := al.factory.ValidateAllAPIs(enabledAPIs)
	
	for name, api := range enabledAPIs {
		if validationErr := validationResults[name]; validationErr != nil {
			log.Printf("Warning: %s API validation failed: %v", name, validationErr)
			// Still add the API in case validation is too strict
		} else {
			log.Printf("%s API validated successfully", name)
		}
		al.apis[name] = api
	}

	if len(al.apis) == 0 {
		return fmt.Errorf("no API clients are available")
	}

	log.Printf("Initialized %d API client(s): %v", len(al.apis), al.getAPINames())
	return nil
}

// processLoop processes auto-logging requests
func (al *AutoLogger) processLoop() {
	for {
		select {
		case <-al.stopChan:
			return
		case request := <-al.logChan:
			al.processLogRequest(request)
		}
	}
}

// processLogRequest processes a single log request
func (al *AutoLogger) processLogRequest(request *LogRequest) {
	if request == nil || request.Entry == nil {
		return
	}

	entry := request.Entry

	// Check if we should skip this entry
	if !request.Force && (entry.AutoLogged || !al.ShouldAutoLog(entry)) {
		return
	}

	log.Printf("Processing auto-log for %s (%.1f hours)", entry.Date, float64(entry.ActiveMinutes)/60.0)

	// Create time entry
	timeEntry := NewTimeEntry(entry, request.Description)

	// Try to log to preferred API first
	preferredAPI := al.config.API.PreferredProvider
	if api, exists := al.apis[preferredAPI]; exists {
		if response, err := al.logToAPI(api, timeEntry); err == nil {
			// Success - mark as logged
			if err := al.markAsLogged(entry, response); err != nil {
				log.Printf("Error marking entry as logged: %v", err)
			} else {
				log.Printf("Successfully logged %s to %s", entry.Date, preferredAPI)
			}
			return
		} else {
			log.Printf("Failed to log to preferred API (%s): %v", preferredAPI, err)
		}
	}

	// If preferred API failed, try other APIs
	for name, api := range al.apis {
		if name == preferredAPI {
			continue // Already tried
		}

		if response, err := al.logToAPI(api, timeEntry); err == nil {
			// Success - mark as logged
			if err := al.markAsLogged(entry, response); err != nil {
				log.Printf("Error marking entry as logged: %v", err)
			} else {
				log.Printf("Successfully logged %s to %s (fallback)", entry.Date, name)
			}
			return
		} else {
			log.Printf("Failed to log to %s: %v", name, err)
		}
	}

	// All APIs failed
	log.Printf("Error: Failed to log %s to any API", entry.Date)
}

// logToAPI attempts to log a time entry to a specific API
func (al *AutoLogger) logToAPI(api TimeTrackingAPI, timeEntry *TimeEntry) (*models.APIResponse, error) {
	// Add workspace and project IDs if not already set
	if timeEntry.WorkspaceID == "" || timeEntry.ProjectID == "" {
		al.addAPISpecificIDs(api, timeEntry)
	}

	// Create the time entry
	response, err := api.CreateTimeEntry(timeEntry)
	if err != nil {
		return nil, fmt.Errorf("%s API error: %w", api.Name(), err)
	}

	if !response.Success {
		return nil, fmt.Errorf("%s API returned error: %s", api.Name(), response.Message)
	}

	return response, nil
}

// addAPISpecificIDs adds workspace and project IDs based on the API type
func (al *AutoLogger) addAPISpecificIDs(api TimeTrackingAPI, timeEntry *TimeEntry) {
	switch api.Name() {
	case "Magnetic":
		if timeEntry.WorkspaceID == "" {
			timeEntry.WorkspaceID = al.config.API.Magnetic.WorkspaceID
		}
		if timeEntry.ProjectID == "" {
			timeEntry.ProjectID = al.config.API.Magnetic.ProjectID
		}
	case "Clockify":
		if timeEntry.WorkspaceID == "" {
			timeEntry.WorkspaceID = al.config.API.Clockify.WorkspaceID
		}
		if timeEntry.ProjectID == "" {
			timeEntry.ProjectID = al.config.API.Clockify.ProjectID
		}
	}
}

// markAsLogged marks an entry as auto-logged in the database
func (al *AutoLogger) markAsLogged(entry *models.DailyTimeEntry, response *models.APIResponse) error {
	responseData := ""
	if response != nil {
		if data, err := response.Data.(string); err {
			responseData = data
		} else {
			responseData = response.Message
		}
	}

	return al.db.MarkAsAutoLogged(entry.Date, responseData)
}

// getAPINames returns a slice of API names for logging
func (al *AutoLogger) getAPINames() []string {
	names := make([]string, 0, len(al.apis))
	for name := range al.apis {
		names = append(names, name)
	}
	return names
}

// GetStats returns auto-logging statistics
func (al *AutoLogger) GetStats() (*AutoLogStats, error) {
	// Get entries that need auto-logging
	thresholdMinutes := int(al.thresholdHours * 60)
	needingLog, err := al.db.GetEntriesNeedingAutoLog(thresholdMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries needing auto-log: %w", err)
	}

	return &AutoLogStats{
		ThresholdHours:      al.thresholdHours,
		EnabledAPIs:         al.getAPINames(),
		EntriesNeedingLog:   len(needingLog),
		QueueLength:         len(al.logChan),
		IsRunning:           al.isRunning,
	}, nil
}

// AutoLogStats represents auto-logging statistics
type AutoLogStats struct {
	ThresholdHours    float64  `json:"threshold_hours"`
	EnabledAPIs       []string `json:"enabled_apis"`
	EntriesNeedingLog int      `json:"entries_needing_log"`
	QueueLength       int      `json:"queue_length"`
	IsRunning         bool     `json:"is_running"`
}