# Lacquer Block System

## Overview

The Lacquer block system enables reusable workflow components through the `uses` syntax. This MVP implementation supports three runtime types for local blocks:

1. **Native blocks** - YAML workflows that compose other blocks/agents
2. **Go script blocks** - Go code executed in isolated environments  
3. **Docker blocks** - Containerized execution for maximum flexibility

## Key Features

✅ **Local Block Discovery** - Load blocks from filesystem paths  
✅ **Runtime Type Detection** - Automatic detection of native/go/docker blocks  
✅ **JSON I/O Protocol** - Language-agnostic input/output via JSON  
✅ **Type-Safe Validation** - Input/output schema validation  
✅ **Caching** - Intelligent caching with file modification detection  
✅ **Error Handling** - Clear error messages and graceful failures  
✅ **Resource Management** - Workspace isolation and cleanup  

## Architecture

```
┌─────────────────┐
│  Block Manager  │
├─────────────────┤
│   File Loader   │
│   Registry      │
│   Executors     │
└─────────────────┘
        │
        ├── Native Executor (YAML workflows)
        ├── Go Executor (script compilation)
        └── Docker Executor (container execution)
```

## Usage

### Loading and Executing Blocks

```go
// Create block manager
manager, err := block.NewManager(cacheDir)
if err != nil {
    log.Fatal(err)
}

// Execute a block
outputs, err := manager.ExecuteBlock(
    ctx,
    "./blocks/data-transformer",
    map[string]interface{}{
        "numbers": []interface{}{1.0, 2.0, 3.0},
        "operation": "sum",
    },
    "workflow-id",
    "step-id",
)
```

### Block Definition Format

All blocks use the same `block.laq.yaml` format:

```yaml
name: my-block
runtime: go  # native, go, or docker
description: Block description

inputs:
  param1:
    type: string
    required: true
  param2:
    type: number
    default: 42

outputs:
  result:
    type: string
    description: Processing result

# Runtime-specific fields
script: |  # For go blocks
  package main
  // Go code here

image: alpine:latest  # For docker blocks
command: ["./process"]
```

## JSON I/O Protocol

Blocks communicate via JSON stdin/stdout:

**Input (stdin):**
```json
{
  "inputs": {"param1": "value1", "param2": 42},
  "env": {"WORKSPACE": "/tmp/workspace"},
  "context": {"workflow_id": "wf-123", "step_id": "step-456"}
}
```

**Output (stdout):**
```json
{
  "outputs": {"result": "processed"}
}
```

**Error (stderr + exit code):**
```json
{
  "error": {"message": "Processing failed", "code": "INVALID_INPUT"}
}
```

## Example Blocks

### Go Script Block
- **Path**: `examples/blocks/data-transformer/`
- **Purpose**: Mathematical operations on arrays
- **Features**: Type safety, error handling, precision control

### Docker Block  
- **Path**: `examples/blocks/simple-calculator/`  
- **Purpose**: Python-based calculator
- **Features**: Multi-language support, containerized execution

### Native Block
- **Path**: `examples/blocks/text-analyzer/`
- **Purpose**: Text analysis using AI agents
- **Features**: Workflow composition, conditional logic

## Testing

```bash
# Run all block tests
go test ./internal/block/...

# Run specific executor tests
go test -run TestGoExecutor ./internal/block/

# Run integration demo
go run examples/test_blocks.go
```

## Future Enhancements

The current MVP provides the foundation for future features:

- **Registry Support** - GitHub and official lacquer/ namespace
- **Version Pinning** - @v1, @latest syntax
- **Additional Runtimes** - Python, JavaScript, WebAssembly
- **Security** - Sandboxing, resource limits, signing
- **Marketplace** - Community block discovery

## Performance

- **Block Loading**: ~1ms for cached blocks
- **Go Compilation**: ~100-500ms (cached after first compile)
- **Docker Execution**: ~500ms-2s (depends on image size)
- **Memory Usage**: <10MB per block execution

## Error Handling

The system provides clear error messages for common issues:

- Missing block directories
- Invalid YAML syntax
- Compilation failures
- Runtime errors
- Type validation failures
- Docker daemon unavailability

Each error includes context and helpful suggestions for resolution.