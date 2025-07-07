package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketTransport implements MCP transport over WebSocket
type WebSocketTransport struct {
	url         string
	conn        *websocket.Conn
	authHeader  string
	mu          sync.Mutex
	closed      bool
	closeChan   chan struct{}
	reconnect   bool
	pingTicker  *time.Ticker
	readTimeout time.Duration
}

// NewWebSocketTransport creates a new WebSocket transport
func NewWebSocketTransport(url string, timeout time.Duration) *WebSocketTransport {
	return &WebSocketTransport{
		url:         url,
		closeChan:   make(chan struct{}),
		reconnect:   true,
		readTimeout: timeout,
	}
}

// SetAuthHeader sets the authorization header for the WebSocket handshake
func (t *WebSocketTransport) SetAuthHeader(header string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.authHeader = header
}

// Connect establishes the WebSocket connection
func (t *WebSocketTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	// Set up dialer with auth header
	dialer := websocket.DefaultDialer
	header := http.Header{}
	if t.authHeader != "" {
		header.Set("Authorization", t.authHeader)
	}

	// Connect with context
	conn, _, err := dialer.DialContext(ctx, t.url, header)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	t.conn = conn

	// Start ping ticker to keep connection alive
	t.pingTicker = time.NewTicker(30 * time.Second)
	go t.pingLoop()

	return nil
}

// Send sends a message over the WebSocket
func (t *WebSocketTransport) Send(ctx context.Context, message []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	if t.conn == nil {
		// Try to reconnect if allowed
		if t.reconnect {
			t.mu.Unlock()
			if err := t.Connect(ctx); err != nil {
				return err
			}
			t.mu.Lock()
		} else {
			return fmt.Errorf("not connected")
		}
	}

	// Set write deadline based on context
	deadline, ok := ctx.Deadline()
	if ok {
		t.conn.SetWriteDeadline(deadline)
	}

	// Send message
	err := t.conn.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		// Connection error, close and allow reconnect
		t.conn.Close()
		t.conn = nil
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// Receive receives a message from the WebSocket
func (t *WebSocketTransport) Receive(ctx context.Context) ([]byte, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, fmt.Errorf("transport is closed")
	}

	if t.conn == nil {
		t.mu.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	conn := t.conn
	t.mu.Unlock()

	// Set read deadline if timeout is configured
	if t.readTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(t.readTimeout))
	}

	// Read message
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		t.mu.Lock()
		// Connection error, close and allow reconnect
		if t.conn != nil {
			t.conn.Close()
			t.conn = nil
		}
		t.mu.Unlock()
		return nil, fmt.Errorf("failed to receive message: %w", err)
	}

	if messageType != websocket.TextMessage {
		return nil, fmt.Errorf("unexpected message type: %d", messageType)
	}

	return message, nil
}

// Close closes the WebSocket connection
func (t *WebSocketTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	t.reconnect = false
	close(t.closeChan)

	// Stop ping ticker
	if t.pingTicker != nil {
		t.pingTicker.Stop()
	}

	// Close WebSocket connection
	if t.conn != nil {
		// Send close message
		deadline := time.Now().Add(5 * time.Second)
		t.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			deadline)

		// Close connection
		err := t.conn.Close()
		t.conn = nil
		return err
	}

	return nil
}

// pingLoop sends periodic ping messages to keep the connection alive
func (t *WebSocketTransport) pingLoop() {
	defer func() {
		if t.pingTicker != nil {
			t.pingTicker.Stop()
		}
	}()

	for {
		select {
		case <-t.closeChan:
			return
		case <-t.pingTicker.C:
			t.mu.Lock()
			if t.conn != nil && !t.closed {
				deadline := time.Now().Add(10 * time.Second)
				err := t.conn.WriteControl(websocket.PingMessage, nil, deadline)
				if err != nil {
					// Connection error, close it
					t.conn.Close()
					t.conn = nil
				}
			}
			t.mu.Unlock()
		}
	}
}

// WebSocketMCPTransport wraps WebSocketTransport to handle initial connection
type WebSocketMCPTransport struct {
	*WebSocketTransport
	connectOnce sync.Once
	connectErr  error
}

// NewWebSocketMCPTransport creates a new WebSocket MCP transport
func NewWebSocketMCPTransport(url string, timeout time.Duration) *WebSocketMCPTransport {
	return &WebSocketMCPTransport{
		WebSocketTransport: NewWebSocketTransport(url, timeout),
	}
}

// Send ensures connection before sending
func (t *WebSocketMCPTransport) Send(ctx context.Context, message []byte) error {
	// Ensure we're connected
	t.connectOnce.Do(func() {
		t.connectErr = t.Connect(ctx)
	})

	if t.connectErr != nil {
		return t.connectErr
	}

	return t.WebSocketTransport.Send(ctx, message)
}

// Receive ensures connection before receiving
func (t *WebSocketMCPTransport) Receive(ctx context.Context) ([]byte, error) {
	// Ensure we're connected
	t.connectOnce.Do(func() {
		t.connectErr = t.Connect(ctx)
	})

	if t.connectErr != nil {
		return nil, t.connectErr
	}

	return t.WebSocketTransport.Receive(ctx)
}
