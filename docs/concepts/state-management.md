# State Management

State management in Lacquer allows workflows to maintain and update data throughout execution. This is essential for iterative processes, maintaining context across steps, and building complex stateful workflows.

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

## Related Documentation

- [Variable Interpolation](variables.md) - Using state in expressions
- [Workflow Steps](workflow-steps.md) - Updating state from steps
- [Control Flow](control-flow.md) - Using state in conditions
- [Examples](examples/) - State management patterns