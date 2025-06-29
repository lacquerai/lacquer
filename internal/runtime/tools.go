package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
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
	// GetType returns the tool type this provider handles
	GetType() ast.ToolType

	// AddTool adds a tool to the provider
	AddTool(tool *ast.Tool) error

	// ExecuteTool executes a tool with the given parameters
	ExecuteTool(ctx context.Context, toolName string, parameters json.RawMessage, execCtx *ToolExecutionContext) (*ToolResult, error)

	// Close cleans up resources
	Close() error
}

// ToolRegistry manages tool providers and available tools
type ToolRegistry struct {
	providers      map[ast.ToolType]ToolProvider
	toolsProviders map[string]ToolProvider
	mu             sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		providers:      make(map[ast.ToolType]ToolProvider),
		toolsProviders: make(map[string]ToolProvider),
	}
}

// RegisterProvider registers a tool provider
func (tr *ToolRegistry) RegisterProvider(provider ToolProvider) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	name := provider.GetType()
	if _, exists := tr.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	tr.providers[name] = provider
	return nil
}

func (tr *ToolRegistry) RegisterToolsForAgent(agent *ast.Agent) error {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	for _, tool := range agent.Tools {
		provider, exists := tr.providers[tool.Type()]
		if !exists {
			return fmt.Errorf("provider for tool type %s not found", tool.Type())
		}

		err := provider.AddTool(tool)
		if err != nil {
			return fmt.Errorf("failed to add tool %s: %w", tool.Name, err)
		}

		tr.toolsProviders[tool.Name] = provider
	}

	return nil
}

// GetProvider returns a provider by name
func (tr *ToolRegistry) ExecuteTool(ctx context.Context, toolName string, parameters json.RawMessage, execCtx *ToolExecutionContext) (*ToolResult, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	provider, exists := tr.toolsProviders[toolName]
	if !exists {
		return nil, fmt.Errorf("provider for tool %s not found", toolName)
	}

	return provider.ExecuteTool(ctx, toolName, parameters, execCtx)
}
