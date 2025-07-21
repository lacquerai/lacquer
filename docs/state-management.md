# State Management

State management in Lacquer allows workflows to maintain and update data throughout execution. This is essential for iterative processes, maintaining context across steps, and building complex stateful workflows.

## Table of Contents

- [Workflow State](#workflow-state)
- [State Data Types](#state-data-types)
- [Accessing State](#accessing-state)
- [Updating State](#updating-state)
- [State Patterns](#state-patterns)
- [Best Practices](#best-practices)

## Workflow State

### Basic State Definition

Define initial state at the workflow level:

```yaml
workflow:
  state:
    counter: 0
    items_processed: []
    status: "initialized"
    settings:
      max_retries: 3
      timeout: 300
```

## State Data Types

State supports all JSON-compatible data types, allowing you to store any structured data:

```yaml
workflow:
  state:
    # Primitives
    count: 0                    # integer
    percentage: 0.0             # float
    is_complete: false          # boolean
    current_item: null          # null
    message: "Starting..."      # string
    
    # Collections
    results: []                 # array
    metadata: {}                # object
    
    # Nested structures
    progress:
      completed: 0
      total: 100
      errors: []
```

## Accessing State

### Reading State

Access state values anywhere in your workflow using the `state` context:

```yaml
steps:
  - id: check_progress
    agent: monitor
    prompt: |
      Current progress: ${{ state.progress.completed }}/${{ state.progress.total }}
      Status: ${{ state.status }}
      Errors: ${{ length(state.progress.errors) }}
```

### State in Conditions

Use state in conditional logic:

```yaml
steps:
  - id: continue_processing
    condition: "${{ state.counter < state.max_items && state.status != 'error' }}"
    agent: processor
    prompt: "Process item ${{ state.counter + 1 }}"
```

## Updating State

State updates happen through the `updates` property in workflow steps. Each update modifies the workflow state when the step completes successfully.

### Basic Updates

```yaml
steps:
  - id: process_item
    agent: processor
    prompt: "Process: ${{ inputs.item }}"
    updates:
      counter: "${{ state.counter + 1 }}"
      status: "${{ steps.process_item.outputs.success ? 'processing' : 'error' }}"
```

### Complex Updates

Update nested structures and arrays:

```yaml
steps:
  - id: add_result
    agent: processor
    prompt: "Process item"
    outputs:
      result:
        type: object
    updates:
      # Add to array
      results: "${{ state.results + [steps.add_result.outputs.result] }}"
      # Update nested property
      "progress.completed": "${{ state.progress.completed + 1 }}"
      # Conditional update
      status: "${{ state.progress.completed >= state.progress.total ? 'complete' : 'processing' }}"
```

## State Patterns

### Counter Pattern

Track iterations or processed items:

```yaml
workflow:
  state:
    processed_count: 0
    total_items: 10
  
  steps:
    - id: process_item
      agent: processor
      prompt: "Process item number ${{ state.processed_count + 1 }}"
      updates:
        processed_count: "${{ state.processed_count + 1 }}"
```

### Accumulator Pattern

Collect results across multiple steps:

```yaml
workflow:
  state:
    all_findings: []
    total_score: 0
  
  steps:
    - id: analyze_section
      agent: analyzer
      prompt: "Analyze section"
      outputs:
        findings:
          type: array
        score:
          type: integer
      updates:
        all_findings: "${{ state.all_findings + steps.analyze_section.outputs.findings }}"
        total_score: "${{ state.total_score + steps.analyze_section.outputs.score }}"
```

### Status Tracking Pattern

Maintain workflow status and metadata:

```yaml
workflow:
  state:
    status: "initializing"
    start_time: null
    errors: []
    metadata:
      version: "1.0"
      retries: 0
  
  steps:
    - id: initialize
      agent: initializer
      prompt: "Initialize workflow"
      updates:
        status: "initialized"
        start_time: "${{ steps.initialize.outputs.timestamp }}"
    
    - id: process
      agent: processor
      prompt: "Process data"
      updates:
        status: "${{ steps.process.outputs.success ? 'completed' : 'failed' }}"
        "metadata.retries": "${{ steps.process.outputs.retry_count }}"
```

### Checkpoint Pattern

Save progress for long-running workflows:

```yaml
workflow:
  state:
    checkpoint:
      stage: "start"
      data: null
      timestamp: null
  
  steps:
    - id: stage_one
      agent: processor
      prompt: "Complete stage one"
      outputs:
        data:
          type: object
      updates:
        checkpoint:
          stage: "stage_one_complete"
          data: "${{ steps.stage_one.outputs.data }}"
          timestamp: "${{ steps.stage_one.outputs.timestamp }}"
```

## Best Practices

### 1. Initialize State Clearly

Always provide clear initial values:

```yaml
workflow:
  state:
    # Good: Clear initial state
    items_processed: 0
    errors: []
    status: "pending"
    
    # Avoid: Undefined or unclear state
    # counter: null
    # data: {}
```

### 2. Use Descriptive Names

Choose state variable names that clearly indicate their purpose:

```yaml
workflow:
  state:
    # Good
    validation_errors: []
    processing_stage: "initialization"
    retry_attempts: 0
    
    # Avoid
    errors: []  # What kind of errors?
    stage: "init"  # Too abbreviated
    r: 0  # Unclear purpose
```

### 3. Keep State Minimal

Only store what's necessary for workflow execution:

```yaml
# Good: Minimal state
workflow:
  state:
    current_page: 1
    total_processed: 0
    has_errors: false

# Avoid: Storing unnecessary data
workflow:
  state:
    current_page: 1
    total_processed: 0
    has_errors: false
    debug_info: {}  # Use logging instead
    temp_data: []  # Use step outputs
```

### 4. Document State Structure

Add comments to explain complex state:

```yaml
workflow:
  state:
    # Tracks the current processing stage
    stage: "initialization"
    
    # Queue of items to process
    queue: []
    
    # Results grouped by category
    results:
      successful: []
      failed: []
      skipped: []
```

### 5. Handle State Updates Atomically

Update related state variables together:

```yaml
steps:
  - id: process_batch
    agent: processor
    prompt: "Process batch"
    outputs:
      processed:
        type: integer
      failed:
        type: integer
    updates:
      # Update related counters together
      "stats.processed": "${{ state.stats.processed + steps.process_batch.outputs.processed }}"
      "stats.failed": "${{ state.stats.failed + steps.process_batch.outputs.failed }}"
      "stats.total": "${{ state.stats.processed + state.stats.failed }}"
```

## Related Documentation

- [Variable Interpolation](./variables.md) - Using state in expressions
- [Workflow Steps](./workflow-steps.md) - Updating state from steps
- [Control Flow](./control-flow.md) - Using state in conditions
- [Examples](./examples/) - State management patterns