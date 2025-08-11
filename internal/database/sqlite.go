package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"timeclip/internal/models"
)

// DB wraps the SQLite database connection and provides time tracking operations
type DB struct {
	conn   *sql.DB
	dbPath string
}

// NewDB creates a new database instance and initializes the schema
func NewDB(dbPath string) (*DB, error) {
	// Expand ~ in path
	if strings.HasPrefix(dbPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, dbPath[2:])
	}

	// Create directory if it doesn't exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
	}

	// Open database connection
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}

	// Configure connection
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		conn:   conn,
		dbPath: dbPath,
	}

	// Initialize database schema
	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// initSchema creates the necessary tables if they don't exist
func (db *DB) initSchema() error {
	// Create daily_time table
	createDailyTimeTable := `
	CREATE TABLE IF NOT EXISTS daily_time (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT UNIQUE NOT NULL,
		active_minutes INTEGER DEFAULT 0,
		goal_minutes INTEGER DEFAULT 480,
		is_paused BOOLEAN DEFAULT FALSE,
		auto_logged BOOLEAN DEFAULT FALSE,
		auto_log_response TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.conn.Exec(createDailyTimeTable); err != nil {
		return fmt.Errorf("failed to create daily_time table: %w", err)
	}

	// Create system_events table for debugging
	createSystemEventsTable := `
	CREATE TABLE IF NOT EXISTS system_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		details TEXT DEFAULT ''
	);`

	if _, err := db.conn.Exec(createSystemEventsTable); err != nil {
		return fmt.Errorf("failed to create system_events table: %w", err)
	}

	// Create indexes for better performance
	createIndexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_daily_time_date ON daily_time(date);",
		"CREATE INDEX IF NOT EXISTS idx_system_events_timestamp ON system_events(timestamp);",
		"CREATE INDEX IF NOT EXISTS idx_system_events_type ON system_events(event_type);",
	}

	for _, indexSQL := range createIndexes {
		if _, err := db.conn.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// GetTodayEntry gets or creates today's time entry
func (db *DB) GetTodayEntry() (*models.DailyTimeEntry, error) {
	today := time.Now().Format("2006-01-02")
	return db.GetEntryForDate(today)
}

// GetEntryForDate gets or creates a time entry for a specific date
func (db *DB) GetEntryForDate(date string) (*models.DailyTimeEntry, error) {
	entry := &models.DailyTimeEntry{}
	
	query := `
	SELECT id, date, active_minutes, goal_minutes, is_paused, auto_logged, 
	       auto_log_response, created_at, updated_at
	FROM daily_time 
	WHERE date = ?`

	err := db.conn.QueryRow(query, date).Scan(
		&entry.ID, &entry.Date, &entry.ActiveMinutes, &entry.GoalMinutes,
		&entry.IsPaused, &entry.AutoLogged, &entry.AutoLogResponse,
		&entry.CreatedAt, &entry.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create new entry for this date
		return db.createEntryForDate(date)
	} else if err != nil {
		return nil, fmt.Errorf("failed to query daily time entry: %w", err)
	}

	return entry, nil
}

// createEntryForDate creates a new time entry for a specific date
func (db *DB) createEntryForDate(date string) (*models.DailyTimeEntry, error) {
	query := `
	INSERT INTO daily_time (date, active_minutes, goal_minutes, is_paused, auto_logged)
	VALUES (?, 0, 480, FALSE, FALSE)`

	result, err := db.conn.Exec(query, date)
	if err != nil {
		return nil, fmt.Errorf("failed to create daily time entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	// Return the newly created entry
	return db.GetEntryByID(int(id))
}

// GetEntryByID gets a time entry by its ID
func (db *DB) GetEntryByID(id int) (*models.DailyTimeEntry, error) {
	entry := &models.DailyTimeEntry{}
	
	query := `
	SELECT id, date, active_minutes, goal_minutes, is_paused, auto_logged,
	       auto_log_response, created_at, updated_at
	FROM daily_time 
	WHERE id = ?`

	err := db.conn.QueryRow(query, id).Scan(
		&entry.ID, &entry.Date, &entry.ActiveMinutes, &entry.GoalMinutes,
		&entry.IsPaused, &entry.AutoLogged, &entry.AutoLogResponse,
		&entry.CreatedAt, &entry.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get entry by ID %d: %w", id, err)
	}

	return entry, nil
}

// IncrementActiveTime adds one minute to today's active time
func (db *DB) IncrementActiveTime() error {
	return db.IncrementActiveTimeForDate(time.Now().Format("2006-01-02"))
}

// IncrementActiveTimeForDate adds one minute to the active time for a specific date
func (db *DB) IncrementActiveTimeForDate(date string) error {
	// First ensure the entry exists
	_, err := db.GetEntryForDate(date)
	if err != nil {
		return fmt.Errorf("failed to ensure entry exists: %w", err)
	}

	// Increment active minutes and update timestamp
	query := `
	UPDATE daily_time 
	SET active_minutes = active_minutes + 1,
	    updated_at = CURRENT_TIMESTAMP
	WHERE date = ? AND is_paused = FALSE`

	result, err := db.conn.Exec(query, date)
	if err != nil {
		return fmt.Errorf("failed to increment active time: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Entry might be paused, log this event
		db.LogSystemEvent("increment_skipped_paused", fmt.Sprintf("Date: %s", date))
	}

	return nil
}

// SetPauseState sets the pause state for today's entry
func (db *DB) SetPauseState(paused bool) error {
	today := time.Now().Format("2006-01-02")
	return db.SetPauseStateForDate(today, paused)
}

// SetPauseStateForDate sets the pause state for a specific date
func (db *DB) SetPauseStateForDate(date string, paused bool) error {
	// First ensure the entry exists
	_, err := db.GetEntryForDate(date)
	if err != nil {
		return fmt.Errorf("failed to ensure entry exists: %w", err)
	}

	query := `
	UPDATE daily_time 
	SET is_paused = ?, updated_at = CURRENT_TIMESTAMP
	WHERE date = ?`

	_, err = db.conn.Exec(query, paused, date)
	if err != nil {
		return fmt.Errorf("failed to set pause state: %w", err)
	}

	// Log the event
	eventType := "resume"
	if paused {
		eventType = "pause"
	}
	db.LogSystemEvent(eventType, fmt.Sprintf("Date: %s", date))

	return nil
}

// MarkAsAutoLogged marks an entry as having been auto-logged
func (db *DB) MarkAsAutoLogged(date string, response string) error {
	query := `
	UPDATE daily_time 
	SET auto_logged = TRUE, 
	    auto_log_response = ?,
	    updated_at = CURRENT_TIMESTAMP
	WHERE date = ?`

	_, err := db.conn.Exec(query, response, date)
	if err != nil {
		return fmt.Errorf("failed to mark as auto-logged: %w", err)
	}

	db.LogSystemEvent("auto_logged", fmt.Sprintf("Date: %s", date))
	return nil
}

// LogSystemEvent logs a system event for debugging
func (db *DB) LogSystemEvent(eventType, details string) error {
	query := `
	INSERT INTO system_events (event_type, details)
	VALUES (?, ?)`

	_, err := db.conn.Exec(query, eventType, details)
	if err != nil {
		return fmt.Errorf("failed to log system event: %w", err)
	}

	return nil
}

// GetRecentEntries returns the most recent time entries
func (db *DB) GetRecentEntries(limit int) ([]*models.DailyTimeEntry, error) {
	query := `
	SELECT id, date, active_minutes, goal_minutes, is_paused, auto_logged,
	       auto_log_response, created_at, updated_at
	FROM daily_time 
	ORDER BY date DESC
	LIMIT ?`

	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent entries: %w", err)
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

// GetDatabasePath returns the database file path
func (db *DB) GetDatabasePath() string {
	return db.dbPath
}