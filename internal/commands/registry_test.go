package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers ---------------------------------------------------------------

func buildRegistry(t *testing.T) *CommandRegistry {
	t.Helper()
	InitCommands()
	return DefaultRegistry
}

// Parse -----------------------------------------------------------------

func TestParse_TableDriven(t *testing.T) {
	r := buildRegistry(t)

	helpDef, ok := r.Get("help")
	require.True(t, ok)
	agentDef, ok := r.Get("agent")
	require.True(t, ok)
	timeoutDef, ok := r.Get("timeout")
	require.True(t, ok)

	tests := []struct {
		input     string
		isCommand bool
		wantCmd   *CommandDef
		wantArgs  []string
	}{
		{"/help", true, helpDef, []string{}},
		{"/agent daedalus", true, agentDef, []string{"daedalus"}},
		{"/HELP", true, helpDef, []string{}},
		{"  /help  agent  ", true, helpDef, []string{"agent"}},
		{"/timeout 120000 extra", true, timeoutDef, []string{"120000", "extra"}},
		{"/foo", true, nil, []string{}},
		{"hello world", false, nil, []string{}},
		{"use /tmp/file", false, nil, []string{}},
		{"/", false, nil, []string{}},
		{"", false, nil, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args, isCmd := r.Parse(tt.input)
			assert.Equal(t, tt.isCommand, isCmd, "isCommand mismatch")
			assert.Equal(t, tt.wantCmd, cmd, "cmd mismatch")
			// normalise nil vs empty slice for args comparison
			if len(tt.wantArgs) == 0 {
				assert.Empty(t, args)
			} else {
				assert.Equal(t, tt.wantArgs, args)
			}
		})
	}
}

// Complete --------------------------------------------------------------

func TestComplete_Prefix(t *testing.T) {
	r := buildRegistry(t)

	got := r.Complete("/he")
	assert.Equal(t, []string{"/help"}, got)
}

func TestComplete_AllCommands(t *testing.T) {
	r := buildRegistry(t)

	got := r.Complete("/")
	assert.Len(t, got, 11, "expected 11 commands from Complete(\"/\")")
	// must be sorted
	for i := 1; i < len(got); i++ {
		assert.True(t, got[i-1] <= got[i], "not sorted at index %d", i)
	}
}

func TestComplete_NoMatch(t *testing.T) {
	r := buildRegistry(t)
	got := r.Complete("/z")
	assert.Empty(t, got)
}

// Get -------------------------------------------------------------------

func TestGet_AliasResolvesToPrimary(t *testing.T) {
	r := buildRegistry(t)

	exitDef, ok := r.Get("exit")
	require.True(t, ok)

	quitDef, ok := r.Get("quit")
	require.True(t, ok)

	assert.Same(t, exitDef, quitDef, "quit alias should resolve to exit CommandDef")
	assert.Equal(t, "exit", exitDef.Name)
}

func TestGet_CaseInsensitive(t *testing.T) {
	r := buildRegistry(t)
	def1, ok1 := r.Get("HELP")
	def2, ok2 := r.Get("help")
	require.True(t, ok1)
	require.True(t, ok2)
	assert.Same(t, def1, def2)
}

func TestGet_Unknown(t *testing.T) {
	r := buildRegistry(t)
	_, ok := r.Get("notacommand")
	assert.False(t, ok)
}

// All -------------------------------------------------------------------

func TestAll_Returns11AfterInitCommands(t *testing.T) {
	r := buildRegistry(t)
	all := r.All()
	assert.Len(t, all, 11)
}

// Register panics -------------------------------------------------------

func TestRegister_PanicOnDuplicateName(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(&CommandDef{Name: "foo"})
	assert.Panics(t, func() {
		r.Register(&CommandDef{Name: "foo"})
	})
}

func TestRegister_PanicOnDuplicateAlias(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(&CommandDef{Name: "foo", Aliases: []string{"bar"}})
	assert.Panics(t, func() {
		r.Register(&CommandDef{Name: "baz", Aliases: []string{"bar"}})
	})
}

func TestRegister_PanicOnAliasMatchingExistingName(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(&CommandDef{Name: "foo"})
	assert.Panics(t, func() {
		r.Register(&CommandDef{Name: "bar", Aliases: []string{"foo"}})
	})
}
