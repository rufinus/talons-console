package tui

import (
	"testing"
	"time"

	"github.com/rufinus/talons-console/internal/gateway"
)

func TestNewMessagesModel(t *testing.T) {
	m := NewMessagesModel(80, 24)

	if m.width != 80 {
		t.Errorf("expected width 80, got %d", m.width)
	}
	if m.height != 24 {
		t.Errorf("expected height 24, got %d", m.height)
	}
	if m.showSpinner {
		t.Error("expected showSpinner to be false initially")
	}
}

func TestMessagesModel_AppendUserMessage(t *testing.T) {
	m := NewMessagesModel(80, 24)
	m.AppendUserMessage("Hello, world!")

	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.messages))
	}

	msg := m.messages[0]
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got '%s'", msg.Role)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got '%s'", msg.Content)
	}
	if msg.Streaming {
		t.Error("expected Streaming to be false for user message")
	}
}

func TestMessagesModel_AppendToken(t *testing.T) {
	m := NewMessagesModel(80, 24)

	// First token creates a new assistant message
	m.AppendToken("Hello")
	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.messages))
	}
	if !m.messages[0].Streaming {
		t.Error("expected Streaming to be true after AppendToken")
	}

	// Subsequent tokens append to existing
	m.AppendToken(" world")
	if m.messages[0].Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got '%s'", m.messages[0].Content)
	}
}

func TestMessagesModel_AppendToken_NoExistingMessage(t *testing.T) {
	m := NewMessagesModel(80, 24)
	m.AppendToken("First token")

	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.messages))
	}
	if m.messages[0].Role != "assistant" {
		t.Errorf("expected role 'assistant', got '%s'", m.messages[0].Role)
	}
}

func TestMessagesModel_FinalizeMessage(t *testing.T) {
	m := NewMessagesModel(80, 24)
	m.AppendUserMessage("Hello")
	m.AppendAssistantMessage("")
	m.AppendToken("Response")

	m.FinalizeMessage()

	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}

	lastMsg := m.messages[1]
	if lastMsg.Streaming {
		t.Error("expected Streaming to be false after FinalizeMessage")
	}
}

func TestMessagesModel_FinalizeMessage_NoStreaming(t *testing.T) {
	m := NewMessagesModel(80, 24)
	m.AppendUserMessage("Hello")

	// Should not panic, just no-op
	m.FinalizeMessage()
}

func TestMessagesModel_SetSize(t *testing.T) {
	m := NewMessagesModel(80, 24)
	m.SetSize(100, 40)

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestMessagesModel_ScrollToBottom(t *testing.T) {
	m := NewMessagesModel(80, 24)
	// Should not panic
	m.ScrollToBottom()
}

func TestMessagesModel_LoadHistory(t *testing.T) {
	m := NewMessagesModel(80, 24)

	history := []gateway.HistoryMessage{
		{Role: "user", Content: "Hello", Timestamp: time.Now().Unix()},
		{Role: "assistant", Content: "Hi there", Timestamp: time.Now().Unix()},
	}

	m.LoadHistory(history)

	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}

	if m.messages[0].Role != "user" {
		t.Errorf("expected first message role 'user', got '%s'", m.messages[0].Role)
	}
	if m.messages[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got '%s'", m.messages[1].Role)
	}
}

func TestMessagesModel_Messages(t *testing.T) {
	m := NewMessagesModel(80, 24)
	m.AppendUserMessage("Test")

	msgs := m.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}
