package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rufinus/talons-console/internal/config"
)

var configTestCmd = &cobra.Command{
	Use:   "config-test",
	Short: "Validate talons configuration",
	Long:  "Load and validate talons configuration, displaying resolved settings.",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.New()
		cfg, err := config.Load(v)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		problems := cfg.Validate()
		if len(problems) > 0 {
			fmt.Fprintln(os.Stderr, "Configuration errors:")
			for _, p := range problems {
				fmt.Fprintf(os.Stderr, "  - %s\n", p)
			}
			os.Exit(1)
		}

		// Display resolved configuration
		fmt.Println("Configuration valid!")
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
