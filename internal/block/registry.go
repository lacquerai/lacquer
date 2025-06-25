package block

import (
	"context"
	"fmt"
	"sync"
)

// ExecutorRegistry manages block executors by runtime type
type ExecutorRegistry struct {
	executors map[RuntimeType]Executor
	mu        sync.RWMutex
}

// NewExecutorRegistry creates a new executor registry
func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{
		executors: make(map[RuntimeType]Executor),
	}
}

// Register registers an executor for a runtime type
func (r *ExecutorRegistry) Register(runtime RuntimeType, executor Executor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[runtime] = executor
}

// Get returns the executor for a runtime type
func (r *ExecutorRegistry) Get(runtime RuntimeType) (Executor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	executor, ok := r.executors[runtime]
	return executor, ok
}

// Execute runs a block using the appropriate executor
func (r *ExecutorRegistry) Execute(ctx context.Context, block *Block, inputs map[string]interface{}, execCtx *ExecutionContext) (map[string]interface{}, error) {
	executor, ok := r.Get(block.Runtime)
	if !ok {
		return nil, fmt.Errorf("no executor registered for runtime: %s", block.Runtime)
	}

	// Validate the block with the executor
	if err := executor.Validate(block); err != nil {
		return nil, fmt.Errorf("block validation failed: %w", err)
	}

	// Execute the block
	outputs, err := executor.Execute(ctx, block, inputs, execCtx)
	if err != nil {
		return nil, fmt.Errorf("block execution failed: %w", err)
	}

	return map[string]interface{}{
		"outputs": outputs,
	}, nil
}
