package gateway

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildHelloOK returns a raw WebSocket text frame for a successful hello-ok response.
func buildHelloOK(t *testing.T) []byte {
	t.Helper()
	okTrue := true
	frame := InboundFrame{
		Type: "res",
		ID:   "connect-1",
		OK:   &okTrue,
		Payload: mustJSON(t, HelloOKPayload{
			Type:     "hello-ok",
			Protocol: 3,
			Server:   HelloServerInfo{Version: "1.0.0", ConnID: "test-conn"},
			Features: HelloFeatures{
				Methods: []string{"chat.send", "chat.history"},
				Events:  []string{"chat.event", "connect.challenge"},
			},
		}),
	}
	data, err := json.Marshal(frame)
	require.NoError(t, err)
	return data
}
