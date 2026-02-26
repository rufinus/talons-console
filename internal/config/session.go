package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// SessionLock contains the information written to the PID file.
type SessionLock struct {
	PID     int    `json:"pid"`
	Started string `json:"started"` // ISO8601
	URL     string `json:"url"`
}

// CheckConcurrentSession checks whether another talons instance is already
// running with the same agent+session combination.
// Returns a non-empty warning string if a concurrent session is detected,
// or an empty string if it's safe to proceed.
func CheckConcurrentSession(agent, session string) string {
	path := getPIDFilePath(agent, session)

	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist or can't be read → no conflict
		return ""
	}

	var lock SessionLock
	if err := json.Unmarshal(data, &lock); err != nil {
		// Corrupt PID file → treat as stale, remove it
		_ = os.Remove(path)
		return ""
	}

	if isProcessRunning(lock.PID) {
		return fmt.Sprintf(
			"⚠  Another talons instance (PID %d, started %s) is already connected to %s:%s. Both will connect.",
			lock.PID, lock.Started, agent, session,
		)
	}

	// Stale PID file — process is no longer running
	_ = os.Remove(path)
	return ""
}

// WritePIDFile writes a PID file for the current process and returns a cleanup
// function that removes the file. The cleanup should be deferred by the caller.
func WritePIDFile(agent, session, url string) func() {
	path := getPIDFilePath(agent, session)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		// Can't create directory — skip PID file silently
		return func() {}
	}

	lock := SessionLock{
		PID:     os.Getpid(),
		Started: time.Now().UTC().Format(time.RFC3339),
		URL:     url,
	}

	data, err := json.Marshal(lock)
	if err != nil {
		return func() {}
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		// Can't write PID file — skip silently
		return func() {}
	}

	var cleaned bool
	return func() {
		if !cleaned {
			cleaned = true
			_ = os.Remove(path)
		}
	}
}

// getPIDFilePath returns the full path for the PID file of an agent+session pair.
// Non-alphanumeric characters are replaced with underscores for filesystem safety.
func getPIDFilePath(agent, session string) string {
	safe := reUnsafe.ReplaceAllString(agent+"-"+session, "_")
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "talons", "sessions", safe+".pid")
}

// reUnsafe matches characters that are unsafe in a PID file name.
var reUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
