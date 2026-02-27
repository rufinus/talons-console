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

// Validate() must NOT fail when URL/token are absent — those are gateway-only fields.
func TestValidate_EmptyGatewayFields_NoProblems(t *testing.T) {
	cfg := &Config{
		URL:      "",
		Token:    "",
		Password: "",
		Thinking: "off",
	}
	problems := cfg.Validate()
	if len(problems) != 0 {
		t.Errorf("Validate() must not fail on empty gateway fields, got: %v", problems)
	}
}

// ValidateGateway tests

func TestValidateGateway_MissingURL(t *testing.T) {
	cfg := &Config{
		URL:   "",
		Token: "some-token",
	}
	err := cfg.ValidateGateway()
	if err == nil {
		t.Fatal("expected error for missing URL, got nil")
	}
	if !containsSubstr(err.Error(), "gateway URL is required") {
		t.Errorf("expected 'Gateway URL is required' in error, got: %v", err)
	}
}

func TestValidateGateway_InvalidScheme(t *testing.T) {
	cfg := &Config{
		URL:   "http://localhost:8080",
		Token: "some-token",
	}
	err := cfg.ValidateGateway()
	if err == nil {
		t.Fatal("expected error for invalid scheme, got nil")
	}
	if !containsSubstr(err.Error(), "ws://") {
		t.Errorf("expected scheme hint in error, got: %v", err)
	}
}

func TestValidateGateway_MissingAuth(t *testing.T) {
	cfg := &Config{
		URL:      "wss://example.com",
		Token:    "",
		Password: "",
	}
	err := cfg.ValidateGateway()
	if err == nil {
		t.Fatal("expected error for missing auth, got nil")
	}
	if !containsSubstr(err.Error(), "authentication required") { //nolint:gocritic
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestValidateGateway_Valid(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{"token+ws", &Config{URL: "ws://localhost:3000", Token: "tok"}},
		{"password+wss", &Config{URL: "wss://example.com", Password: "secret"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.ValidateGateway(); err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
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
			name: "all fields valid",
			cfg: &Config{
				Thinking:     "off",
				TimeoutMs:    5000,
				HistoryLimit: 100,
			},
		},
		{
			name: "empty thinking string",
			cfg: &Config{
				Thinking: "",
			},
		},
		{
			name: "minimal thinking level",
			cfg: &Config{
				Thinking: "minimal",
			},
		},
		{
			name: "high thinking level",
			cfg: &Config{
				Thinking:  "high",
				TimeoutMs: 0,
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

func TestValidateGateway_URLValidation(t *testing.T) {
	type wantKind int
	const (
		wantValid wantKind = iota
		wantErrContains
	)
	tests := []struct {
		name    string
		url     string
		kind    wantKind
		errSubs string
	}{
		{name: "empty URL", url: "", kind: wantErrContains, errSubs: "gateway URL is required"},
		{name: "ws localhost", url: "ws://localhost:8080", kind: wantValid},
		{name: "wss no port", url: "wss://gw.example.com", kind: wantValid},
		{name: "wss with port", url: "wss://gw.example.com:443", kind: wantValid},
		{name: "http rejected", url: "http://example.com", kind: wantErrContains, errSubs: "scheme"},
		{name: "https rejected", url: "https://example.com", kind: wantErrContains, errSubs: "scheme"},
		{name: "ws no host", url: "ws://", kind: wantErrContains, errSubs: "hostname"},
		{name: "wss port no host", url: "wss://:8080", kind: wantErrContains, errSubs: "hostname"},
		{name: "non-numeric port", url: "wss://host:abc", kind: wantErrContains, errSubs: "not a number"},
		{name: "port 99999 out of range", url: "wss://host:99999", kind: wantErrContains, errSubs: "out of range"},
		{name: "port 0 out of range", url: "wss://host:0", kind: wantErrContains, errSubs: "out of range"},
		{name: "port 65535 valid boundary", url: "wss://host:65535", kind: wantValid},
		{name: "port 1 valid boundary", url: "wss://host:1", kind: wantValid},
		{name: "broken URL", url: "://broken", kind: wantErrContains, errSubs: "invalid gateway URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{URL: tt.url, Token: "tok"}
			err := cfg.ValidateGateway()
			if tt.kind == wantValid {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSubs)
			}
			if !containsSubstr(err.Error(), tt.errSubs) {
				t.Errorf("expected error to contain %q, got: %v", tt.errSubs, err)
			}
		})
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
