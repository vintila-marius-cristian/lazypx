// Package audit provides an append-only audit log for destructive operations.
package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	logFile *os.File
	logPath string
)

// Init opens the audit log file. Called once at startup.
// Profile is embedded in each line for multi-profile setups.
func Init(configDir string) error {
	mu.Lock()
	defer mu.Unlock()

	logPath = filepath.Join(configDir, "audit.log")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	logFile = f
	return nil
}

// Close flushes and closes the audit log.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		_ = logFile.Sync()
		_ = logFile.Close()
		logFile = nil
	}
}

// Log writes a single audit entry. Never panics — errors are silently dropped
// so that audit failures do not interrupt the primary operation.
func Log(profile, user, action, resource string) {
	mu.Lock()
	defer mu.Unlock()
	if logFile == nil {
		return
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	line := fmt.Sprintf("[%s] [%s] [%s] %s %s\n", ts, profile, user, action, resource)
	_, _ = logFile.WriteString(line)
}

// LogPath returns the current audit log path.
func LogPath() string {
	return logPath
}
