package tui

import (
	"sync"
	"time"

	"github.com/rufinus/talons-console/internal/config"
	"github.com/rufinus/talons-console/internal/gateway"
)

// StateSnapshot is a point-in-time copy of all SessionState fields.
// It is safe to read without any mutex.
type StateSnapshot struct {
	Agent       string
	Session     string
	Model       string
	Thinking    string
	TimeoutMs   int
	ConnectedAt time.Time
	GatewayURL  string
	Version     string
	MsgSent     int
	MsgRecv     int
}

// SessionState is the single source of truth for all mutable session state.
// The TUI goroutine owns writes to most fields; the gateway reader goroutine
// may call IncrRecv concurrently — all access is protected by mu.
type SessionState struct {
	mu          sync.RWMutex
	Agent       string
	Session     string
	Model       string
	Thinking    string
	TimeoutMs   int
	ConnectedAt time.Time
	GatewayURL  string
	Version     string
	MsgSent     int
	MsgRecv     int
}

// NewSessionState initialises a SessionState from the provided Config.
func NewSessionState(cfg *config.Config) *SessionState {
	return &SessionState{
		Agent:      cfg.Agent,
		Session:    cfg.Session,
		Model:      "", // no model override by default
		Thinking:   cfg.Thinking,
		TimeoutMs:  cfg.TimeoutMs,
		GatewayURL: cfg.URL,
	}
}

// ─────────────────────────────────────────────
// Thread-safe getters
// ─────────────────────────────────────────────

func (s *SessionState) GetAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Agent
}

func (s *SessionState) GetSession() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Session
}

func (s *SessionState) GetModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Model
}

func (s *SessionState) GetThinking() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Thinking
}

func (s *SessionState) GetTimeoutMs() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TimeoutMs
}

func (s *SessionState) GetGatewayURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.GatewayURL
}

func (s *SessionState) GetVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Version
}

func (s *SessionState) GetConnectedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ConnectedAt
}

func (s *SessionState) GetMsgSent() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MsgSent
}

func (s *SessionState) GetMsgRecv() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MsgRecv
}

// ─────────────────────────────────────────────
// Thread-safe setters
// ─────────────────────────────────────────────

func (s *SessionState) SetAgent(v string) {
	s.mu.Lock()
	s.Agent = v
	s.mu.Unlock()
}

func (s *SessionState) SetSession(v string) {
	s.mu.Lock()
	s.Session = v
	s.mu.Unlock()
}

func (s *SessionState) SetModel(v string) {
	s.mu.Lock()
	s.Model = v
	s.mu.Unlock()
}

func (s *SessionState) SetThinking(v string) {
	s.mu.Lock()
	s.Thinking = v
	s.mu.Unlock()
}

func (s *SessionState) SetTimeoutMs(v int) {
	s.mu.Lock()
	s.TimeoutMs = v
	s.mu.Unlock()
}

func (s *SessionState) SetConnectedAt(v time.Time) {
	s.mu.Lock()
	s.ConnectedAt = v
	s.mu.Unlock()
}

func (s *SessionState) SetGatewayURL(v string) {
	s.mu.Lock()
	s.GatewayURL = v
	s.mu.Unlock()
}

func (s *SessionState) SetVersion(v string) {
	s.mu.Lock()
	s.Version = v
	s.mu.Unlock()
}

// ─────────────────────────────────────────────
// Counters
// ─────────────────────────────────────────────

// IncrSent increments MsgSent under a write lock.
func (s *SessionState) IncrSent() {
	s.mu.Lock()
	s.MsgSent++
	s.mu.Unlock()
}

// IncrRecv increments MsgRecv under a write lock.
// Safe to call from the gateway reader goroutine concurrently with TUI writes.
func (s *SessionState) IncrRecv() {
	s.mu.Lock()
	s.MsgRecv++
	s.mu.Unlock()
}

// ResetCounters sets both MsgSent and MsgRecv to 0 under a single write lock.
func (s *SessionState) ResetCounters() {
	s.mu.Lock()
	s.MsgSent = 0
	s.MsgRecv = 0
	s.mu.Unlock()
}

// ─────────────────────────────────────────────
// Protocol bridge
// ─────────────────────────────────────────────

// ApplyToSendParams stamps the current agent, session, model, thinking, and
// timeoutMs onto params under a single read lock (atomic snapshot).
func (s *SessionState) ApplyToSendParams(params *gateway.ChatSendParams) {
	s.mu.RLock()
	agent := s.Agent
	session := s.Session
	model := s.Model
	thinking := s.Thinking
	timeoutMs := s.TimeoutMs
	s.mu.RUnlock()

	params.AgentID = agent
	params.SessionKey = session
	params.Model = model
	params.Thinking = thinking
	params.TimeoutMs = timeoutMs
}

// Snapshot returns a point-in-time copy of all fields under a single read lock.
func (s *SessionState) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return StateSnapshot{
		Agent:       s.Agent,
		Session:     s.Session,
		Model:       s.Model,
		Thinking:    s.Thinking,
		TimeoutMs:   s.TimeoutMs,
		ConnectedAt: s.ConnectedAt,
		GatewayURL:  s.GatewayURL,
		Version:     s.Version,
		MsgSent:     s.MsgSent,
		MsgRecv:     s.MsgRecv,
	}
}
