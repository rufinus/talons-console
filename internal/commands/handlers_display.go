package commands

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleHelp implements the /help command.
// With no args it prints a categorized command list; with an arg it prints
// detailed help for that specific command.
func handleHelp(ctx HandlerContext, args []string) tea.Cmd {
	if len(args) == 0 {
		ctx.AppendSystemMessage(buildHelpList())
		return nil
	}

	// Strip optional leading slash from the argument.
	name := strings.TrimPrefix(args[0], "/")
	def, ok := DefaultRegistry.Get(name)
	if !ok {
		ctx.AppendSystemMessage(fmt.Sprintf("Unknown command: /%s. Type /help for available commands.", name))
		return nil
	}

	ctx.AppendSystemMessage(buildHelpDetail(def))
	return nil
}

// buildHelpList returns a categorized command listing that fits within 80 cols.
func buildHelpList() string {
	// Collect commands by category.
	categories := []string{"Session Control", "Display", "System"}
	byCategory := make(map[string][]*CommandDef)
	for _, def := range DefaultRegistry.All() {
		byCategory[def.Category] = append(byCategory[def.Category], def)
	}

	var sb strings.Builder
	sb.WriteString("Available commands:\n")

	for _, cat := range categories {
		defs, ok := byCategory[cat]
		if !ok || len(defs) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n  %s\n", cat)
		for _, def := range defs {
			// Format: "  /name [aliases]  — description"
			nameField := "/" + def.Name
			if len(def.Aliases) > 0 {
				aliasStrs := make([]string, len(def.Aliases))
				for i, a := range def.Aliases {
					aliasStrs[i] = "/" + a
				}
				nameField += " (" + strings.Join(aliasStrs, ", ") + ")"
			}
			desc := def.Description
			maxDesc := 80 - 4 - 22 - 1 // 4 indent, 22 name field, 1 space
			if len([]rune(desc)) > maxDesc {
				desc = string([]rune(desc)[:maxDesc-1]) + "…"
			}
			line := fmt.Sprintf("    %-22s %s", nameField, desc)
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString("\nType /help <command> for detailed usage.")
	return sb.String()
}

// buildHelpDetail returns a detailed help string for one command.
func buildHelpDetail(def *CommandDef) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "/%s — %s\n", def.Name, def.Description)
	if def.LongDesc != "" {
		sb.WriteString("\n" + def.LongDesc + "\n")
	}
	fmt.Fprintf(&sb, "\nUsage: %s\n", def.Usage)
	if len(def.Aliases) > 0 {
		aliasStrs := make([]string, len(def.Aliases))
		for i, a := range def.Aliases {
			aliasStrs[i] = "/" + a
		}
		sb.WriteString("Aliases: " + strings.Join(aliasStrs, ", ") + "\n")
	}
	if len(def.Examples) > 0 {
		sb.WriteString("\nExamples:\n")
		for _, ex := range def.Examples {
			sb.WriteString("  " + ex + "\n")
		}
	}
	if len(def.Related) > 0 {
		relStrs := make([]string, len(def.Related))
		for i, r := range def.Related {
			relStrs[i] = "/" + r
		}
		sb.WriteString("\nSee also: " + strings.Join(relStrs, ", "))
	}
	return sb.String()
}

// handleStatus implements the /status command.
func handleStatus(ctx HandlerContext, args []string) tea.Cmd {
	model := ctx.GetModel()
	if model == "" {
		model = "(default)"
	}
	thinking := ctx.GetThinking()
	if thinking == "" {
		thinking = "(default)"
	}
	session := ctx.GetSession()
	if session == "" {
		session = "(none)"
	}
	version := ctx.GetVersion()
	if version == "" {
		version = "unknown"
	}

	uptime := ctx.GetUptime()
	var uptimeStr string
	if uptime == 0 {
		uptimeStr = "not connected"
	} else {
		uptimeStr = formatDuration(uptime)
	}

	connected := "false"
	if ctx.IsConnected() {
		connected = "true"
	}
	_ = connected // connection shown via uptime / gateway URL context

	width := 60 // default panel width
	if ctx.GetWidth() > 20 {
		w := ctx.GetWidth()
		if w < 80 {
			width = w - 2
		} else {
			width = 78
		}
	}

	// Build the panel.
	var sb strings.Builder
	top := "┌" + strings.Repeat("─", width) + "┐"
	bot := "└" + strings.Repeat("─", width) + "┘"
	row := func(label, value string) string {
		content := fmt.Sprintf(" %-24s %s", label, value)
		// Pad to width.
		pad := width - len([]rune(content))
		if pad > 0 {
			content += strings.Repeat(" ", pad)
		} else if pad < 0 {
			// Truncate value to fit.
			maxVal := width - 26 // 1 + 24 + 1
			if maxVal < 1 {
				maxVal = 1
			}
			runes := []rune(value)
			if len(runes) > maxVal {
				value = string(runes[:maxVal-1]) + "…"
			}
			content = fmt.Sprintf(" %-24s %s", label, value)
			pad2 := width - len([]rune(content))
			if pad2 > 0 {
				content += strings.Repeat(" ", pad2)
			}
		}
		return "│" + content + "│"
	}

	sb.WriteString(top + "\n")
	sb.WriteString(row("Agent:", ctx.GetAgent()) + "\n")
	sb.WriteString(row("Session:", session) + "\n")
	sb.WriteString(row("Model:", model) + "\n")
	sb.WriteString(row("Thinking:", thinking) + "\n")
	sb.WriteString(row("Timeout:", fmt.Sprintf("%dms", ctx.GetTimeoutMs())) + "\n")
	sb.WriteString(row("Gateway URL:", ctx.GetGatewayURL()) + "\n")
	sb.WriteString(row("Version:", version) + "\n")
	sb.WriteString(row("Uptime:", uptimeStr) + "\n")
	sb.WriteString(row("Messages sent:", fmt.Sprintf("%d", ctx.GetMsgSent())) + "\n")
	sb.WriteString(row("Messages received:", fmt.Sprintf("%d", ctx.GetMsgRecv())) + "\n")
	sb.WriteString(bot)

	ctx.AppendSystemMessage(sb.String())
	return nil
}

// formatDuration formats a duration as human-readable: Xs, Xm Ys, Xh Ym Zs.
func formatDuration(d time.Duration) string {
	totalSec := int(d.Seconds())
	if totalSec < 0 {
		totalSec = 0
	}
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// handleClear implements the /clear command.
func handleClear(ctx HandlerContext, args []string) tea.Cmd {
	ctx.ClearMessages()
	ctx.AppendSystemMessage("Messages cleared")
	return nil
}

// handleExit implements the /exit and /quit commands.
func handleExit(ctx HandlerContext, args []string) tea.Cmd {
	_ = ctx.CloseGateway()
	return tea.Quit
}

// handleHistory implements the /history stub.
func handleHistory(ctx HandlerContext, args []string) tea.Cmd {
	ctx.AppendSystemMessage("Coming in v0.3")
	return nil
}

func init() {
	// Wire up handlers after DefaultRegistry is created by InitCommands.
	// We use a separate init to avoid circular init ordering issues.
}

// WireDisplayHandlers assigns the display/system handlers to the default registry.
// This is called by InitCommands or by tests after InitCommands.
func WireDisplayHandlers(r *CommandRegistry) {
	names := []string{"help", "status", "clear", "exit", "history"}
	handlers := map[string]HandlerFunc{
		"help":    handleHelp,
		"status":  handleStatus,
		"clear":   handleClear,
		"exit":    handleExit,
		"history": handleHistory,
	}
	for _, name := range names {
		def, ok := r.Get(name)
		if !ok {
			continue
		}
		def.Handler = handlers[name]
		// Also wire alias (quit → exit).
		if name == "exit" {
			for _, alias := range def.Aliases {
				if ad, ok2 := r.Get(alias); ok2 {
					ad.Handler = handlers["exit"]
				}
			}
		}
	}
}
