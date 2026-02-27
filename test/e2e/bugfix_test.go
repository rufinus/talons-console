// Package e2e provides end-to-end integration tests.
package e2e

// bugfix_test.go contains regression tests for specific bug fixes:
//   - config-test and update-check exit 0 without TALONS_GATEWAY_URL
//   - URL validation rejects http:// and empty URL gracefully
//   - Valid ws:// and wss:// URLs are accepted

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rufinus/talons-console/internal/config"
)

// ─────────────────────────────────────────────────────────────────────────────
// Binary helpers
// ─────────────────────────────────────────────────────────────────────────────

// buildBinary compiles the talons binary into a temp directory and returns its
// path. The binary is built once per test invocation (TestMain would be ideal
// but we keep it simple by building on first call). Tests that need the binary
// call this helper; it is safe to call multiple times — each call rebuilds.
func buildBinary(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binaryName := "talons"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(dir, binaryName)

	// Find module root relative to this file's source location.
	// We use the presence of go.mod as the anchor.
	moduleRoot := findModuleRoot(t)

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/talons/") //nolint:gosec
	cmd.Dir = moduleRoot
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build failed:\n%s", string(out))

	return binaryPath
}

// findModuleRoot walks up from the test source directory until it finds go.mod.
func findModuleRoot(t *testing.T) string {
	t.Helper()
	// __file__ is not available at runtime; use os.Getwd() as starting point.
	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod starting from", dir)
		}
		dir = parent
	}
}

// runBinary executes the binary with the given args and a clean environment
// (no TALONS_* vars). Returns (stdout+stderr combined, exit code).
func runBinary(t *testing.T, binary string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...) //nolint:gosec
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if ok := isExitError(err, &exitErr); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("unexpected error running binary: %v", err)
		}
	}
	return string(out), code
}

// isExitError type-asserts err to *exec.ExitError and fills target.
func isExitError(err error, target **exec.ExitError) bool {
	var e *exec.ExitError
	if ok := errAs(err, &e); ok {
		*target = e
		return true
	}
	return false
}

// errAs is a thin wrapper around errors.As to avoid importing errors in the
// call site just for this helper.
func errAs(err error, target any) bool {
	type asInterface interface{ As(any) bool }
	// Use the stdlib errors.As behaviour via exec.ExitError type assertion.
	if e, ok := err.(*exec.ExitError); ok { //nolint:errorlint
		if t, ok2 := target.(**exec.ExitError); ok2 {
			*t = e
			return true
		}
	}
	return false
}

// cleanEnv returns a minimal environment that strips all TALONS_* variables
// so tests start with a known-clean slate. PATH is preserved so Go tooling works.
func cleanEnv() []string {
	var result []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "TALONS_") {
			continue
		}
		result = append(result, kv)
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// config-test regression
// ─────────────────────────────────────────────────────────────────────────────

// TestConfigTest_NoGatewayURL_ExitsZero ensures that `talons config-test`
// exits 0 even when no gateway URL is configured. Before the bug fix, the
// command would attempt to dial the Gateway and exit non-zero on connection error.
func TestConfigTest_NoGatewayURL_ExitsZero(t *testing.T) {
	binary := buildBinary(t)
	env := cleanEnv()

	output, code := runBinary(t, binary, env, "config-test")
	assert.Equal(t, 0, code,
		"config-test must exit 0 without TALONS_GATEWAY_URL; output: %s", output)
	assert.Contains(t, output, "Configuration loaded successfully",
		"expected success message in output")
}

// TestConfigTest_WithValidMockGateway verifies that config-test accepts a
// valid ws:// URL pointing at our mock and exits 0.
func TestConfigTest_WithValidMockGateway(t *testing.T) {
	binary := buildBinary(t)

	env := cleanEnv()
	// Pick a port that is not used. We validate the URL format, not connectivity.
	env = append(env, "TALONS_URL=ws://localhost:12345")
	env = append(env, "TALONS_TOKEN=test-token")

	output, code := runBinary(t, binary, env, "config-test")
	assert.Equal(t, 0, code,
		"config-test with a ws:// URL must exit 0; output: %s", output)
	assert.Contains(t, output, "Configuration loaded successfully")
}

// ─────────────────────────────────────────────────────────────────────────────
// update-check regression
// ─────────────────────────────────────────────────────────────────────────────

// TestUpdateCheck_NoGatewayURL_ExitsZero ensures that `talons update-check`
// exits 0 when no gateway URL is configured.
func TestUpdateCheck_NoGatewayURL_ExitsZero(t *testing.T) {
	binary := buildBinary(t)
	env := cleanEnv()

	_, code := runBinary(t, binary, env, "update-check")
	assert.Equal(t, 0, code, "update-check must exit 0 without TALONS_GATEWAY_URL")
}

// ─────────────────────────────────────────────────────────────────────────────
// URL validation (config.ValidateGateway)
// ─────────────────────────────────────────────────────────────────────────────

// TestURLValidation_ValidWS verifies that ws:// is accepted.
func TestURLValidation_ValidWS(t *testing.T) {
	cfg := &config.Config{URL: "ws://localhost:9000", Token: "tok"}
	err := cfg.ValidateGateway()
	assert.NoError(t, err, "ws:// scheme must be accepted")
}

// TestURLValidation_ValidWSS verifies that wss:// is accepted.
func TestURLValidation_ValidWSS(t *testing.T) {
	cfg := &config.Config{URL: "wss://gw.example.com", Token: "tok"}
	err := cfg.ValidateGateway()
	assert.NoError(t, err, "wss:// scheme must be accepted")
}

// TestURLValidation_HTTPRejected verifies that http:// is rejected with an
// actionable error message directing users to use ws:// or wss://.
func TestURLValidation_HTTPRejected(t *testing.T) {
	cfg := &config.Config{URL: "http://example.com", Token: "tok"}
	err := cfg.ValidateGateway()
	require.Error(t, err, "http:// scheme must be rejected")
	assert.Contains(t, strings.ToLower(err.Error()), "ws://",
		"error message should mention ws:// or wss://")
}

// TestURLValidation_EmptyURL verifies that an empty URL is rejected with a
// helpful error (not a panic).
func TestURLValidation_EmptyURL(t *testing.T) {
	cfg := &config.Config{URL: "", Token: "tok"}
	err := cfg.ValidateGateway()
	require.Error(t, err, "empty URL must be rejected")
	// Must not panic — we just need a non-nil error with a meaningful message
	assert.NotEmpty(t, err.Error(), "error message must not be empty")
}

// TestURLValidation_MissingScheme verifies that a URL without a scheme is
// rejected rather than panicking.
func TestURLValidation_MissingScheme(t *testing.T) {
	cfg := &config.Config{URL: "example.com:9000", Token: "tok"}
	err := cfg.ValidateGateway()
	require.Error(t, err, "URL without ws/wss scheme must be rejected")
}
