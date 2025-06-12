# Error Handling

Lacquer provides comprehensive error handling mechanisms to build robust, fault-tolerant workflows. This includes retry strategies, fallback options, error recovery, and detailed error tracking.

## Basic Error Handling

### Step-Level Error Handling

Handle errors at individual steps:

```yaml
steps:
  - id: risky_operation
    agent: processor
    prompt: "Process sensitive data"
    on_error:
      - log: "Operation failed: {{ error.message }}"
      - continue: true  # Continue workflow despite error
```

### Error Context

Access error information in handlers:

```yaml
steps:
  - id: api_call
    uses: lacquer/http-request@v1
    with:
      url: "{{ inputs.api_url }}"
    on_error:
      - id: handle_error
        agent: error_handler
        prompt: |
          Handle API error:
          Error Type: {{ error.type }}
          Error Message: {{ error.message }}
          Error Code: {{ error.code }}
          Stack Trace: {{ error.stack }}
          Step ID: {{ error.step_id }}
```

## Retry Mechanisms

### Basic Retry

Configure automatic retries:

```yaml
steps:
  - id: unstable_operation
    agent: processor
    prompt: "Perform operation"
    retry:
      max_attempts: 3
      delay: 5s
```

### Advanced Retry Configuration

```yaml
steps:
  - id: complex_retry
    agent: api_caller
    prompt: "Call external service"
    retry:
      max_attempts: 5
      initial_delay: 1s
      max_delay: 60s
      backoff: exponential  # exponential, linear, constant
      backoff_multiplier: 2
      retry_on:
        - "timeout"
        - "rate_limit"
        - "connection_error"
      retry_condition: "{{ error.code in [500, 502, 503, 504] }}"
```

### Retry with State Updates

Track retry attempts in state:

```yaml
workflow:
  state:
    retry_counts: {}
  
  steps:
    - id: retryable_task
      agent: processor
      prompt: "Process item"
      retry:
        max_attempts: 3
        on_retry:
          - action: update_state
            updates:
              retry_counts: |
                {{ state.retry_counts | merge({
                  step.id: (state.retry_counts[step.id] | default(0)) + 1
                }) }}
          - log: "Retry attempt {{ retry.attempt }} of {{ retry.max_attempts }}"
```

## Fallback Strategies

### Simple Fallback

Define fallback steps:

```yaml
steps:
  - id: primary_method
    agent: advanced_processor
    prompt: "Use advanced algorithm"
    timeout: 30s
    on_error:
      - fallback: simple_method
  
  - id: simple_method
    agent: basic_processor
    prompt: "Use simple algorithm"
    skip_if: "{{ steps.primary_method.success }}"
```

### Cascading Fallbacks

Multiple fallback options:

```yaml
steps:
  - id: try_premium_api
    uses: lacquer/premium-data@v1
    with:
      query: "{{ inputs.query }}"
    on_error:
      - log: "Premium API failed, trying standard"
      - fallback: try_standard_api
  
  - id: try_standard_api
    uses: lacquer/standard-data@v1
    with:
      query: "{{ inputs.query }}"
    skip_if: "{{ steps.try_premium_api.success }}"
    on_error:
      - log: "Standard API failed, using cache"
      - fallback: use_cache
  
  - id: use_cache
    uses: lacquer/cache-lookup@v1
    with:
      key: "{{ inputs.query }}"
    skip_if: "{{ steps.try_standard_api.success }}"
    on_error:
      - id: return_default
        output:
          data: []
          source: "default"
          error: "All data sources unavailable"
```

## Error Recovery Patterns

### Circuit Breaker Pattern

Prevent cascading failures:

```yaml
workflow:
  state:
    circuit_breaker:
      failure_count: 0
      last_failure_time: null
      status: "closed"  # closed, open, half-open
      threshold: 5
      timeout: 300  # 5 minutes
  
  steps:
    - id: check_circuit
      condition: "{{ state.circuit_breaker.status != 'open' or (now() - state.circuit_breaker.last_failure_time) > state.circuit_breaker.timeout }}"
      agent: service_caller
      prompt: "Call service"
      on_error:
        - action: update_state
          updates:
            circuit_breaker:
              failure_count: "{{ state.circuit_breaker.failure_count + 1 }}"
              last_failure_time: "{{ now() }}"
              status: "{{ state.circuit_breaker.failure_count + 1 >= state.circuit_breaker.threshold ? 'open' : state.circuit_breaker.status }}"
      on_success:
        - action: update_state
          updates:
            circuit_breaker:
              failure_count: 0
              status: "closed"
```

### Compensation Pattern

Undo operations on failure:

```yaml
steps:
  - id: create_order
    uses: lacquer/create-order@v1
    with:
      customer: "{{ inputs.customer_id }}"
      items: "{{ inputs.items }}"
    outputs:
      order_id: string
  
  - id: charge_payment
    uses: lacquer/payment-processor@v1
    with:
      order_id: "{{ steps.create_order.outputs.order_id }}"
      amount: "{{ inputs.total }}"
    on_error:
      - id: compensate_order
        uses: lacquer/cancel-order@v1
        with:
          order_id: "{{ steps.create_order.outputs.order_id }}"
          reason: "Payment failed"
      - fail: "Payment processing failed"
```

### Bulkhead Pattern

Isolate failures:

```yaml
steps:
  - id: parallel_processing
    parallel:
      max_concurrency: 3  # Limit concurrent operations
      isolation: true     # Isolate failures
      steps:
        - id: process_1
          agent: processor
          prompt: "Process partition 1"
          on_error:
            - log: "Partition 1 failed"
            - continue: true  # Don't fail entire parallel block
        
        - id: process_2
          agent: processor
          prompt: "Process partition 2"
          on_error:
            - log: "Partition 2 failed"
            - continue: true
```

## Error Types and Handling

### Timeout Errors

```yaml
steps:
  - id: long_operation
    agent: analyzer
    prompt: "Perform complex analysis"
    timeout: 5m
    on_timeout:
      - log: "Operation timed out after 5 minutes"
      - id: quick_analysis
        agent: analyzer
        prompt: "Perform quick analysis instead"
        timeout: 30s
```

### Validation Errors

```yaml
steps:
  - id: validate_input
    agent: validator
    prompt: "Validate: {{ inputs.data }}"
    outputs:
      is_valid: boolean
      errors: array
  
  - id: handle_validation_errors
    condition: "{{ not steps.validate_input.outputs.is_valid }}"
    parallel:
      for_each: "{{ steps.validate_input.outputs.errors }}"
      as: error
      steps:
        - id: fix_error
          agent: fixer
          prompt: |
            Fix validation error:
            Field: {{ error.field }}
            Message: {{ error.message }}
            Value: {{ error.value }}
```

### Resource Errors

```yaml
steps:
  - id: allocate_resources
    uses: lacquer/resource-manager@v1
    with:
      type: "gpu"
      count: 4
    on_error:
      - switch:
          on: "{{ error.type }}"
          cases:
            insufficient_resources:
              - id: scale_down
                uses: lacquer/resource-manager@v1
                with:
                  type: "gpu"
                  count: 2
            
            quota_exceeded:
              - id: request_increase
                uses: lacquer/quota-request@v1
                with:
                  resource: "gpu"
                  requested: 4
              - wait: 60s
              - retry: true
```

## Error Aggregation

### Collecting Errors

```yaml
workflow:
  state:
    all_errors: []
    error_summary: {}
  
  steps:
    - id: batch_process
      parallel:
        for_each: "{{ inputs.items }}"
        as: item
        continue_on_error: true
        steps:
          - id: process_item
            agent: processor
            prompt: "Process: {{ item }}"
            on_error:
              - action: update_state
                updates:
                  all_errors: |
                    {{ state.all_errors + [{
                      'item': item,
                      'error': error.message,
                      'timestamp': now()
                    }] }}
    
    - id: summarize_errors
      condition: "{{ state.all_errors | length > 0 }}"
      agent: analyzer
      prompt: |
        Analyze these errors and provide summary:
        {{ state.all_errors | json }}
```

## Error Notification

### Alert on Errors

```yaml
steps:
  - id: critical_operation
    agent: processor
    prompt: "Perform critical operation"
    on_error:
      - parallel:
          steps:
            - uses: lacquer/email@v1
              with:
                to: "{{ env.ADMIN_EMAIL }}"
                subject: "Critical Operation Failed"
                body: |
                  Error in workflow: {{ metadata.name }}
                  Step: {{ error.step_id }}
                  Message: {{ error.message }}
                  Time: {{ now() }}
            
            - uses: lacquer/slack@v1
              with:
                channel: "#alerts"
                message: "🚨 Critical operation failed: {{ error.message }}"
            
            - uses: lacquer/pagerduty@v1
              with:
                severity: "critical"
                summary: "{{ error.message }}"
```

## Error Testing

### Simulating Errors

```yaml
steps:
  - id: test_error_handling
    agent: tester
    prompt: |
      {% if env.TEST_MODE == 'true' %}
        SIMULATE_ERROR: {{ inputs.error_type }}
      {% else %}
        Process normally
      {% endif %}
    on_error:
      - log: "Error simulation successful: {{ error.message }}"
```

## Best Practices

### 1. Fail Fast for Critical Errors
```yaml
steps:
  - id: critical_check
    agent: validator
    prompt: "Validate critical requirements"
    on_error:
      - fail: "Critical validation failed: {{ error.message }}"
```

### 2. Provide Context in Error Messages
```yaml
on_error:
  - log: |
      Error in {{ step.id }} at {{ now() }}:
      Input: {{ step.inputs | json }}
      Error: {{ error.message }}
      Workflow: {{ metadata.name }}
```

### 3. Use Structured Error Handling
```yaml
on_error:
  - id: structured_handler
    outputs:
      error_code: "{{ error.code | default('UNKNOWN') }}"
      error_type: "{{ error.type }}"
      recoverable: "{{ error.code not in ['FATAL', 'CRITICAL'] }}"
      suggested_action: |
        {{ switch(error.type, {
          'timeout': 'Increase timeout or optimize operation',
          'validation': 'Fix input data',
          'resource': 'Retry later or scale down',
          'default': 'Contact support'
        }) }}
```

### 4. Clean Up on Errors
```yaml
steps:
  - id: allocate_resource
    uses: lacquer/allocate@v1
    outputs:
      resource_id: string
  
  - id: use_resource
    agent: processor
    prompt: "Use resource {{ steps.allocate_resource.outputs.resource_id }}"
    on_error:
      - uses: lacquer/deallocate@v1
        with:
          resource_id: "{{ steps.allocate_resource.outputs.resource_id }}"
      - fail: "Processing failed"
    on_success:
      - uses: lacquer/deallocate@v1
        with:
          resource_id: "{{ steps.allocate_resource.outputs.resource_id }}"
```

### 5. Document Error Scenarios
```yaml
# Document expected errors in workflow metadata
metadata:
  name: data-processor
  error_scenarios:
    - type: "validation_error"
      description: "Input data doesn't match schema"
      handling: "Return detailed validation errors"
    - type: "api_timeout"
      description: "External API takes too long"
      handling: "Retry with exponential backoff"
```

## Advanced Error Patterns

### Async Error Handling
```yaml
steps:
  - id: start_async_job
    uses: lacquer/async-job@v1
    outputs:
      job_id: string
  
  - id: monitor_job
    while: "{{ state.job_status != 'complete' and state.job_status != 'failed' }}"
    steps:
      - uses: lacquer/check-job@v1
        with:
          job_id: "{{ steps.start_async_job.outputs.job_id }}"
        outputs:
          status: string
          error: object
      
      - action: update_state
        updates:
          job_status: "{{ steps.check_job.outputs.status }}"
          job_error: "{{ steps.check_job.outputs.error }}"
      
      - condition: "{{ state.job_status == 'failed' }}"
        on_error:
          - handle_async_error:
              error: "{{ state.job_error }}"
```

### Error Recovery with Machine Learning
```yaml
steps:
  - id: ml_error_handler
    agent: ml_handler
    prompt: |
      Analyze this error and suggest recovery:
      Error: {{ error | json }}
      Context: {{ context | json }}
      History: {{ state.error_history | json }}
      
      Provide recovery strategy.
    outputs:
      strategy: string
      confidence: float
  
  - id: apply_recovery
    condition: "{{ steps.ml_error_handler.outputs.confidence > 0.8 }}"
    agent: executor
    prompt: "Apply recovery: {{ steps.ml_error_handler.outputs.strategy }}"
```

## Next Steps

- Learn about [Variable Interpolation](./variables.md) for dynamic error handling
- Explore [Examples](./examples/error-handling/) for real-world patterns
- See [Best Practices](./best-practices.md) for production-ready workflows