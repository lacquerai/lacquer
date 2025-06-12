# Examples

This directory contains comprehensive examples demonstrating all Lacquer workflow features. Each example focuses on specific aspects of the DSL while showing real-world usage patterns.

## Basic Examples

### [hello-world.laq.yaml](./hello-world.laq.yaml)
The simplest possible workflow - getting started with Lacquer.

### [research-workflow.laq.yaml](./research-workflow.laq.yaml)
Multi-step research workflow showing agents, outputs, and variable interpolation.

### [conditional-logic.laq.yaml](./conditional-logic.laq.yaml)
Demonstrates conditional execution and branching logic.

## Intermediate Examples

### [parallel-processing.laq.yaml](./parallel-processing.laq.yaml)
Shows parallel execution, for-each loops, and result aggregation.

### [error-handling.laq.yaml](./error-handling.laq.yaml)
Comprehensive error handling with retries, fallbacks, and recovery.

### [state-management.laq.yaml](./state-management.laq.yaml)
Stateful workflow with counters, progress tracking, and iterative improvement.

### [tool-integration.laq.yaml](./tool-integration.laq.yaml)
Using various tool types: official Lacquer tools, scripts, and MCP servers.

## Advanced Examples

### [content-pipeline.laq.yaml](./content-pipeline.laq.yaml)
Complete content creation pipeline using blocks and complex control flow.

### [enterprise-integration.laq.yaml](./enterprise-integration.laq.yaml)
Enterprise workflow with CRM, email notifications, and human approval.

### [machine-learning.laq.yaml](./machine-learning.laq.yaml)
ML workflow with data preprocessing, model training, and evaluation.

### [microservices-orchestration.laq.yaml](./microservices-orchestration.laq.yaml)
Orchestrating multiple microservices with circuit breakers and compensation.

## Running Examples

Each example can be run independently:

```bash
# Basic hello world
laq run examples/hello-world.laq.yaml

# Research with input
laq run examples/research-workflow.laq.yaml --input topic="AI safety"

# Validate before running
laq validate examples/content-pipeline.laq.yaml
laq run examples/content-pipeline.laq.yaml
```

## Example Structure

Each example includes:
- **Clear comments** explaining each feature
- **Real-world scenarios** showing practical usage
- **Error handling** demonstrating best practices
- **Variable usage** showing interpolation patterns
- **Output examples** showing expected results

## Custom Examples

Feel free to use these examples as templates for your own workflows. They demonstrate:

- Different agent configurations
- Various tool integrations
- Control flow patterns
- Error handling strategies
- State management techniques
- Output formatting
- Variable interpolation

Copy an example and modify it for your specific use case!