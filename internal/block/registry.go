package block

import (
	"fmt"
	"sync"

	"github.com/lacquerai/lacquer/internal/execcontext"
)

// ExecutorRegistry manages executors for different runtime types
type ExecutorRegistry struct {
	executors map[RuntimeType]Executor
	mu        sync.RWMutex
}

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

// Execute runs a block using the appropriate executor and validates the block
func (r *ExecutorRegistry) Execute(execCtx *execcontext.ExecutionContext, block *Block, inputs map[string]interface{}) (interface{}, error) {
	executor, ok := r.Get(block.Runtime)
	if !ok {
		return nil, fmt.Errorf("no executor registered for runtime: %s", block.Runtime)
	}

	if err := executor.Validate(block); err != nil {
		return nil, fmt.Errorf("block validation failed: %w", err)
	}

	outputs, err := executor.Execute(execCtx, block, inputs)
	if err != nil {
		return nil, fmt.Errorf("block execution failed: %w", err)
	}

	return outputs, nil
}
