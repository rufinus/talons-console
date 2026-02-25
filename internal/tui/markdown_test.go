package tui

import (
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// RenderMarkdown tests
// ---------------------------------------------------------------------------

func TestRenderMarkdown_Bold(t *testing.T) {
	out := RenderMarkdown("**bold**", 80)
	if !strings.Contains(out, "bold") {
		t.Errorf("expected output to contain 'bold', got: %q", out)
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	input := "```\nhello world\n```"
	out := RenderMarkdown(input, 80)
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected output to contain 'hello world', got: %q", out)
	}
}

func TestRenderMarkdown_Fallback(t *testing.T) {
	raw := "# raw content that should come back unchanged"
	out := renderMarkdownWith(nil, errors.New("simulated renderer error"), raw)
	if out != raw {
		t.Errorf("expected fallback to return raw content, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// DetectTerminal tests
// ---------------------------------------------------------------------------

func TestDetectTerminal_Defaults(t *testing.T) {
	// Clear colour-related env so we get a known baseline.
	t.Setenv("COLORTERM", "")
	t.Setenv("TERM", "")

	caps := DetectTerminal()
	if caps.Width <= 0 {
		t.Errorf("expected Width > 0, got %d", caps.Width)
	}
	if caps.Height <= 0 {
		t.Errorf("expected Height > 0, got %d", caps.Height)
	}
}

func TestDetectTerminal_TrueColor(t *testing.T) {
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("TERM", "")

	caps := DetectTerminal()
	if caps.ColorMode != ColorTrueColor {
		t.Errorf("expected ColorTrueColor (%d), got %d", ColorTrueColor, caps.ColorMode)
	}
}

func TestDetectTerminal_256Color(t *testing.T) {
	t.Setenv("COLORTERM", "")
	t.Setenv("TERM", "xterm-256color")

	caps := DetectTerminal()
	if caps.ColorMode != Color256 {
		t.Errorf("expected Color256 (%d), got %d", Color256, caps.ColorMode)
	}
}
