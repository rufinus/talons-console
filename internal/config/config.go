package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all talons configuration.
type Config struct {
	URL          string `mapstructure:"url"`
	Token        string `mapstructure:"token"`
	Password     string `mapstructure:"password"`
	Agent        string `mapstructure:"agent"`
	Session      string `mapstructure:"session"`
	Deliver      bool   `mapstructure:"deliver"`
	Thinking     string `mapstructure:"thinking"`
	TimeoutMs    int    `mapstructure:"timeout_ms"`
	HistoryLimit int    `mapstructure:"history_limit"`
	LogLevel     string `mapstructure:"log_level"`
	LogFile      string `mapstructure:"log_file"`
}

// configKeys lists all known config keys (for explicit env binding).
var configKeys = []string{
	"url", "token", "password", "agent", "session",
	"deliver", "thinking", "timeout_ms", "history_limit",
	"log_level", "log_file",
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Agent:        "main",
		Session:      "main",
		HistoryLimit: 200,
		LogLevel:     "warn",
		Thinking:     "off",
	}
}

// configFilePath returns the path to the talons config file.
// Prefers os.UserConfigDir(); falls back to ~/.config/talons/config.yaml.
func configFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to manual ~/.config
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "talons", "config.yaml")
}

// Load reads configuration with precedence: defaults < file < env < flags.
// Cobra flag bindings must be set up before calling Load.
func Load(v *viper.Viper) (*Config, error) {
	// 1. Defaults
	v.SetDefault("agent", "main")
	v.SetDefault("session", "main")
	v.SetDefault("history_limit", 200)
	v.SetDefault("log_level", "warn")
	v.SetDefault("thinking", "off")

	// 2. Config file: use the default path if none was pre-configured.
	cfgPath := configFilePath()
	v.SetConfigFile(cfgPath)
	if err := v.ReadInConfig(); err != nil {
		// Ignore "file not found" — config file is optional.
		var configFileNotFoundErr viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundErr) && !os.IsNotExist(err) {
			// Only return real parse/read errors.
			return nil, fmt.Errorf("reading config file %s: %w", cfgPath, err)
		}
	}

	// 3. Environment variables.
	v.SetEnvPrefix("TALONS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	// Explicit BindEnv calls are required for Unmarshal to honour AutomaticEnv
	// (viper does not auto-map env vars to struct fields without them).
	for _, key := range configKeys {
		_ = v.BindEnv(key)
	}

	// 4. Unmarshal (Cobra flags already bound before this call)
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return cfg, nil
}

// Validate returns a list of validation problems for non-gateway fields.
// Empty slice = valid. Gateway credentials (URL, token, password) are NOT
// checked here — use ValidateGateway() for those.
func (c *Config) Validate() []string {
	var problems []string

	// Thinking level check
	validThinking := map[string]bool{
		"":        true,
		"off":     true,
		"minimal": true,
		"low":     true,
		"medium":  true,
		"high":    true,
	}
	if !validThinking[c.Thinking] {
		problems = append(problems, fmt.Sprintf("invalid thinking level: %s", c.Thinking))
	}

	// TimeoutMs check
	if c.TimeoutMs < 0 {
		problems = append(problems, "timeout_ms must be non-negative")
	}

	// HistoryLimit check
	if c.HistoryLimit < 0 {
		problems = append(problems, "history_limit must be non-negative")
	}

	return problems
}

// ValidateGateway validates Gateway connection fields (URL and authentication).
// Returns a descriptive error if any required gateway field is missing or invalid.
func (c *Config) ValidateGateway() error {
	// 1. Presence check
	if c.URL == "" {
		return errors.New("gateway URL is required")
	}

	// 2. Parse the URL
	u, parseErr := url.Parse(c.URL)
	if parseErr != nil {
		var urlErr *url.Error
		if errors.As(parseErr, &urlErr) && strings.Contains(urlErr.Error(), "invalid port") {
			port := rawPort(c.URL)
			return fmt.Errorf("invalid port in gateway URL: %q is not a number", port)
		}
		return fmt.Errorf("invalid gateway URL: %s \u2014 %s", c.URL, parseErr)
	}

	// 3. Scheme check
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("invalid URL scheme %q. Use ws:// or wss://", u.Scheme)
	}

	// 4. Hostname check
	if u.Hostname() == "" {
		return errors.New("gateway URL must include a hostname (e.g., wss://gateway.example.com)")
	}

	// 5. Port range check (port is optional; only validate when present)
	if portStr := u.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port in gateway URL: %q is not a number", portStr)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid port in gateway URL: %d is out of range (1-65535)", port)
		}
	}

	// 6. Authentication check
	if c.Token == "" && c.Password == "" {
		return errors.New("gateway authentication required: provide --token or --password (or set in config file)")
	}

	return nil
}

// rawPort attempts to extract the port string from a raw URL string for
// use in error messages when net/url.Parse itself rejects the port.
func rawPort(rawURL string) string {
	// Find the authority (host[:port]) portion after "://"
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// Strip path/query/fragment
	for _, sep := range []string{"/", "?", "#"} {
		if i := strings.Index(s, sep); i >= 0 {
			s = s[:i]
		}
	}
	// Strip userinfo
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	// Handle IPv6 literal
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]"); i >= 0 {
			s = s[i+1:]
			if strings.HasPrefix(s, ":") {
				return s[1:]
			}
		}
		return ""
	}
	// Regular host:port
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[i+1:]
	}
	return ""
}

// CheckFilePermissions returns warnings for unsafe config file permissions.
func CheckFilePermissions(path string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	mode := info.Mode()
	var warnings []string
	if mode&0044 != 0 {
		warnings = append(warnings, fmt.Sprintf("config file %s is world-readable, credentials may be exposed", path))
	}
	return warnings
}
