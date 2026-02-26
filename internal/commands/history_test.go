package commands

import (
	"fmt"
	"testing"
)

func TestEmptyBuffer(t *testing.T) {
	h := NewHistory()
	if s, ok := h.Prev("draft"); ok || s != "" {
		t.Errorf("Prev on empty: want (\"\", false), got (%q, %v)", s, ok)
	}
	if s, ok := h.Next(); ok || s != "" {
		t.Errorf("Next on empty: want (\"\", false), got (%q, %v)", s, ok)
	}
}

func TestSingleEntry(t *testing.T) {
	h := NewHistory()
	h.Add("hello")

	s, ok := h.Prev("")
	if !ok || s != "hello" {
		t.Fatalf("first Prev: want (\"hello\", true), got (%q, %v)", s, ok)
	}
	// clamped — should still return "hello"
	s, ok = h.Prev("")
	if !ok || s != "hello" {
		t.Errorf("clamped Prev: want (\"hello\", true), got (%q, %v)", s, ok)
	}
}

func TestDraftPreservation(t *testing.T) {
	h := NewHistory()
	h.Add("a")
	h.Add("b")

	// User has typed "c" (not added yet)
	s, ok := h.Prev("c")
	if !ok || s != "b" {
		t.Fatalf("first Prev: want (\"b\", true), got (%q, %v)", s, ok)
	}
	if h.Draft() != "c" {
		t.Errorf("draft should be \"c\", got %q", h.Draft())
	}

	s, ok = h.Prev("")
	if !ok || s != "a" {
		t.Fatalf("second Prev: want (\"a\", true), got (%q, %v)", s, ok)
	}

	s, ok = h.Next()
	if !ok || s != "b" {
		t.Fatalf("Next to b: want (\"b\", true), got (%q, %v)", s, ok)
	}

	// Next past newest — draft restored
	s, ok = h.Next()
	if !ok || s != "c" {
		t.Fatalf("Next past newest (draft): want (\"c\", true), got (%q, %v)", s, ok)
	}

	// Next when not navigating
	s, ok = h.Next()
	if ok || s != "" {
		t.Errorf("Next when not navigating: want (\"\", false), got (%q, %v)", s, ok)
	}
}

func TestDeduplication(t *testing.T) {
	h := NewHistory()
	h.Add("hello")
	h.Add("hello")
	if h.Len() != 1 {
		t.Errorf("consecutive dedup: want Len=1, got %d", h.Len())
	}
}

func TestNonConsecutiveDuplicates(t *testing.T) {
	h := NewHistory()
	h.Add("a")
	h.Add("b")
	h.Add("a")
	if h.Len() != 3 {
		t.Errorf("non-consecutive dup: want Len=3, got %d", h.Len())
	}
}

func TestOverflow(t *testing.T) {
	h := NewHistory()
	for i := 1; i <= 51; i++ {
		h.Add(fmt.Sprintf("entry-%d", i))
	}
	if h.Len() != 50 {
		t.Errorf("overflow: want Len=50, got %d", h.Len())
	}
	// oldest (entry-1) dropped; entry-2 is now first
	s, ok := h.Prev("")
	// navigate all the way to oldest
	var oldest string
	for ok {
		oldest = s
		s, ok = h.Prev("")
		if s == oldest {
			break // clamped
		}
	}
	// Navigate to oldest by pressing Prev 49 more times
	h2 := NewHistory()
	for i := 1; i <= 51; i++ {
		h2.Add(fmt.Sprintf("entry-%d", i))
	}
	// Go all the way back
	h2.Prev("")
	for i := 0; i < 49; i++ {
		h2.Prev("")
	}
	got, _ := h2.Prev("") // clamped at oldest
	if got != "entry-2" {
		t.Errorf("after overflow, oldest should be entry-2, got %q", got)
	}
	// Newest should be entry-51
	h3 := NewHistory()
	for i := 1; i <= 51; i++ {
		h3.Add(fmt.Sprintf("entry-%d", i))
	}
	newest, _ := h3.Prev("")
	if newest != "entry-51" {
		t.Errorf("newest after overflow should be entry-51, got %q", newest)
	}
}

func TestAddResetsNavigation(t *testing.T) {
	h := NewHistory()
	h.Add("a")
	h.Prev("")
	h.Add("b")
	s, ok := h.Next()
	if ok || s != "" {
		t.Errorf("Next after Add should return (\"\", false), got (%q, %v)", s, ok)
	}
}

func TestReset(t *testing.T) {
	h := NewHistory()
	h.Add("a")
	h.Prev("")
	h.Reset()
	s, ok := h.Next()
	if ok || s != "" {
		t.Errorf("Next after Reset should return (\"\", false), got (%q, %v)", s, ok)
	}
	if h.Draft() != "" {
		t.Errorf("Draft after Reset should be empty, got %q", h.Draft())
	}
}

func TestAddEmptyNoOp(t *testing.T) {
	h := NewHistory()
	h.Add("")
	if h.Len() != 0 {
		t.Errorf("Add(\"\") should be no-op, got Len=%d", h.Len())
	}
}

func TestBoolBoundaries(t *testing.T) {
	h := NewHistory()
	h.Add("only")

	// Navigate to oldest
	h.Prev("")
	// Clamped Prev still returns true
	s, ok := h.Prev("")
	if !ok || s != "only" {
		t.Errorf("clamped Prev at oldest: want (\"only\", true), got (%q, %v)", s, ok)
	}

	h2 := NewHistory()
	// cursor == -1: Next returns ("", false)
	s, ok = h2.Next()
	if ok || s != "" {
		t.Errorf("Next at cursor=-1: want (\"\", false), got (%q, %v)", s, ok)
	}
}

func TestLen(t *testing.T) {
	h := NewHistory()
	if h.Len() != 0 {
		t.Errorf("initial Len: want 0, got %d", h.Len())
	}
	h.Add("x")
	if h.Len() != 1 {
		t.Errorf("after Add: want 1, got %d", h.Len())
	}
}
