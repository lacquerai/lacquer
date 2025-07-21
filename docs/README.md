# Lacquer Workflow Syntax Documentation

Welcome to the comprehensive documentation for Lacquer's workflow DSL (Domain Specific Language). Lacquer provides a declarative, YAML-based syntax for orchestrating AI agent workflows, similar to how GitHub Actions works for CI/CD workflows.

## Table of Contents

1. [Workflow Structure](./workflow-structure.md) - Basic workflow anatomy and metadata
2. [Agents](./agents.md) - Defining and configuring AI agents
3. [Workflow Steps](./workflow-steps.md) - Step definitions and execution
4. [Control Flow](./control-flow.md) - Parallel execution, conditions, and loops
5. [Tool Integration](./tools.md) - Integrating tools with agents
6. [State Management](./state-management.md) - Managing workflow state and variables
7. [Variable Interpolation](./variables.md) - Using variables and outputs
8. [Examples](./examples/) - Comprehensive examples for all features

## Quick Start

A minimal Lacquer workflow looks like this:

```yaml
version: "1.0"
metadata:
  name: hello-world

agents:
  assistant:
    provider: openai
    model: gpt-4
    temperature: 0.7

inputs:
  name:
    type: string
    description: The name of the person to greet
    default: "world"

workflow:
  steps:
    - id: greet
      agent: assistant
      prompt: "Say hello to ${{ inputs.name }} in a creative way!"

  outputs:
    farewell: ${{ steps.greet.output }}
```

Run it with:
```bash
laq run hello.laq.yaml
```

## Key Concepts

### Declarative Syntax
Lacquer uses YAML to define workflows declaratively. You describe *what* you want to happen, not *how* to do it.

### Agent-Based
Workflows are executed by AI agents that you configure with models, prompts, and tools.

### Composable
Workflows can reference other workflows for reusability and modularity.

### Portable
Workflows run anywhere - local machines, cloud platforms, or edge devices.

## File Extension Convention

Lacquer workflow files use the `.laq.yaml` extension

## Next Steps

- Start with [Workflow Structure](./workflow-structure.md) to understand the basic anatomy
- Learn about [Agents](./agents.md) to configure AI models
- Explore [Examples](./examples/) for practical workflows