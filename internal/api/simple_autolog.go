package api

import (
	"fmt"
	"log"
	"sync"
	"time"

	"timeclip/internal/api/clockify"
	"timeclip/internal/api/magnetic"
	"timeclip/internal/database"
	"timeclip/internal/models"
)

// SimpleAutoLogger handles automatic time logging with a concrete implementation
type SimpleAutoLogger struct {
	mu             sync.RWMutex
	db             *database.DB
	config         *models.Config
	factory        *Factory
	isRunning      bool
	thresholdHours float64
}

// NewSimpleAutoLogger creates a new simple auto-logger
func NewSimpleAutoLogger(db *database.DB, config *models.Config) *SimpleAutoLogger {
	return &SimpleAutoLogger{
		db:             db,
		config:         config,
		factory:        NewFactory(),
		thresholdHours: config.General.AutoLogThresholdHours,
	}
}

// Start begins the auto-logging service
func (sal *SimpleAutoLogger) Start() error {
	sal.mu.Lock()
	defer sal.mu.Unlock()

	if sal.isRunning {
		return fmt.Errorf("auto-logger is already running")
	}

	// Test API connections
	if err := sal.testAPIConnections(); err != nil {
		return fmt.Errorf("failed to initialize APIs: %w", err)
	}

	sal.isRunning = true
	log.Printf("Simple auto-logger started with threshold: %.1f hours", sal.thresholdHours)
	return nil
}

// Stop stops the auto-logging service
func (sal *SimpleAutoLogger) Stop() {
	sal.mu.Lock()
	defer sal.mu.Unlock()

	if !sal.isRunning {
		return
	}

	sal.isRunning = false
	log.Println("Simple auto-logger stopped")
}

// CheckAndLog checks if an entry should be auto-logged and logs it
func (sal *SimpleAutoLogger) CheckAndLog(entry *models.DailyTimeEntry) {
	if !sal.ShouldAutoLog(entry) {
		return
	}

	// Log in background to avoid blocking
	go sal.logEntry(entry)
}

// ShouldAutoLog returns true if an entry should be auto-logged
func (sal *SimpleAutoLogger) ShouldAutoLog(entry *models.DailyTimeEntry) bool {
	if entry == nil || entry.AutoLogged {
		return false
	}

	actualHours := float64(entry.ActiveMinutes) / 60.0
	return actualHours >= sal.thresholdHours
}

// IsRunning returns true if the auto-logger is currently running
func (sal *SimpleAutoLogger) IsRunning() bool {
	sal.mu.RLock()
	defer sal.mu.RUnlock()
	return sal.isRunning
}

// ForceLog forces logging of an entry regardless of threshold
func (sal *SimpleAutoLogger) ForceLog(entry *models.DailyTimeEntry) error {
	if entry == nil {
		return fmt.Errorf("entry cannot be nil")
	}

	return sal.logEntry(entry)
}

// logEntry handles the actual logging process
func (sal *SimpleAutoLogger) logEntry(entry *models.DailyTimeEntry) error {
	sal.mu.RLock()
	config := sal.config
	sal.mu.RUnlock()

	log.Printf("Auto-logging entry for %s (%.1f hours)", entry.Date, float64(entry.ActiveMinutes)/60.0)

	description := fmt.Sprintf("Timeclip auto-log for %s", entry.Date)

	// Try preferred API first
	preferredProvider := config.API.PreferredProvider
	
	var err error
	switch preferredProvider {
	case "magnetic":
		if config.API.Magnetic.Enabled && config.API.Magnetic.APIKey != "" {
			err = sal.logToMagnetic(entry, description)
			if err == nil {
				sal.markAsLogged(entry, fmt.Sprintf("Successfully logged to Magnetic: %s", description))
				log.Printf("✅ Successfully logged %s to Magnetic", entry.Date)
				return nil
			}
			log.Printf("❌ Failed to log to Magnetic: %v", err)
		}
	case "clockify":
		if config.API.Clockify.Enabled && config.API.Clockify.APIKey != "" {
			err = sal.logToClockify(entry, description)
			if err == nil {
				sal.markAsLogged(entry, fmt.Sprintf("Successfully logged to Clockify: %s", description))
				log.Printf("✅ Successfully logged %s to Clockify", entry.Date)
				return nil
			}
			log.Printf("❌ Failed to log to Clockify: %v", err)
		}
	}

	// Try fallback APIs if preferred failed
	if preferredProvider != "magnetic" && config.API.Magnetic.Enabled && config.API.Magnetic.APIKey != "" {
		if err := sal.logToMagnetic(entry, description); err == nil {
			sal.markAsLogged(entry, fmt.Sprintf("Successfully logged to Magnetic (fallback): %s", description))
			log.Printf("✅ Successfully logged %s to Magnetic (fallback)", entry.Date)
			return nil
		}
	}

	if preferredProvider != "clockify" && config.API.Clockify.Enabled && config.API.Clockify.APIKey != "" {
		if err := sal.logToClockify(entry, description); err == nil {
			sal.markAsLogged(entry, fmt.Sprintf("Successfully logged to Clockify (fallback): %s", description))
			log.Printf("✅ Successfully logged %s to Clockify (fallback)", entry.Date)
			return nil
		}
	}

	log.Printf("❌ Failed to log %s to any API", entry.Date)
	return fmt.Errorf("failed to log to any available API")
}

// logToMagnetic logs an entry to Magnetic API
func (sal *SimpleAutoLogger) logToMagnetic(entry *models.DailyTimeEntry, description string) error {
	config := sal.config.API.Magnetic

	client, err := magnetic.NewClient(&magnetic.Config{
		BaseURL:     config.BaseURL,
		APIKey:      config.APIKey,
		WorkspaceID: config.WorkspaceID,
		ProjectID:   config.ProjectID,
		Timeout:     sal.config.API.TimeoutSeconds,
		Retries:     sal.config.API.RetryAttempts,
	})
	if err != nil {
		return fmt.Errorf("failed to create Magnetic client: %w", err)
	}

	// Create time entry
	date, _ := time.Parse("2006-01-02", entry.Date)
	timeEntry := &magnetic.TimeEntry{
		Date:        date,
		Hours:       float64(entry.ActiveMinutes) / 60.0,
		Minutes:     entry.ActiveMinutes,
		Description: description,
		ProjectID:   config.ProjectID,
		WorkspaceID: config.WorkspaceID,
	}

	response, err := client.CreateTimeEntry(timeEntry)
	if err != nil {
		return fmt.Errorf("failed to create time entry: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("API returned error: %s", response.Message)
	}

	return nil
}

// logToClockify logs an entry to Clockify API
func (sal *SimpleAutoLogger) logToClockify(entry *models.DailyTimeEntry, description string) error {
	config := sal.config.API.Clockify

	client, err := clockify.NewClient(&clockify.Config{
		BaseURL:     config.BaseURL,
		APIKey:      config.APIKey,
		WorkspaceID: config.WorkspaceID,
		ProjectID:   config.ProjectID,
		Timeout:     sal.config.API.TimeoutSeconds,
		Retries:     sal.config.API.RetryAttempts,
	})
	if err != nil {
		return fmt.Errorf("failed to create Clockify client: %w", err)
	}

	// Create time entry
	date, _ := time.Parse("2006-01-02", entry.Date)
	timeEntry := &clockify.TimeEntry{
		Date:        date,
		Hours:       float64(entry.ActiveMinutes) / 60.0,
		Minutes:     entry.ActiveMinutes,
		Description: description,
		ProjectID:   config.ProjectID,
		WorkspaceID: config.WorkspaceID,
	}

	response, err := client.CreateTimeEntry(timeEntry)
	if err != nil {
		return fmt.Errorf("failed to create time entry: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("API returned error: %s", response.Message)
	}

	return nil
}

// markAsLogged marks an entry as auto-logged in the database
func (sal *SimpleAutoLogger) markAsLogged(entry *models.DailyTimeEntry, response string) {
	if err := sal.db.MarkAsAutoLogged(entry.Date, response); err != nil {
		log.Printf("Error marking entry as logged: %v", err)
	}
}

// testAPIConnections tests the configured API connections
func (sal *SimpleAutoLogger) testAPIConnections() error {
	var errors []string

	// Test Magnetic if enabled
	if sal.config.API.Magnetic.Enabled && sal.config.API.Magnetic.APIKey != "" {
		client, err := magnetic.NewClient(&magnetic.Config{
			BaseURL:     sal.config.API.Magnetic.BaseURL,
			APIKey:      sal.config.API.Magnetic.APIKey,
			WorkspaceID: sal.config.API.Magnetic.WorkspaceID,
			ProjectID:   sal.config.API.Magnetic.ProjectID,
			Timeout:     sal.config.API.TimeoutSeconds,
			Retries:     sal.config.API.RetryAttempts,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("Magnetic client creation failed: %v", err))
		} else {
			log.Printf("✅ Magnetic API client created successfully")
			// Test basic authentication
			if authErr := client.Authenticate(); authErr != nil {
				log.Printf("⚠️  Magnetic API authentication warning: %v", authErr)
			} else {
				log.Printf("✅ Magnetic API authentication successful")
			}
		}
	}

	// Test Clockify if enabled
	if sal.config.API.Clockify.Enabled && sal.config.API.Clockify.APIKey != "" {
		client, err := clockify.NewClient(&clockify.Config{
			BaseURL:     sal.config.API.Clockify.BaseURL,
			APIKey:      sal.config.API.Clockify.APIKey,
			WorkspaceID: sal.config.API.Clockify.WorkspaceID,
			ProjectID:   sal.config.API.Clockify.ProjectID,
			Timeout:     sal.config.API.TimeoutSeconds,
			Retries:     sal.config.API.RetryAttempts,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("Clockify client creation failed: %v", err))
		} else {
			log.Printf("✅ Clockify API client created successfully")
			// Test basic authentication
			if authErr := client.Authenticate(); authErr != nil {
				log.Printf("⚠️  Clockify API authentication warning: %v", authErr)
			} else {
				log.Printf("✅ Clockify API authentication successful")
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("API connection errors: %v", errors)
	}

	return nil
}