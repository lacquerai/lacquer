# Lacquer Workflow Syntax Documentation

Welcome to the comprehensive documentation for Lacquer's workflow DSL (Domain Specific Language). Lacquer provides a declarative, YAML-based syntax for orchestrating AI agent workflows, similar to how GitHub Actions revolutionized CI/CD workflows.

## Table of Contents

1. [Workflow Structure](./workflow-structure.md) - Basic workflow anatomy and metadata
2. [Agents](./agents.md) - Defining and configuring AI agents
3. [Workflow Steps](./workflow-steps.md) - Step definitions and execution
4. [Control Flow](./control-flow.md) - Parallel execution, conditions, and loops
5. [Blocks and Reusability](./blocks.md) - Using and creating reusable blocks
6. [Tool Integration](./tools.md) - Integrating tools with agents
7. [State Management](./state-management.md) - Managing workflow state and variables
8. [Error Handling](./error-handling.md) - Retry mechanisms and error strategies
9. [Variable Interpolation](./variables.md) - Using variables and outputs
10. [Examples](./examples/) - Comprehensive examples for all features

## Quick Start

A minimal Lacquer workflow looks like this:

```yaml
# hello.laq.yaml
version: "1.0"
metadata:
  name: hello-world

agents:
  assistant:
    provider: openai
    model: gpt-4
    temperature: 0.7

workflow:
  steps:
    - id: greet
      agent: assistant
      prompt: "Say hello to the world in a creative way!"
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
Workflows can use pre-built blocks, making complex workflows easy to assemble from tested components.

### Portable
Workflows run anywhere - local machines, cloud platforms, or edge devices.

## File Extension Convention

Lacquer workflow files use the `.laq.yaml` extension:
- `my-workflow.laq.yaml` - Standard workflow file
- `block.laq.yaml` - Block definition file (in block directories)

## Next Steps

- Start with [Workflow Structure](./workflow-structure.md) to understand the basic anatomy
- Learn about [Agents](./agents.md) to configure AI models
- Explore [Examples](./examples/) for practical workflows