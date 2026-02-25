package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// writeConfig creates a talons/config.yaml under the given root directory.
func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, "talons")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config %s: %v", path, err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Agent != "main" {
		t.Errorf("Agent: got %q, want %q", cfg.Agent, "main")
	}
	if cfg.Session != "main" {
		t.Errorf("Session: got %q, want %q", cfg.Session, "main")
	}
	if cfg.HistoryLimit != 200 {
		t.Errorf("HistoryLimit: got %d, want 200", cfg.HistoryLimit)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "warn")
	}
	if cfg.Thinking != "off" {
		t.Errorf("Thinking: got %q, want %q", cfg.Thinking, "off")
	}
	// Zero values
	if cfg.URL != "" {
		t.Errorf("URL: got %q, want empty", cfg.URL)
	}
	if cfg.Token != "" {
		t.Errorf("Token: got %q, want empty", cfg.Token)
	}
	if cfg.Password != "" {
		t.Errorf("Password: got %q, want empty", cfg.Password)
	}
	if cfg.Deliver != false {
		t.Errorf("Deliver: got %v, want false", cfg.Deliver)
	}
	if cfg.TimeoutMs != 0 {
		t.Errorf("TimeoutMs: got %d, want 0", cfg.TimeoutMs)
	}
}

func TestValidate_MissingURL(t *testing.T) {
	cfg := &Config{
		URL:   "",
		Token: "some-token",
	}
	problems := cfg.Validate()
	if len(problems) == 0 {
		t.Fatal("expected validation problems, got none")
	}
	found := false
	for _, p := range problems {
		if p == "url is required" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'url is required' in problems, got: %v", problems)
	}
}

func TestValidate_InvalidScheme(t *testing.T) {
	cfg := &Config{
		URL:   "http://localhost:8080",
		Token: "some-token",
	}
	problems := cfg.Validate()
	if len(problems) == 0 {
		t.Fatal("expected validation problems, got none")
	}
	found := false
	for _, p := range problems {
		if p == "url must use ws:// or wss:// scheme" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'url must use ws:// or wss:// scheme' in problems, got: %v", problems)
	}
}

func TestValidate_MissingAuth(t *testing.T) {
	cfg := &Config{
		URL:      "wss://example.com",
		Token:    "",
		Password: "",
	}
	problems := cfg.Validate()
	if len(problems) == 0 {
		t.Fatal("expected validation problems, got none")
	}
	found := false
	for _, p := range problems {
		if p == "authentication required: provide --token or --password" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected auth error in problems, got: %v", problems)
	}
}

func TestValidate_InvalidThinking(t *testing.T) {
	cfg := &Config{
		URL:      "wss://example.com",
		Token:    "tok",
		Thinking: "turbo",
	}
	problems := cfg.Validate()
	if len(problems) == 0 {
		t.Fatal("expected validation problems, got none")
	}
	found := false
	for _, p := range problems {
		if p == "invalid thinking level: turbo" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected thinking level error in problems, got: %v", problems)
	}
}

func TestValidate_Valid(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "token auth with ws",
			cfg: &Config{
				URL:          "ws://localhost:3000",
				Token:        "valid-token",
				Thinking:     "off",
				TimeoutMs:    5000,
				HistoryLimit: 100,
			},
		},
		{
			name: "password auth with wss",
			cfg: &Config{
				URL:          "wss://example.com",
				Password:     "secret",
				Thinking:     "high",
				TimeoutMs:    0,
				HistoryLimit: 0,
			},
		},
		{
			name: "empty thinking string",
			cfg: &Config{
				URL:      "ws://localhost",
				Token:    "tok",
				Thinking: "",
			},
		},
		{
			name: "minimal thinking level",
			cfg: &Config{
				URL:      "wss://example.com",
				Token:    "tok",
				Thinking: "minimal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := tt.cfg.Validate()
			if len(problems) != 0 {
				t.Errorf("expected no validation problems, got: %v", problems)
			}
		})
	}
}

func TestValidate_NegativeTimeoutMs(t *testing.T) {
	cfg := &Config{
		URL:       "wss://example.com",
		Token:     "tok",
		TimeoutMs: -1,
	}
	problems := cfg.Validate()
	found := false
	for _, p := range problems {
		if p == "timeout_ms must be non-negative" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout_ms error, got: %v", problems)
	}
}

func TestValidate_NegativeHistoryLimit(t *testing.T) {
	cfg := &Config{
		URL:          "wss://example.com",
		Token:        "tok",
		HistoryLimit: -1,
	}
	problems := cfg.Validate()
	found := false
	for _, p := range problems {
		if p == "history_limit must be non-negative" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected history_limit error, got: %v", problems)
	}
}

func TestCheckFilePermissions(t *testing.T) {
	// Create a temp file with world-readable permissions (0644)
	tmp := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmp, []byte("url: ws://localhost\n"), 0644); err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}

	warnings := CheckFilePermissions(tmp)
	if len(warnings) == 0 {
		t.Fatal("expected permission warning for 0644 file, got none")
	}
	found := false
	for _, w := range warnings {
		if len(w) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning message, got: %v", warnings)
	}
}

func TestCheckFilePermissions_SecureFile(t *testing.T) {
	// Create a temp file with secure permissions (0600)
	tmp := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmp, []byte("url: ws://localhost\n"), 0600); err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}

	warnings := CheckFilePermissions(tmp)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for 0600 file, got: %v", warnings)
	}
}

func TestCheckFilePermissions_NonExistentFile(t *testing.T) {
	warnings := CheckFilePermissions("/nonexistent/path/config.yaml")
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for non-existent file, got: %v", warnings)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Point XDG_CONFIG_HOME at an empty temp dir so no config file exists.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	v := viper.New()
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Agent != "main" {
		t.Errorf("Agent: got %q, want %q", cfg.Agent, "main")
	}
	if cfg.Session != "main" {
		t.Errorf("Session: got %q, want %q", cfg.Session, "main")
	}
	if cfg.HistoryLimit != 200 {
		t.Errorf("HistoryLimit: got %d, want 200", cfg.HistoryLimit)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "warn")
	}
	if cfg.Thinking != "off" {
		t.Errorf("Thinking: got %q, want %q", cfg.Thinking, "off")
	}
}

func TestLoad_FromFile(t *testing.T) {
	content := `url: ws://localhost:3000
token: file-token
agent: custom-agent
session: custom-session
history_limit: 50
log_level: debug
thinking: low
`
	// Place config file where Load() will look: XDG_CONFIG_HOME/talons/config.yaml
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	writeConfig(t, tmpDir, content)

	v := viper.New()
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.URL != "ws://localhost:3000" {
		t.Errorf("URL: got %q, want %q", cfg.URL, "ws://localhost:3000")
	}
	if cfg.Token != "file-token" {
		t.Errorf("Token: got %q, want %q", cfg.Token, "file-token")
	}
	if cfg.Agent != "custom-agent" {
		t.Errorf("Agent: got %q, want %q", cfg.Agent, "custom-agent")
	}
	if cfg.HistoryLimit != 50 {
		t.Errorf("HistoryLimit: got %d, want 50", cfg.HistoryLimit)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.Thinking != "low" {
		t.Errorf("Thinking: got %q, want %q", cfg.Thinking, "low")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	// Use an empty config dir so only env vars are set.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("TALONS_URL", "wss://env-host:443")
	t.Setenv("TALONS_TOKEN", "env-token")

	v := viper.New()
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.URL != "wss://env-host:443" {
		t.Errorf("URL from env: got %q, want %q", cfg.URL, "wss://env-host:443")
	}
	if cfg.Token != "env-token" {
		t.Errorf("Token from env: got %q, want %q", cfg.Token, "env-token")
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	content := `url: ws://file-host:3000
token: file-token
`
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	writeConfig(t, tmpDir, content)

	// Env overrides the file value.
	t.Setenv("TALONS_TOKEN", "env-token-override")

	v := viper.New()
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.URL != "ws://file-host:3000" {
		t.Errorf("URL: got %q, want %q (from file)", cfg.URL, "ws://file-host:3000")
	}
	if cfg.Token != "env-token-override" {
		t.Errorf("Token: got %q, want %q (env should override file)", cfg.Token, "env-token-override")
	}
}
