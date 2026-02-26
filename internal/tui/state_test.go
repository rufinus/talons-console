package tui

import (
	"sync"
	"testing"
	"time"

	"github.com/rufinus/talons-console/internal/config"
	"github.com/rufinus/talons-console/internal/gateway"
)

func testConfig() *config.Config {
	return &config.Config{
		Agent:     "test-agent",
		Session:   "test-session",
		Thinking:  "low",
		TimeoutMs: 30000,
		URL:       "wss://example.com",
	}
}

func TestNewSessionState(t *testing.T) {
	cfg := testConfig()
	s := NewSessionState(cfg)

	if s.GetAgent() != cfg.Agent {
		t.Errorf("agent: got %q, want %q", s.GetAgent(), cfg.Agent)
	}
	if s.GetSession() != cfg.Session {
		t.Errorf("session: got %q, want %q", s.GetSession(), cfg.Session)
	}
	if s.GetModel() != "" {
		t.Errorf("model: got %q, want empty", s.GetModel())
	}
	if s.GetThinking() != cfg.Thinking {
		t.Errorf("thinking: got %q, want %q", s.GetThinking(), cfg.Thinking)
	}
	if s.GetTimeoutMs() != cfg.TimeoutMs {
		t.Errorf("timeoutMs: got %d, want %d", s.GetTimeoutMs(), cfg.TimeoutMs)
	}
	if s.GetGatewayURL() != cfg.URL {
		t.Errorf("gatewayURL: got %q, want %q", s.GetGatewayURL(), cfg.URL)
	}
}

func TestSetters(t *testing.T) {
	s := NewSessionState(testConfig())

	s.SetAgent("agent2")
	if s.GetAgent() != "agent2" {
		t.Errorf("SetAgent: got %q", s.GetAgent())
	}
	s.SetSession("sess2")
	if s.GetSession() != "sess2" {
		t.Errorf("SetSession: got %q", s.GetSession())
	}
	s.SetModel("gpt-4o")
	if s.GetModel() != "gpt-4o" {
		t.Errorf("SetModel: got %q", s.GetModel())
	}
	s.SetThinking("high")
	if s.GetThinking() != "high" {
		t.Errorf("SetThinking: got %q", s.GetThinking())
	}
	s.SetTimeoutMs(5000)
	if s.GetTimeoutMs() != 5000 {
		t.Errorf("SetTimeoutMs: got %d", s.GetTimeoutMs())
	}
	s.SetGatewayURL("wss://other.example.com")
	if s.GetGatewayURL() != "wss://other.example.com" {
		t.Errorf("SetGatewayURL: got %q", s.GetGatewayURL())
	}
	now := time.Now()
	s.SetConnectedAt(now)
	if !s.GetConnectedAt().Equal(now) {
		t.Errorf("SetConnectedAt: got %v, want %v", s.GetConnectedAt(), now)
	}
	s.SetVersion("1.2.3")
	if s.GetVersion() != "1.2.3" {
		t.Errorf("SetVersion: got %q", s.GetVersion())
	}
}

func TestCounters(t *testing.T) {
	s := NewSessionState(testConfig())

	s.IncrSent()
	s.IncrSent()
	s.IncrRecv()

	if s.GetMsgSent() != 2 {
		t.Errorf("MsgSent: got %d, want 2", s.GetMsgSent())
	}
	if s.GetMsgRecv() != 1 {
		t.Errorf("MsgRecv: got %d, want 1", s.GetMsgRecv())
	}

	s.ResetCounters()
	if s.GetMsgSent() != 0 || s.GetMsgRecv() != 0 {
		t.Errorf("ResetCounters: sent=%d recv=%d, want both 0", s.GetMsgSent(), s.GetMsgRecv())
	}
}

func TestResetCounters_OtherFieldsUnchanged(t *testing.T) {
	s := NewSessionState(testConfig())
	s.SetAgent("special-agent")
	s.IncrSent()
	s.ResetCounters()

	if s.GetAgent() != "special-agent" {
		t.Errorf("agent changed after ResetCounters: got %q", s.GetAgent())
	}
}

func TestApplyToSendParams(t *testing.T) {
	s := NewSessionState(testConfig())
	s.SetModel("gpt-4o")

	params := &gateway.ChatSendParams{Message: "hello"}
	s.ApplyToSendParams(params)

	if params.SessionKey != "agent:test-agent:test-session" {
		t.Errorf("AgentID: got %q, want %q", params.SessionKey, "agent:test-agent:test-session")
	}
	// SessionKey already checked above
	if params.Thinking != "low" {
		t.Errorf("Thinking: got %q", params.Thinking)
	}
	if params.TimeoutMs != 30000 {
		t.Errorf("TimeoutMs: got %d", params.TimeoutMs)
	}
}



func TestApplyToSendParams_RaceSafety(t *testing.T) {
	s := NewSessionState(testConfig())

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: repeatedly set thinking
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			if i%2 == 0 {
				s.SetThinking("high")
			} else {
				s.SetThinking("off")
			}
		}
	}()

	// Goroutine 2: repeatedly call ApplyToSendParams
	go func() {
		defer wg.Done()
		params := &gateway.ChatSendParams{}
		for i := 0; i < 1000; i++ {
			s.ApplyToSendParams(params)
		}
	}()

	wg.Wait()
}

func TestSnapshot(t *testing.T) {
	s := NewSessionState(testConfig())
	s.SetModel("claude-3")
	s.SetVersion("2.0.0")
	s.IncrSent()
	s.IncrRecv()
	s.IncrRecv()

	snap := s.Snapshot()

	if snap.Agent != "test-agent" {
		t.Errorf("Snapshot.Agent: got %q", snap.Agent)
	}
	if snap.Model != "claude-3" {
		t.Errorf("Snapshot.Model: got %q", snap.Model)
	}
	if snap.Version != "2.0.0" {
		t.Errorf("Snapshot.Version: got %q", snap.Version)
	}
	if snap.MsgSent != 1 {
		t.Errorf("Snapshot.MsgSent: got %d", snap.MsgSent)
	}
	if snap.MsgRecv != 2 {
		t.Errorf("Snapshot.MsgRecv: got %d", snap.MsgRecv)
	}
}

func TestSnapshot_ConcurrentIncrRecv(t *testing.T) {
	s := NewSessionState(testConfig())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			s.IncrRecv()
		}
	}()

	// Take snapshots concurrently — should not race
	for i := 0; i < 500; i++ {
		snap := s.Snapshot()
		// MsgRecv must be consistent (0–2000), not a partial write
		if snap.MsgRecv < 0 || snap.MsgRecv > 2000 {
			t.Errorf("inconsistent MsgRecv: %d", snap.MsgRecv)
		}
	}
	wg.Wait()
}

func TestIncrRecv_ConcurrentRead(t *testing.T) {
	s := NewSessionState(testConfig())

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			s.IncrRecv()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = s.GetMsgRecv()
		}
	}()

	wg.Wait()
}
