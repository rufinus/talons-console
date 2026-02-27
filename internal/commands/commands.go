package commands

// DefaultRegistry is the package-level registry populated by InitCommands.
var DefaultRegistry = NewCommandRegistry()

// InitCommands registers all 11 slash commands (10 active + /history stub).
// Handler functions are intentionally nil at this phase; they are filled in
// during Phase 4 (TASK-006 through TASK-009).
func InitCommands() {
	DefaultRegistry = NewCommandRegistry()

	DefaultRegistry.Register(&CommandDef{
		Name:        "help",
		Aliases:     nil,
		Category:    "System",
		Description: "Show available commands and usage information",
		LongDesc: "Display a list of all available slash commands with their descriptions and usage. " +
			"Pass a command name to see detailed help for that specific command.",
		Usage:    "/help [command]",
		Examples: []string{"/help", "/help agent", "/help timeout"},
		Related:  []string{"exit"},
		Handler:  nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "exit",
		Aliases:     []string{"quit"},
		Category:    "System",
		Description: "Exit the application",
		LongDesc:    "Close the gateway connection and exit talons-console. Use /quit as an alias.",
		Usage:       "/exit",
		Examples:    []string{"/exit", "/quit"},
		Related:     []string{"reconnect"},
		Handler:     nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "clear",
		Aliases:     nil,
		Category:    "Display",
		Description: "Clear the message history from the screen",
		LongDesc:    "Remove all displayed messages from the conversation view. This does not delete server-side history.",
		Usage:       "/clear",
		Examples:    []string{"/clear"},
		Related:     []string{"history"},
		Handler:     nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "agent",
		Aliases:     nil,
		Category:    "Session Control",
		Description: "Switch the active agent",
		LongDesc: "Change the agent that receives your messages. " +
			"The new agent name is sent to the gateway and the header is updated.",
		Usage:    "/agent <name>",
		Examples: []string{"/agent daedalus", "/agent metis"},
		Related:  []string{"session", "status"},
		Handler:  nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "session",
		Aliases:     nil,
		Category:    "Session Control",
		Description: "Switch or display the current session key",
		LongDesc: "Change the session key used to scope conversation history. " +
			"Without arguments, displays the current session key.",
		Usage:    "/session [key]",
		Examples: []string{"/session", "/session my-project"},
		Related:  []string{"agent", "history"},
		Handler:  nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "model",
		Aliases:     nil,
		Category:    "Session Control",
		Description: "Set the LLM model override",
		LongDesc: "Override the language model used by the gateway for this session. " +
			"Pass no argument to clear the override and use the server default.",
		Usage:    "/model [name]",
		Examples: []string{"/model anthropic/claude-opus-4-6", "/model gpt-4o", "/model"},
		Related:  []string{"thinking", "agent"},
		Handler:  nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "thinking",
		Aliases:     nil,
		Category:    "Session Control",
		Description: "Set the reasoning/thinking level",
		LongDesc: "Control the reasoning depth for models that support extended thinking. " +
			"Pass no argument to display the current level.",
		Usage:    "/thinking <off|minimal|low|medium|high>",
		Examples: []string{"/thinking low", "/thinking high", "/thinking off", "/thinking"},
		Related:  []string{"model", "timeout"},
		Handler:  nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "timeout",
		Aliases:     nil,
		Category:    "Session Control",
		Description: "Set the request timeout in milliseconds",
		LongDesc: "Override the gateway request timeout. Value must be a positive integer in milliseconds. " +
			"Pass no argument to display the current timeout.",
		Usage:    "/timeout [ms]",
		Examples: []string{"/timeout 30000", "/timeout 120000", "/timeout"},
		Related:  []string{"reconnect"},
		Handler:  nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "status",
		Aliases:     nil,
		Category:    "Display",
		Description: "Display connection and session status",
		LongDesc:    "Show a summary of the current connection state, session parameters, message counters, and uptime.",
		Usage:       "/status",
		Examples:    []string{"/status"},
		Related:     []string{"reconnect", "session"},
		Handler:     nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "reconnect",
		Aliases:     nil,
		Category:    "System",
		Description: "Reconnect to the gateway",
		LongDesc:    "Close the current WebSocket connection and establish a new one using the existing configuration.",
		Usage:       "/reconnect",
		Examples:    []string{"/reconnect"},
		Related:     []string{"exit", "status"},
		Handler:     nil,
	})

	DefaultRegistry.Register(&CommandDef{
		Name:        "history",
		Aliases:     nil,
		Category:    "Display",
		Description: "Browse message history (coming in v0.3)",
		LongDesc: "Retrieve and display historical messages for the current session from the gateway. " +
			"This feature is planned for v0.3 and is not yet active.",
		Usage:    "/history [session-key]",
		Examples: []string{"/history", "/history my-project"},
		Related:  []string{"clear", "session"},
		Handler:  nil,
	})

	WireDisplayHandlers(DefaultRegistry)
	WireStateHandlers(DefaultRegistry)
	WireSessionHandlers(DefaultRegistry)
	WireReconnectHandler(DefaultRegistry)
}
