package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPMCPTransport_SendReceive(t *testing.T) {
	// Create test server
	responses := map[int64]string{
		1: `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"test_tool","description":"A test tool"}]}}`,
		2: `{"jsonrpc":"2.0","id":2,"result":{"output":"test output"}}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request
		var req MCPMessage
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Send response based on ID
		if req.ID != nil {
			if resp, ok := responses[*req.ID]; ok {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(resp))
				return
			}
		}

		// For notifications, just return OK
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create transport
	transport := NewHTTPMCPTransport(server.URL, 10*time.Second)

	// Test RPC call
	t.Run("RPC Call", func(t *testing.T) {
		ctx := context.Background()
		id := int64(1)
		msg := MCPMessage{
			JSONRPC: "2.0",
			ID:      &id,
			Method:  "tools/list",
		}

		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		// Send request
		err = transport.Send(ctx, msgBytes)
		require.NoError(t, err)

		// Receive response
		resp, err := transport.Receive(ctx)
		require.NoError(t, err)

		// Verify response
		var respMsg MCPMessage
		err = json.Unmarshal(resp, &respMsg)
		require.NoError(t, err)
		assert.Equal(t, int64(1), *respMsg.ID)
		assert.NotNil(t, respMsg.Result)
	})

	// Test notification
	t.Run("Notification", func(t *testing.T) {
		ctx := context.Background()
		msg := MCPMessage{
			JSONRPC: "2.0",
			Method:  "initialized",
		}

		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		// Send notification (no response expected)
		err = transport.Send(ctx, msgBytes)
		require.NoError(t, err)
	})

	// Test auth header
	t.Run("Auth Header", func(t *testing.T) {
		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check auth header
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer authServer.Close()

		authTransport := NewHTTPMCPTransport(authServer.URL, 10*time.Second)
		authTransport.SetAuthHeader("Bearer test-token")

		ctx := context.Background()
		msg := MCPMessage{
			JSONRPC: "2.0",
			Method:  "test",
		}

		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = authTransport.Send(ctx, msgBytes)
		require.NoError(t, err)
	})

	// Test error response
	t.Run("HTTP Error", func(t *testing.T) {
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer errorServer.Close()

		errorTransport := NewHTTPMCPTransport(errorServer.URL, 10*time.Second)

		ctx := context.Background()
		id := int64(1)
		msg := MCPMessage{
			JSONRPC: "2.0",
			ID:      &id,
			Method:  "test",
		}

		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = errorTransport.Send(ctx, msgBytes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP error 500")
	})

	// Test timeout
	t.Run("Timeout", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer slowServer.Close()

		// Create transport with short timeout
		timeoutTransport := NewHTTPMCPTransport(slowServer.URL, 10*time.Millisecond)

		ctx := context.Background()
		msg := MCPMessage{
			JSONRPC: "2.0",
			Method:  "test",
		}

		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = timeoutTransport.Send(ctx, msgBytes)
		assert.Error(t, err)
	})

	// Test close
	t.Run("Close", func(t *testing.T) {
		closeTransport := NewHTTPMCPTransport(server.URL, 10*time.Second)

		err := closeTransport.Close()
		require.NoError(t, err)

		// Should error after close
		ctx := context.Background()
		msg := MCPMessage{
			JSONRPC: "2.0",
			Method:  "test",
		}

		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		err = closeTransport.Send(ctx, msgBytes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})
}

func TestHTTPRequestResponseTransport(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo request back
		var req json.RawMessage
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(req)
		require.NoError(t, err)
	}))
	defer server.Close()

	transport := NewHTTPRequestResponseTransport(server.URL, 10*time.Second)

	t.Run("SendAndReceive", func(t *testing.T) {
		ctx := context.Background()
		msg := map[string]string{"test": "message"}
		msgBytes, err := json.Marshal(msg)
		require.NoError(t, err)

		resp, err := transport.SendAndReceive(ctx, msgBytes)
		require.NoError(t, err)

		var respMsg map[string]string
		err = json.Unmarshal(resp, &respMsg)
		require.NoError(t, err)
		assert.Equal(t, msg, respMsg)
	})

	t.Run("Close", func(t *testing.T) {
		err := transport.Close()
		require.NoError(t, err)

		// Should error after close
		ctx := context.Background()
		_, err = transport.SendAndReceive(ctx, []byte("{}"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})
}
