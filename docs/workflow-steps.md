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
**Required**: Yes (unless using `uses` or `action`)  
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
      {{ inputs.text }}
```

### action
**Required**: No (only for system actions)  
**Type**: String  
**Description**: System actions like `human_input` or `update_state`.

```yaml
steps:
  - id: get_approval
    action: human_input
    prompt: "Please review and approve this content"
    timeout: 1h
```

### uses
**Required**: No (alternative to agent)  
**Type**: String  
**Description**: Reference to a reusable block.

```yaml
steps:
  - id: research
    uses: lacquer/deep-research@v1
    with:
      topic: "{{ inputs.topic }}"
      depth: comprehensive
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
      trends: array
      summary: string
      confidence: float
```

## Step Types

### 1. Agent Steps

The most common type - executes a prompt using an AI agent:

```yaml
steps:
  - id: generate_ideas
    agent: creative_writer
    prompt: |
      Generate 5 creative ideas for articles about {{ inputs.topic }}.
      Format as a numbered list.
    outputs:
      ideas: array
```

### 2. Block Steps

Uses pre-built or custom blocks:

```yaml
steps:
  - id: fetch_data
    uses: lacquer/http-request@v1
    with:
      url: "https://api.example.com/data"
      method: GET
      headers:
        Authorization: "Bearer {{ secrets.API_TOKEN }}"
```

### 3. System Action Steps

Performs system-level actions:

```yaml
steps:
  # Human input
  - id: human_review
    action: human_input
    prompt: "Please review the generated content and provide feedback"
    timeout: 30m
    outputs:
      approved: boolean
      feedback: string
  
  # Update state
  - id: update_progress
    action: update_state
    updates:
      current_step: "{{ step_index }}"
      processed_items: "{{ state.processed_items + 1 }}"
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
    prompt: "Second task - uses {{ steps.step1.output }}"
  
  - id: step3
    agent: agent3
    prompt: "Third task - uses {{ steps.step2.output }}"
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
      {{ steps.research.outputs.findings | join('\n') }}
      
      Cite these sources:
      {{ steps.research.outputs.sources | join('\n') }}
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
    prompt: "Enhance: {{ steps.generate.output }}"
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
    condition: "{{ steps.check_quality.outputs.score < 7 }}"
    agent: writer
    prompt: "Improve this content: {{ inputs.content }}"
```

## Step Context Variables

Steps have access to various context variables:

```yaml
steps:
  - id: process
    agent: processor
    prompt: |
      Current step index: {{ step_index }}
      Total steps: {{ total_steps }}
      Workflow name: {{ metadata.name }}
      Environment: {{ env.ENVIRONMENT }}
      Previous output: {{ steps.previous.output }}
```

## Step Timeouts

Set execution timeouts for steps:

```yaml
steps:
  - id: long_task
    agent: analyst
    prompt: "Perform complex analysis"
    timeout: 5m  # 5 minutes
  
  - id: quick_task
    agent: assistant
    prompt: "Quick response needed"
    timeout: 30s  # 30 seconds
```

## Step Validation

Steps can include validation logic:

```yaml
steps:
  - id: validate_input
    agent: validator
    prompt: "Validate: {{ inputs.data }}"
    outputs:
      is_valid: boolean
      errors: array
  
  - id: process_data
    condition: "{{ steps.validate_input.outputs.is_valid }}"
    agent: processor
    prompt: "Process the validated data"
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
        Research "{{ inputs.topic }}" thoroughly.
        Find at least 5 credible sources.
        Focus on recent developments.
      timeout: 3m
      outputs:
        findings: array
        sources: array
        summary: string
    
    # Conditional quality check
    - id: check_research_quality
      agent: reviewer
      prompt: |
        Rate the quality of this research (1-10):
        {{ steps.research_topic.outputs.summary }}
      outputs:
        score: integer
        feedback: string
    
    # Conditional improvement
    - id: enhance_research
      condition: "{{ steps.check_research_quality.outputs.score < 8 }}"
      agent: researcher
      prompt: |
        Enhance the research based on this feedback:
        {{ steps.check_research_quality.outputs.feedback }}
        
        Original findings:
        {{ steps.research_topic.outputs.findings }}
    
    # Human approval
    - id: approve_research
      condition: "{{ env.REQUIRE_APPROVAL == 'true' }}"
      action: human_input
      prompt: |
        Please review and approve this research:
        {{ steps.enhance_research.output || steps.research_topic.outputs.summary }}
      timeout: 1h
      outputs:
        approved: boolean
        comments: string
    
    # Write article using block
    - id: write_article
      uses: lacquer/article-writer@v1
      with:
        research: "{{ steps.enhance_research.output || steps.research_topic.outputs }}"
        style: "{{ inputs.writing_style }}"
        word_count: "{{ inputs.word_count }}"
    
    # Update workflow state
    - id: track_progress
      action: update_state
      updates:
        articles_written: "{{ state.articles_written + 1 }}"
        last_topic: "{{ inputs.topic }}"
        last_score: "{{ steps.check_research_quality.outputs.score }}"
```

## Step Best Practices

1. **Use descriptive IDs**: `analyze_customer_data` not `step1`
2. **Keep prompts focused**: One task per step
3. **Define outputs explicitly**: Makes data flow clear
4. **Handle errors gracefully**: Use conditions to check for failures
5. **Set appropriate timeouts**: Prevent hanging workflows
6. **Validate data early**: Check inputs before processing
7. **Use blocks for reusability**: Don't repeat complex logic

## Next Steps

- Learn about [Control Flow](./control-flow.md) for parallel execution and loops
- Explore [Blocks](./blocks.md) for reusable components
- Understand [Error Handling](./error-handling.md) for robust workflows