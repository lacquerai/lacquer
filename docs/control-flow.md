# Control Flow

**Note: Advanced control flow features are not yet implemented in Lacquer. This document describes planned functionality.**

Lacquer currently supports basic conditional execution of steps. Advanced features like parallel execution, loops, and complex branching are planned for future releases.

## Currently Implemented: Basic Conditionals

### Basic Conditions

Use the `condition` field to conditionally execute steps:

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

### Skip Conditions

Alternative syntax for skipping steps:

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

### Condition Expressions

Basic comparison and boolean operations:

```yaml
# Boolean operators
condition: ${{ steps.validate.outputs.is_valid && !steps.validate.outputs.has_errors }}

# Comparison operators  
condition: ${{ steps.score.outputs.value >= 8 }}
condition: ${{ steps.count.outputs.total != 0 }}

# String operations
condition: ${{ steps.classify.outputs.category == 'urgent' }}
condition: ${{ contains(steps.check.outputs.message, 'error') }}

# Environment variables
condition: ${{ env.ENVIRONMENT == 'production' }}
```

### While Loops (Limited Implementation)

Basic while loop support exists:

```yaml
steps:
  - id: process_until_done
    while: ${{ state.items_remaining > 0 }}
    steps:
      - id: process_item
        run: "go run ./scripts/process_item.go"
        updates:
          items_remaining: ${{ state.items_remaining - 1 }}
```

## Planned Features (Not Yet Implemented)

The following control flow features are planned for future releases:

### Parallel Execution
- `parallel:` blocks with concurrent step execution
- `max_concurrency` limits
- `for_each` parallel processing

### Advanced Loops
- `for_each` with `as` and `index_as`
- Do-while patterns
- Loop break conditions

### Switch Statements
- Multi-way branching based on values
- Complex case conditions

### Map-Reduce Patterns
- Parallel map operations
- Result aggregation

## Current Limitations

1. **No parallel execution** - all steps run sequentially
2. **Basic while loops only** - limited loop constructs
3. **No for_each loops** - cannot iterate over collections
4. **No switch statements** - only if/else through conditions
5. **No nested control structures** - limited complexity

## Workarounds

For complex control flow needs, consider:

1. **Use external scripts** with the `run` field for complex logic
2. **Break workflows into smaller blocks** that can be composed
3. **Use state management** to track progress across steps
4. **Implement logic in container steps** for complex processing

## Migration Path

When advanced control flow features are implemented, the syntax will be backward compatible. Current condition-based workflows will continue to work.

## Next Steps

- Learn about [State Management](./state-management.md) for tracking workflow progress  
- Explore [Blocks](./blocks.md) for composing complex workflows
- Understand [Error Handling](./error-handling.md) for robust execution