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

### State Data Types

State supports all JSON-compatible types:

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

### Reading State Values

Access state values using the `state` context:

```yaml
steps:
  - id: check_progress
    agent: monitor
    prompt: |
      Current progress: {{ state.progress.completed }}/{{ state.progress.total }}
      Status: {{ state.status }}
      Errors: {{ state.progress.errors | length }}
```

### State in Conditions

Use state in conditional logic:

```yaml
steps:
  - id: continue_processing
    condition: "{{ state.counter < state.max_items and state.status != 'error' }}"
    agent: processor
    prompt: "Process item {{ state.counter + 1 }}"
```

## Updating State

### Using update_state Action

The `update_state` action modifies workflow state:

```yaml
steps:
  - id: process_item
    agent: processor
    prompt: "Process: {{ inputs.item }}"
    outputs:
      result: object
      success: boolean
  
  - id: update_progress
    action: update_state
    updates:
      counter: "{{ state.counter + 1 }}"
      items_processed: "{{ state.items_processed + [steps.process_item.outputs.result] }}"
      last_updated: "{{ now() }}"
      status: "{{ steps.process_item.outputs.success ? 'processing' : 'error' }}"
```

### Conditional State Updates

Update state based on conditions:

```yaml
steps:
  - id: conditional_update
    action: update_state
    condition: "{{ steps.validate.outputs.is_valid }}"
    updates:
      valid_count: "{{ state.valid_count + 1 }}"
      last_valid_item: "{{ steps.validate.outputs.item }}"
```

### Complex State Updates

```yaml
steps:
  - id: complex_update
    action: update_state
    updates:
      # Increment counter
      processed: "{{ state.processed + 1 }}"
      
      # Append to array
      results: "{{ state.results + [steps.analyze.outputs] }}"
      
      # Update nested object
      statistics:
        total: "{{ state.statistics.total + 1 }}"
        average: "{{ (state.statistics.sum + steps.analyze.outputs.value) / (state.statistics.total + 1) }}"
        sum: "{{ state.statistics.sum + steps.analyze.outputs.value }}"
      
      # Conditional update
      max_value: "{{ max(state.max_value, steps.analyze.outputs.value) }}"
      
      # Object merge
      metadata: "{{ state.metadata | merge({'last_run': now()}) }}"
```

## State Patterns

### Accumulator Pattern

Collect results over multiple steps:

```yaml
workflow:
  state:
    all_insights: []
    total_score: 0
  
  steps:
    - id: analyze_documents
      for_each: "{{ inputs.documents }}"
      as: doc
      steps:
        - id: extract_insights
          agent: analyzer
          prompt: "Extract insights from: {{ doc.content }}"
          outputs:
            insights: array
            score: float
        
        - id: accumulate_results
          action: update_state
          updates:
            all_insights: "{{ state.all_insights + steps.extract_insights.outputs.insights }}"
            total_score: "{{ state.total_score + steps.extract_insights.outputs.score }}"
```

### State Machine Pattern

Implement state transitions:

```yaml
workflow:
  state:
    current_state: "start"
    transitions: []
  
  steps:
    - id: state_machine
      while: "{{ state.current_state != 'end' }}"
      steps:
        - id: process_state
          switch:
            on: "{{ state.current_state }}"
            cases:
              start:
                agent: initializer
                prompt: "Initialize process"
                
              processing:
                agent: processor
                prompt: "Process data"
                
              review:
                agent: reviewer
                prompt: "Review results"
              
              default:
                agent: handler
                prompt: "Handle state: {{ state.current_state }}"
        
        - id: determine_next_state
          agent: state_controller
          prompt: |
            Current state: {{ state.current_state }}
            Last result: {{ steps.process_state.output }}
            Determine next state.
          outputs:
            next_state: string
        
        - id: transition
          action: update_state
          updates:
            current_state: "{{ steps.determine_next_state.outputs.next_state }}"
            transitions: "{{ state.transitions + [{'from': state.current_state, 'to': steps.determine_next_state.outputs.next_state, 'timestamp': now()}] }}"
```

### Retry Counter Pattern

Track retry attempts:

```yaml
workflow:
  state:
    retry_count: 0
    max_retries: 3
    last_error: null
  
  steps:
    - id: attempt_operation
      while: "{{ state.retry_count < state.max_retries }}"
      steps:
        - id: try_operation
          agent: operator
          prompt: "Perform operation (attempt {{ state.retry_count + 1 }})"
          on_error:
            - id: record_error
              action: update_state
              updates:
                retry_count: "{{ state.retry_count + 1 }}"
                last_error: "{{ error.message }}"
        
        - id: check_success
          condition: "{{ steps.try_operation.success }}"
          action: update_state
          updates:
            retry_count: "{{ state.max_retries }}"  # Exit loop
            status: "success"
```

## State Persistence

### Checkpoint Pattern

Save state at key points:

```yaml
workflow:
  state:
    checkpoint: null
    stage: "initialization"
  
  steps:
    - id: stage_1
      agent: processor
      prompt: "Complete stage 1"
    
    - id: checkpoint_1
      action: update_state
      updates:
        checkpoint:
          stage: "stage_1_complete"
          timestamp: "{{ now() }}"
          data: "{{ steps.stage_1.outputs }}"
        stage: "processing"
    
    - id: stage_2
      agent: processor
      prompt: "Complete stage 2 using: {{ state.checkpoint.data }}"
```

### Recovery Pattern

Resume from saved state:

```yaml
workflow:
  state:
    last_processed_index: "{{ restored_state.last_processed_index | default(0) }}"
    results: "{{ restored_state.results | default([]) }}"
  
  steps:
    - id: resume_processing
      for_each: "{{ inputs.items[state.last_processed_index:] }}"
      as: item
      index_as: idx
      steps:
        - id: process
          agent: processor
          prompt: "Process: {{ item }}"
        
        - id: save_progress
          action: update_state
          updates:
            last_processed_index: "{{ state.last_processed_index + idx + 1 }}"
            results: "{{ state.results + [steps.process.output] }}"
```

## State Scoping

### Global vs Local State

```yaml
workflow:
  # Global workflow state
  state:
    global_counter: 0
  
  steps:
    - id: parallel_tasks
      parallel:
        for_each: "{{ inputs.tasks }}"
        as: task
        # Local state within parallel execution
        local_state:
          task_retries: 0
          task_status: "pending"
        steps:
          - id: process_task
            agent: processor
            prompt: "Process: {{ task }}"
          
          - id: update_local
            action: update_local_state
            updates:
              task_status: "completed"
          
          - id: update_global
            action: update_state
            updates:
              global_counter: "{{ state.global_counter + 1 }}"
```

## State Functions

### Built-in State Functions

```yaml
steps:
  - id: use_state_functions
    action: update_state
    updates:
      # Array operations
      unique_items: "{{ state.items | unique }}"
      sorted_scores: "{{ state.scores | sort }}"
      filtered_results: "{{ state.results | select('score', '>', 0.8) }}"
      
      # Math operations
      sum_total: "{{ state.values | sum }}"
      average_score: "{{ state.scores | average }}"
      max_value: "{{ state.values | max }}"
      
      # Object operations
      merged_config: "{{ state.config | merge(inputs.overrides) }}"
      keys_list: "{{ state.data | keys }}"
      
      # String operations
      formatted_message: "{{ state.template | format(state.values) }}"
      
      # Date operations
      days_elapsed: "{{ (now() - state.start_time) | days }}"
```

## State Best Practices

### 1. Initialize State Clearly
```yaml
workflow:
  state:
    # Good: Clear initial values
    processed_count: 0
    error_messages: []
    status: "not_started"
    
    # Avoid: Undefined initial state
    # some_value: null  # What type? What does null mean?
```

### 2. Use Descriptive Names
```yaml
state:
  # Good
  customers_processed: 0
  failed_validation_count: 0
  
  # Less clear
  count1: 0
  count2: 0
```

### 3. Keep State Flat When Possible
```yaml
# Good: Flat structure
state:
  user_count: 0
  order_count: 0
  total_revenue: 0.0

# Avoid deep nesting unless necessary
state:
  metrics:
    users:
      counts:
        total: 0
```

### 4. Document State Purpose
```yaml
workflow:
  state:
    # Tracks number of API calls for rate limiting
    api_calls_count: 0
    
    # Stores validation errors for final report
    validation_errors: []
    
    # Current processing phase: "init", "processing", "complete"
    phase: "init"
```

### 5. Atomic State Updates
```yaml
# Good: Update related state together
- id: update_transaction
  action: update_state
  updates:
    balance: "{{ state.balance - transaction.amount }}"
    transactions: "{{ state.transactions + [transaction] }}"
    last_transaction_time: "{{ now() }}"

# Avoid: Separate updates that should be atomic
```

## Advanced State Patterns

### Event Sourcing Pattern
```yaml
workflow:
  state:
    events: []
    current_state: {}
  
  steps:
    - id: record_event
      action: update_state
      updates:
        events: |
          {{ state.events + [{
            'type': 'item_processed',
            'timestamp': now(),
            'data': steps.process.outputs
          }] }}
    
    - id: replay_events
      agent: event_processor
      prompt: |
        Replay these events to reconstruct state:
        {{ state.events | json }}
```

### State Validation
```yaml
steps:
  - id: validate_state
    agent: validator
    prompt: |
      Validate workflow state:
      {{ state | json }}
      
      Rules:
      - processed_count >= 0
      - status in ["init", "processing", "complete", "error"]
      - error_count <= max_errors
    outputs:
      is_valid: boolean
      violations: array
  
  - id: handle_invalid_state
    condition: "{{ not steps.validate_state.outputs.is_valid }}"
    agent: error_handler
    prompt: "Recover from invalid state: {{ steps.validate_state.outputs.violations }}"
```

## Next Steps

- Learn about [Error Handling](./error-handling.md) for state recovery
- Explore [Variable Interpolation](./variables.md) for dynamic state usage
- See [Examples](./examples/state-management/) for real-world patterns