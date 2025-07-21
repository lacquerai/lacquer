# Workflow Steps

Steps are the individual units of execution within a Lacquer workflow. Each step performs a specific task using an agent, block, or system action.

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

```yaml
steps:
  - id: analyze_data
    agent: analyst
    prompt: "Analyze this dataset"
```

### agent
**Required**: Yes (unless using `uses` or `run` or `container`)  
**Type**: String  
**Description**: The agent to use for this step.

```yaml
steps:
  - id: write_content
    agent: writer
    prompt: "Write an article"
```

### prompt
**Required**: Yes (for agent steps)  
**Type**: String  
**Description**: The instruction/prompt for the agent.

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
**Description**: Bash script or command to execute.

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
**Description**: Reference to child workflow.

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
**Description**: Docker container to run.

```yaml
steps:
  - id: process_in_container
    container: alpine:latest
```

### command
**Required**: No 
**Type**: Array of strings
**Description**: Command to run in the container.

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
**Required**: No (only used with `run`, `uses`)
**Type**: Object
**Description**: Input parameters for the step.

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
**Description**: Updates to the workflow state when the step completes.

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
**Description**: Named outputs from the step.

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

### 3. Container Steps

Runs commands in Docker containers:

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

## Step Execution Order

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

Steps can also reference themselves

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

## Complex Step Example

Here's a comprehensive example showing various step features:

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

## Step Best Practices

1. **Use descriptive IDs**: `analyze_customer_data` not `step1`
2. **Keep prompts focused**: One task per step
3. **Define outputs explicitly**: Makes data flow clear
4. **Validate data early**: Check inputs before processing
5. **Use child workflows for reusability**: Keep things DRY and maintainable

## Next Steps

- Learn about [Control Flow](./control-flow.md) for parallel execution and loops
- Explore [Tool Integration](./tools.md) to extend agent capabilities
- See [Examples](./examples/agents/) for more agent configurations