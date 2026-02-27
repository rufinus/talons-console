package commands

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// modelPattern validates model ID characters. Compiled once at package level.
var modelPattern = regexp.MustCompile(`^[a-zA-Z0-9\-_./]+$`)

// validThinkingLevels is the single source of truth for accepted thinking levels.
var validThinkingLevels = map[string]struct{}{
	"off":     {},
	"minimal": {},
	"low":     {},
	"medium":  {},
	"high":    {},
}

// CmdError formats a command error message in the standard warning format.
func CmdError(cmd, msg string) string {
	return fmt.Sprintf("⚠ /%s: %s", cmd, msg)
}

// CmdErrorWithUsage formats an error with an appended usage hint.
func CmdErrorWithUsage(cmd, msg, usage string) string {
	return fmt.Sprintf("⚠ /%s: %s\nUsage: %s", cmd, msg, usage)
}

// HandleModel implements the /model command.
func HandleModel(ctx HandlerContext, args []string) tea.Cmd {
	if len(args) == 0 {
		cur := ctx.GetModel()
		display := cur
		if display == "" {
			display = "(default)"
		}
		ctx.AppendSystemMessage(fmt.Sprintf("Current model: %s\nUsage: /model <model-id>", display))
		return nil
	}

	id := args[0]
	// Strip a leading slash if provided.
	id = strings.TrimPrefix(id, "/")

	const invalidCharsMsg = "model ID may only contain alphanumeric characters, hyphens, underscores, dots, and slashes"

	if id == "" {
		ctx.AppendSystemMessage(CmdError("model", invalidCharsMsg))
		return nil
	}

	if len(id) > 256 {
		ctx.AppendSystemMessage(CmdError("model", "model ID must be 256 characters or fewer"))
		return nil
	}

	if !modelPattern.MatchString(id) {
		ctx.AppendSystemMessage(CmdError("model", invalidCharsMsg))
		return nil
	}

	if ctx.GetModel() == id {
		ctx.AppendSystemMessage(fmt.Sprintf("Already using model '%s'", id))
		return nil
	}

	return func() tea.Msg {
		if err := ctx.PatchSession(SessionPatch{Model: &id}); err != nil {
			ctx.AppendSystemMessage(CmdError("model", fmt.Sprintf("patch failed: %s", err)))
			return nil
		}
		ctx.SetModel(id)
		ctx.AppendSystemMessage(fmt.Sprintf("Model set to '%s'", id))
		return nil
	}
}

// HandleThinking implements the /thinking command.
func HandleThinking(ctx HandlerContext, args []string) tea.Cmd {
	if len(args) == 0 {
		cur := ctx.GetThinking()
		display := cur
		if display == "" {
			display = "(default)"
		}
		ctx.AppendSystemMessage(fmt.Sprintf("Current thinking: %s\nUsage: /thinking <off|minimal|low|medium|high>", display))
		return nil
	}

	level := strings.ToLower(args[0])

	if _, ok := validThinkingLevels[level]; !ok {
		ctx.AppendSystemMessage(CmdError("thinking", "thinking level must be one of: off, minimal, low, medium, high"))
		return nil
	}

	return func() tea.Msg {
		if err := ctx.PatchSession(SessionPatch{ThinkingLevel: &level}); err != nil {
			ctx.AppendSystemMessage(CmdError("thinking", fmt.Sprintf("patch failed: %s", err)))
			return nil
		}
		ctx.SetThinking(level)
		ctx.AppendSystemMessage(fmt.Sprintf("Thinking set to '%s'", level))
		return nil
	}
}

// HandleTimeout implements the /timeout command.
func HandleTimeout(ctx HandlerContext, args []string) tea.Cmd {
	if len(args) == 0 {
		ctx.AppendSystemMessage(fmt.Sprintf("Current timeout: %dms\nUsage: /timeout <1000-600000>", ctx.GetTimeoutMs()))
		return nil
	}

	ms, err := strconv.Atoi(args[0])
	if err != nil {
		ctx.AppendSystemMessage(CmdError("timeout", "timeout must be an integer (milliseconds)"))
		return nil
	}

	if ms < 1000 || ms > 600000 {
		ctx.AppendSystemMessage(CmdError("timeout", "timeout must be between 1000 and 600000 milliseconds"))
		return nil
	}

	ctx.SetTimeoutMs(ms)
	ctx.AppendSystemMessage(fmt.Sprintf("Timeout set to %dms", ms))
	return nil
}

// WireStateHandlers assigns the model/thinking/timeout handlers to the given registry.
func WireStateHandlers(r *CommandRegistry) {
	names := map[string]HandlerFunc{
		"model":    HandleModel,
		"thinking": HandleThinking,
		"timeout":  HandleTimeout,
	}
	for name, fn := range names {
		if def, ok := r.Get(name); ok {
			def.Handler = fn
		}
	}
}
