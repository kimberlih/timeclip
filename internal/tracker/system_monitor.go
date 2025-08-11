package tracker

/*
#cgo LDFLAGS: -framework CoreGraphics -framework IOKit -framework Foundation
#include <CoreGraphics/CoreGraphics.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/pwr_mgt/IOPMLib.h>
#include <stdlib.h>

// Check if screensaver is running
bool isScreenSaverRunning() {
    CFDictionaryRef sessionDict = CGSessionCopyCurrentDictionary();
    if (sessionDict == NULL) {
        return true; // Assume screensaver if we can't get session info
    }
    
    CFBooleanRef sessionState = CFDictionaryGetValue(sessionDict, kCGSessionOnConsoleKey);
    bool onConsole = (sessionState != NULL && CFBooleanGetValue(sessionState));
    
    CFRelease(sessionDict);
    return !onConsole;
}

// Check if user session is active
bool isUserSessionActive() {
    CFDictionaryRef sessionDict = CGSessionCopyCurrentDictionary();
    if (sessionDict == NULL) {
        return false;
    }
    
    CFBooleanRef sessionState = CFDictionaryGetValue(sessionDict, kCGSessionOnConsoleKey);
    bool onConsole = (sessionState != NULL && CFBooleanGetValue(sessionState));
    
    CFRelease(sessionDict);
    return onConsole;
}

// Check if laptop lid is open (approximation using display state)
bool isLidOpen() {
    // Get the number of active displays
    CGDirectDisplayID displays[32];
    uint32_t displayCount;
    CGGetActiveDisplayList(32, displays, &displayCount);
    
    // If we have active displays, assume lid is open
    // This is an approximation - perfect lid detection requires private APIs
    return displayCount > 0;
}
*/
import "C"

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// SystemState represents the current state of the system
type SystemState struct {
	IsUserSessionActive bool      `json:"is_user_session_active"`
	IsScreenSaverRunning bool      `json:"is_screensaver_running"`
	IsLidOpen           bool      `json:"is_lid_open"`
	IsActive            bool      `json:"is_active"`
	LastChecked         time.Time `json:"last_checked"`
}

// Monitor handles system state monitoring for macOS
type Monitor struct {
	mu           sync.RWMutex
	currentState *SystemState
	callbacks    []StateChangeCallback
	stopChan     chan bool
	isRunning    bool
}

// StateChangeCallback is called when system state changes
type StateChangeCallback func(oldState, newState *SystemState)

// NewMonitor creates a new system monitor
func NewMonitor() *Monitor {
	return &Monitor{
		currentState: &SystemState{
			LastChecked: time.Now(),
		},
		stopChan: make(chan bool),
	}
}

// GetCurrentState returns the current system state (thread-safe)
func (m *Monitor) GetCurrentState() *SystemState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	return &SystemState{
		IsUserSessionActive: m.currentState.IsUserSessionActive,
		IsScreenSaverRunning: m.currentState.IsScreenSaverRunning,
		IsLidOpen:           m.currentState.IsLidOpen,
		IsActive:            m.currentState.IsActive,
		LastChecked:         m.currentState.LastChecked,
	}
}

// IsSystemActive returns true if the system is currently active for time tracking
func (m *Monitor) IsSystemActive() bool {
	state := m.GetCurrentState()
	return state.IsActive
}

// AddStateChangeCallback adds a callback that will be called when state changes
func (m *Monitor) AddStateChangeCallback(callback StateChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// Start begins monitoring system state at the specified interval
func (m *Monitor) Start(checkInterval time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return fmt.Errorf("monitor is already running")
	}

	m.isRunning = true
	
	// Perform initial state check
	initialState := m.checkSystemState()
	m.currentState = initialState
	
	log.Printf("System monitor started - Initial state: Active=%v, Session=%v, Screensaver=%v, Lid=%v", 
		initialState.IsActive,
		initialState.IsUserSessionActive, 
		initialState.IsScreenSaverRunning,
		initialState.IsLidOpen)

	// Start monitoring goroutine
	go m.monitorLoop(checkInterval)

	return nil
}

// Stop stops the system monitoring
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return
	}

	m.isRunning = false
	close(m.stopChan)
	log.Println("System monitor stopped")
}

// monitorLoop runs the main monitoring loop
func (m *Monitor) monitorLoop(checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateState()
		case <-m.stopChan:
			return
		}
	}
}

// updateState checks current system state and updates internal state
func (m *Monitor) updateState() {
	newState := m.checkSystemState()

	m.mu.Lock()
	oldState := m.currentState
	m.currentState = newState

	// Check if state has changed
	stateChanged := (oldState.IsActive != newState.IsActive ||
		oldState.IsUserSessionActive != newState.IsUserSessionActive ||
		oldState.IsScreenSaverRunning != newState.IsScreenSaverRunning ||
		oldState.IsLidOpen != newState.IsLidOpen)

	callbacks := make([]StateChangeCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.Unlock()

	// Call callbacks if state changed
	if stateChanged {
		log.Printf("System state changed - Active=%v, Session=%v, Screensaver=%v, Lid=%v",
			newState.IsActive,
			newState.IsUserSessionActive,
			newState.IsScreenSaverRunning,
			newState.IsLidOpen)

		for _, callback := range callbacks {
			go callback(oldState, newState)
		}
	}
}

// checkSystemState performs the actual system state checking using macOS APIs
func (m *Monitor) checkSystemState() *SystemState {
	now := time.Now()

	// Check individual system components
	isUserSessionActive := bool(C.isUserSessionActive())
	isScreenSaverRunning := bool(C.isScreenSaverRunning())
	isLidOpen := bool(C.isLidOpen())

	// Determine if system is "active" for time tracking
	// Active = user logged in + lid open + screensaver not running
	isActive := isUserSessionActive && isLidOpen && !isScreenSaverRunning

	return &SystemState{
		IsUserSessionActive: isUserSessionActive,
		IsScreenSaverRunning: isScreenSaverRunning,
		IsLidOpen:           isLidOpen,
		IsActive:            isActive,
		LastChecked:         now,
	}
}

// GetStateDescription returns a human-readable description of the current state
func (m *Monitor) GetStateDescription() string {
	state := m.GetCurrentState()
	
	if state.IsActive {
		return "Active"
	}

	var reasons []string
	if !state.IsUserSessionActive {
		reasons = append(reasons, "not logged in")
	}
	if !state.IsLidOpen {
		reasons = append(reasons, "lid closed")
	}
	if state.IsScreenSaverRunning {
		reasons = append(reasons, "screensaver active")
	}

	if len(reasons) == 0 {
		return "Inactive (unknown reason)"
	}

	desc := "Inactive ("
	for i, reason := range reasons {
		if i > 0 {
			desc += ", "
		}
		desc += reason
	}
	desc += ")"

	return desc
}