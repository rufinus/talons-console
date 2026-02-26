package tui

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// ColorMode represents terminal color support level.
type ColorMode int

const (
	ColorNone    ColorMode = iota
	ColorBasic16           //nolint:deadcode
	Color256
	ColorTrueColor
)

// TerminalCaps describes the capabilities of the current terminal.
type TerminalCaps struct {
	ColorMode ColorMode
	Unicode   bool
	Width     int
	Height    int
}

// DetectTerminal detects current terminal capabilities.
func DetectTerminal() TerminalCaps {
	caps := TerminalCaps{Width: 80, Height: 24} // sensible defaults

	// Color detection
	colorterm := strings.ToLower(os.Getenv("COLORTERM"))
	if colorterm == "truecolor" || colorterm == "24bit" {
		caps.ColorMode = ColorTrueColor
	} else if strings.Contains(os.Getenv("TERM"), "256color") {
		caps.ColorMode = Color256
	} else if os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb" {
		caps.ColorMode = ColorBasic16
	}

	// Unicode detection
	lang := os.Getenv("LANG") + os.Getenv("LC_ALL") + os.Getenv("LC_CTYPE")
	caps.Unicode = strings.Contains(strings.ToUpper(lang), "UTF")

	// Terminal size
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		caps.Width, caps.Height = w, h
	}

	return caps
}
