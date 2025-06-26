package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
)

// ToolType represents the type of tool
type ToolType string

const (
	ToolTypeMCP    ToolType = "mcp"
	ToolTypeScript ToolType = "script"
	ToolTypeNative ToolType = "native"
)

// ToolExecutionContext provides context for tool execution
type ToolExecutionContext struct {
	WorkflowID string
	StepID     string
	AgentID    string
	RunID      string
	Context    context.Context
	Timeout    time.Duration
}

// ToolParameter represents a tool parameter definition
type ToolParameter struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Required    bool                   `json:"required,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	Enum        []string               `json:"enum,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// ToolDefinition represents a complete tool definition
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Type        ToolType        `json:"type"`
	Parameters  []ToolParameter `json:"parameters"`

	// Provider-specific configuration
	Config map[string]interface{} `json:"config,omitempty"`

	// Source information
	Source     string `json:"source,omitempty"`      // MCP server URL, script path, etc.
	ProviderID string `json:"provider_id,omitempty"` // Provider that owns this tool
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolName string                 `json:"tool_name"`
	Success  bool                   `json:"success"`
	Output   map[string]interface{} `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolProvider defines the interface for tool providers
type ToolProvider interface {
	// GetName returns the provider name
	GetName() string

	// GetType returns the tool type this provider handles
	GetType() ToolType

	// DiscoverTools discovers available tools from this provider
	DiscoverTools(ctx context.Context) ([]ToolDefinition, error)

	// ExecuteTool executes a tool with the given parameters
	ExecuteTool(ctx context.Context, toolName string, parameters map[string]interface{}, execCtx *ToolExecutionContext) (*ToolResult, error)

	// ValidateTool validates that a tool can be executed
	ValidateTool(toolDef *ToolDefinition) error

	// Close cleans up resources
	Close() error
}

// ToolRegistry manages tool providers and available tools
type ToolRegistry struct {
	providers map[string]ToolProvider
	tools     map[string]*ToolDefinition
	mu        sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		providers: make(map[string]ToolProvider),
		tools:     make(map[string]*ToolDefinition),
	}
}

// RegisterProvider registers a tool provider
func (tr *ToolRegistry) RegisterProvider(provider ToolProvider) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	name := provider.GetName()
	if _, exists := tr.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	tr.providers[name] = provider
	return nil
}

// GetProvider returns a provider by name
func (tr *ToolRegistry) GetProvider(name string) (ToolProvider, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	provider, exists := tr.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// ListProviders returns all registered provider names
func (tr *ToolRegistry) ListProviders() []string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	names := make([]string, 0, len(tr.providers))
	for name := range tr.providers {
		names = append(names, name)
	}
	return names
}

// DiscoverTools discovers tools from all registered providers
func (tr *ToolRegistry) DiscoverTools(ctx context.Context) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	// Clear existing tools
	tr.tools = make(map[string]*ToolDefinition)

	for providerName, provider := range tr.providers {
		tools, err := provider.DiscoverTools(ctx)
		if err != nil {
			return fmt.Errorf("failed to discover tools from provider %s: %w", providerName, err)
		}

		for _, tool := range tools {
			tool.ProviderID = providerName

			// Check for name conflicts
			if existing, exists := tr.tools[tool.Name]; exists {
				return fmt.Errorf("tool name conflict: %s (providers: %s, %s)",
					tool.Name, existing.ProviderID, providerName)
			}

			tr.tools[tool.Name] = &tool
		}
	}

	return nil
}

// GetTool returns a tool definition by name
func (tr *ToolRegistry) GetTool(name string) (*ToolDefinition, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tool, exists := tr.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// ListTools returns all available tool names
func (tr *ToolRegistry) ListTools() []string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	names := make([]string, 0, len(tr.tools))
	for name := range tr.tools {
		names = append(names, name)
	}
	return names
}

// GetToolsForAgent returns tools available to a specific agent
func (tr *ToolRegistry) GetToolsForAgent(agent *ast.Agent) ([]*ToolDefinition, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if agent.Tools == nil || len(agent.Tools) == 0 {
		return []*ToolDefinition{}, nil
	}

	var tools []*ToolDefinition
	for _, toolRef := range agent.Tools {
		var toolDef *ToolDefinition
		var err error

		// Handle different tool reference types
		switch {
		case toolRef.Name != "":
			// Direct tool name reference
			toolDef, err = tr.GetTool(toolRef.Name)
			if err != nil {
				return nil, fmt.Errorf("tool %s not found for agent: %w", toolRef.Name, err)
			}

		case toolRef.Uses != "":
			// TODO: Handle tool blocks (future enhancement)
			continue

		case toolRef.Script != "":
			// Create dynamic tool definition for script
			toolDef = &ToolDefinition{
				Name:        fmt.Sprintf("script_%s", toolRef.Name),
				Description: fmt.Sprintf("Script tool: %s", toolRef.Script),
				Type:        ToolTypeScript,
				Parameters:  []ToolParameter{}, // TODO: Extract from script
				Config:      toolRef.Config,
				Source:      toolRef.Script,
			}

		case toolRef.MCPServer != "":
			// TODO: Handle MCP server tools (implemented in MCP provider)
			continue
		}

		if toolDef != nil {
			tools = append(tools, toolDef)
		}
	}

	return tools, nil
}

// ExecuteTool executes a tool by name with the given parameters
func (tr *ToolRegistry) ExecuteTool(ctx context.Context, toolName string, parameters map[string]interface{}, execCtx *ToolExecutionContext) (*ToolResult, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tool, exists := tr.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	provider, exists := tr.providers[tool.ProviderID]
	if !exists {
		return nil, fmt.Errorf("provider %s not found for tool %s", tool.ProviderID, toolName)
	}

	return provider.ExecuteTool(ctx, toolName, parameters, execCtx)
}

// Close closes all providers
func (tr *ToolRegistry) Close() error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	var lastErr error
	for _, provider := range tr.providers {
		if err := provider.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// GenerateFunctionSchema generates OpenAI/Anthropic function schemas from tool definitions
func GenerateFunctionSchema(tools []*ToolDefinition, provider string) (interface{}, error) {
	switch provider {
	case "anthropic":
		return generateAnthropicToolSchema(tools)
	case "openai":
		return generateOpenAIToolSchema(tools)
	default:
		return nil, fmt.Errorf("unsupported provider for function schema: %s", provider)
	}
}

// generateAnthropicToolSchema generates Anthropic tool schema
func generateAnthropicToolSchema(tools []*ToolDefinition) ([]AnthropicTool, error) {
	var anthTools []AnthropicTool

	for _, tool := range tools {
		schema := make(map[string]interface{})
		schema["type"] = "object"

		properties := make(map[string]interface{})
		required := []string{}

		for _, param := range tool.Parameters {
			paramSchema := map[string]interface{}{
				"type": param.Type,
			}

			if param.Description != "" {
				paramSchema["description"] = param.Description
			}

			if param.Enum != nil && len(param.Enum) > 0 {
				paramSchema["enum"] = param.Enum
			}

			if param.Properties != nil {
				paramSchema["properties"] = param.Properties
			}

			properties[param.Name] = paramSchema

			if param.Required {
				required = append(required, param.Name)
			}
		}

		schema["properties"] = properties
		if len(required) > 0 {
			schema["required"] = required
		}

		anthTool := AnthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		}

		anthTools = append(anthTools, anthTool)
	}

	return anthTools, nil
}

// generateOpenAIToolSchema generates OpenAI tool schema
func generateOpenAIToolSchema(tools []*ToolDefinition) ([]OpenAITool, error) {
	var openaiTools []OpenAITool

	for _, tool := range tools {
		schema := make(map[string]interface{})
		schema["type"] = "object"

		properties := make(map[string]interface{})
		required := []string{}

		for _, param := range tool.Parameters {
			paramSchema := map[string]interface{}{
				"type": param.Type,
			}

			if param.Description != "" {
				paramSchema["description"] = param.Description
			}

			if param.Enum != nil && len(param.Enum) > 0 {
				paramSchema["enum"] = param.Enum
			}

			if param.Properties != nil {
				paramSchema["properties"] = param.Properties
			}

			properties[param.Name] = paramSchema

			if param.Required {
				required = append(required, param.Name)
			}
		}

		schema["properties"] = properties
		if len(required) > 0 {
			schema["required"] = required
		}

		openaiTool := OpenAITool{
			Type: "function",
			Function: &OpenAIFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  schema,
			},
		}

		openaiTools = append(openaiTools, openaiTool)
	}

	return openaiTools, nil
}

// ParseToolCall parses a tool call from different providers
func ParseToolCall(provider string, data interface{}) (string, map[string]interface{}, error) {
	switch provider {
	case "anthropic":
		return parseAnthropicToolCall(data)
	case "openai":
		return parseOpenAIToolCall(data)
	default:
		return "", nil, fmt.Errorf("unsupported provider for tool call parsing: %s", provider)
	}
}

// parseAnthropicToolCall parses Anthropic tool call
func parseAnthropicToolCall(data interface{}) (string, map[string]interface{}, error) {
	content, ok := data.(AnthropicContent)
	if !ok {
		return "", nil, fmt.Errorf("invalid Anthropic tool call data")
	}

	if content.Type != "tool_use" {
		return "", nil, fmt.Errorf("not a tool use content type")
	}

	return content.Name, content.Input, nil
}

// parseOpenAIToolCall parses OpenAI tool call
func parseOpenAIToolCall(data interface{}) (string, map[string]interface{}, error) {
	toolCall, ok := data.(OpenAIToolCall)
	if !ok {
		return "", nil, fmt.Errorf("invalid OpenAI tool call data")
	}

	if toolCall.Function == nil {
		return "", nil, fmt.Errorf("no function in tool call")
	}

	var parameters map[string]interface{}
	if toolCall.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &parameters); err != nil {
			return "", nil, fmt.Errorf("failed to parse function arguments: %w", err)
		}
	}

	return toolCall.Function.Name, parameters, nil
}
