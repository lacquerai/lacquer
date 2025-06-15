# Lacquer DSL Syntax Review and Improvements

After extensive documentation and example creation, this document identifies potential improvements to make the Lacquer workflow syntax more intuitive, consistent, and powerful.

## Current Syntax Strengths

1. **Clear separation of concerns** - agents, workflow, and metadata are well-separated
2. **Familiar YAML structure** - leverages existing developer knowledge
3. **Flexible variable interpolation** - Jinja2-style templates work well
4. **Comprehensive error handling** - Good coverage of error scenarios
5. **Extensible block system** - `uses` syntax provides good reusability

## Identified Improvement Areas

### 1. Step Definition Consistency

**Current Issue**: Steps have multiple ways to specify actions, which can be confusing:

```yaml
# Method 1: Agent with prompt (action inferred)
- id: step1
  agent: my_agent
  prompt: "Do something"

# Method 2: Explicit action
- id: step2
  action: human_input
  prompt: "Get approval"

# Method 3: Block usage
- id: step3
  uses: lacquer/block@v1
  with: {...}
```

**Proposed Improvement**: Introduce an optional `type` field for clarity:

```yaml
# Clearer syntax with explicit types
- id: step1
  type: agent  # Optional, inferred from presence of 'agent'
  agent: my_agent
  prompt: "Do something"

- id: step2
  type: action
  action: human_input
  prompt: "Get approval"

- id: step3
  type: block
  uses: lacquer/block@v1
  with: {...}
```

### 2. Enhanced Conditional Syntax

**Current Issue**: Complex conditions can become hard to read:

```yaml
condition: "{{ (steps.a.outputs.score > 7 and steps.b.outputs.valid) or (inputs.force_execute and env.ENVIRONMENT == 'dev') }}"
```

**Proposed Improvement**: Structured conditional syntax:

```yaml
# Option A: Named conditions
conditions:
  high_quality: "{{ steps.a.outputs.score > 7 and steps.b.outputs.valid }}"
  force_dev: "{{ inputs.force_execute and env.ENVIRONMENT == 'dev' }}"

condition: "{{ conditions.high_quality or conditions.force_dev }}"

# Option B: Structured conditions
condition:
  any_of:
    - all_of:
        - "{{ steps.a.outputs.score > 7 }}"
        - "{{ steps.b.outputs.valid }}"
    - all_of:
        - "{{ inputs.force_execute }}"
        - "{{ env.ENVIRONMENT == 'dev' }}"
```

### 3. Simplified Agent Tool Configuration

**Current Issue**: Tool configuration is verbose and inconsistent:

```yaml
agents:
  researcher:
    model: gpt-4
    tools:
      - name: search
        uses: lacquer/web-search@v1
        config:
          max_results: 5
          timeout: 30s
```

**Proposed Improvement**: Simplified tool syntax:

```yaml
agents:
  researcher:
    provider: openai
    model: gpt-4
    tools:
      # Shorthand for simple cases
      - lacquer/web-search@v1
      
      # Full syntax when needed
      - name: search
        uses: lacquer/web-search@v1
        config:
          max_results: 5
          timeout: 30s
      
      # Inline tool configuration
      - search: lacquer/web-search@v1
        max_results: 5
        timeout: 30s
```

### 4. Enhanced State Management

**Current Issue**: State updates are verbose and error-prone:

```yaml
- action: update_state
  updates:
    counter: "{{ state.counter + 1 }}"
    items: "{{ state.items + [new_item] }}"
    metadata:
      last_updated: "{{ now() }}"
      total_processed: "{{ state.metadata.total_processed + 1 }}"
```

**Proposed Improvement**: State operation helpers:

```yaml
# State operations with helpers
- action: update_state
  operations:
    - increment: counter
    - append: 
        to: items
        value: "{{ new_item }}"
    - set:
        metadata.last_updated: "{{ now() }}"
    - increment: metadata.total_processed

# Or more concise syntax
- state:
    counter: increment
    items: append(new_item)
    metadata.last_updated: "{{ now() }}"
    metadata.total_processed: increment
```

### 5. Better Error Handling Syntax

**Current Issue**: Error handling is verbose and requires multiple blocks:

```yaml
on_error:
  - log: "Error occurred: {{ error.message }}"
  - action: update_state
    updates:
      error_count: "{{ state.error_count + 1 }}"
  - condition: "{{ error.code == 'timeout' }}"
    fallback: backup_step
  - condition: "{{ error.code == 'rate_limit' }}"
    retry: true
```

**Proposed Improvement**: Structured error handling:

```yaml
on_error:
  log: "Error occurred: {{ error.message }}"
  state:
    error_count: increment
  
  # Error routing
  route:
    timeout: backup_step
    rate_limit: retry
    validation_error: 
      - fix_validation
      - retry
    default: fail
```

### 6. Workflow-level Constants and Imports

**Current Addition**: Support for constants and imports:

```yaml
version: "1.0"

# Import other workflows or blocks
imports:
  utils: ./common/utils.laq.yaml
  validators: lacquer/validators@v1

# Workflow-level constants
constants:
  MAX_RETRIES: 3
  DEFAULT_TIMEOUT: 30s
  API_BASE_URL: "https://api.example.com"

# Use in workflow
workflow:
  steps:
    - id: validate
      uses: validators.schema_validator
      with:
        schema: "{{ constants.DEFAULT_SCHEMA }}"
```

### 7. Enhanced Parallel Processing

**Current Issue**: Parallel syntax is limited for complex scenarios:

```yaml
parallel:
  for_each: "{{ items }}"
  as: item
  steps:
    - id: process
      agent: processor
      prompt: "Process {{ item }}"
```

**Proposed Improvement**: More flexible parallel syntax:

```yaml
# Enhanced parallel with better control
parallel:
  strategy: fan_out  # fan_out, map_reduce, pipeline
  max_concurrency: 5
  timeout: 10m
  fail_fast: true  # Stop all on first failure
  
  items: "{{ inputs.data }}"
  item_as: data_item
  index_as: item_index
  
  steps:
    - id: process
      agent: processor
      prompt: "Process item {{ item_index }}: {{ data_item }}"
  
  # Aggregation step (for map_reduce)
  aggregate:
    agent: aggregator
    prompt: "Combine results: {{ parallel_results | json }}"

# Pipeline parallel processing
parallel:
  strategy: pipeline
  stages:
    - name: extract
      agent: extractor
      prompt: "Extract from {{ item }}"
    
    - name: transform
      agent: transformer
      prompt: "Transform {{ stages.extract.output }}"
    
    - name: load
      agent: loader
      prompt: "Load {{ stages.transform.output }}"
```

### 8. Step Dependencies and Ordering

**Current Addition**: Explicit step dependencies:

```yaml
workflow:
  steps:
    - id: fetch_data
      agent: fetcher
      prompt: "Fetch data"
    
    - id: validate_data
      depends_on: [fetch_data]
      agent: validator
      prompt: "Validate {{ steps.fetch_data.output }}"
    
    - id: process_data
      depends_on: [validate_data]
      condition: "{{ steps.validate_data.outputs.is_valid }}"
      agent: processor
      prompt: "Process validated data"
    
    # Parallel steps that both depend on process_data
    - id: store_data
      depends_on: [process_data]
      parallel: true
      agent: storer
      prompt: "Store processed data"
    
    - id: notify_completion
      depends_on: [process_data]
      parallel: true
      agent: notifier
      prompt: "Notify about completion"
    
    # Final step depends on both parallel steps
    - id: cleanup
      depends_on: [store_data, notify_completion]
      agent: cleaner
      prompt: "Clean up resources"
```

### 9. Enhanced Variable Scoping

**Current Addition**: Local variable definitions:

```yaml
workflow:
  # Global variables
  variables:
    api_timeout: 30s
    retry_count: 3
    base_url: "{{ env.API_BASE_URL }}"
  
  steps:
    - id: complex_step
      # Local variables for this step
      variables:
        processed_items: "{{ inputs.items | map('process') }}"
        total_count: "{{ processed_items | length }}"
        batch_size: "{{ min(total_count, 10) }}"
      
      agent: processor
      prompt: |
        Process {{ batch_size }} items out of {{ total_count }} total.
        Items: {{ processed_items[:batch_size] | json }}
```

### 10. Workflow Templates and Inheritance

**Current Addition**: Template system for workflow reuse:

```yaml
# base-workflow.laq.yaml
template: true
version: "1.0"

agents:
  base_processor:
    model: "{{ template.model | default('gpt-4') }}"
    temperature: "{{ template.temperature | default(0.5) }}"

workflow:
  abstract: true
  inputs:
    data: 
      type: "{{ template.input_type | default('string') }}"
  
  steps:
    - id: validate_input
      agent: base_processor
      prompt: "Validate {{ template.validation_prompt }}"

# Concrete workflow extending template
version: "1.0"
extends: ./base-workflow.laq.yaml

template:
  model: gpt-4-turbo
  temperature: 0.3
  input_type: array
  validation_prompt: "array structure"

workflow:
  steps:
    # Inherited validate_input step is automatically included
    
    - id: process_items
      agent: base_processor
      prompt: "Process validated items"
```

## Priority Ranking

### High Priority (Should be in MVP+1)
1. **Step Dependencies and Ordering** - Enables better workflow optimization
2. **Enhanced State Management** - Reduces verbosity and errors
3. **Simplified Agent Tool Configuration** - Better developer experience

### Medium Priority (Should be in v1.1)
4. **Enhanced Conditional Syntax** - Improves readability
5. **Better Error Handling Syntax** - Reduces boilerplate
6. **Workflow-level Constants and Imports** - Enables better organization

### Low Priority (Future versions)
7. **Enhanced Parallel Processing** - Advanced features for power users
8. **Enhanced Variable Scoping** - Nice-to-have for complex workflows
9. **Workflow Templates and Inheritance** - Enterprise feature
10. **Step Definition Consistency** - Breaking change, needs careful consideration

## Implementation Considerations

### Backward Compatibility
- All improvements should be additive, not breaking changes
- Existing syntax should continue to work
- New syntax should be opt-in

### Migration Path
- Provide migration tools for syntax upgrades
- Clear documentation on old vs new syntax
- Deprecation warnings for old patterns

### Validation
- Enhanced JSON Schema validation for new syntax
- Better error messages for syntax errors
- IDE support for new syntax features

## Conclusion

These improvements would significantly enhance the developer experience while maintaining the declarative nature and simplicity that makes Lacquer powerful. The priority ranking ensures that the most impactful changes are implemented first, while maintaining backward compatibility.