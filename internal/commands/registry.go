package commands

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// CommandDef holds the metadata and handler for a single slash command.
type CommandDef struct {
	Name        string
	Aliases     []string
	Category    string
	Description string
	LongDesc    string
	Usage       string
	Examples    []string
	Related     []string
	Handler     HandlerFunc
}

// commandPattern matches a slash command at the start of the input.
// Group 1 = command name, Group 2 = remainder (args string).
var commandPattern = regexp.MustCompile(`^\s*/(\w+)(.*)$`)

// CommandRegistry is the central registry for all known slash commands.
type CommandRegistry struct {
	// lookup maps lower-cased primary name and aliases to their CommandDef.
	lookup map[string]*CommandDef
	// ordered preserves registration order for /help display.
	ordered []*CommandDef
}

// NewCommandRegistry creates an empty registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		lookup: make(map[string]*CommandDef),
	}
}

// Register adds a CommandDef to the registry.
// Panics if the primary name or any alias is already registered — this is a
// programming error that must be caught at startup, not at runtime.
func (r *CommandRegistry) Register(def *CommandDef) {
	key := strings.ToLower(def.Name)
	if _, exists := r.lookup[key]; exists {
		panic(fmt.Sprintf("commands: duplicate registration for name %q", def.Name))
	}
	r.lookup[key] = def

	for _, alias := range def.Aliases {
		akey := strings.ToLower(alias)
		if _, exists := r.lookup[akey]; exists {
			panic(fmt.Sprintf("commands: duplicate registration for alias %q (command %q)", alias, def.Name))
		}
		r.lookup[akey] = def
	}

	r.ordered = append(r.ordered, def)
}

// Get looks up a CommandDef by primary name or alias (case-insensitive).
func (r *CommandRegistry) Get(name string) (*CommandDef, bool) {
	def, ok := r.lookup[strings.ToLower(name)]
	return def, ok
}

// All returns the registered commands in registration order.
func (r *CommandRegistry) All() []*CommandDef {
	result := make([]*CommandDef, len(r.ordered))
	copy(result, r.ordered)
	return result
}

// Complete returns a sorted slice of "/name" strings whose primary name starts
// with the given prefix (case-insensitive, prefix may or may not include the
// leading slash). Returns an empty slice when there are no matches.
func (r *CommandRegistry) Complete(prefix string) []string {
	// Normalise: strip leading slash if present, lower-case.
	stripped := strings.TrimPrefix(strings.ToLower(prefix), "/")

	var matches []string
	for _, def := range r.ordered {
		if strings.HasPrefix(strings.ToLower(def.Name), stripped) {
			matches = append(matches, "/"+def.Name)
		}
	}
	sort.Strings(matches)
	return matches
}

// Parse inspects raw input and returns the matching CommandDef (if any),
// the split argument list, and whether the input looked like a slash command.
//
//   - isCommand=false → treat as a regular chat message
//   - isCommand=true, cmd==nil → looked like a command but name unknown
//   - isCommand=true, cmd!=nil → known command, handler available
func (r *CommandRegistry) Parse(input string) (cmd *CommandDef, args []string, isCommand bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil, false
	}

	m := commandPattern.FindStringSubmatch(trimmed)
	if m == nil {
		return nil, nil, false
	}

	name := strings.ToLower(m[1])
	rawArgs := strings.TrimSpace(m[2])

	var splitArgs []string
	if rawArgs != "" {
		splitArgs = strings.Fields(rawArgs)
	}

	def, ok := r.lookup[name]
	if !ok {
		return nil, splitArgs, true
	}
	return def, splitArgs, true
}
