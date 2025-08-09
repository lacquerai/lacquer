package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/schema"
	"github.com/lacquerai/lacquer/internal/tools"
)

// MCPToolProvider implements the ToolProvider interface for MCP servers
type MCPToolProvider struct {
	servers map[string]*Server // Map of tool name to MCP server
	mu      sync.RWMutex
}

// NewMCPToolProvider creates a new MCP tool provider
func NewMCPToolProvider() *MCPToolProvider {
	return &MCPToolProvider{
		servers: make(map[string]*Server),
	}
}

// GetType returns the tool type this provider handles
func (p *MCPToolProvider) GetType() ast.ToolType {
	return ast.ToolTypeMCP
}

// AddToolDefinition adds an MCP tool to the provider
func (p *MCPToolProvider) AddToolDefinition(tool *ast.Tool) ([]tools.Tool, error) {
	if tool.MCPServer == nil {
		return nil, fmt.Errorf("MCP server configuration is required for MCP tools")
	}

	if err := validateMCPConfig(tool.MCPServer); err != nil {
		return nil, fmt.Errorf("invalid MCP server configuration: %w", err)
	}

	server, err := p.getOrCreateServer(tool)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP server: %w", err)
	}

	toolsList := make([]tools.Tool, len(server.tools))
	p.mu.Lock()
	for i, tool := range server.tools {
		var jsonSchema schema.JSON
		err := json.Unmarshal(tool.InputSchema, &jsonSchema)
		if err != nil {
			p.mu.Unlock()
			return nil, fmt.Errorf("failed to unmarshal JSON schema: %w", err)
		}
		p.servers[tool.Name] = server
		toolsList[i] = tools.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  jsonSchema,
		}
	}
	p.mu.Unlock()

	return toolsList, nil
}

// ExecuteTool executes an MCP tool
func (p *MCPToolProvider) ExecuteTool(execCtx *execcontext.ExecutionContext, toolName string, parameters json.RawMessage) (*tools.Result, error) {
	p.mu.RLock()
	server, exists := p.servers[toolName]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("MCP tool '%s' not found", toolName)
	}

	result, err := server.ExecuteTool(execCtx, toolName, parameters)
	if err != nil {
		return &tools.Result{
			ToolName: toolName,
			Success:  false,
			Error:    err.Error(),
			Metadata: map[string]interface{}{
				"server_type": server.config.Type,
			},
		}, nil //nolint:nilerr // Intentional: converting error to result object
	}

	return &tools.Result{
		ToolName: toolName,
		Success:  true,
		Output:   result,
		Metadata: map[string]interface{}{
			"server_type": server.config.Type,
		},
	}, nil
}

// Close shuts down all MCP servers
func (p *MCPToolProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errors []error
	for _, server := range p.servers {
		if err := server.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing MCP servers: %v", errors)
	}

	return nil
}

// getOrCreateServer gets an existing server or creates a new one
func (p *MCPToolProvider) getOrCreateServer(tool *ast.Tool) (*Server, error) {
	config := tool.MCPServer

	server := NewServer(config)

	if err := server.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	tools, err := server.DiscoverTools(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to discover tools: %w", err)
	}
	server.tools = tools

	return server, nil
}

// validateMCPConfig validates the MCP server configuration
func validateMCPConfig(config *ast.MCPServerConfig) error {
	if config.Type == "" {
		config.Type = "local" // Default to local
	}

	switch config.Type {
	case "local":
		if config.Command == "" {
			return fmt.Errorf("command is required for local MCP servers")
		}
	case "remote":
		if config.URL == "" {
			return fmt.Errorf("URL is required for remote MCP servers")
		}
	default:
		return fmt.Errorf("invalid MCP server type: %s", config.Type)
	}

	if config.Auth != nil {
		if err := validateAuthConfig(config.Auth); err != nil {
			return fmt.Errorf("invalid auth configuration: %w", err)
		}
	}

	return nil
}

// validateAuthConfig validates the authentication configuration
func validateAuthConfig(auth *ast.MCPAuthConfig) error {
	switch auth.Type {
	case "oauth2":
		if auth.ClientID == "" || auth.ClientSecret == "" || auth.TokenURL == "" {
			return fmt.Errorf("client_id, client_secret, and token_url are required for OAuth2")
		}
	case "api_key":
		if auth.APIKey == "" {
			return fmt.Errorf("api_key is required for API key authentication")
		}
	case "basic":
		if auth.Username == "" || auth.Password == "" {
			return fmt.Errorf("username and password are required for basic authentication")
		}
	case "none":
		// No validation needed
	default:
		return fmt.Errorf("invalid auth type: %s", auth.Type)
	}

	return nil
}
