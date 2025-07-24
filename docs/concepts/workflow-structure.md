# Workflow Structure

This document describes the fundamental structure of a Lacquer workflow file and all its components. Understanding the workflow structure is essential for creating effective AI-powered automation.

## Basic Structure

Every Lacquer workflow follows this basic structure:

```yaml
version: "1.0"
metadata:
  name: workflow-name
  description: Optional description

inputs:
  # Input definitions for the workflow

agents:
  # Agent definitions (optional)

requirements:
  # Runtime requirements (optional)

workflow:
  # Workflow definition with steps and outputs
```

## Version

The `version` field specifies the Lacquer DSL version.

**Required**: Yes  
**Type**: String  
**Valid Values**: `"1.0"`

```yaml
version: "1.0"
```

> **Note**: Future versions may introduce new features while maintaining backward compatibility.

## Metadata

The `metadata` section contains information about the workflow.

```yaml
metadata:
  name: my-workflow
  description: A workflow that does something useful
```

### Metadata Fields

### name

**Required**: Yes  
**Type**: String  
**Description**: A unique identifier for the workflow.

**Naming conventions**:
- Use kebab-case (lowercase with hyphens)
- Be descriptive and specific
- Avoid generic names

```yaml
metadata:
  name: content-generator
```

### description

**Required**: No (but recommended)  
**Type**: String  
**Description**: A clear explanation of the workflow's purpose and functionality.

```yaml
metadata:
  description: Generates blog posts from research topics
```

## Agents

The `agents` section defines reusable AI agent configurations. Each agent represents a configured AI model with specific parameters and tools.

For detailed agent configuration, see [Agents Documentation](./agents.md).

```yaml
agents:
  researcher:
    provider: openai
    model: gpt-4
    temperature: 0.3
  
  writer:
    provider: anthropic
    model: claude-3-opus-20240229
    temperature: 0.7
```

## Inputs

The `inputs` section defines parameters that users provide when running the workflow. Well-designed inputs make workflows flexible and reusable.

```yaml
inputs:
  topic:
    type: string
    description: The topic to research
    required: true
  max_words:
    type: integer
    default: 1000
```

## Requirements

The optional `requirements` section specifies runtime dependencies needed to execute the workflow. This ensures the workflow runs in the correct environment.

```yaml
requirements:
  runtimes:
    - name: go
      version: "1.21"
    - name: python
      version: "3.9"
```

### Runtime Requirements

```yaml
requirements:
  runtimes:
    - name: go
      version: "1.21"
    - name: python
      version: "3.9"
    - name: node
      version: "18"
```

### Container Requirements

```yaml
requirements:
  containers:
    - name: postgres
      image: "postgres:15"
    - name: redis
      image: "redis:7"
```

## Workflow

The `workflow` section contains the execution logic, including state management, steps, and outputs.

```yaml
workflow:
  state:
    # Workflow state (optional)
  
  steps:
    # Workflow steps
  
  outputs:
    # Workflow outputs
```

### Defining Inputs

```yaml
inputs:
  topic:
    type: string
    description: The topic to research
    required: true
  
  max_words:
    type: integer
    description: Maximum word count
    default: 1000
    required: false
  
  include_images:
    type: boolean
    description: Whether to include images
    default: false
```

### Input Properties

#### type

**Required**: Yes  
**Valid Values**: `string`, `integer`, `boolean`, `array`, `object`  
**Description**: The data type of the input parameter.

#### description

**Required**: No (but recommended)  
**Type**: String  
**Description**: Clear explanation of what the input is for.

#### required

**Required**: No  
**Type**: Boolean  
**Default**: `false`  
**Description**: Whether the user must provide this input.

#### default

**Required**: No  
**Type**: Must match the parameter type  
**Description**: Value to use if the input is not provided.

### Input Examples

```yaml
inputs:
  # Simple string input
  topic:
    type: string
    description: The topic to research
    required: true
  
  # Integer with default
  max_results:
    type: integer
    description: Maximum number of results to return
    default: 10
  
  # Boolean flag
  verbose:
    type: boolean
    description: Enable detailed logging
    default: false
  
  # Array input
  keywords:
    type: array
    description: Keywords to search for
    default: []
  
  # Complex object
  config:
    type: object
    description: Advanced configuration options
    default:
      timeout: 300
      retries: 3
```

### Workflow Properties

#### state

Defines initial workflow state:

```yaml
workflow:
  state:
    counter: 0
    results: []
    status: "pending"
```

#### steps

Defines the sequence of operations:

```yaml
workflow:
  steps:
    - id: process
      agent: processor
      prompt: "Process data"
```

#### outputs

Defines what the workflow returns:

```yaml
workflow:
  outputs:
    result: ${{ steps.final.output }}
    summary: ${{ state.summary }}
```

## Complete Examples

### Simple Workflow

A minimal workflow structure:

```yaml
version: "1.0"
metadata:
  name: simple-processor
  description: Basic data processing workflow

agents:
  processor:
    provider: openai
    model: gpt-4
    temperature: 0.5

inputs:
  data:
    type: string
    description: Data to process
    required: true

workflow:
  steps:
    - id: process
      agent: processor
      prompt: "Process this data: ${{ inputs.data }}"
  
  outputs:
    result: ${{ steps.process.output }}
```

### Advanced Workflow

A comprehensive workflow showing all features:

```yaml
version: "1.0"
metadata:
  name: research-assistant
  description: Researches topics and creates summaries

inputs:
  topic:
    type: string
    description: Topic to research
    required: true
  
  depth:
    type: string
    description: Research depth (basic, moderate, comprehensive)
    default: moderate
  
  format:
    type: string
    description: Output format (text, markdown, html)
    default: markdown

agents:
  researcher:
    provider: openai
    model: gpt-4
    temperature: 0.3
    system_prompt: You are a thorough researcher
  
  summarizer:
    provider: openai
    model: gpt-4
    temperature: 0.5
    system_prompt: You create concise summaries

workflow:
  
  steps:
    - id: research
      agent: researcher
      prompt: |
        Research the topic: ${{ inputs.topic }}
        Depth level: ${{ inputs.depth }}
    
    - id: summarize
      agent: summarizer
      prompt: |
        Create a ${{ inputs.format }} summary of:
        ${{ steps.research.output }}
  
  outputs:
    summary: ${{ steps.summarize.output }}
    word_count: ${{ length(steps.summarize.output) }}
    format: ${{ inputs.format }}
```

## File Organization

### File Naming Conventions

- **Extension**: Always use `.laq.yaml`
- **Naming**: Use kebab-case (e.g., `data-processor.laq.yaml`)
- **Organization**: Group related workflows in directories

## Related Documentation

- [Agents](agents.md) - Configure AI models and tools
- [Workflow Steps](workflow-steps.md) - Define execution logic
- [Variable Interpolation](variables.md) - Create dynamic workflows
- [State Management](state-management.md) - Maintain workflow state
- [Examples](examples/) - See complete workflow examples