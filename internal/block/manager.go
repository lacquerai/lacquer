package block

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lacquerai/lacquer/internal/execcontext"
)

// Manager manages block loading and execution
type Manager struct {
	loader   Loader
	registry Registry
	cacheDir string
}

// NewManager creates a new block manager
func NewManager(cacheDir string) (*Manager, error) {
	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	loader := NewFileLoader()
	registry := NewExecutorRegistry()

	// Create executors
	bashExecutor, err := NewBashExecutor(filepath.Join(cacheDir, "bash"))
	if err != nil {
		return nil, fmt.Errorf("failed to create bash executor: %w", err)
	}

	dockerExecutor := NewDockerExecutor()

	registry.Register(RuntimeBash, bashExecutor)
	registry.Register(RuntimeDocker, dockerExecutor)

	return &Manager{
		loader:   loader,
		registry: registry,
		cacheDir: cacheDir,
	}, nil
}

// RegisterNativeExecutor registers the native executor with a workflow engine
func (m *Manager) RegisterNativeExecutor(engine WorkflowEngine) {
	nativeExecutor := NewNativeExecutor(engine)
	m.registry.Register(RuntimeNative, nativeExecutor)
}

// LoadBlock loads a block from the given path
func (m *Manager) LoadBlock(ctx context.Context, path string) (*Block, error) {
	return m.loader.Load(ctx, path)
}

// ExecuteBlock executes a block at the given path with the given inputs
func (m *Manager) ExecuteBlock(execCtx *execcontext.ExecutionContext, blockPath string, inputs map[string]interface{}) (map[string]interface{}, error) {
	block, err := m.LoadBlock(execCtx.Context.Context, blockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load block: %w", err)
	}

	if err := m.validateInputs(block, inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	return m.ExecuteRawBlock(execCtx, block, inputs)
}

// ExecuteRawBlock executes a block with the given inputs, it does not validate inputs
func (m *Manager) ExecuteRawBlock(execCtx *execcontext.ExecutionContext, block *Block, inputs map[string]interface{}) (map[string]interface{}, error) {
	executor, ok := m.registry.Get(block.Runtime)
	if !ok {
		return nil, fmt.Errorf("no executor registered for runtime: %s", block.Runtime)
	}

	outputs, err := executor.Execute(execCtx, block, inputs)
	if err != nil {
		return nil, fmt.Errorf("block execution failed: %w", err)
	}

	return outputs, nil
}

// GetBlockInfo returns information about a block without executing it
func (m *Manager) GetBlockInfo(ctx context.Context, path string) (*Block, error) {
	return m.loader.Load(ctx, path)
}

// InvalidateCache invalidates the cache for a block
func (m *Manager) InvalidateCache(path string) {
	m.loader.InvalidateCache(path)
}

func (m *Manager) validateInputs(block *Block, inputs map[string]interface{}) error {
	// Check required inputs
	for name, schema := range block.Inputs {
		value, exists := inputs[name]

		if !exists {
			if schema.Required {
				return fmt.Errorf("required input '%s' is missing", name)
			}
			// Use default value if provided
			if schema.Default != nil {
				inputs[name] = schema.Default
			}
			continue
		}

		// Type validation
		if err := validateType(name, value, schema.Type); err != nil {
			return err
		}

		// Enum validation
		if len(schema.Enum) > 0 {
			if err := validateEnum(name, value, schema.Enum); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateType(name string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("'%s' must be a string, got %T", name, value)
		}
	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64:
			// Valid number types
		default:
			return fmt.Errorf("'%s' must be a number, got %T", name, value)
		}
	case "integer":
		switch v := value.(type) {
		case int, int32, int64:
			// Valid integer types
		case float64:
			// JSON numbers are always float64, check if it's a whole number
			if v != float64(int64(v)) {
				return fmt.Errorf("'%s' must be an integer, got %v", name, v)
			}
		default:
			return fmt.Errorf("'%s' must be an integer, got %T", name, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("'%s' must be a boolean, got %T", name, value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("'%s' must be an array, got %T", name, value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("'%s' must be an object, got %T", name, value)
		}
	}
	return nil
}

func validateEnum(name string, value interface{}, enum []string) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("'%s' must be a string for enum validation", name)
	}

	for _, allowed := range enum {
		if str == allowed {
			return nil
		}
	}

	return fmt.Errorf("'%s' must be one of: %v, got: %s", name, enum, str)
}
