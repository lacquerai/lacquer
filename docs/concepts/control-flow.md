# Control Flow

Control flow in Lacquer allows you to add logic to your workflows through conditional execution. This enables dynamic workflows that adapt based on data, outputs, and state.

## Conditional Execution

Lacquer supports conditional execution of steps using the `condition` field. Steps with conditions only execute when the condition evaluates to `true`.

### Basic Conditional Step

```yaml
steps:
  - id: check_length
    agent: analyzer
    prompt: "Count words in: ${{ inputs.text }}"
    outputs:
      word_count: integer
  
  - id: expand_text
    condition: ${{ steps.check_length.outputs.word_count < 100 }}
    agent: writer
    prompt: "Expand this text to at least 100 words: ${{ inputs.text }}"
```

### Using skip_if

For readability, you can use `skip_if` as an alternative to negated conditions:

```yaml
steps:
  - id: premium_feature
    condition: ${{ inputs.plan == 'premium' }}
    agent: premium_processor
    prompt: "Process premium features"
  
  - id: basic_feature
    skip_if: ${{ inputs.plan == 'premium' }}
    agent: basic_processor
    prompt: "Process basic features"
```

## Condition Syntax

Conditions use the same expression syntax as variable interpolation. You can use:

### Comparison Operators

```yaml
condition: ${{ steps.check.outputs.score > 80 }}
condition: ${{ inputs.environment == 'production' }}
condition: ${{ state.retries <= 3 }}
```

### Logical Operators

```yaml
condition: ${{ inputs.validate && steps.test.outputs.passed }}
condition: ${{ inputs.skip_validation || state.validated }}
condition: ${{ !state.completed }}
```

### Complex Expressions

```yaml
condition: ${{ (inputs.mode == 'auto' && state.confidence > 0.8) || inputs.force }}
```

## While Loops

While loops allow you to execute a set of sub-steps repeatedly while a condition remains true. This is essential for iterative processing, data collection, and progressive refinement workflows.

### Basic While Loop

```yaml
workflow:
  state:
    counter: 0
    items: []

  steps:
    - id: process_items
      while: ${{ state.counter < 10 }}
      steps:
        - id: process_item
          agent: processor
          prompt: "Process item number ${{ state.counter + 1 }}"
          outputs:
            result: string
        
        - id: update_counter
          action: update_state
          updates:
            counter: ${{ state.counter + 1 }}
            items: ${{ state.items + [steps.process_item.outputs.result] }}
```

### While Loop Syntax

A while step requires two key components:

- **while**: A condition expression that evaluates to a boolean
- **steps**: An array of sub-steps to execute on each iteration

```yaml
- id: my_loop
  while: ${{ condition_expression }}
  steps:
    - id: substep1
      # ... step configuration
    - id: substep2
      # ... step configuration
```

### Loop Outputs

When the loop completes, the outputs will contain the last iteration's outputs.

For example:

```yaml
state:
  counter: 0

workflow:
  steps:
    - id: counter_loop
      while: ${{ state.counter < 3 }}
      steps:
        - id: increment_counter
          run: "echo 'counter: ${{ state.counter }}'"
          updates:
            counter: ${{ state.counter + 1 }}
  outputs:
    counter: ${{ steps.counter_loop.outputs.steps.increment_counter.outputs }}
```

would output:

```bash
"counter: 3"
```

## Related Documentation

- [Variable Interpolation](variables.md) - Expression syntax for conditions
- [State Management](state-management.md) - Using state in conditions
- [Workflow Steps](workflow-steps.md) - Step execution details
- [Examples](examples/) - See conditions in action