// Package main provides the CLI entry point for talons-console.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rufinus/talons-console/internal/config"
	"github.com/rufinus/talons-console/internal/gateway"
	"github.com/rufinus/talons-console/internal/tui"
	"github.com/rufinus/talons-console/internal/version"
)

// Global flags
var (
	flagURL          string
	flagToken        string
	flagPassword     string
	flagAgent        string
	flagSession      string
	flagDeliver      bool
	flagThinking     string
	flagTimeoutMs    int
	flagHistoryLimit int
	flagLogLevel     string
	flagLogFile      string
	flagMessage      string
)

// globalConfig holds the parsed configuration set by PersistentPreRunE.
// All subcommands may read from this after the root pre-run hook executes.
var globalConfig *config.Config

// Root command
var rootCmd = &cobra.Command{
	Use:   "talons",
	Short: "talons — lightweight TUI client for OpenClaw Gateway",
	Long: `talons is a single-binary TUI client for OpenClaw Gateway.
Connect to any OpenClaw Gateway via WebSocket and interact with your agents
from any terminal.`,
	Version: version.Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return loadConfig(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Non-interactive mode
		if flagMessage != "" {
			return runNonInteractive(flagMessage)
		}
		// Interactive TUI mode
		return runInteractive()
	},
}

// chatCmd is the explicit chat subcommand (requires gateway credentials).
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Connect to the Gateway and start a TUI chat session",
	Long:  "Open an interactive TUI session with the OpenClaw Gateway.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireGatewayConfig()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagMessage != "" {
			return runNonInteractive(flagMessage)
		}
		return runInteractive()
	},
}

func init() {
	// Add global flags
	rootCmd.PersistentFlags().StringVar(&flagURL, "url", "", "Gateway WebSocket URL (wss://...)")
	rootCmd.PersistentFlags().StringVar(&flagToken, "token", "", "Authentication token")
	rootCmd.PersistentFlags().StringVar(&flagPassword, "password", "", "Authentication password")
	rootCmd.PersistentFlags().StringVarP(&flagAgent, "agent", "a", "main", "Agent name")
	rootCmd.PersistentFlags().StringVarP(&flagSession, "session", "s", "main", "Session key")
	rootCmd.PersistentFlags().BoolVar(&flagDeliver, "deliver", false, "Enable delivery receipts")
	rootCmd.PersistentFlags().StringVar(&flagThinking, "thinking", "off",
		"Thinking level (off, minimal, low, medium, high)")
	rootCmd.PersistentFlags().IntVar(&flagTimeoutMs, "timeout-ms", 60000, "Agent timeout in milliseconds")
	rootCmd.PersistentFlags().IntVar(&flagHistoryLimit, "history-limit", 200, "Number of history messages to fetch")
	rootCmd.PersistentFlags().StringVar(&flagLogLevel, "log-level", "warn", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&flagLogFile, "log-file", "", "Log file path (default: stdout)")

	// Non-interactive message flag
	rootCmd.Flags().StringVarP(&flagMessage, "message", "m", "", "Send message non-interactively (implies --no-tui)")
	chatCmd.Flags().StringVarP(&flagMessage, "message", "m", "", "Send message non-interactively (implies --no-tui)")

	// Add subcommands
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(configTestCmd)
	rootCmd.AddCommand(updateCheckCmd)
}

// loadConfig reads and parses the config file (safe for all commands).
// It does NOT validate Gateway credentials — use requireGatewayConfig() for that.
// On success it sets globalConfig so all subcommands can access parsed settings.
func loadConfig(cmd *cobra.Command) error {
	v := viper.New()

	// Bind flags to viper
	if err := v.BindPFlags(cmd.Flags()); err != nil {
		return fmt.Errorf("binding flags: %w", err)
	}

	// Set environment variable prefixes
	v.SetEnvPrefix("TALONS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Load configuration (file parse errors are returned; missing gateway fields are NOT)
	cfg, err := config.Load(v)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Validate non-gateway fields only
	problems := cfg.Validate()
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", p)
		}
		return fmt.Errorf("configuration validation failed")
	}

	globalConfig = cfg
	return nil
}

// requireGatewayConfig validates that Gateway URL and authentication are present.
// Call this from subcommand PreRunE hooks that need a Gateway connection.
func requireGatewayConfig() error {
	if globalConfig == nil {
		return fmt.Errorf("internal error: config not loaded (loadConfig must run first)")
	}
	if err := globalConfig.ValidateGateway(); err != nil {
		return err
	}

	// Warn about concurrent sessions
	warning := config.CheckConcurrentSession(globalConfig.Agent, globalConfig.Session)
	if warning != "" {
		fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Render(warning))
	}

	return nil
}

// runInteractive starts the TUI.
func runInteractive() error {
	cfg := globalConfig

	// Write PID file for session detection
	cleanup := config.WritePIDFile(cfg.Agent, cfg.Session, cfg.URL)
	defer cleanup()

	// Create Gateway client
	client := gateway.NewClient(gateway.ClientConfig{
		URL:          cfg.URL,
		Token:        cfg.Token,
		Password:     cfg.Password,
		Agent:        cfg.Agent,
		Session:      cfg.Session,
		HistoryLimit: cfg.HistoryLimit,
	})

	// Connect with context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Connect
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	// Create and run TUI
	m := tui.NewModel(client, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Handle quit signal from TUI
	go func() {
		<-sigCh
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Cleanup
	return client.Close()
}

// runNonInteractive sends a message and streams the response.
func runNonInteractive(message string) error {
	cfg := globalConfig

	client := gateway.NewClient(gateway.ClientConfig{
		URL:      cfg.URL,
		Token:    cfg.Token,
		Password: cfg.Password,
		Agent:    cfg.Agent,
		Session:  cfg.Session,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutMs)*time.Millisecond)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	// Send message
	if err := client.Send(gateway.OutboundMessage{
		Type: "chat.send",
		Payload: gateway.ChatSendParams{
			Content:    message,
			SessionKey: cfg.Session,
			AgentID:    cfg.Agent,
			Thinking:   cfg.Thinking,
		},
	}); err != nil {
		return fmt.Errorf("send: %w", err)
	}

	// Stream response
	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return ctx.Err()
		case event, ok := <-client.Messages():
			if !ok {
				return nil
			}
			switch event.Kind {
			case gateway.KindToken:
				fmt.Print(event.Content)
			case gateway.KindMessage:
				fmt.Println(event.Content)
			case gateway.KindError:
				fmt.Fprintf(os.Stderr, "Error: %s\n", event.Error)
				os.Exit(1)
			}
		}
	}
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
