# Workflow Structure

This document describes the fundamental structure of a Lacquer workflow file and all its components.

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

The `version` field specifies the Lacquer DSL version. Currently, only version `"1.0"` is supported.

```yaml
version: "1.0"
```

**Required**: Yes  
**Type**: String  
**Valid Values**: `"1.0"`

## Metadata

The `metadata` section contains information about the workflow.

```yaml
metadata:
  name: my-workflow
  description: A workflow that does something useful
  author: user@example.com
  tags:
    - automation
    - ai
  version: 1.2.3
```

### Metadata Fields

#### name
**Required**: Yes  
**Type**: String  
**Description**: A unique identifier for the workflow. Should be kebab-case.

```yaml
metadata:
  name: content-generator
```

#### description
**Required**: No  
**Type**: String  
**Description**: A human-readable description of what the workflow does.

```yaml
metadata:
  description: Generates blog posts from research topics
```

## Agents Section

The `agents` section defines reusable AI agent configurations. See [Agents Documentation](./agents.md) for details.

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

## Inputs Section

The `inputs` section (at the root level) defines parameters for the workflow:

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

## Requirements Section

The optional `requirements` section specifies runtime environments:

```yaml
requirements:
  runtimes:
    - name: go
      version: "1.21"
    - name: python
      version: "3.9"
```

## Workflow Section

The `workflow` section contains the execution logic:

```yaml
workflow:
  state:
    # Workflow state (optional)
  
  steps:
    # Workflow steps
  
  outputs:
    # Workflow outputs
```

### Input Parameters (Root Level)

Define parameters at the root level of the workflow:

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

#### Input Field Properties

##### type
**Required**: Yes  
**Valid Values**: `string`, `integer`, `boolean`, `array`, `object`  
**Description**: The data type of the input parameter.

##### description
**Required**: No  
**Type**: String  
**Description**: Human-readable description of the parameter.

##### required
**Required**: No  
**Type**: Boolean  
**Default**: `false`  
**Description**: Whether the parameter must be provided.

##### default
**Required**: No  
**Type**: Matches the parameter type  
**Description**: Default value if not provided.

## Complete Example

Here's a complete workflow showing all structural elements:

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

## File Naming Conventions

- Use `.laq.yaml` extension for all workflow files
- Use kebab-case for filenames: `my-workflow.laq.yaml`

## Best Practices

1. **Always include metadata**: Even if optional, metadata helps with workflow management
2. **Use descriptive names**: Both for the workflow and step IDs
3. **Document inputs**: Always include descriptions for input parameters

## Next Steps

- Learn about [Agents](./agents.md) to configure AI models
- Understand [Workflow Steps](./workflow-steps.md) for defining workflow logic
- Explore [Variable Interpolation](./variables.md) for dynamic workflows