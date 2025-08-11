package instance

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Lock represents a single instance lock
type Lock struct {
	lockFile *os.File
	lockPath string
}

// NewLock creates a new single instance lock
func NewLock() (*Lock, error) {
	// Get lock file path in the same directory as config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	
	lockDir := filepath.Join(homeDir, ".timeclip")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}
	
	lockPath := filepath.Join(lockDir, "timeclip.lock")
	
	return &Lock{
		lockPath: lockPath,
	}, nil
}

// TryLock attempts to acquire the single instance lock
func (l *Lock) TryLock() error {
	// Try to create/open the lock file
	lockFile, err := os.OpenFile(l.lockPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists, check if it's from a running process
			return l.checkExistingLock()
		}
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	
	// Try to acquire an exclusive lock
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lockFile.Close()
		os.Remove(l.lockPath)
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("another instance of Timeclip is already running")
		}
		return fmt.Errorf("failed to acquire file lock: %w", err)
	}
	
	// Write our PID to the lock file
	pid := os.Getpid()
	if _, err := fmt.Fprintf(lockFile, "%d\n", pid); err != nil {
		lockFile.Close()
		os.Remove(l.lockPath)
		return fmt.Errorf("failed to write PID to lock file: %w", err)
	}
	
	// Flush the file
	if err := lockFile.Sync(); err != nil {
		lockFile.Close()
		os.Remove(l.lockPath)
		return fmt.Errorf("failed to sync lock file: %w", err)
	}
	
	l.lockFile = lockFile
	return nil
}

// checkExistingLock checks if an existing lock file is from a running process
func (l *Lock) checkExistingLock() error {
	// Try to open the existing lock file
	existingFile, err := os.OpenFile(l.lockPath, os.O_RDWR, 0644)
	if err != nil {
		// If we can't open it, try to remove and create new
		if os.IsNotExist(err) {
			// File disappeared, try again
			return l.TryLock()
		}
		return fmt.Errorf("failed to open existing lock file: %w", err)
	}
	defer existingFile.Close()
	
	// Try to acquire exclusive lock (non-blocking)
	if err := syscall.Flock(int(existingFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("another instance of Timeclip is already running")
		}
		return fmt.Errorf("failed to check existing lock: %w", err)
	}
	
	// If we got the lock, it means the previous process died without cleanup
	// Read the old PID for informational purposes
	var oldPID int
	fmt.Fscanf(existingFile, "%d", &oldPID)
	
	// Release the lock and remove the stale lock file
	syscall.Flock(int(existingFile.Fd()), syscall.LOCK_UN)
	existingFile.Close()
	os.Remove(l.lockPath)
	
	fmt.Printf("⚠️  Found stale lock file from PID %d, cleaning up...\n", oldPID)
	
	// Try to acquire lock again
	return l.TryLock()
}

// Release releases the single instance lock
func (l *Lock) Release() error {
	if l.lockFile == nil {
		return nil
	}
	
	// Release the file lock
	if err := syscall.Flock(int(l.lockFile.Fd()), syscall.LOCK_UN); err != nil {
		// Continue with cleanup even if unlock fails
		fmt.Printf("Warning: failed to release file lock: %v\n", err)
	}
	
	// Close the file
	if err := l.lockFile.Close(); err != nil {
		fmt.Printf("Warning: failed to close lock file: %v\n", err)
	}
	
	// Remove the lock file
	if err := os.Remove(l.lockPath); err != nil {
		fmt.Printf("Warning: failed to remove lock file: %v\n", err)
	}
	
	l.lockFile = nil
	return nil
}

// IsLocked returns true if this instance holds the lock
func (l *Lock) IsLocked() bool {
	return l.lockFile != nil
}

// GetLockPath returns the path to the lock file
func (l *Lock) GetLockPath() string {
	return l.lockPath
}

// WaitForLockRelease waits for another instance to release the lock (with timeout)
func (l *Lock) WaitForLockRelease(timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("another instance of Timeclip is already running")
	}
	
	fmt.Printf("⏳ Another instance is running, waiting up to %v for it to exit...\n", timeout)
	
	start := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if time.Since(start) >= timeout {
				return fmt.Errorf("timeout waiting for other instance to exit")
			}
			
			// Try to acquire lock
			if err := l.TryLock(); err == nil {
				fmt.Println("✅ Lock acquired, continuing...")
				return nil
			}
			
		case <-time.After(timeout):
			return fmt.Errorf("timeout waiting for other instance to exit")
		}
	}
}