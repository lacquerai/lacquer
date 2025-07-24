# Workflow Steps

Steps are the individual units of execution within a Lacquer workflow. Each step performs a specific task using an agent, child workflow, script, or container.

## Basic Step Structure

```yaml
workflow:
  steps:
    - id: my_step
      agent: agent_name
      prompt: "Do something"
```

## Step Properties

### id

**Required**: Yes  
**Type**: String  
**Description**: Unique identifier for the step within the workflow.

> **Important**: Step IDs must be unique within a workflow and should use lowercase letters, numbers, and underscores.

```yaml
steps:
  - id: analyze_data
    agent: analyst
    prompt: "Analyze this dataset"
```

### agent

**Required**: Yes (unless using `uses`, `run`, or `container`)  
**Type**: String  
**Description**: References an agent defined in the `agents` section.

```yaml
steps:
  - id: write_content
    agent: writer
    prompt: "Write an article"
```

### prompt

**Required**: Yes (for agent steps)  
**Type**: String  
**Description**: The instruction or query for the agent to process.

```yaml
steps:
  - id: summarize
    agent: summarizer
    prompt: |
      Summarize the following text in 3 bullet points:
      ${{ inputs.text }}
```

### run

**Required**: No  
**Type**: String  
**Description**: Shell command or script to execute.

> **Note**: Commands run in a bash shell by default.

```yaml
steps:
  - id: process_data
    run: "go run ./scripts/processor.go"
    with:
      data: ${{ inputs.raw_data }}
```

### uses

**Required**: No  
**Type**: String  
**Description**: Path to another workflow file to execute as a step.

```yaml
steps:
  - id: research
    uses: "./workflows/research.laq.yml"
    with:
      topic: ${{ inputs.topic }}
      depth: comprehensive
```

### container

**Required**: No  
**Type**: String  
**Description**: Docker image to run the step in.

```yaml
steps:
  - id: process_in_container
    container: alpine:latest
```

### command

**Required**: No (for container steps)  
**Type**: Array of strings  
**Description**: Command and arguments to run in the container.

```yaml
steps:
  - id: process_in_container
    container: alpine:latest
    command:
      - "sh"
      - "-c"
      - "echo 'Processing data'"
```

### with

**Required**: No  
**Type**: Object  
**Description**: Input parameters for child workflows or environment variables for scripts.

```yaml
steps:
  - id: process_in_container
    run: "go run ./scripts/processor.go"
    with:
      data: ${{ inputs.data }}
```

### updates

**Required**: No  
**Type**: Object  
**Description**: State variables to update when the step completes successfully.

```yaml
steps:
  - id: research
    agent: researcher
    prompt: "Research {{ inputs.topic }}"
    outputs:
      findings:
        type: string
    updates:
      findings: ${{ steps.research.outputs.findings }}
```

### outputs

**Required**: No  
**Type**: Object  
**Description**: Defines expected outputs from the step with their types.

```yaml
steps:
  - id: analyze
    agent: analyst
    prompt: "Analyze market trends"
    outputs:
      trends: 
        type: array
        items: string
      summary:
        type: string
      confidence:
        type: number
```

## Step Types

### 1. Agent Steps

The most common type - executes a prompt using an AI agent:

```yaml
steps:
  - id: generate_ideas
    agent: creative_writer
    prompt: |
      Generate 5 creative ideas for articles about ${{ inputs.topic }}.
      Format as a numbered list.
    outputs:
      ideas:
        type: array
        items: string
```

Read more about how to use agents in steps [here](agents.md).

### 2. Child Workflow Steps

Use another workflow as a step:

```yaml
steps:
  - id: fetch_data
    uses: "./workflows/http-client.laq.yml"
    with:
      url: "https://api.example.com/data"
      method: GET
      headers:
        Authorization: "Bearer ${{ env.API_TOKEN }}"
```

### 3. Script Steps

Execute shell commands or scripts:

You can use variables directly in the run step:
```yaml
steps:
  - id: process_data
    run: |
      echo "Processing data..."
      python3 process.py --input "${{ inputs.data }}"
```

or use the `with` property to pass variables to the stdin with JSON

```yaml
steps:
  - id: process_data
    run: "python3 ./process.py"
    with:
      input: ${{ inputs.data }}
```

```python
# process.py

import json
import sys

def main():
    # Read input from stdin
    try:
        input_data = sys.stdin.read()
        if input_data:
            inputs = json.loads(input_data)
        else:
            inputs = {}
    except json.JSONDecodeError:
        inputs = {}
    
    # Extract the data from the input
    test_param = inputs.get('inputs', {}).get('data')
    
    ...

if __name__ == '__main__':
    main()
```

### 4. Container Steps

Run commands in Docker containers:

```yaml
steps:
  - id: process_in_container
    container: alpine:latest
    command:
      - "sh"
      - "-c"
      - "echo 'Processing data'"
    with:
      data: ${{ inputs.data }}
```

## Step Execution

### Sequential Execution

By default, steps execute sequentially in the order they appear:

```yaml
steps:
  - id: step1
    agent: agent1
    prompt: "First task"
  
  - id: step2
    agent: agent2
    prompt: "Second task - uses ${{ steps.step1.output }}"
  
  - id: step3
    agent: agent3
    prompt: "Third task - uses ${{ steps.step2.output }}"
```

## Accessing Step Outputs

Reference outputs from previous steps using the `steps` context:

```yaml
steps:
  - id: research
    agent: researcher
    prompt: "Research {{ inputs.topic }}"
    outputs:
      findings: array
      sources: array
  
  - id: write
    agent: writer
    prompt: |
      Write an article based on these findings:
      ${{ join(steps.research.outputs.findings, '\n') }}
      
      Cite these sources:
      ${{ join(steps.research.outputs.sources, '\n') }}
```

### Self-referencing Steps

Steps can reference their own outputs in updates:

```yaml
steps:
  - id: research
    agent: researcher
    prompt: "Research {{ inputs.topic }}"
    outputs:
      findings:
        type: string
    updates:
      findings: ${{ steps.research.outputs.findings }}
```

### Default Output

If no specific outputs are defined, use `output` to access the default:

```yaml
steps:
  - id: generate
    agent: generator
    prompt: "Generate content"
  
  - id: enhance
    agent: enhancer
    prompt: "Enhance: ${{ steps.generate.output }}"
```

## Conditional Steps

Steps can be conditionally executed:

```yaml
steps:
  - id: check_quality
    agent: reviewer
    prompt: "Rate quality (1-10): {{ inputs.content }}"
    outputs:
      score: integer
  
  - id: improve_content
    condition: ${{ steps.check_quality.outputs.score < 7 }}
    agent: writer
    prompt: "Improve this content: ${{ inputs.content }}"
```

Read more about conditional steps [here](control-flow.md).

## Related Documentation

- [Control Flow](control-flow.md) - Conditional execution patterns
- [Variable Interpolation](variables.md) - Dynamic values in steps
- [State Management](state-management.md) - Updating workflow state
- [Tool Integration](tools.md) - Extending agent capabilities
- [Examples](examples/) - Complete workflow examples