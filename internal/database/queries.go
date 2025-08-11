package database

import (
	"fmt"
	"time"

	"timeclip/internal/models"
)

// GetWeeklyStats returns aggregated statistics for the current week
func (db *DB) GetWeeklyStats() (*WeeklyStats, error) {
	now := time.Now()
	
	// Get start of current week (Monday)
	weekday := int(now.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7
	}
	startOfWeek := now.AddDate(0, 0, -weekday+1)
	endOfWeek := startOfWeek.AddDate(0, 0, 6)

	query := `
	SELECT 
		COUNT(*) as days_tracked,
		SUM(active_minutes) as total_minutes,
		AVG(active_minutes) as avg_minutes_per_day,
		SUM(CASE WHEN active_minutes >= goal_minutes THEN 1 ELSE 0 END) as goal_days,
		MIN(date) as week_start,
		MAX(date) as week_end
	FROM daily_time 
	WHERE date >= ? AND date <= ?`

	stats := &WeeklyStats{}
	err := db.conn.QueryRow(query, 
		startOfWeek.Format("2006-01-02"),
		endOfWeek.Format("2006-01-02"),
	).Scan(
		&stats.DaysTracked,
		&stats.TotalMinutes,
		&stats.AvgMinutesPerDay,
		&stats.GoalDays,
		&stats.WeekStart,
		&stats.WeekEnd,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get weekly stats: %w", err)
	}

	return stats, nil
}

// GetMonthlyStats returns aggregated statistics for the current month
func (db *DB) GetMonthlyStats() (*MonthlyStats, error) {
	now := time.Now()
	
	// Get start and end of current month
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, -1)

	query := `
	SELECT 
		COUNT(*) as days_tracked,
		SUM(active_minutes) as total_minutes,
		AVG(active_minutes) as avg_minutes_per_day,
		SUM(CASE WHEN active_minutes >= goal_minutes THEN 1 ELSE 0 END) as goal_days,
		MIN(date) as month_start,
		MAX(date) as month_end
	FROM daily_time 
	WHERE date >= ? AND date <= ?`

	stats := &MonthlyStats{}
	err := db.conn.QueryRow(query,
		startOfMonth.Format("2006-01-02"),
		endOfMonth.Format("2006-01-02"),
	).Scan(
		&stats.DaysTracked,
		&stats.TotalMinutes,
		&stats.AvgMinutesPerDay,
		&stats.GoalDays,
		&stats.MonthStart,
		&stats.MonthEnd,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get monthly stats: %w", err)
	}

	return stats, nil
}

// GetEntriesNeedingAutoLog returns entries that should be auto-logged
func (db *DB) GetEntriesNeedingAutoLog(thresholdMinutes int) ([]*models.DailyTimeEntry, error) {
	query := `
	SELECT id, date, active_minutes, goal_minutes, is_paused, auto_logged,
	       auto_log_response, created_at, updated_at
	FROM daily_time 
	WHERE auto_logged = FALSE AND active_minutes >= ?
	ORDER BY date ASC`

	rows, err := db.conn.Query(query, thresholdMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries needing auto-log: %w", err)
	}
	defer rows.Close()

	var entries []*models.DailyTimeEntry
	for rows.Next() {
		entry := &models.DailyTimeEntry{}
		err := rows.Scan(
			&entry.ID, &entry.Date, &entry.ActiveMinutes, &entry.GoalMinutes,
			&entry.IsPaused, &entry.AutoLogged, &entry.AutoLogResponse,
			&entry.CreatedAt, &entry.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating entries: %w", err)
	}

	return entries, nil
}

// CleanupOldEntries removes entries older than the specified number of days
func (db *DB) CleanupOldEntries(retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02")

	// Clean up daily_time entries
	query := `DELETE FROM daily_time WHERE date < ?`
	result, err := db.conn.Exec(query, cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to cleanup old daily entries: %w", err)
	}

	dailyDeleted, _ := result.RowsAffected()

	// Clean up system_events entries  
	query = `DELETE FROM system_events WHERE DATE(timestamp) < ?`
	result, err = db.conn.Exec(query, cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to cleanup old system events: %w", err)
	}

	eventsDeleted, _ := result.RowsAffected()

	// Log the cleanup
	db.LogSystemEvent("cleanup", fmt.Sprintf("Deleted %d daily entries and %d system events older than %s", 
		dailyDeleted, eventsDeleted, cutoffDate))

	return nil
}

// WeeklyStats represents weekly time tracking statistics
type WeeklyStats struct {
	DaysTracked       int     `json:"days_tracked"`
	TotalMinutes      int     `json:"total_minutes"`
	AvgMinutesPerDay  float64 `json:"avg_minutes_per_day"`
	GoalDays          int     `json:"goal_days"`
	WeekStart         string  `json:"week_start"`
	WeekEnd           string  `json:"week_end"`
}

// TotalHours returns total hours worked this week
func (ws *WeeklyStats) TotalHours() float64 {
	return float64(ws.TotalMinutes) / 60.0
}

// AvgHoursPerDay returns average hours per day this week
func (ws *WeeklyStats) AvgHoursPerDay() float64 {
	return ws.AvgMinutesPerDay / 60.0
}

// MonthlyStats represents monthly time tracking statistics
type MonthlyStats struct {
	DaysTracked       int     `json:"days_tracked"`
	TotalMinutes      int     `json:"total_minutes"`
	AvgMinutesPerDay  float64 `json:"avg_minutes_per_day"`
	GoalDays          int     `json:"goal_days"`
	MonthStart        string  `json:"month_start"`
	MonthEnd          string  `json:"month_end"`
}

// TotalHours returns total hours worked this month
func (ms *MonthlyStats) TotalHours() float64 {
	return float64(ms.TotalMinutes) / 60.0
}

// AvgHoursPerDay returns average hours per day this month
func (ms *MonthlyStats) AvgHoursPerDay() float64 {
	return ms.AvgMinutesPerDay / 60.0
}