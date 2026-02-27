// Package commands provides command-related types and utilities for the talons-console TUI.
// All types in this package are designed for single-goroutine access; no mutexes are used.
package commands

const historyCapacity = 50

// History is a fixed-capacity ring buffer that stores user input entries for
// arrow-key navigation. It supports draft preservation: when the user navigates
// backward through history, their current unsaved input is saved and restored
// when they navigate back to the present.
//
// Not thread-safe — intended for exclusive use by the Bubble Tea event loop.
type History struct {
	entries []string
	cursor  int    // -1 means "not navigating"
	draft   string // saved input before navigation started
}

// NewHistory creates a new History with the fixed capacity.
func NewHistory() *History {
	return &History{
		entries: make([]string, 0, historyCapacity),
		cursor:  -1,
	}
}

// Add appends an entry to the history buffer.
// Empty strings and consecutive duplicates are ignored.
// When the buffer is full (50 entries), the oldest entry is dropped.
// Add resets navigation state (cursor = -1, draft = "").
func (h *History) Add(entry string) {
	if entry == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == entry {
		return
	}
	if len(h.entries) == historyCapacity {
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, entry)
	h.cursor = -1
	h.draft = ""
}

// Prev moves to the next-older entry. On the first call in a navigation sequence
// (cursor == -1), currentInput is saved as the draft. Returns the entry and true
// when entries exist, or ("", false) on an empty buffer. Clamps at the oldest entry.
func (h *History) Prev(currentInput string) (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	if h.cursor == -1 {
		h.draft = currentInput
		h.cursor = len(h.entries) - 1
	} else if h.cursor > 0 {
		h.cursor--
	}
	// cursor == 0: clamped at oldest, return it
	return h.entries[h.cursor], true
}

// Next moves toward newer entries. Past the newest entry, the saved draft is
// returned and true. Returns ("", false) when not currently navigating (cursor == -1).
func (h *History) Next() (string, bool) {
	if h.cursor == -1 {
		return "", false
	}
	if h.cursor < len(h.entries)-1 {
		h.cursor++
		return h.entries[h.cursor], true
	}
	// Past the newest entry — restore draft and stop navigating.
	draft := h.draft
	h.cursor = -1
	h.draft = ""
	return draft, true
}

// Draft returns the currently saved draft string (empty if no draft is saved).
func (h *History) Draft() string {
	return h.draft
}

// Reset sets cursor to -1 and clears the draft.
// Called when navigation should stop (e.g. the user types a new character).
func (h *History) Reset() {
	h.cursor = -1
	h.draft = ""
}

// Len returns the current number of entries in the buffer.
func (h *History) Len() int {
	return len(h.entries)
}
