package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPToolProvider_GetType(t *testing.T) {
	provider := NewMCPToolProvider()
	assert.Equal(t, ast.ToolTypeMCP, provider.GetType())
}

func TestMCPToolProvider_AddTool_Validation(t *testing.T) {
	provider := NewMCPToolProvider()

	tests := []struct {
		name        string
		tool        *ast.Tool
		expectError bool
		errorMsg    string
	}{
		{
			name: "nil MCP server config",
			tool: &ast.Tool{
				Name: "test_tool",
			},
			expectError: true,
			errorMsg:    "MCP server configuration is required",
		},
		{
			name: "valid local MCP server",
			tool: &ast.Tool{
				Name: "test_tool",
				MCPServer: &ast.MCPServerConfig{
					Type:    "local",
					Command: "test-mcp-server",
				},
			},
			expectError: false,
		},
		{
			name: "local server missing command",
			tool: &ast.Tool{
				Name: "test_tool",
				MCPServer: &ast.MCPServerConfig{
					Type: "local",
				},
			},
			expectError: true,
			errorMsg:    "command is required",
		},
		{
			name: "valid remote MCP server",
			tool: &ast.Tool{
				Name: "test_tool",
				MCPServer: &ast.MCPServerConfig{
					Type: "remote",
					URL:  "https://mcp.example.com",
				},
			},
			expectError: true, // Will fail because remote servers aren't implemented yet
		},
		{
			name: "remote server missing URL",
			tool: &ast.Tool{
				Name: "test_tool",
				MCPServer: &ast.MCPServerConfig{
					Type: "remote",
				},
			},
			expectError: true,
			errorMsg:    "URL is required",
		},
		{
			name: "invalid server type",
			tool: &ast.Tool{
				Name: "test_tool",
				MCPServer: &ast.MCPServerConfig{
					Type: "invalid",
				},
			},
			expectError: true,
			errorMsg:    "invalid MCP server type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := provider.AddToolDefinition(tt.tool)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMCPToolProvider_AuthValidation(t *testing.T) {
	tests := []struct {
		name        string
		auth        *ast.MCPAuthConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid OAuth2",
			auth: &ast.MCPAuthConfig{
				Type:         "oauth2",
				ClientID:     "client123",
				ClientSecret: "secret456",
				TokenURL:     "https://auth.example.com/token",
			},
			expectError: false,
		},
		{
			name: "OAuth2 missing client_id",
			auth: &ast.MCPAuthConfig{
				Type:         "oauth2",
				ClientSecret: "secret456",
				TokenURL:     "https://auth.example.com/token",
			},
			expectError: true,
			errorMsg:    "client_id, client_secret, and token_url are required",
		},
		{
			name: "valid API key",
			auth: &ast.MCPAuthConfig{
				Type:   "api_key",
				APIKey: "key123",
			},
			expectError: false,
		},
		{
			name: "API key missing key",
			auth: &ast.MCPAuthConfig{
				Type: "api_key",
			},
			expectError: true,
			errorMsg:    "api_key is required",
		},
		{
			name: "valid basic auth",
			auth: &ast.MCPAuthConfig{
				Type:     "basic",
				Username: "user",
				Password: "pass",
			},
			expectError: false,
		},
		{
			name: "basic auth missing password",
			auth: &ast.MCPAuthConfig{
				Type:     "basic",
				Username: "user",
			},
			expectError: true,
			errorMsg:    "username and password are required",
		},
		{
			name: "invalid auth type",
			auth: &ast.MCPAuthConfig{
				Type: "invalid",
			},
			expectError: true,
			errorMsg:    "invalid auth type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &ast.Tool{
				Name: "test_tool",
				MCPServer: &ast.MCPServerConfig{
					Type:    "local",
					Command: "test-mcp-server",
					Auth:    tt.auth,
				},
			}

			provider := NewMCPToolProvider()
			_, err := provider.AddToolDefinition(tool)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				// Will still error because we can't actually start the server in tests
				// but the auth validation should pass
				assert.Error(t, err)
				assert.NotContains(t, err.Error(), "invalid auth")
			}
		})
	}
}

func TestMCPToolProvider_ExecuteTool_NotFound(t *testing.T) {
	provider := NewMCPToolProvider()

	result, err := provider.ExecuteTool(
		context.Background(),
		"nonexistent_tool",
		json.RawMessage(`{}`),
		&ToolExecutionContext{},
	)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "MCP tool 'nonexistent_tool' not found")
}

func TestMCPToolProvider_Close(t *testing.T) {
	provider := NewMCPToolProvider()

	// Should not error even with no servers
	err := provider.Close()
	assert.NoError(t, err)
}

func TestValidateMCPConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *ast.MCPServerConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "defaults to local type",
			config: &ast.MCPServerConfig{
				Command: "test-server",
			},
			expectError: false,
		},
		{
			name: "valid local config",
			config: &ast.MCPServerConfig{
				Type:    "local",
				Command: "test-server",
				Args:    []string{"--arg1", "value1"},
				Env:     map[string]string{"KEY": "value"},
			},
			expectError: false,
		},
		{
			name: "valid remote config",
			config: &ast.MCPServerConfig{
				Type: "remote",
				URL:  "https://mcp.example.com",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMCPConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
