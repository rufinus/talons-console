package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Validate returns a list of validation problems. Empty = valid.
func (c *Config) Validate() []string {
	var problems []string

	// URL checks
	if c.URL == "" {
		problems = append(problems, "url is required")
	} else if !strings.HasPrefix(c.URL, "ws://") && !strings.HasPrefix(c.URL, "wss://") {
		problems = append(problems, "url must use ws:// or wss:// scheme")
	}

	// Authentication check
	if c.Token == "" && c.Password == "" {
		problems = append(problems, "authentication required: provide --token or --password")
	}

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
