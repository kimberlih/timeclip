package menubar

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/getlantern/systray"
	"timeclip/internal/models"
)

// SystrayMenuBar manages the macOS menu bar using systray library
type SystrayMenuBar struct {
	mu             sync.RWMutex
	isInitialized  bool
	pauseHandler   func() error
	quitHandler    func()
	currentStats   *MenuBarStats
	initialStats   *MenuBarStats  // Stats to use when systray becomes ready
	pauseMenuItem  *systray.MenuItem
	statsMenuItem  *systray.MenuItem
}

// MenuBarStats represents the current statistics for menu bar display
type MenuBarStats struct {
	ActiveMinutes  int     `json:"active_minutes"`
	GoalMinutes    int     `json:"goal_minutes"`
	Progress       float64 `json:"progress"`
	IsGoalReached  bool    `json:"is_goal_reached"`
	IsPaused       bool    `json:"is_paused"`
	IsSystemActive bool    `json:"is_system_active"`
}

// NewSystrayMenuBar creates a new systray-based menu bar
func NewSystrayMenuBar() *SystrayMenuBar {
	return &SystrayMenuBar{
		currentStats: &MenuBarStats{},
	}
}

// Run starts the systray menu bar (this should be called from main goroutine)
func (smb *SystrayMenuBar) Run(pauseHandler func() error, quitHandler func()) {
	smb.pauseHandler = pauseHandler
	smb.quitHandler = quitHandler
	
	log.Println("Starting systray menu bar...")
	systray.Run(smb.onReady, smb.onExit)
}

// onReady is called when systray is ready
func (smb *SystrayMenuBar) onReady() {
	smb.mu.Lock()
	
	// Use initial stats if available, otherwise use defaults
	var initialStats *MenuBarStats
	if smb.initialStats != nil {
		initialStats = smb.initialStats
	} else {
		initialStats = &MenuBarStats{
			ActiveMinutes: 0,
			GoalMinutes: 480, // Default 8 hours
		}
	}
	
	// Set initial icon, title and tooltip based on actual data
	state := smb.determineMenuState(initialStats)
	smb.setIcon(state)
	
	title := smb.generateTitle(initialStats)
	systray.SetTitle(title)
	
	tooltip := smb.generateTooltip(initialStats)
	systray.SetTooltip(tooltip)

	// Create menu items with actual data
	statsText := smb.generateStatsText(initialStats)
	smb.statsMenuItem = systray.AddMenuItem(statsText, "Current day statistics")
	smb.statsMenuItem.Disable()

	systray.AddSeparator()

	pauseText := "Resume"
	if !initialStats.IsPaused {
		pauseText = "Pause"
	}
	smb.pauseMenuItem = systray.AddMenuItem(pauseText, "Pause/Resume time tracking")
	
	systray.AddSeparator()
	
	configMenuItem := systray.AddMenuItem("Configuration...", "Open configuration file")
	
	systray.AddSeparator()
	
	quitMenuItem := systray.AddMenuItem("Quit Timeclip", "Exit the application")

	smb.isInitialized = true
	smb.mu.Unlock()
	
	log.Println("✅ Menu bar initialized successfully")

	// Handle menu clicks in separate goroutines
	go smb.handlePauseClicks()
	go smb.handleConfigClicks(configMenuItem)
	go smb.handleQuitClicks(quitMenuItem)
}

// onExit is called when systray is exiting
func (smb *SystrayMenuBar) onExit() {
	log.Println("Menu bar exiting...")
	if smb.quitHandler != nil {
		smb.quitHandler()
	}
}

// UpdateStats updates the menu bar display with current statistics
func (smb *SystrayMenuBar) UpdateStats(stats *MenuBarStats) {
	smb.mu.Lock()
	smb.currentStats = stats
	
	// If not initialized yet, store as initial stats
	if !smb.isInitialized {
		smb.initialStats = stats
		smb.mu.Unlock()
		return
	}
	smb.mu.Unlock()

	// Update title
	title := smb.generateTitle(stats)
	systray.SetTitle(title)

	// Update icon based on state
	state := smb.determineMenuState(stats)
	smb.setIcon(state)

	// Update tooltip
	tooltip := smb.generateTooltip(stats)
	systray.SetTooltip(tooltip)

	// Update stats menu item
	statsText := smb.generateStatsText(stats)
	smb.statsMenuItem.SetTitle(statsText)

	// Update pause menu item
	pauseText := "Resume"
	if !stats.IsPaused {
		pauseText = "Pause"
	}
	smb.pauseMenuItem.SetTitle(pauseText)
}

// handlePauseClicks handles pause/resume menu clicks
func (smb *SystrayMenuBar) handlePauseClicks() {
	for {
		select {
		case <-smb.pauseMenuItem.ClickedCh:
			if smb.pauseHandler != nil {
				if err := smb.pauseHandler(); err != nil {
					log.Printf("Error toggling pause: %v", err)
				}
			}
		}
	}
}

// handleConfigClicks handles configuration menu clicks
func (smb *SystrayMenuBar) handleConfigClicks(menuItem *systray.MenuItem) {
	for {
		select {
		case <-menuItem.ClickedCh:
			// Open configuration file with default editor
			smb.openConfigFile()
		}
	}
}

// handleQuitClicks handles quit menu clicks
func (smb *SystrayMenuBar) handleQuitClicks(menuItem *systray.MenuItem) {
	for {
		select {
		case <-menuItem.ClickedCh:
			log.Println("Quit requested from menu bar")
			systray.Quit()
		}
	}
}

// MenuState represents different visual states of the menu bar
type MenuState int

const (
	MenuStateInactive MenuState = iota // Red - less than goal
	MenuStatePaused                    // Orange - paused
	MenuStateActive                    // Green - goal reached
)

// determineMenuState determines the appropriate menu state
func (smb *SystrayMenuBar) determineMenuState(stats *MenuBarStats) MenuState {
	if stats.IsPaused {
		return MenuStatePaused
	}
	if stats.IsGoalReached {
		return MenuStateActive
	}
	return MenuStateInactive
}

// setIcon sets the appropriate icon based on menu state
func (smb *SystrayMenuBar) setIcon(state MenuState) {
	switch state {
	case MenuStatePaused:
		systray.SetTemplateIcon(pauseIcon, pauseIcon)
	case MenuStateActive:
		systray.SetTemplateIcon(activeIcon, activeIcon)
	case MenuStateInactive:
		systray.SetTemplateIcon(inactiveIcon, inactiveIcon)
	}
}

// generateTitle creates the menu bar title text
func (smb *SystrayMenuBar) generateTitle(stats *MenuBarStats) string {
	hours := float64(stats.ActiveMinutes) / 60.0
	
	var prefix string
	switch smb.determineMenuState(stats) {
	case MenuStatePaused:
		prefix = "⏸"
	case MenuStateActive:
		prefix = "✅"
	case MenuStateInactive:
		prefix = "⏱"
	}
	
	if hours < 1 {
		return fmt.Sprintf("%s %dm", prefix, stats.ActiveMinutes)
	} else if hours >= 10 {
		return fmt.Sprintf("%s %.0fh", prefix, hours)
	} else {
		return fmt.Sprintf("%s %.1fh", prefix, hours)
	}
}

// generateTooltip creates detailed tooltip text
func (smb *SystrayMenuBar) generateTooltip(stats *MenuBarStats) string {
	hours := float64(stats.ActiveMinutes) / 60.0
	goalHours := float64(stats.GoalMinutes) / 60.0
	progress := int(stats.Progress * 100)
	
	var status string
	switch smb.determineMenuState(stats) {
	case MenuStatePaused:
		status = "Paused"
	case MenuStateActive:
		status = "Goal Reached!"
	case MenuStateInactive:
		if stats.IsSystemActive {
			status = "Tracking"
		} else {
			status = "Inactive"
		}
	}
	
	tooltip := fmt.Sprintf("Timeclip - %s\nToday: %.1fh / %.0fh (%d%%)", 
		status, hours, goalHours, progress)
	
	if stats.IsGoalReached {
		overtime := stats.ActiveMinutes - stats.GoalMinutes
		if overtime > 0 {
			overtimeHours := float64(overtime) / 60.0
			tooltip += fmt.Sprintf("\nOvertime: %.1fh", overtimeHours)
		}
	} else {
		remaining := stats.GoalMinutes - stats.ActiveMinutes
		if remaining > 0 {
			remainingHours := float64(remaining) / 60.0
			tooltip += fmt.Sprintf("\nRemaining: %.1fh", remainingHours)
		}
	}
	
	return tooltip
}

// generateStatsText creates text for the stats menu item
func (smb *SystrayMenuBar) generateStatsText(stats *MenuBarStats) string {
	hours := float64(stats.ActiveMinutes) / 60.0
	goalHours := float64(stats.GoalMinutes) / 60.0
	progress := int(stats.Progress * 100)
	
	return fmt.Sprintf("Today: %.1fh / %.0fh (%d%%)", hours, goalHours, progress)
}

// openConfigFile launches the separate configuration application
func (smb *SystrayMenuBar) openConfigFile() {
	log.Println("Configuration menu clicked - launching configuration application...")
	
	// Launch configuration application in background
	go func() {
		// Get the directory of the current executable
		execPath, err := os.Executable()
		if err != nil {
			log.Printf("Failed to get executable path: %v", err)
			return
		}
		
		configAppPath := filepath.Join(filepath.Dir(execPath), "timeclip-config")
		
		// Try to run the config app
		cmd := exec.Command(configAppPath)
		if err := cmd.Start(); err != nil {
			log.Printf("Failed to launch configuration app: %v", err)
			// Could fall back to building it on the fly, but for now just log
		} else {
			log.Println("Configuration application launched successfully")
		}
	}()
}

// StatsFromTimeEntry converts a time entry to menu bar stats
func StatsFromTimeEntry(entry *models.DailyTimeEntry, isSystemActive bool) *MenuBarStats {
	if entry == nil {
		return &MenuBarStats{
			GoalMinutes: 480, // Default 8 hours
		}
	}

	return &MenuBarStats{
		ActiveMinutes:  entry.ActiveMinutes,
		GoalMinutes:    entry.GoalMinutes,
		Progress:       entry.Progress(),
		IsGoalReached:  entry.IsGoalReached(),
		IsPaused:       entry.IsPaused,
		IsSystemActive: isSystemActive,
	}
}

// Simple icon data (using minimal icons for now)
var (
	// Red circle for inactive state
	inactiveIcon = []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, 0x00, 0x00, 0x00,
		0x13, 0x49, 0x44, 0x41, 0x54, 0x38, 0xCB, 0x63, 0xF8, 0x0F, 0x00, 0x01,
		0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB4, 0x1D, 0x00, 0x00, 0x00, 0x00,
		0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	
	// Orange circle for paused state  
	pauseIcon = []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, 0x00, 0x00, 0x00,
		0x13, 0x49, 0x44, 0x41, 0x54, 0x38, 0xCB, 0x63, 0xF8, 0x8F, 0x00, 0x01,
		0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB4, 0x1D, 0x00, 0x00, 0x00, 0x00,
		0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	
	// Green circle for active/goal reached state
	activeIcon = []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, 0x00, 0x00, 0x00,
		0x13, 0x49, 0x44, 0x41, 0x54, 0x38, 0xCB, 0x63, 0xF8, 0x0F, 0x80, 0x01,
		0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB4, 0x1D, 0x00, 0x00, 0x00, 0x00,
		0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
)