# Control Flow

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

## Next Steps

- Learn about [State Management](./state-management.md) for tracking workflow progress
- Explore [Tool Integration](./tools.md) to extend agent capabilities
- See [Examples](./examples/agents/) for more agent configurations