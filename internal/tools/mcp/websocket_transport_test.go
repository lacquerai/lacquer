package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketMCPTransport(t *testing.T) {
	// Create WebSocket echo server
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	var serverConn *websocket.Conn
	var serverMu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth header during handshake
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && authHeader != "Bearer test-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		serverMu.Lock()
		serverConn = conn
		serverMu.Unlock()

		// Echo messages back
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			// For testing, echo back with modified ID
			if messageType == websocket.TextMessage {
				var msg MCPMessage
				if err := json.Unmarshal(message, &msg); err == nil && msg.ID != nil {
					// Echo back as response
					resp := MCPMessage{
						JSONRPC: "2.0",
						ID:      msg.ID,
						Result:  json.RawMessage(`{"echo":true}`),
					}
					respBytes, _ := json.Marshal(resp)
					conn.WriteMessage(websocket.TextMessage, respBytes)
				} else {
					// Just echo notifications
					conn.WriteMessage(messageType, message)
				}
			}
		}
	}))
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	t.Run("Connect and Send/Receive", func(t *testing.T) {
		transport := NewWebSocketMCPTransport(wsURL, 10*time.Second)
		defer transport.Close()

		ctx := context.Background()

		// Send RPC message
		id := int64(1)
		msg := MCPMessage{
			JSONRPC: "2.0",
			ID:      &id,
			Method:  "test",
		}
		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = transport.Send(ctx, msgBytes)
		require.NoError(t, err)

		// Receive response
		resp, err := transport.Receive(ctx)
		require.NoError(t, err)

		var respMsg MCPMessage
		err = json.Unmarshal(resp, &respMsg)
		require.NoError(t, err)
		assert.Equal(t, int64(1), *respMsg.ID)
		assert.NotNil(t, respMsg.Result)
	})

	t.Run("Notification", func(t *testing.T) {
		transport := NewWebSocketMCPTransport(wsURL, 10*time.Second)
		defer transport.Close()

		ctx := context.Background()

		// Send notification
		msg := MCPMessage{
			JSONRPC: "2.0",
			Method:  "initialized",
		}
		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = transport.Send(ctx, msgBytes)
		require.NoError(t, err)
	})

	t.Run("Auth Header", func(t *testing.T) {
		transport := NewWebSocketMCPTransport(wsURL, 10*time.Second)
		transport.SetAuthHeader("Bearer test-token")
		defer transport.Close()

		ctx := context.Background()

		// Should connect successfully with auth
		msg := MCPMessage{
			JSONRPC: "2.0",
			Method:  "test",
		}
		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = transport.Send(ctx, msgBytes)
		require.NoError(t, err)
	})

	t.Run("Multiple Messages", func(t *testing.T) {
		transport := NewWebSocketMCPTransport(wsURL, 10*time.Second)
		defer transport.Close()

		ctx := context.Background()

		// Send multiple messages
		for i := int64(1); i <= 3; i++ {
			msg := MCPMessage{
				JSONRPC: "2.0",
				ID:      &i,
				Method:  "test",
			}
			msgBytes, err := json.Marshal(msg)
			require.NoError(t, err)

			err = transport.Send(ctx, msgBytes)
			require.NoError(t, err)
		}

		// Receive responses
		for i := int64(1); i <= 3; i++ {
			resp, err := transport.Receive(ctx)
			require.NoError(t, err)

			var respMsg MCPMessage
			err = json.Unmarshal(resp, &respMsg)
			require.NoError(t, err)
			assert.NotNil(t, respMsg.ID)
		}
	})

	t.Run("Reconnect", func(t *testing.T) {
		transport := NewWebSocketMCPTransport(wsURL, 10*time.Second)
		defer transport.Close()

		ctx := context.Background()

		// Send first message
		id := int64(1)
		msg := MCPMessage{
			JSONRPC: "2.0",
			ID:      &id,
			Method:  "test",
		}
		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = transport.Send(ctx, msgBytes)
		require.NoError(t, err)

		// Force close server connection
		serverMu.Lock()
		if serverConn != nil {
			serverConn.Close()
		}
		serverMu.Unlock()

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Should reconnect on next send
		id = 2
		msg.ID = &id
		msgBytes, err = json.Marshal(msg)
		require.NoError(t, err)

		err = transport.Send(ctx, msgBytes)
		require.NoError(t, err)
	})

	t.Run("Close", func(t *testing.T) {
		transport := NewWebSocketMCPTransport(wsURL, 10*time.Second)

		ctx := context.Background()

		// Connect first
		msg := MCPMessage{
			JSONRPC: "2.0",
			Method:  "test",
		}
		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = transport.Send(ctx, msgBytes)
		require.NoError(t, err)

		// Close
		err = transport.Close()
		require.NoError(t, err)

		// Should error after close
		err = transport.Send(ctx, msgBytes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})

	t.Run("Ping Keepalive", func(t *testing.T) {
		// Create server that tracks pings
		pingCount := 0
		pingMu := sync.Mutex{}

		pingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			// Set ping handler
			conn.SetPingHandler(func(appData string) error {
				pingMu.Lock()
				pingCount++
				pingMu.Unlock()
				return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
			})

			// Keep connection open
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					break
				}
			}
		}))
		defer pingServer.Close()

		wsURL := "ws" + strings.TrimPrefix(pingServer.URL, "http")

		// Create transport with short ping interval for testing
		transport := NewWebSocketTransport(wsURL, 10*time.Second)
		transport.pingTicker = time.NewTicker(100 * time.Millisecond)

		ctx := context.Background()

		// Connect
		err := transport.Connect(ctx)
		require.NoError(t, err)
		defer transport.Close()

		// Wait for pings
		time.Sleep(350 * time.Millisecond)

		pingMu.Lock()
		count := pingCount
		pingMu.Unlock()

		assert.GreaterOrEqual(t, count, 2)
	})
}

func TestWebSocketTransport_InvalidURL(t *testing.T) {
	transport := NewWebSocketMCPTransport("ws://invalid.test.url:99999", 1*time.Second)
	defer transport.Close()

	ctx := context.Background()
	msg := MCPMessage{
		JSONRPC: "2.0",
		Method:  "test",
	}
	msgBytes, err := json.Marshal(msg)
	require.NoError(t, err)

	err = transport.Send(ctx, msgBytes)
	assert.Error(t, err)
}
