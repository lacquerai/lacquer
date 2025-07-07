package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPServer creates a mock MCP server for testing
type mockMCPServer struct {
	server    *httptest.Server
	tools     []MCPTool
	transport string // "http" or "websocket"
}

func newMockHTTPMCPServer(t *testing.T) *mockMCPServer {
	mock := &mockMCPServer{
		transport: "http",
		tools: []MCPTool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}}}`),
			},
			{
				Name:        "another_tool",
				Description: "Another test tool",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
	}

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check content type
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request
		var req MCPMessage
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Handle different methods
		var response MCPMessage
		response.JSONRPC = "2.0"
		response.ID = req.ID

		switch req.Method {
		case "initialize":
			response.Result = json.RawMessage(`{
				"protocolVersion": "0.1.0",
				"serverInfo": {
					"name": "mock-mcp-server",
					"version": "1.0.0"
				},
				"capabilities": {
					"tools": {}
				}
			}`)

		case "initialized":
			// No response for notifications
			w.WriteHeader(http.StatusOK)
			return

		case "tools/list":
			toolsResp := struct {
				Tools []MCPTool `json:"tools"`
			}{Tools: mock.tools}
			respBytes, _ := json.Marshal(toolsResp)
			response.Result = respBytes

		case "tools/call":
			// Parse parameters
			var params map[string]interface{}
			json.Unmarshal(req.Params, &params)

			toolName, _ := params["name"].(string)
			result := map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Executed " + toolName + " successfully",
					},
				},
			}
			respBytes, _ := json.Marshal(result)
			response.Result = respBytes

		default:
			response.Error = &MCPError{
				Code:    -32601,
				Message: "Method not found",
			}
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	return mock
}

func newMockWebSocketMCPServer(t *testing.T) *mockMCPServer {
	mock := &mockMCPServer{
		transport: "websocket",
		tools: []MCPTool{
			{
				Name:        "ws_tool",
				Description: "A WebSocket test tool",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			if messageType != websocket.TextMessage {
				continue
			}

			// Parse request
			var req MCPMessage
			if err := json.Unmarshal(message, &req); err != nil {
				continue
			}

			// Handle different methods
			var response MCPMessage
			response.JSONRPC = "2.0"
			response.ID = req.ID

			switch req.Method {
			case "initialize":
				response.Result = json.RawMessage(`{
					"protocolVersion": "0.1.0",
					"serverInfo": {
						"name": "mock-ws-mcp-server",
						"version": "1.0.0"
					},
					"capabilities": {
						"tools": {}
					}
				}`)

			case "initialized":
				// No response for notifications
				continue

			case "tools/list":
				toolsResp := struct {
					Tools []MCPTool `json:"tools"`
				}{Tools: mock.tools}
				respBytes, _ := json.Marshal(toolsResp)
				response.Result = respBytes

			case "tools/call":
				// Parse parameters
				var params map[string]interface{}
				json.Unmarshal(req.Params, &params)

				toolName, _ := params["name"].(string)
				result := map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "WebSocket executed " + toolName,
						},
					},
				}
				respBytes, _ := json.Marshal(result)
				response.Result = respBytes

			default:
				response.Error = &MCPError{
					Code:    -32601,
					Message: "Method not found",
				}
			}

			// Send response if it has an ID
			if response.ID != nil {
				respBytes, _ := json.Marshal(response)
				conn.WriteMessage(websocket.TextMessage, respBytes)
			}
		}
	}))

	return mock
}

func (m *mockMCPServer) URL() string {
	if m.transport == "websocket" {
		return "ws" + strings.TrimPrefix(m.server.URL, "http")
	}
	return m.server.URL
}

func (m *mockMCPServer) Close() {
	m.server.Close()
}

func TestMCPServer_RemoteHTTP(t *testing.T) {
	mock := newMockHTTPMCPServer(t)
	defer mock.Close()

	config := &ast.MCPServerConfig{
		Type: "remote",
		URL:  mock.URL(),
	}

	server := NewMCPServer(config)
	defer server.Close()

	ctx := context.Background()

	// Initialize
	err := server.Initialize(ctx)
	require.NoError(t, err)

	// Discover tools
	tools, err := server.DiscoverTools(ctx)
	require.NoError(t, err)
	assert.Len(t, tools, 2)
	assert.Equal(t, "test_tool", tools[0].Name)
	assert.Equal(t, "another_tool", tools[1].Name)

	// Execute tool
	params := json.RawMessage(`{"input": "test"}`)
	result, err := server.ExecuteTool(ctx, "test_tool", params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	content, ok := result["content"].([]interface{})
	require.True(t, ok)
	require.Len(t, content, 1)

	textContent := content[0].(map[string]interface{})
	assert.Equal(t, "text", textContent["type"])
	assert.Equal(t, "Executed test_tool successfully", textContent["text"])
}

func TestMCPServer_RemoteWebSocket(t *testing.T) {
	mock := newMockWebSocketMCPServer(t)
	defer mock.Close()

	config := &ast.MCPServerConfig{
		Type: "remote",
		URL:  mock.URL(),
	}

	server := NewMCPServer(config)
	defer server.Close()

	ctx := context.Background()

	// Initialize
	err := server.Initialize(ctx)
	require.NoError(t, err)

	// Discover tools
	tools, err := server.DiscoverTools(ctx)
	require.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "ws_tool", tools[0].Name)

	// Execute tool
	params := json.RawMessage(`{}`)
	result, err := server.ExecuteTool(ctx, "ws_tool", params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	content, ok := result["content"].([]interface{})
	require.True(t, ok)
	require.Len(t, content, 1)

	textContent := content[0].(map[string]interface{})
	assert.Equal(t, "text", textContent["type"])
	assert.Equal(t, "WebSocket executed ws_tool", textContent["text"])
}

func TestMCPServer_RemoteWithAuth(t *testing.T) {
	// Create server that checks auth
	authChecked := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		authChecked = true

		// Simple response
		var req MCPMessage
		json.NewDecoder(r.Body).Decode(&req)

		response := MCPMessage{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"status":"ok"}`),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &ast.MCPServerConfig{
		Type: "remote",
		URL:  server.URL,
		Auth: &ast.MCPAuthConfig{
			Type:   "api_key",
			APIKey: "test-api-key",
		},
	}

	mcpServer := NewMCPServer(config)
	defer mcpServer.Close()

	ctx := context.Background()
	err := mcpServer.Initialize(ctx)
	require.NoError(t, err)
	assert.True(t, authChecked)
}

func TestMCPServer_RemoteWithTimeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	timeout := &ast.Duration{Duration: 50 * time.Millisecond}
	config := &ast.MCPServerConfig{
		Type:    "remote",
		URL:     server.URL,
		Timeout: timeout,
	}

	mcpServer := NewMCPServer(config)
	defer mcpServer.Close()

	ctx := context.Background()
	err := mcpServer.Initialize(ctx)
	assert.Error(t, err)
}

func TestCreateTransportFromURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		auth        *ast.MCPAuthConfig
		expectError bool
		checkType   string
	}{
		{
			name:      "HTTP URL",
			url:       "http://example.com/mcp",
			checkType: "*runtime.HTTPMCPTransport",
		},
		{
			name:      "HTTPS URL",
			url:       "https://example.com/mcp",
			checkType: "*runtime.HTTPMCPTransport",
		},
		{
			name:      "WebSocket URL",
			url:       "ws://example.com/mcp",
			checkType: "*runtime.WebSocketMCPTransport",
		},
		{
			name:      "Secure WebSocket URL",
			url:       "wss://example.com/mcp",
			checkType: "*runtime.WebSocketMCPTransport",
		},
		{
			name: "With API Key Auth",
			url:  "https://example.com/mcp",
			auth: &ast.MCPAuthConfig{
				Type:   "api_key",
				APIKey: "test-key",
			},
			checkType: "*runtime.HTTPMCPTransport",
		},
		{
			name: "With OAuth2 Auth",
			url:  "https://example.com/mcp",
			auth: &ast.MCPAuthConfig{
				Type:         "oauth2",
				ClientID:     "client",
				ClientSecret: "secret",
				TokenURL:     "https://example.com/token",
			},
			checkType: "*runtime.HTTPMCPTransport",
		},
		{
			name:        "Invalid URL",
			url:         "not-a-url",
			expectError: true,
		},
		{
			name:        "Unsupported Scheme",
			url:         "ftp://example.com/mcp",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := CreateTransportFromURL(tt.url, tt.auth)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, transport)
			} else {
				require.NoError(t, err)
				require.NotNil(t, transport)

				// Check transport type
				actualType := fmt.Sprintf("%T", transport)
				assert.Equal(t, tt.checkType, actualType)

				// Clean up
				transport.Close()
			}
		})
	}
}
