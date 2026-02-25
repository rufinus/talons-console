package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// redirectConfigDir sets XDG_CONFIG_HOME (Linux) and HOME to a temp dir so that
// os.UserConfigDir() points to our controlled directory for the duration of the test.
func redirectConfigDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp) // Linux/BSD
	t.Setenv("HOME", tmp)            // Fallback if XDG not honoured
	return tmp
}

// writePIDFileDirectly writes a raw PID file to the path returned by getPIDFilePath,
// bypassing WritePIDFile so we can inject arbitrary PID values.
func writePIDFileDirectly(t *testing.T, agent, session string, lock SessionLock) string {
	t.Helper()
	path := getPIDFilePath(agent, session)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// stalePID returns a PID that is guaranteed to be no longer running.
// It starts a trivial subprocess and waits for it to exit.
func stalePID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true") // exits immediately with code 0
	if err := cmd.Run(); err != nil {
		t.Fatalf("stalePID subprocess: %v", err)
	}
	return cmd.ProcessState.Pid()
}

// ── CheckConcurrentSession ────────────────────────────────────────────────────

func TestCheckConcurrentSession_NoPIDFile(t *testing.T) {
	redirectConfigDir(t)
	result := CheckConcurrentSession("agent1", "sess1")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCheckConcurrentSession_StaleFile(t *testing.T) {
	redirectConfigDir(t)

	pid := stalePID(t)
	path := writePIDFileDirectly(t, "agent1", "sess1", SessionLock{
		PID:     pid,
		Started: time.Now().UTC().Format(time.RFC3339),
		URL:     "wss://example.com",
	})

	result := CheckConcurrentSession("agent1", "sess1")
	if result != "" {
		t.Errorf("expected empty string for stale PID, got %q", result)
	}

	// PID file should have been removed.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected stale PID file to be removed, but it still exists")
	}
}

func TestCheckConcurrentSession_CorruptFile(t *testing.T) {
	redirectConfigDir(t)

	pidPath := getPIDFilePath("agent1", "sess1")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(pidPath, []byte("not-valid-json{{{{"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result := CheckConcurrentSession("agent1", "sess1")
	if result != "" {
		t.Errorf("expected empty string for corrupt file, got %q", result)
	}

	// Corrupt PID file should have been removed.
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("expected corrupt PID file to be removed, but it still exists")
	}
}

func TestCheckConcurrentSession_LiveProcess(t *testing.T) {
	redirectConfigDir(t)

	// Write a PID file for the current process — it is definitely running.
	writePIDFileDirectly(t, "agentLive", "sessLive", SessionLock{
		PID:     os.Getpid(),
		Started: time.Now().UTC().Format(time.RFC3339),
		URL:     "wss://example.com",
	})

	result := CheckConcurrentSession("agentLive", "sessLive")
	if result == "" {
		t.Error("expected a warning message for a live process, got empty string")
	}
	if !strings.Contains(result, "warning:") {
		t.Errorf("expected warning prefix, got %q", result)
	}
	if !strings.Contains(result, fmt.Sprintf("PID %d", os.Getpid())) {
		t.Errorf("expected PID %d in warning, got %q", os.Getpid(), result)
	}
}

// ── WritePIDFile ──────────────────────────────────────────────────────────────

func TestWritePIDFile_CreatesFile(t *testing.T) {
	redirectConfigDir(t)

	cleanup := WritePIDFile("agent1", "sess1", "wss://example.com")
	defer cleanup()

	path := getPIDFilePath("agent1", "sess1")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("PID file not created: %v", err)
	}

	var lock SessionLock
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatalf("PID file is not valid JSON: %v", err)
	}

	if lock.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), lock.PID)
	}
	if lock.URL != "wss://example.com" {
		t.Errorf("expected URL wss://example.com, got %q", lock.URL)
	}
	if lock.Started == "" {
		t.Error("expected Started to be set")
	}
	// Verify Started is parseable as RFC3339.
	if _, err := time.Parse(time.RFC3339, lock.Started); err != nil {
		t.Errorf("Started is not RFC3339: %v", err)
	}
}

func TestWritePIDFile_FilePermissions(t *testing.T) {
	redirectConfigDir(t)

	cleanup := WritePIDFile("agent1", "sess1", "wss://example.com")
	defer cleanup()

	path := getPIDFilePath("agent1", "sess1")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("expected permissions 0600, got %04o", mode)
	}
}

func TestWritePIDFile_Cleanup(t *testing.T) {
	redirectConfigDir(t)

	cleanup := WritePIDFile("agent1", "sess1", "wss://example.com")
	path := getPIDFilePath("agent1", "sess1")

	// File should exist before cleanup.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("PID file should exist before cleanup")
	}

	cleanup()

	// File should be gone after cleanup.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("PID file should be removed after cleanup")
	}
}

func TestWritePIDFile_CleanupIdempotent(t *testing.T) {
	redirectConfigDir(t)

	cleanup := WritePIDFile("agent1", "sess1", "wss://example.com")

	// Should not panic when called twice.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("second cleanup call panicked: %v", r)
		}
	}()
	cleanup()
	cleanup() // second call must be a no-op
}

// ── getPIDFilePath ────────────────────────────────────────────────────────────

func TestGetPIDFilePath(t *testing.T) {
	redirectConfigDir(t)

	path := getPIDFilePath("myagent", "mysession")

	if !strings.Contains(path, "talons") {
		t.Errorf("path should contain 'talons', got %q", path)
	}
	if !strings.Contains(path, "sessions") {
		t.Errorf("path should contain 'sessions', got %q", path)
	}
	if !strings.HasSuffix(path, ".pid") {
		t.Errorf("path should end with .pid, got %q", path)
	}
	// Verify agent-session combo appears (sanitized).
	base := filepath.Base(path)
	if !strings.Contains(base, "myagent") {
		t.Errorf("filename should contain agent name, got %q", base)
	}
	if !strings.Contains(base, "mysession") {
		t.Errorf("filename should contain session name, got %q", base)
	}
}

func TestGetPIDFilePath_Separator(t *testing.T) {
	redirectConfigDir(t)

	// Agent and session should be joined by a hyphen in the filename.
	path := getPIDFilePath("foo", "bar")
	base := filepath.Base(path)
	if !strings.Contains(base, "foo-bar") {
		t.Errorf("expected 'foo-bar' in filename, got %q", base)
	}
}

// ── sanitizeName ─────────────────────────────────────────────────────────────

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with space", "with_space"},
		{"with/slash", "with_slash"},
		{"UPPER", "upper"},
		{"agent-session", "agent-session"}, // hyphen is preserved
		{"foo.bar", "foo_bar"},
		{"foo@bar!baz", "foo_bar_baz"},
		{"mixed-Case_123", "mixed-case_123"},
		{"../traversal", "___traversal"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeName(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
