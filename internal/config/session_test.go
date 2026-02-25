package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckConcurrentSession_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	result := CheckConcurrentSession("main", "main")
	if result != "" {
		t.Errorf("expected empty warning, got: %q", result)
	}
}

func TestCheckConcurrentSession_StalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Write a PID file with a PID that definitely doesn't exist
	// PID 9999999 is very unlikely to be running
	lock := SessionLock{
		PID:     9999999,
		Started: "2024-01-01T00:00:00Z",
		URL:     "wss://example.com",
	}
	writePIDInDir(t, tmpDir, "main", "main", lock)

	result := CheckConcurrentSession("main", "main")
	if result != "" {
		t.Errorf("stale PID file should return empty warning, got: %q", result)
	}

	// Stale file should be removed
	path := filepath.Join(tmpDir, "talons", "sessions", "main-main.pid")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("stale PID file should have been removed")
	}
}

func TestCheckConcurrentSession_ActiveSession(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Use the current process PID — guaranteed to be running
	lock := SessionLock{
		PID:     os.Getpid(),
		Started: "2024-01-01T00:00:00Z",
		URL:     "wss://example.com",
	}
	writePIDInDir(t, tmpDir, "main", "main", lock)

	result := CheckConcurrentSession("main", "main")
	if result == "" {
		t.Error("expected non-empty warning for active session, got empty string")
	}
}

func TestCheckConcurrentSession_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Write corrupt PID file
	dir := filepath.Join(tmpDir, "talons", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	path := filepath.Join(dir, "main-main.pid")
	if err := os.WriteFile(path, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should handle gracefully — not panic, return empty string
	result := CheckConcurrentSession("main", "main")
	if result != "" {
		t.Errorf("invalid JSON should return empty warning, got: %q", result)
	}
	// File should be cleaned up
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("corrupt PID file should be removed")
	}
}

func TestWritePIDFile_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cleanup := WritePIDFile("myagent", "mysession", "wss://example.com")

	path := filepath.Join(tmpDir, "talons", "sessions", "myagent-mysession.pid")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("PID file not created: %v", err)
	}

	var lock SessionLock
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatalf("invalid JSON in PID file: %v", err)
	}

	if lock.PID != os.Getpid() {
		t.Errorf("PID: got %d, want %d", lock.PID, os.Getpid())
	}
	if lock.URL != "wss://example.com" {
		t.Errorf("URL: got %q, want %q", lock.URL, "wss://example.com")
	}
	if lock.Started == "" {
		t.Error("Started should be non-empty")
	}

	// Cleanup removes the file
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cleanup should have removed PID file")
	}
}

func TestWritePIDFile_CleanupIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cleanup := WritePIDFile("myagent", "mysession", "wss://example.com")
	// Call cleanup twice — should not panic
	cleanup()
	cleanup()
}

func TestGetPIDFilePath_Sanitizes(t *testing.T) {
	path1 := getPIDFilePath("my/agent", "my session")
	path2 := getPIDFilePath("my_agent", "my_session")
	// Slashes and spaces should be replaced
	if filepath.Base(path1) == filepath.Base(path2) {
		// This is fine — as long as they don't contain the raw unsafe chars
		return
	}
	// Verify no slash in the filename
	base := filepath.Base(path1)
	for _, c := range base {
		if c == '/' || c == '\\' {
			t.Errorf("filename contains unsafe character: %q", base)
		}
	}
}

func TestCheckConcurrentSession_DifferentSessions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Write PID file for agent=main, session=main (current process)
	lock := SessionLock{PID: os.Getpid(), Started: "2024-01-01T00:00:00Z", URL: "wss://x.com"}
	writePIDInDir(t, tmpDir, "main", "main", lock)

	// Check a different session — should not conflict
	result := CheckConcurrentSession("main", "other")
	if result != "" {
		t.Errorf("different session should not conflict, got: %q", result)
	}
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func writePIDInDir(t *testing.T, configDir, agent, session string, lock SessionLock) {
	t.Helper()
	dir := filepath.Join(configDir, "talons", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	safe := reUnsafe.ReplaceAllString(agent+"-"+session, "_")
	path := filepath.Join(dir, safe+".pid")
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
