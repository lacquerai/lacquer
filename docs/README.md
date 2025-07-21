# Lacquer Workflow Syntax Documentation

Welcome to the comprehensive documentation for Lacquer's workflow DSL (Domain Specific Language). Lacquer provides a declarative, YAML-based syntax for orchestrating AI agent workflows, similar to how GitHub Actions works for CI/CD workflows.

## Overview

Lacquer enables you to:
- **Orchestrate AI agents** with different models and configurations
- **Build complex workflows** with conditional logic and state management
- **Integrate external tools** through scripts and MCP servers
- **Create reusable components** for common tasks

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

Here's a minimal Lacquer workflow to get you started:

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

To run this workflow:

```bash
laq run hello.laq.yaml
```

## Key Concepts

### Declarative Syntax

Lacquer uses YAML to define workflows declaratively. You describe *what* you want to happen, not *how* to do it.

### Agent-Based Architecture

Workflows are executed by AI agents that you configure with:
- **Models**: Choose from OpenAI, Anthropic, or local models
- **Prompts**: Define agent behavior and expertise
- **Tools**: Extend capabilities with external integrations

### Composable Workflows

Workflows can reference other workflows as steps, enabling:
- **Reusability**: Share common patterns across projects
- **Modularity**: Break complex workflows into manageable pieces
- **Maintainability**: Update shared components in one place

### Portable Execution

Workflows run anywhere:
- **Local machines**: Development and testing
- **Cloud platforms**: Production deployments
- **Edge devices**: Distributed processing

## File Extension Convention

Lacquer workflow files use the `.laq.yaml` extension to distinguish them from regular YAML files.

## Getting Started Guide

1. **Learn the Basics**
   - [Workflow Structure](./workflow-structure.md) - Understand workflow anatomy
   - [Agents](./agents.md) - Configure AI models and behavior
   - [Workflow Steps](./workflow-steps.md) - Define execution logic

2. **Add Advanced Features**
   - [Control Flow](./control-flow.md) - Conditional execution
   - [Tool Integration](./tools.md) - Extend agent capabilities
   - [State Management](./state-management.md) - Maintain workflow state

3. **Master Variable Usage**
   - [Variable Interpolation](./variables.md) - Dynamic values and expressions

4. **Explore Examples**
   - [Examples Directory](./examples/) - Real-world workflow patterns

## Support and Resources

- **Documentation**: You're here!
- **GitHub**: Report issues and contribute
- **Community**: Join discussions and share workflows