package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/schema"
)

// ExecutionContext provides context for tool execution
type ExecutionContext struct {
	WorkflowID string
	StepID     string
	AgentID    string
	RunID      string
	Context    context.Context
	Timeout    time.Duration
}

// Result represents the result of a tool execution
type Result struct {
	ToolName string                 `json:"tool_name"`
	Success  bool                   `json:"success"`
	Output   map[string]interface{} `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Tool represents a tool available to an agent
// This is a simplified version of the ast.Tool type
// which is used to provide
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  schema.JSON `json:"parameters"`
}

// ToolProvider defines the interface for tool providers
type Provider interface {
	// GetType returns the tool type this provider handles
	GetType() ast.ToolType

	// AddToolDefinition adds a tool definition to the provider
	// and returns the tools available from the definition.
	// This could return multiple tools if the definition is a MCP server.
	AddToolDefinition(tool *ast.Tool) ([]Tool, error)

	// ExecuteTool executes a tool with the given parameters
	ExecuteTool(execCtx *execcontext.ExecutionContext, toolName string, parameters json.RawMessage) (*Result, error)

	// Close cleans up resources
	Close() error
}

// ToolRegistry manages tool providers and available tools
type Registry struct {
	providers      map[ast.ToolType]Provider
	toolsProviders map[string]Provider
	agentTools     map[string][]Tool
	mu             sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		providers:      make(map[ast.ToolType]Provider),
		toolsProviders: make(map[string]Provider),
		agentTools:     make(map[string][]Tool),
	}
}

// RegisterProvider registers a tool provider
func (tr *Registry) RegisterProvider(provider Provider) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	name := provider.GetType()
	if _, exists := tr.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	tr.providers[name] = provider
	return nil
}

// RegisterToolsForAgent registers tools for an agent
// This will add the tools to the tool registry and return the tools available from the agent.
func (tr *Registry) RegisterToolsForAgent(agent *ast.Agent) error {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	for _, tool := range agent.Tools {
		provider, exists := tr.providers[tool.Type()]
		if !exists {
			return fmt.Errorf("provider for tool type %s not found", tool.Type())
		}

		tools, err := provider.AddToolDefinition(tool)
		if err != nil {
			return fmt.Errorf("failed to add tool %s: %w", tool.Name, err)
		}

		for _, tool := range tools {
			if _, exists := tr.agentTools[tool.Name]; exists {
				return fmt.Errorf("tool %s already registered by provider %s", tool.Name, tr.toolsProviders[tool.Name].GetType())
			}

			tr.agentTools[agent.Name] = append(tr.agentTools[agent.Name], tool)
			tr.toolsProviders[tool.Name] = provider
		}
	}

	return nil
}

// GetProvider returns a provider by name
func (tr *Registry) ExecuteTool(execCtx *execcontext.ExecutionContext, toolName string, parameters json.RawMessage) (*Result, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	provider, exists := tr.toolsProviders[toolName]
	if !exists {
		return nil, fmt.Errorf("provider for tool %s not found", toolName)
	}

	return provider.ExecuteTool(execCtx, toolName, parameters)
}

func (tr *Registry) GetToolsForAgent(agentName string) []Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tools := tr.agentTools[agentName]

	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	return tools
}
