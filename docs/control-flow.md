# Control Flow

Control flow in Lacquer allows you to add logic to your workflows through conditional execution. This enables dynamic workflows that adapt based on data, outputs, and state.

## Table of Contents

- [Conditional Execution](#conditional-execution)
- [Condition Syntax](#condition-syntax)
- [Common Patterns](#common-patterns)
- [Best Practices](#best-practices)

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

## Common Patterns

### Quality Gates

Only proceed if quality thresholds are met:

```yaml
steps:
  - id: analyze_code
    agent: code_reviewer
    prompt: "Analyze code quality and return a score (0-100)"
    outputs:
      score:
        type: integer
      issues:
        type: array
  
  - id: require_fixes
    condition: ${{ steps.analyze_code.outputs.score < 70 }}
    agent: developer
    prompt: "Fix critical issues: ${{ join(steps.analyze_code.outputs.issues, ', ') }}"
```

### Feature Toggles

Enable or disable features based on inputs:

```yaml
inputs:
  enable_advanced:
    type: boolean
    default: false

steps:
  - id: basic_analysis
    agent: analyzer
    prompt: "Perform basic analysis"
  
  - id: advanced_analysis
    condition: ${{ inputs.enable_advanced }}
    agent: advanced_analyzer
    prompt: "Perform deep analysis with ML models"
```

### Error Handling

Conditionally handle errors:

```yaml
steps:
  - id: try_primary
    agent: processor
    prompt: "Process using primary method"
    outputs:
      success:
        type: boolean
      error:
        type: string
  
  - id: fallback
    condition: ${{ !steps.try_primary.outputs.success }}
    agent: processor
    prompt: "Process using fallback method. Previous error: ${{ steps.try_primary.outputs.error }}"
```

### Progressive Enhancement

Add features based on available data:

```yaml
steps:
  - id: basic_report
    agent: reporter
    prompt: "Generate basic report"
  
  - id: add_visualizations
    condition: ${{ length(state.data_points) > 10 }}
    agent: visualizer
    prompt: "Add charts to the report"
  
  - id: add_predictions
    condition: ${{ length(state.data_points) > 100 }}
    agent: predictor
    prompt: "Add trend predictions to the report"
```

## Best Practices

### 1. Keep Conditions Simple

Break complex conditions into multiple steps for clarity:

```yaml
# Instead of this:
condition: ${{ (inputs.type == 'premium' && state.credits > 0) || (inputs.admin && !state.limited) }}

# Do this:
steps:
  - id: check_access
    agent: validator
    prompt: "Check user access level"
    outputs:
      has_access:
        type: boolean
  
  - id: premium_feature
    condition: ${{ steps.check_access.outputs.has_access }}
    agent: processor
    prompt: "Process premium feature"
```

### 2. Use Meaningful Variable Names

Make conditions self-documenting:

```yaml
# Good
condition: ${{ state.payment_verified && !state.subscription_expired }}

# Avoid
condition: ${{ state.pv && !state.se }}
```

### 3. Consider Default Paths

Always have a path that executes:

```yaml
steps:
  - id: check_type
    agent: classifier
    prompt: "Classify input type"
    outputs:
      type:
        type: string
  
  - id: handle_type_a
    condition: ${{ steps.check_type.outputs.type == 'A' }}
    agent: handler_a
    prompt: "Handle type A"
  
  - id: handle_type_b
    condition: ${{ steps.check_type.outputs.type == 'B' }}
    agent: handler_b
    prompt: "Handle type B"
  
  - id: handle_default
    condition: ${{ steps.check_type.outputs.type != 'A' && steps.check_type.outputs.type != 'B' }}
    agent: default_handler
    prompt: "Handle unknown type: ${{ steps.check_type.outputs.type }}"
```

### 4. Document Complex Logic

Add comments for complex conditions:

```yaml
steps:
  # Only proceed with expensive operation if:
  # 1. User has premium access OR
  # 2. It's a small request AND rate limit not exceeded
  - id: expensive_operation
    condition: ${{ inputs.premium || (state.request_size < 1000 && state.rate_limit_remaining > 0) }}
    agent: processor
    prompt: "Perform expensive operation"
```

## Related Documentation

- [Variable Interpolation](./variables.md) - Expression syntax for conditions
- [State Management](./state-management.md) - Using state in conditions
- [Workflow Steps](./workflow-steps.md) - Step execution details
- [Examples](./examples/) - See conditions in action