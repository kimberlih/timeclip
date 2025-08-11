package tracker

import (
	"fmt"
	"log"
	"sync"
	"time"

	"timeclip/internal/database"
	"timeclip/internal/models"
)

// ActivityDetector manages time tracking based on system activity
type ActivityDetector struct {
	mu                 sync.RWMutex
	db                 *database.DB
	monitor            *Monitor
	config             *ActivityConfig
	isTracking         bool
	stopChan           chan bool
	lastActiveTime     time.Time
	currentEntry       *models.DailyTimeEntry
	stateChangeCallbacks []ActivityStateChangeCallback
}

// ActivityConfig contains configuration for activity detection
type ActivityConfig struct {
	CheckInterval         time.Duration `json:"check_interval"`
	GoalMinutes          int           `json:"goal_minutes"`
	AutoLogThresholdMinutes int        `json:"auto_log_threshold_minutes"`
}

// ActivityStateChangeCallback is called when tracking state changes
type ActivityStateChangeCallback func(isActive bool, entry *models.DailyTimeEntry)

// NewActivityDetector creates a new activity detector
func NewActivityDetector(db *database.DB, config *ActivityConfig) *ActivityDetector {
	return &ActivityDetector{
		db:         db,
		monitor:    NewMonitor(),
		config:     config,
		stopChan:   make(chan bool),
	}
}

// Start begins activity detection and time tracking
func (ad *ActivityDetector) Start() error {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if ad.isTracking {
		return fmt.Errorf("activity detector is already running")
	}

	// Start system monitor
	if err := ad.monitor.Start(ad.config.CheckInterval); err != nil {
		return fmt.Errorf("failed to start system monitor: %w", err)
	}

	// Register state change callback
	ad.monitor.AddStateChangeCallback(ad.onSystemStateChange)

	// Get or create today's entry
	entry, err := ad.db.GetTodayEntry()
	if err != nil {
		return fmt.Errorf("failed to get today's entry: %w", err)
	}
	ad.currentEntry = entry

	ad.isTracking = true
	ad.lastActiveTime = time.Now()

	log.Printf("Activity detector started - Today: %d minutes (%.1f hours)", 
		entry.ActiveMinutes, float64(entry.ActiveMinutes)/60.0)

	// Start tracking loop
	go ad.trackingLoop()

	return nil
}

// Stop stops activity detection
func (ad *ActivityDetector) Stop() {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if !ad.isTracking {
		return
	}

	ad.isTracking = false
	ad.monitor.Stop()
	close(ad.stopChan)

	log.Println("Activity detector stopped")
}

// IsTracking returns true if currently tracking time
func (ad *ActivityDetector) IsTracking() bool {
	ad.mu.RLock()
	defer ad.mu.RUnlock()
	return ad.isTracking
}

// GetCurrentEntry returns the current day's time entry
func (ad *ActivityDetector) GetCurrentEntry() *models.DailyTimeEntry {
	ad.mu.RLock()
	defer ad.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	if ad.currentEntry == nil {
		return nil
	}
	
	entryCopy := *ad.currentEntry
	return &entryCopy
}

// TogglePause toggles the pause state
func (ad *ActivityDetector) TogglePause() error {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if ad.currentEntry == nil {
		return fmt.Errorf("no current entry to pause/resume")
	}

	newPauseState := !ad.currentEntry.IsPaused
	
	if err := ad.db.SetPauseState(newPauseState); err != nil {
		return fmt.Errorf("failed to set pause state: %w", err)
	}

	// Update local entry
	ad.currentEntry.IsPaused = newPauseState

	log.Printf("Time tracking %s", map[bool]string{true: "paused", false: "resumed"}[newPauseState])

	// Notify callbacks
	ad.notifyStateChange(ad.monitor.IsSystemActive(), ad.currentEntry)

	return nil
}

// SetPause sets the pause state explicitly
func (ad *ActivityDetector) SetPause(paused bool) error {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if ad.currentEntry == nil {
		return fmt.Errorf("no current entry to pause/resume")
	}

	if ad.currentEntry.IsPaused == paused {
		return nil // No change needed
	}

	if err := ad.db.SetPauseState(paused); err != nil {
		return fmt.Errorf("failed to set pause state: %w", err)
	}

	// Update local entry
	ad.currentEntry.IsPaused = paused

	log.Printf("Time tracking %s", map[bool]string{true: "paused", false: "resumed"}[paused])

	// Notify callbacks
	ad.notifyStateChange(ad.monitor.IsSystemActive(), ad.currentEntry)

	return nil
}

// AddStateChangeCallback adds a callback for activity state changes
func (ad *ActivityDetector) AddStateChangeCallback(callback ActivityStateChangeCallback) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	ad.stateChangeCallbacks = append(ad.stateChangeCallbacks, callback)
}

// GetSystemState returns the current system monitoring state
func (ad *ActivityDetector) GetSystemState() *SystemState {
	return ad.monitor.GetCurrentState()
}

// GetStateDescription returns a description of the current state
func (ad *ActivityDetector) GetStateDescription() string {
	entry := ad.GetCurrentEntry()

	if entry != nil && entry.IsPaused {
		return "Paused"
	}

	return ad.monitor.GetStateDescription()
}

// trackingLoop runs the main time tracking loop
func (ad *ActivityDetector) trackingLoop() {
	ticker := time.NewTicker(time.Minute) // Check every minute for time increments
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ad.processMinuteIncrement()
		case <-ad.stopChan:
			return
		}
	}
}

// processMinuteIncrement handles the minutely time increment logic
func (ad *ActivityDetector) processMinuteIncrement() {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	// Check if we should increment time
	systemState := ad.monitor.GetCurrentState()
	shouldIncrement := systemState.IsActive && !ad.currentEntry.IsPaused

	if shouldIncrement {
		// Increment time in database
		if err := ad.db.IncrementActiveTime(); err != nil {
			log.Printf("Error incrementing active time: %v", err)
			return
		}

		// Refresh current entry from database
		entry, err := ad.db.GetTodayEntry()
		if err != nil {
			log.Printf("Error refreshing current entry: %v", err)
			return
		}
		ad.currentEntry = entry

		log.Printf("Time incremented - Total: %d minutes (%.1f hours)", 
			entry.ActiveMinutes, float64(entry.ActiveMinutes)/60.0)

		// Check for auto-log threshold
		if entry.ShouldAutoLog(float64(ad.config.AutoLogThresholdMinutes)/60.0) {
			log.Printf("Auto-log threshold reached: %d minutes", entry.ActiveMinutes)
			// TODO: Trigger auto-logging (will be implemented with API clients)
		}

		// Notify callbacks about time update
		ad.notifyStateChange(systemState.IsActive, entry)
	}

	// Handle day rollover
	now := time.Now()
	todayStr := now.Format("2006-01-02")
	if ad.currentEntry.Date != todayStr {
		log.Println("Day rollover detected, creating new entry")
		entry, err := ad.db.GetTodayEntry()
		if err != nil {
			log.Printf("Error creating new day entry: %v", err)
			return
		}
		ad.currentEntry = entry
		ad.notifyStateChange(systemState.IsActive, entry)
	}
}

// onSystemStateChange is called when the system monitor detects state changes
func (ad *ActivityDetector) onSystemStateChange(oldState, newState *SystemState) {
	ad.mu.RLock()
	currentEntry := ad.currentEntry
	ad.mu.RUnlock()

	// Log state change to database for debugging
	eventType := "inactive"
	if newState.IsActive {
		eventType = "active"
	}

	details := fmt.Sprintf("Session:%v, Lid:%v, Screensaver:%v", 
		newState.IsUserSessionActive, newState.IsLidOpen, !newState.IsScreenSaverRunning)
	
	if err := ad.db.LogSystemEvent(eventType, details); err != nil {
		log.Printf("Error logging system event: %v", err)
	}

	// Notify callbacks
	if currentEntry != nil {
		ad.notifyStateChange(newState.IsActive, currentEntry)
	}
}

// notifyStateChange calls all registered state change callbacks
func (ad *ActivityDetector) notifyStateChange(isActive bool, entry *models.DailyTimeEntry) {
	for _, callback := range ad.stateChangeCallbacks {
		go callback(isActive, entry)
	}
}

// GetTodayStats returns statistics for today
func (ad *ActivityDetector) GetTodayStats() (*TodayStats, error) {
	entry := ad.GetCurrentEntry()
	if entry == nil {
		return nil, fmt.Errorf("no current entry available")
	}

	systemState := ad.GetSystemState()

	return &TodayStats{
		Date:            entry.Date,
		ActiveMinutes:   entry.ActiveMinutes,
		GoalMinutes:     entry.GoalMinutes,
		Progress:        entry.Progress(),
		IsGoalReached:   entry.IsGoalReached(),
		IsPaused:        entry.IsPaused,
		IsSystemActive:  systemState.IsActive,
		AutoLogged:      entry.AutoLogged,
		LastUpdated:     entry.UpdatedAt,
	}, nil
}

// TodayStats represents today's tracking statistics
type TodayStats struct {
	Date           string    `json:"date"`
	ActiveMinutes  int       `json:"active_minutes"`
	GoalMinutes    int       `json:"goal_minutes"`
	Progress       float64   `json:"progress"`
	IsGoalReached  bool      `json:"is_goal_reached"`
	IsPaused       bool      `json:"is_paused"`
	IsSystemActive bool      `json:"is_system_active"`
	AutoLogged     bool      `json:"auto_logged"`
	LastUpdated    time.Time `json:"last_updated"`
}

// ActiveHours returns active time in hours
func (ts *TodayStats) ActiveHours() float64 {
	return float64(ts.ActiveMinutes) / 60.0
}

// GoalHours returns goal time in hours
func (ts *TodayStats) GoalHours() float64 {
	return float64(ts.GoalMinutes) / 60.0
}

// RemainingMinutes returns minutes remaining to reach goal
func (ts *TodayStats) RemainingMinutes() int {
	remaining := ts.GoalMinutes - ts.ActiveMinutes
	if remaining < 0 {
		return 0
	}
	return remaining
}