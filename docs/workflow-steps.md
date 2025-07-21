# Workflow Steps

Steps are the individual units of execution within a Lacquer workflow. Each step performs a specific task using an agent, child workflow, script, or container.

## Table of Contents

- [Basic Step Structure](#basic-step-structure)
- [Step Properties](#step-properties)
- [Step Types](#step-types)
- [Step Execution](#step-execution)
- [Accessing Step Outputs](#accessing-step-outputs)
- [Conditional Steps](#conditional-steps)
- [Best Practices](#best-practices)

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

```yaml
steps:
  - id: process_data
    run: |
      echo "Processing data..."
      python3 process.py --input "$DATA_FILE"
    with:
      DATA_FILE: ${{ inputs.file_path }}
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

### Error Handling

Steps fail if:
- An agent returns an error
- A script exits with non-zero status
- A container command fails
- Output parsing fails for defined outputs

Use conditions to handle failures:

```yaml
steps:
  - id: try_process
    agent: processor
    prompt: "Attempt to process"
    outputs:
      success:
        type: boolean
  
  - id: handle_failure
    condition: ${{ !steps.try_process.outputs.success }}
    agent: error_handler
    prompt: "Handle processing failure"
```

## Complex Examples

### Multi-Step Research Workflow

```yaml
workflow:
  steps:
    # Research step with tools
    - id: research_topic
      agent: researcher
      prompt: |
        Research "${{ inputs.topic }}" thoroughly.
        Find at least 5 credible sources.
        Focus on recent developments.
      timeout: 3m
      outputs:
        findings:
          type: array
          items: string
        sources:
          type: array
          items: string
        summary:
          type: string
    
    # Conditional quality check
    - id: check_research_quality
      agent: reviewer
      prompt: |
        Rate the quality of this research (1-10):
        ${{ steps.research_topic.outputs.summary }}
      outputs:
        score:
          type: integer
        feedback:
          type: string
    
    # Conditional improvement
    - id: enhance_research
      condition: ${{ steps.check_research_quality.outputs.score < 8 }}
      agent: researcher
      prompt: |
        Enhance the research based on this feedback:
        ${{ steps.check_research_quality.outputs.feedback }}
        
        Original findings:
        ${{ steps.research_topic.outputs.findings }}
    
    # Human approval
    - id: approve_research
      condition: ${{ inputs.require_approval == 'true' }}
      agent: reviewer
      prompt: |
        Please review and approve this research:
        ${{ steps.enhance_research.output }}
        Provide approval status and comments.
    
    # Write article using block
    - id: write_article
      agent: writer
      prompt: |
        Write an article based on this research:
        Research: ${{ steps.enhance_research.output }}
        Style: ${{ inputs.writing_style }}
        Target words: ${{ inputs.word_count }}
    
    # Update workflow state
    - id: track_progress
      run: "go run ./scripts/tracker.go"
      with:
        topic: ${{ inputs.topic }}
        score: ${{ steps.check_research_quality.outputs.score }}
```

### Data Processing Pipeline

```yaml
workflow:
  steps:
    # Load data from source
    - id: load_data
      run: "python3 ./scripts/load_data.py"
      with:
        source: ${{ inputs.data_source }}
      outputs:
        row_count:
          type: integer
        columns:
          type: array
    
    # Validate data quality
    - id: validate
      agent: data_validator
      prompt: |
        Validate data quality:
        - Rows: ${{ steps.load_data.outputs.row_count }}
        - Columns: ${{ join(steps.load_data.outputs.columns, ', ') }}
        
        Check for missing values, duplicates, and anomalies.
      outputs:
        is_valid:
          type: boolean
        issues:
          type: array
    
    # Process only if valid
    - id: transform
      condition: ${{ steps.validate.outputs.is_valid }}
      container: python:3.9
      command:
        - python
        - -c
        - |
          import pandas as pd
          # Transform logic here
          print('{"outputs": {"transformed": true}}')
    
    # Generate report
    - id: report
      agent: report_generator
      prompt: |
        Generate data quality report:
        - Original rows: ${{ steps.load_data.outputs.row_count }}
        - Validation: ${{ steps.validate.outputs.is_valid ? 'Passed' : 'Failed' }}
        - Issues found: ${{ length(steps.validate.outputs.issues) }}
        ${{ steps.transform.outputs.transformed ? '- Transformation: Complete' : '- Transformation: Skipped' }}
```

## Best Practices

### 1. Use Descriptive Step IDs

Choose IDs that clearly indicate the step's purpose:

```yaml
# Good
- id: fetch_user_data
- id: validate_email_format
- id: send_confirmation

# Avoid
- id: step1
- id: process
- id: done
```

### 2. Keep Steps Focused

Each step should have a single, clear responsibility:

```yaml
# Good: Focused steps
steps:
  - id: fetch_data
    agent: fetcher
    prompt: "Fetch user data for ID: ${{ inputs.user_id }}"
  
  - id: validate_data
    agent: validator
    prompt: "Validate the fetched user data"

# Avoid: Multi-purpose steps
steps:
  - id: fetch_and_validate
    agent: processor
    prompt: "Fetch user data and validate it, then prepare for storage"
```

### 3. Define Outputs Explicitly

Always define expected outputs for clarity:

```yaml
steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze customer sentiment"
    outputs:
      sentiment:
        type: string
        description: "positive, negative, or neutral"
      confidence:
        type: number
        description: "Confidence score 0-1"
      keywords:
        type: array
        description: "Key phrases detected"
```

### 4. Handle Edge Cases

Plan for failures and edge cases:

```yaml
steps:
  - id: fetch_external
    agent: fetcher
    prompt: "Fetch from external API"
    outputs:
      data:
        type: object
      status:
        type: string
  
  - id: handle_timeout
    condition: ${{ steps.fetch_external.outputs.status == 'timeout' }}
    agent: handler
    prompt: "Use cached data instead"
  
  - id: handle_error
    condition: ${{ steps.fetch_external.outputs.status == 'error' }}
    agent: notifier
    prompt: "Alert team about API failure"
```

### 5. Use Child Workflows for Reusability

Extract common patterns into separate workflows:

```yaml
# In data-validation.laq.yaml
workflow:
  inputs:
    data:
      type: object
  steps:
    - id: validate
      agent: validator
      prompt: "Validate data structure"

# In main workflow
steps:
  - id: validate_input
    uses: "./workflows/data-validation.laq.yaml"
    with:
      data: ${{ inputs.user_data }}
```

## Related Documentation

- [Control Flow](./control-flow.md) - Conditional execution patterns
- [Variable Interpolation](./variables.md) - Dynamic values in steps
- [State Management](./state-management.md) - Updating workflow state
- [Tool Integration](./tools.md) - Extending agent capabilities
- [Examples](./examples/) - Complete workflow examples