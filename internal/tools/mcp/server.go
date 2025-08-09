package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/rs/zerolog/log"
)

// Server represents a connection to an MCP server
type Server struct {
	config    *ast.MCPServerConfig
	client    *MCPClient
	process   *exec.Cmd // For local servers
	tools     []Tool
	mu        sync.RWMutex
	closed    bool
	closeChan chan struct{}
}

// Tool represents a tool available from an MCP server
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// NewMCPServer creates a new MCP server instance
func NewServer(config *ast.MCPServerConfig) *Server {
	return &Server{
		config:    config,
		closeChan: make(chan struct{}),
	}
}

// Initialize initializes the MCP server connection
func (s *Server) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("server is closed")
	}

	switch s.config.Type {
	case "local":
		return s.initializeLocal(ctx)
	case "remote":
		return s.initializeRemote(ctx)
	default:
		return fmt.Errorf("unsupported server type: %s", s.config.Type)
	}
}

// initializeLocal starts a local MCP server process
func (s *Server) initializeLocal(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, s.config.Command, s.config.Args...)

	if len(s.config.Env) > 0 {
		env := os.Environ()
		for k, v := range s.config.Env {
			expandedValue := os.ExpandEnv(v)
			env = append(env, fmt.Sprintf("%s=%s", k, expandedValue))
		}
		cmd.Env = env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	s.process = cmd

	s.client = NewMCPClient(&StdioTransport{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	})
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				log.Debug().Msgf("MCP server stderr: %s", buf[:n])
			}

			if err != nil {
				if err != io.EOF {
					log.Debug().Msgf("Error reading MCP server stderr: %v", err)
				}
				break
			}
		}
	}()

	if err := s.client.Initialize(ctx); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	go s.monitorProcess()

	return nil
}

// initializeRemote connects to a remote MCP server
func (s *Server) initializeRemote(ctx context.Context) error {
	transport, err := CreateTransportFromURL(s.config.URL, s.config.Auth)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	s.client = NewMCPClient(transport)

	initCtx := ctx
	if s.config.Timeout != nil {
		var cancel context.CancelFunc
		initCtx, cancel = context.WithTimeout(ctx, s.config.Timeout.Duration)
		defer cancel()
	}

	if err := s.client.Initialize(initCtx); err != nil {
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return nil
}

// DiscoverTools discovers available tools from the MCP server
func (s *Server) DiscoverTools(ctx context.Context) ([]Tool, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("server is closed")
	}
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("MCP client not initialized")
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	s.mu.Lock()
	s.tools = tools
	s.mu.Unlock()

	return tools, nil
}

// ExecuteTool executes a tool on the MCP server
func (s *Server) ExecuteTool(execCtx *execcontext.ExecutionContext, toolName string, parameters json.RawMessage) (map[string]interface{}, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("server is closed")
	}
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("MCP client not initialized")
	}

	result, err := client.CallTool(execCtx.Context.Context, toolName, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool: %w", err)
	}

	return result, nil
}

// Close closes the MCP server connection
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.closeChan)

	if s.client != nil {
		if err := s.client.Close(); err != nil {
			return fmt.Errorf("failed to close MCP client: %w", err)
		}
	}

	if s.process != nil {
		if err := s.process.Process.Signal(os.Interrupt); err != nil {
			s.process.Process.Kill()
		}

		done := make(chan error, 1)
		go func() {
			done <- s.process.Wait()
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			s.process.Process.Kill()
		}
	}

	return nil
}

// monitorProcess monitors a local MCP server process
func (s *Server) monitorProcess() {
	if s.process == nil {
		return
	}

	err := s.process.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed && err != nil {
		// Process exited unexpectedly
		// TODO: Add proper error handling/recovery
	}
}

// StdioTransport implements MCP transport over stdio
type StdioTransport struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// Send sends a message over stdio
func (t *StdioTransport) Send(ctx context.Context, message []byte) error {
	_, err := t.stdin.Write(message)
	if err != nil {
		return err
	}
	_, err = t.stdin.Write([]byte("\n"))
	return err
}

// Receive receives a message from stdio
func (t *StdioTransport) Receive(ctx context.Context) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 1)

	for {
		n, err := t.stdout.Read(tmp)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			if tmp[0] == '\n' {
				break
			}
			buf = append(buf, tmp[0])
		}
	}

	return buf, nil
}

// Close closes the stdio pipes
func (t *StdioTransport) Close() error {
	var errs []error

	if err := t.stdin.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := t.stdout.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := t.stderr.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing stdio: %v", errs)
	}

	return nil
}

// CreateTransportFromURL creates a transport based on the URL scheme
func CreateTransportFromURL(serverURL string, auth *ast.MCPAuthConfig) (MCPTransport, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	authProvider, err := CreateAuthProvider(auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth provider: %w", err)
	}

	timeout := 30 * time.Second

	var transport MCPTransport
	switch u.Scheme {
	case "http", "https":
		httpTransport := NewHTTPMCPTransport(serverURL, timeout)

		if authProvider != nil {
			authHeader, err := authProvider.GetAuthHeader(context.Background())
			if err != nil {
				return nil, fmt.Errorf("failed to get auth header: %w", err)
			}
			if authHeader != "" {
				httpTransport.SetAuthHeader(authHeader)
			}
		}

		transport = httpTransport

	case "ws", "wss":
		wsTransport := NewWebSocketMCPTransport(serverURL, timeout)

		if authProvider != nil {
			authHeader, err := authProvider.GetAuthHeader(context.Background())
			if err != nil {
				return nil, fmt.Errorf("failed to get auth header: %w", err)
			}
			if authHeader != "" {
				wsTransport.SetAuthHeader(authHeader)
			}
		}

		transport = wsTransport

	default:
		return nil, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	return transport, nil
}
