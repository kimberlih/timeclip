package models

import "time"

// DailyTimeEntry represents a single day's time tracking data
type DailyTimeEntry struct {
	ID              int       `db:"id"`
	Date            string    `db:"date"`            // YYYY-MM-DD format
	ActiveMinutes   int       `db:"active_minutes"`  // Total active minutes for the day
	GoalMinutes     int       `db:"goal_minutes"`    // Daily goal (usually 480 = 8 hours)
	IsPaused        bool      `db:"is_paused"`       // Current pause state
	AutoLogged      bool      `db:"auto_logged"`     // Whether auto-log completed
	AutoLogResponse string    `db:"auto_log_response"` // API response for debugging
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// SystemEvent represents a system state change event
type SystemEvent struct {
	ID        int       `db:"id"`
	EventType string    `db:"event_type"` // 'active', 'inactive', 'pause', 'resume'
	Timestamp time.Time `db:"timestamp"`
	Details   string    `db:"details"` // JSON metadata
}

// ActiveTime returns the active time as a time.Duration
func (d *DailyTimeEntry) ActiveTime() time.Duration {
	return time.Duration(d.ActiveMinutes) * time.Minute
}

// GoalTime returns the goal time as a time.Duration
func (d *DailyTimeEntry) GoalTime() time.Duration {
	return time.Duration(d.GoalMinutes) * time.Minute
}

// Progress returns the completion percentage (0.0 to 1.0)
func (d *DailyTimeEntry) Progress() float64 {
	if d.GoalMinutes == 0 {
		return 0.0
	}
	progress := float64(d.ActiveMinutes) / float64(d.GoalMinutes)
	if progress > 1.0 {
		return 1.0
	}
	return progress
}

// IsGoalReached returns true if the daily goal has been achieved
func (d *DailyTimeEntry) IsGoalReached() bool {
	return d.ActiveMinutes >= d.GoalMinutes
}

// ShouldAutoLog returns true if the entry should trigger auto-logging
func (d *DailyTimeEntry) ShouldAutoLog(thresholdHours float64) bool {
	thresholdMinutes := int(thresholdHours * 60)
	return !d.AutoLogged && d.ActiveMinutes >= thresholdMinutes
}