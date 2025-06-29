package block

import (
	"context"
	"time"
)

// RuntimeType defines the execution runtime for a block
type RuntimeType string

const (
	RuntimeNative RuntimeType = "native"
	RuntimeGo     RuntimeType = "go"
	RuntimeDocker RuntimeType = "docker"
)

// Block represents a reusable workflow component
type Block struct {
	Name        string                  `yaml:"name"`
	Metadata    map[string]interface{}  `yaml:"metadata,omitempty"`
	Path        string                  `yaml:"-"` // Filesystem path
	Runtime     RuntimeType             `yaml:"runtime"`
	Description string                  `yaml:"description,omitempty"`
	Inputs      map[string]InputSchema  `yaml:"inputs,omitempty"`
	Outputs     map[string]OutputSchema `yaml:"outputs,omitempty"`

	// Runtime-specific fields
	Workflow interface{}       `yaml:"workflow,omitempty"` // For native blocks
	Script   string            `yaml:"script,omitempty"`   // For go blocks
	Image    string            `yaml:"image,omitempty"`    // For docker blocks
	Command  []string          `yaml:"command,omitempty"`  // For docker blocks
	Env      map[string]string `yaml:"env,omitempty"`      // For docker blocks

	// Cached data
	ModTime      time.Time `yaml:"-"`
	CompiledPath string    `yaml:"-"` // For go blocks
}

// InputSchema defines the schema for a block input parameter
type InputSchema struct {
	Type        string      `yaml:"type"`
	Description string      `yaml:"description,omitempty"`
	Required    bool        `yaml:"required,omitempty"`
	Default     interface{} `yaml:"default,omitempty"`
	Enum        []string    `yaml:"enum,omitempty"`
	Items       interface{} `yaml:"items,omitempty"`      // For arrays
	Properties  interface{} `yaml:"properties,omitempty"` // For objects
}

// OutputSchema defines the schema for a block output parameter
type OutputSchema struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description,omitempty"`
	From        string `yaml:"from,omitempty"` // For docker blocks reading from files
}

// ExecutionContext provides context for block execution
type ExecutionContext struct {
	WorkflowID string
	StepID     string
	Workspace  string // Temporary workspace directory
	Timeout    time.Duration
	Context    context.Context
}

// ExecutionInput represents the JSON input sent to blocks
type ExecutionInput struct {
	Inputs  map[string]interface{} `json:"inputs"`
	Env     map[string]string      `json:"env"`
	Context ExecutionContextJSON   `json:"context"`
}

// ExecutionContextJSON is the JSON representation of execution context
type ExecutionContextJSON struct {
	WorkflowID string `json:"workflow_id"`
	StepID     string `json:"step_id"`
	Workspace  string `json:"workspace"`
}

// ExecutionError represents an error from block execution
type ExecutionError struct {
	Message string                 `json:"message"`
	Code    string                 `json:"code,omitempty"`
	Details string                 `json:"details,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// Loader loads blocks from the filesystem
type Loader interface {
	// Load loads a block from the given path
	Load(ctx context.Context, path string) (*Block, error)

	// GetFromCache returns a cached block if available
	GetFromCache(path string) (*Block, bool)

	// InvalidateCache removes a block from the cache
	InvalidateCache(path string)
}

// Executor executes blocks
type Executor interface {
	// Execute runs a block with the given inputs
	Execute(ctx context.Context, block *Block, inputs map[string]interface{}, execCtx *ExecutionContext) (map[string]interface{}, error)

	// Validate checks if the executor can handle the given block
	Validate(block *Block) error
}

// Registry manages block executors by runtime type
type Registry interface {
	// Register registers an executor for a runtime type
	Register(runtime RuntimeType, executor Executor)

	// Get returns the executor for a runtime type
	Get(runtime RuntimeType) (Executor, bool)
}
