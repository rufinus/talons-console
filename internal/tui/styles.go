package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha colour palette — consistent with OpenClaw's own TUI.
var (
	colorBase    = lipgloss.Color("#1e1e2e") // background
	colorText    = lipgloss.Color("#cdd6f4") // text
	colorGreen   = lipgloss.Color("#a6e3a1")
	colorBlue    = lipgloss.Color("#89b4fa")
	colorYellow  = lipgloss.Color("#f9e2af")
	colorRed     = lipgloss.Color("#f38ba8") //nolint:unused
	colorMauve   = lipgloss.Color("#cba6f7")
	colorOverlay = lipgloss.Color("#313244") // subtle background
	colorMuted   = lipgloss.Color("#585b70")
	colorPeach   = lipgloss.Color("#fab387") //nolint:unused
)

// ── Header ────────────────────────────────────────────────────────────────────

var headerStyle = lipgloss.NewStyle().
	Background(colorBase).
	Foreground(colorText).
	Padding(0, 1)

var (
	headerConnectedStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)
	headerConnectingStyle = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true)
	headerDisconnectedStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Bold(true)
)

// ── Messages ──────────────────────────────────────────────────────────────────

var (
	userRoleStyle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)
	assistantRoleStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)
	toolRoleStyle = lipgloss.NewStyle().
			Foreground(colorMauve).
			Bold(true)

	timestampStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	systemDividerStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)

	viewportStyle = lipgloss.NewStyle()
	spinnerStyle  = lipgloss.NewStyle().Foreground(colorYellow)

	// Legacy per-role body styles (kept for renderMessage fallback)
	userMessageStyle      = lipgloss.NewStyle().Foreground(colorBlue)
	assistantMessageStyle = lipgloss.NewStyle().Foreground(colorGreen)
	toolMessageStyle      = lipgloss.NewStyle().Foreground(colorMauve)
	defaultMessageStyle   = lipgloss.NewStyle()
)

// ── Input ─────────────────────────────────────────────────────────────────────

var (
	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorMuted).
				Padding(0, 1)

	statusLineStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	// overlayStyle is used for the input border when focused
	overlayStyle = lipgloss.NewStyle().
			Background(colorOverlay)
)

// overlayStyle is referenced in input.go to prevent import cycles.
var _ = overlayStyle
