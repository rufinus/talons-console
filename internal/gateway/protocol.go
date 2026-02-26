// Package gateway provides a WebSocket client for OpenClaw Gateway.
package gateway

import (
	"encoding/json"
	"fmt"
)

// ─────────────────────────────────────────────
// Outbound (talons → Gateway)
// ─────────────────────────────────────────────

// OutboundFrame is the top-level envelope for every message sent to the Gateway.
// The Gateway uses a req/res/event protocol — all client-originated messages are
// "req" (request) frames.
type OutboundFrame struct {
	Type   string          `json:"type"`
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// OutboundMessage is the high-level message that callers enqueue.
// gateway.Client converts this to a proper OutboundFrame before sending.
type OutboundMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// ChatSendParams is the params block for the chat.send method.
type ChatSendParams struct {
	Content        string `json:"content"`
	SessionKey     string `json:"sessionKey,omitempty"`
	AgentID        string `json:"agentId,omitempty"`
	Deliver        bool   `json:"deliver,omitempty"`
	Thinking       string `json:"thinking,omitempty"`
	TimeoutMs      int    `json:"timeoutMs,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	Model          string `json:"model,omitempty"`
}

// HistoryParams is the params block for the chat.history method.
type HistoryParams struct {
	SessionKey string `json:"sessionKey"`
	Limit      int    `json:"limit,omitempty"`
}

// ConnectParams is the params block for the connect method.
type ConnectParams struct {
	MinProtocol int             `json:"minProtocol"`
	MaxProtocol int             `json:"maxProtocol"`
	Client      ClientInfo      `json:"client"`
	Role        string          `json:"role"`
	Scopes      []string        `json:"scopes"`
	Auth        AuthCredentials `json:"auth"`
	UserAgent   string          `json:"userAgent,omitempty"`
	Device      *DeviceInfo     `json:"device,omitempty"`
}

// ClientInfo describes the connecting client.
type ClientInfo struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Mode     string `json:"mode"`
}

// AuthCredentials holds authentication credentials for the connect params.
type AuthCredentials struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

// DeviceInfo provides device identity for the connect params.
type DeviceInfo struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey,omitempty"`
	Signature string `json:"signature,omitempty"`
	SignedAt  int64  `json:"signedAt,omitempty"`
	Nonce     string `json:"nonce,omitempty"`
}

// ─────────────────────────────────────────────
// Inbound (Gateway → talons)
// ─────────────────────────────────────────────

// InboundFrame is the top-level envelope of every message received from the Gateway.
type InboundFrame struct {
	Type    string          `json:"type"` // "req", "res", "event"
	ID      string          `json:"id,omitempty"`
	OK      *bool           `json:"ok,omitempty"`
	Method  string          `json:"method,omitempty"`
	Event   string          `json:"event,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *FrameError     `json:"error,omitempty"`
	Seq     int64           `json:"seq,omitempty"`
}

// FrameError is the error block in a response frame.
type FrameError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ─────────────────────────────────────────────
// Inbound event payloads
// ─────────────────────────────────────────────

// HelloOKPayload is the payload of a successful connect response.
type HelloOKPayload struct {
	Type     string          `json:"type"` // "hello-ok"
	Protocol int             `json:"protocol"`
	Server   HelloServerInfo `json:"server"`
	Features HelloFeatures   `json:"features"`
	Snapshot *HelloSnapshot  `json:"snapshot,omitempty"`
	Auth     *HelloAuth      `json:"auth,omitempty"`
}

// HelloServerInfo contains server identification.
type HelloServerInfo struct {
	Version string `json:"version"`
	ConnID  string `json:"connId"`
}

// HelloFeatures lists advertised Gateway capabilities.
type HelloFeatures struct {
	Methods []string `json:"methods"`
	Events  []string `json:"events"`
}

// HelloSnapshot is the initial state snapshot in the connect response.
type HelloSnapshot struct {
	SessionDefaults *SessionDefaults `json:"sessionDefaults,omitempty"`
	AuthMode        string           `json:"authMode,omitempty"`
}

// SessionDefaults provides default session configuration.
type SessionDefaults struct {
	DefaultAgentID string `json:"defaultAgentId"`
	MainKey        string `json:"mainKey"`
	MainSessionKey string `json:"mainSessionKey"`
}

// HelloAuth contains the issued auth result.
type HelloAuth struct {
	Role   string   `json:"role"`
	Scopes []string `json:"scopes"`
}

// ConnectChallengePayload is the payload of the connect.challenge event.
type ConnectChallengePayload struct {
	Nonce string `json:"nonce"`
	TS    int64  `json:"ts"`
}

// ChatEventPayload is the payload of a chat.event inbound event.
type ChatEventPayload struct {
	RunID      string        `json:"runId"`
	SessionKey string        `json:"sessionKey"`
	Seq        int64         `json:"seq"`
	State      string        `json:"state"` // "delta", "final", "aborted", "error"
	Message    *ChatEventMsg `json:"message,omitempty"`
	ErrorMsg   string        `json:"errorMessage,omitempty"`
	StopReason string        `json:"stopReason,omitempty"`
}

// ChatEventMsg is a partial or final message in a chat.event payload.
type ChatEventMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AgentEventPayload is the payload of an agent.event (tool calls, etc).
type AgentEventPayload struct {
	RunID  string          `json:"runId"`
	Seq    int64           `json:"seq"`
	Stream string          `json:"stream"`
	TS     int64           `json:"ts"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// HistoryPayload is the payload of a chat.history response.
type HistoryPayload struct {
	Messages []HistoryMessage `json:"messages"`
}

// HistoryMessage represents a single historical chat message.
type HistoryMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// ─────────────────────────────────────────────
// InboundEvent — TUI-facing abstraction
// ─────────────────────────────────────────────

// InboundKind identifies the semantic type of an inbound Gateway event.
type InboundKind int

const (
	KindUnknown     InboundKind = iota // unrecognised message — Raw is populated
	KindToken                          // streaming token (partial response)
	KindMessage                        // final / complete response
	KindAuthResult                     // authentication success or failure
	KindError                          // error from Gateway
	KindToolCall                       // tool invocation or output
	KindSessionInfo                    // session metadata
	KindHistory                        // chat history batch
	KindChallenge                      // connect.challenge (auth nonce)
)

// InboundEvent is the TUI-facing representation of a Gateway event.
// Kind determines which fields are populated. Unknown types preserve Raw.
type InboundEvent struct {
	Kind InboundKind
	Raw  json.RawMessage

	// KindToken, KindMessage
	Content string
	Role    string // "assistant", "system", "tool"
	State   string // "delta", "final", "aborted", "error"

	// KindAuthResult
	Success  bool
	Features []string
	Version  string // Gateway server version

	// KindError
	Error string

	// KindToolCall
	ToolName string
	ToolArgs string
	ToolOut  string

	// KindSessionInfo
	Agent   string
	Session string

	// KindHistory
	HistoryMessages []HistoryMessage

	// KindChallenge
	Nonce string
}

// ─────────────────────────────────────────────
// ParseInbound
// ─────────────────────────────────────────────

// ParseInbound decodes a raw JSON frame from the Gateway into an InboundEvent.
// It never returns an error — unknown or malformed input produces KindUnknown
// with the raw bytes preserved for logging.
func ParseInbound(data []byte) InboundEvent {
	var frame InboundFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		// Completely malformed JSON
		return InboundEvent{Kind: KindUnknown, Raw: json.RawMessage(data)}
	}

	switch frame.Type {
	case "event":
		return parseEventFrame(frame, data)
	case "res":
		return parseResponseFrame(frame, data)
	default:
		return InboundEvent{Kind: KindUnknown, Raw: json.RawMessage(data)}
	}
}

// parseEventFrame handles inbound "event" frames.
func parseEventFrame(frame InboundFrame, raw []byte) InboundEvent {
	switch frame.Event {
	case "connect.challenge":
		return parseChallengeEvent(frame, raw)
	case "chat.event":
		return parseChatEvent(frame, raw)
	case "agent.event":
		return parseAgentEvent(frame, raw)
	default:
		return InboundEvent{Kind: KindUnknown, Raw: json.RawMessage(raw)}
	}
}

func parseChallengeEvent(frame InboundFrame, raw []byte) InboundEvent {
	var p ConnectChallengePayload
	if len(frame.Payload) > 0 {
		_ = json.Unmarshal(frame.Payload, &p)
	}
	return InboundEvent{
		Kind:  KindChallenge,
		Nonce: p.Nonce,
		Raw:   json.RawMessage(raw),
	}
}

func parseChatEvent(frame InboundFrame, raw []byte) InboundEvent {
	var p ChatEventPayload
	if len(frame.Payload) > 0 {
		if err := json.Unmarshal(frame.Payload, &p); err != nil {
			return InboundEvent{Kind: KindUnknown, Raw: json.RawMessage(raw)}
		}
	}

	switch p.State {
	case "delta":
		evt := InboundEvent{
			Kind:  KindToken,
			State: p.State,
			Raw:   json.RawMessage(raw),
		}
		if p.Message != nil {
			evt.Content = p.Message.Content
			evt.Role = p.Message.Role
		}
		return evt

	case "final":
		evt := InboundEvent{
			Kind:  KindMessage,
			State: p.State,
			Raw:   json.RawMessage(raw),
		}
		if p.Message != nil {
			evt.Content = p.Message.Content
			evt.Role = p.Message.Role
		}
		return evt

	case "aborted", "error":
		errMsg := p.ErrorMsg
		if errMsg == "" {
			errMsg = fmt.Sprintf("chat stream %s", p.State)
		}
		return InboundEvent{
			Kind:  KindError,
			Error: errMsg,
			State: p.State,
			Raw:   json.RawMessage(raw),
		}

	default:
		return InboundEvent{Kind: KindUnknown, Raw: json.RawMessage(raw)}
	}
}

func parseAgentEvent(frame InboundFrame, raw []byte) InboundEvent {
	// agent.event carries tool call data
	var p AgentEventPayload
	if len(frame.Payload) > 0 {
		_ = json.Unmarshal(frame.Payload, &p)
	}
	toolName := p.Stream
	toolArgs := ""
	if len(p.Data) > 0 {
		toolArgs = string(p.Data)
	}
	return InboundEvent{
		Kind:     KindToolCall,
		ToolName: toolName,
		ToolArgs: toolArgs,
		Raw:      json.RawMessage(raw),
	}
}

// parseResponseFrame handles inbound "res" frames (replies to client requests).
func parseResponseFrame(frame InboundFrame, raw []byte) InboundEvent {
	// Connect response (hello-ok or hello-fail)
	if frame.ID != "" && len(frame.Payload) > 0 {
		// Try to detect the payload type from its "type" field
		var payloadType struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(frame.Payload, &payloadType)

		if payloadType.Type == "hello-ok" {
			return parseHelloOK(frame, raw)
		}

		// chat.history response
		var historyPayload HistoryPayload
		if err := json.Unmarshal(frame.Payload, &historyPayload); err == nil && len(historyPayload.Messages) > 0 {
			return InboundEvent{
				Kind:            KindHistory,
				HistoryMessages: historyPayload.Messages,
				Raw:             json.RawMessage(raw),
			}
		}
	}

	// Error response
	if frame.OK != nil && !*frame.OK {
		errMsg := "unknown error"
		if frame.Error != nil {
			errMsg = fmt.Sprintf("%s: %s", frame.Error.Code, frame.Error.Message)
		}
		return InboundEvent{
			Kind:  KindError,
			Error: errMsg,
			Raw:   json.RawMessage(raw),
		}
	}

	return InboundEvent{Kind: KindUnknown, Raw: json.RawMessage(raw)}
}

func parseHelloOK(frame InboundFrame, raw []byte) InboundEvent {
	var p HelloOKPayload
	if err := json.Unmarshal(frame.Payload, &p); err != nil {
		return InboundEvent{Kind: KindUnknown, Raw: json.RawMessage(raw)}
	}

	// Flatten features into a string slice
	features := make([]string, 0, len(p.Features.Methods)+len(p.Features.Events))
	features = append(features, p.Features.Methods...)
	features = append(features, p.Features.Events...)

	ok := true
	if frame.OK != nil {
		ok = *frame.OK
	}

	return InboundEvent{
		Kind:     KindAuthResult,
		Success:  ok,
		Version:  p.Server.Version,
		Features: features,
		Raw:      json.RawMessage(raw),
	}
}
