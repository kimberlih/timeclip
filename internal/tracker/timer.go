package tracker

import (
	"fmt"
	"log"
	"time"

	"timeclip/internal/database"
	"timeclip/internal/models"
)

// Timer coordinates system monitoring, activity detection, and time tracking
type Timer struct {
	detector *ActivityDetector
	config   *models.Config
	db       *database.DB
}

// NewTimer creates a new time tracking timer
func NewTimer(db *database.DB, config *models.Config) *Timer {
	// Create activity configuration from main config
	activityConfig := &ActivityConfig{
		CheckInterval:           time.Duration(config.General.CheckIntervalSeconds) * time.Second,
		GoalMinutes:            config.General.GoalTimeHours * 60,
		AutoLogThresholdMinutes: int(config.General.AutoLogThresholdHours * 60),
	}

	detector := NewActivityDetector(db, activityConfig)

	return &Timer{
		detector: detector,
		config:   config,
		db:       db,
	}
}

// Start begins the time tracking process
func (t *Timer) Start() error {
	log.Println("Starting time tracking timer...")

	if err := t.detector.Start(); err != nil {
		return fmt.Errorf("failed to start activity detector: %w", err)
	}

	log.Printf("Timer started - checking every %d seconds", t.config.General.CheckIntervalSeconds)
	return nil
}

// Stop stops the time tracking process
func (t *Timer) Stop() {
	log.Println("Stopping time tracking timer...")
	t.detector.Stop()
}

// IsTracking returns true if the timer is currently tracking
func (t *Timer) IsTracking() bool {
	return t.detector.IsTracking()
}

// GetCurrentEntry returns today's time entry
func (t *Timer) GetCurrentEntry() *models.DailyTimeEntry {
	return t.detector.GetCurrentEntry()
}

// GetTodayStats returns today's tracking statistics
func (t *Timer) GetTodayStats() (*TodayStats, error) {
	return t.detector.GetTodayStats()
}

// GetSystemState returns the current system state
func (t *Timer) GetSystemState() *SystemState {
	return t.detector.GetSystemState()
}

// GetStateDescription returns a human-readable state description
func (t *Timer) GetStateDescription() string {
	return t.detector.GetStateDescription()
}

// TogglePause toggles the pause state
func (t *Timer) TogglePause() error {
	return t.detector.TogglePause()
}

// SetPause sets the pause state explicitly
func (t *Timer) SetPause(paused bool) error {
	return t.detector.SetPause(paused)
}

// IsPaused returns true if tracking is currently paused
func (t *Timer) IsPaused() bool {
	entry := t.GetCurrentEntry()
	return entry != nil && entry.IsPaused
}

// AddStateChangeCallback adds a callback for state changes
func (t *Timer) AddStateChangeCallback(callback ActivityStateChangeCallback) {
	t.detector.AddStateChangeCallback(callback)
}

// ForceIncrement manually increments today's time (for testing)
func (t *Timer) ForceIncrement() error {
	if err := t.db.IncrementActiveTime(); err != nil {
		return fmt.Errorf("failed to force increment: %w", err)
	}
	
	log.Println("Time manually incremented")
	return nil
}

// GetWeeklyStats returns this week's statistics
func (t *Timer) GetWeeklyStats() (*database.WeeklyStats, error) {
	return t.db.GetWeeklyStats()
}

// GetMonthlyStats returns this month's statistics
func (t *Timer) GetMonthlyStats() (*database.MonthlyStats, error) {
	return t.db.GetMonthlyStats()
}

// ShouldTrackToday returns true if today is a tracking day
func (t *Timer) ShouldTrackToday() bool {
	today := time.Now().Weekday().String()
	todayLower := map[string]string{
		"Monday": "monday", "Tuesday": "tuesday", "Wednesday": "wednesday",
		"Thursday": "thursday", "Friday": "friday", "Saturday": "saturday", "Sunday": "sunday",
	}[today]

	for _, day := range t.config.General.TrackDays {
		if day == todayLower {
			return true
		}
	}
	return false
}

// GetConfig returns the configuration
func (t *Timer) GetConfig() *models.Config {
	return t.config
}