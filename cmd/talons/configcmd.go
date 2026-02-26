package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rufinus/talons-console/internal/config"
)

var configTestCmd = &cobra.Command{
	Use:   "config-test",
	Short: "Validate talons configuration",
	Long:  "Load and validate talons configuration, displaying resolved settings.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use the globalConfig already populated by PersistentPreRunE (loadConfig).
		// This ensures --config and other flag overrides are reflected here.
		cfg := globalConfig
		if cfg == nil {
			return fmt.Errorf("internal error: config not loaded")
		}

		// Run non-gateway validation; report warnings but do not exit non-zero.
		problems := cfg.Validate()
		if len(problems) > 0 {
			fmt.Fprintln(os.Stderr, "Configuration warnings:")
			for _, p := range problems {
				fmt.Fprintf(os.Stderr, "  - %s\n", p)
			}
		}

		// Report gateway validation warnings (informational only — not an error).
		if err := cfg.ValidateGateway(); err != nil {
			fmt.Fprintf(os.Stderr, "Gateway not configured: %v\n", err)
		}

		// Display resolved configuration
		fmt.Println("Configuration loaded successfully.")
		fmt.Printf("  URL:         %s\n", cfg.URL)
		fmt.Printf("  Agent:       %s\n", cfg.Agent)
		fmt.Printf("  Session:     %s\n", cfg.Session)
		fmt.Printf("  Thinking:    %s\n", cfg.Thinking)
		fmt.Printf("  Timeout:     %d ms\n", cfg.TimeoutMs)
		fmt.Printf("  History:     %d\n", cfg.HistoryLimit)
		fmt.Printf("  Log Level:   %s\n", cfg.LogLevel)

		// Check for concurrent sessions
		warning := config.CheckConcurrentSession(cfg.Agent, cfg.Session)
		if warning != "" {
			fmt.Printf("\nWarning: %s\n", warning)
		}

		return nil
	},
}
