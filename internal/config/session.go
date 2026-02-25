package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SessionLock contains metadata written to a PID file.
type SessionLock struct {
	PID     int    `json:"pid"`
	Started string `json:"started"`
	URL     string `json:"url"`
}

// CheckConcurrentSession checks if another talons is running with the same agent+session.
// Returns a warning message if a concurrent session is detected, empty string otherwise.
func CheckConcurrentSession(agent, session string) string {
	path := getPIDFilePath(agent, session)
	data, err := os.ReadFile(path)
	if err != nil {
		return "" // no PID file = no conflict
	}

	var lock SessionLock
	if err := json.Unmarshal(data, &lock); err != nil {
		os.Remove(path) // corrupt file, clean up
		return ""
	}

	// isProcessRunning is implemented per-platform via build tags.
	if !isProcessRunning(lock.PID) {
		os.Remove(path) // stale PID file, clean up
		return ""
	}

	return fmt.Sprintf("warning: another talons session is already running (PID %d, started %s)", lock.PID, lock.Started)
}

// WritePIDFile creates a PID file for the current process.
// Returns a cleanup function that removes the file — always defer it.
func WritePIDFile(agent, session, url string) func() {
	path := getPIDFilePath(agent, session)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return func() {} // can't write, return no-op cleanup
	}

	lock := SessionLock{
		PID:     os.Getpid(),
		Started: time.Now().UTC().Format(time.RFC3339),
		URL:     url,
	}
	data, _ := json.Marshal(lock)
	_ = os.WriteFile(path, data, 0600)

	var cleaned bool
	return func() {
		if !cleaned {
			cleaned = true
			os.Remove(path)
		}
	}
}

// getPIDFilePath returns the path for a PID file for the given agent+session.
func getPIDFilePath(agent, session string) string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	safe := sanitizeName(agent + "-" + session)
	return filepath.Join(cfgDir, "talons", "sessions", safe+".pid")
}

// sanitizeName replaces non-alphanumeric chars (except hyphen) with underscore.
var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9\-]`)

func sanitizeName(s string) string {
	return nonAlphaNum.ReplaceAllString(strings.ToLower(s), "_")
}
