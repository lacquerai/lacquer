package block

import (
	"context"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
)

// RuntimeType defines the execution runtime for a block
type RuntimeType string

const (
	RuntimeNative RuntimeType = "native"
	RuntimeDocker RuntimeType = "docker"
	RuntimeBash   RuntimeType = "bash"
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
	Workflow *ast.Workflow     `yaml:"workflow,omitempty"` // For native blocks
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

// ExecutionInput represents the JSON input sent to blocks
type ExecutionInput struct {
	Inputs map[string]interface{} `json:"inputs"`
	Env    map[string]string      `json:"-"`
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
	Execute(execCtx *execcontext.ExecutionContext, block *Block, inputs map[string]interface{}) (interface{}, error)

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
