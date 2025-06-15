# Blocks and Reusability

Blocks are reusable workflow components that encapsulate common patterns, integrations, or complex logic. They're similar to functions in programming or actions in GitHub Actions.

## Using Blocks

### Basic Block Usage

Use the `uses` syntax to include a block in your workflow:

```yaml
steps:
  - id: search_web
    uses: lacquer/web-search@v1
    with:
      query: "{{ inputs.search_term }}"
      max_results: 10
```

### Block Sources

Blocks can come from multiple sources:

```yaml
# Official Lacquer blocks
- uses: lacquer/web-search@v1

# GitHub-hosted blocks
- uses: github.com/user/repo@v2.1.0

# Local blocks
- uses: ./blocks/my-custom-block

# Specific branch/commit
- uses: github.com/user/repo@main
- uses: github.com/user/repo@abc123f
```

## Block Definition

### Basic Block Structure

Create a block by defining a `block.laq.yaml` file:

```yaml
# block.laq.yaml
name: my-analyzer
version: 1.0.0
description: Analyzes data and provides insights
author: team@example.com

inputs:
  data:
    type: array
    description: Data to analyze
    required: true
  
  depth:
    type: string
    description: Analysis depth (basic, detailed, comprehensive)
    default: detailed

workflow:
  steps:
    - id: analyze
      agent: analyst
      prompt: |
        Perform {{ inputs.depth }} analysis on:
        {{ inputs.data | json }}

outputs:
  insights:
    description: Key insights from analysis
    value: "{{ steps.analyze.outputs.insights }}"
  
  summary:
    description: Brief summary
    value: "{{ steps.analyze.outputs.summary }}"
```

## Block Runtimes

Blocks support multiple runtime environments:

### 1. Native Runtime (Default)

Pure Lacquer blocks that compose other blocks and agents:

```yaml
# block.laq.yaml
name: content-pipeline
version: 1.0.0
runtime: native  # Optional, this is default

inputs:
  topic: string
  style: string

workflow:
  steps:
    - id: research
      uses: lacquer/web-search@v1
      with:
        query: "{{ inputs.topic }}"
    
    - id: write
      agent: writer
      prompt: |
        Write about {{ inputs.topic }} in {{ inputs.style }} style
        Based on: {{ steps.research.outputs.results }}

outputs:
  content: "{{ steps.write.output }}"
```

### 2. Go Script Runtime

For complex logic and calculations:

```yaml
# block.laq.yaml
name: data-processor
version: 1.0.0
runtime: go

inputs:
  data: array
  threshold: float

script: |
  package main
  import (
    "encoding/json"
    "github.com/lacquer/laq/sdk"
  )
  
  func main() {
    data := inputs.GetArray("data")
    threshold := inputs.GetFloat("threshold")
    
    var filtered []interface{}
    for _, item := range data {
      if val, ok := item.(map[string]interface{}); ok {
        if score, exists := val["score"].(float64); exists && score > threshold {
          filtered = append(filtered, item)
        }
      }
    }
    
    outputs.Set("filtered_data", filtered)
    outputs.Set("count", len(filtered))
  }

outputs:
  filtered_data:
    description: Data above threshold
  count:
    description: Number of items filtered
```

### 3. Docker Runtime

For maximum flexibility and language independence:

```yaml
# block.laq.yaml
name: ml-predictor
version: 1.0.0
runtime: docker
image: mycompany/ml-predictor:latest

inputs:
  model_name: string
  input_data: object

env:
  MODEL_PATH: "/models/{{ inputs.model_name }}"
  
command: ["python", "/app/predict.py"]

outputs:
  prediction:
    description: Model prediction
  confidence:
    description: Prediction confidence
```

## Official Lacquer Blocks

### Integration Blocks

#### HTTP Request
```yaml
steps:
  - uses: lacquer/http-request@v1
    with:
      url: "https://api.example.com/data"
      method: POST
      headers:
        Authorization: "Bearer {{ secrets.API_TOKEN }}"
        Content-Type: "application/json"
      body:
        query: "{{ inputs.query }}"
        limit: 100
```

#### Database Operations
```yaml
steps:
  - uses: lacquer/postgresql@v1
    with:
      connection_string: "{{ env.DATABASE_URL }}"
      query: |
        SELECT * FROM users 
        WHERE created_at > $1
        ORDER BY created_at DESC
      params:
        - "{{ inputs.start_date }}"
```

#### GitHub Integration
```yaml
steps:
  - uses: lacquer/github@v1
    with:
      action: create_issue
      repo: "{{ inputs.repository }}"
      title: "{{ inputs.issue_title }}"
      body: |
        ## Description
        {{ inputs.issue_description }}
        
        ## Assigned to
        @{{ inputs.assignee }}
      labels:
        - bug
        - high-priority
```

### Agent Blocks

Pre-configured agents for common tasks:

```yaml
steps:
  - id: research
    uses: lacquer/researcher@v1
    with:
      topic: "{{ inputs.topic }}"
      sources: ["academic", "news", "web"]
      min_sources: 5
      focus_areas:
        - recent developments
        - expert opinions
        - statistical data
```

## Creating Custom Blocks

### Block Development Workflow

1. Create block directory structure:
```bash
my-block/
├── block.laq.yaml
├── README.md
├── examples/
│   └── basic-usage.laq.yaml
└── tests/
    └── test-workflow.laq.yaml
```

2. Define block interface:
```yaml
# block.laq.yaml
name: sentiment-analyzer
version: 1.0.0
description: Analyzes sentiment with detailed emotions

inputs:
  text:
    type: string
    description: Text to analyze
    required: true
  
  include_emotions:
    type: boolean
    description: Include detailed emotion breakdown
    default: false

# Implementation...
```

3. Implement block logic:
```yaml
workflow:
  agents:
    analyzer:
      provider: openai
      model: gpt-4
      temperature: 0.1
      system_prompt: |
        You are a sentiment analysis expert.
        Always return structured JSON responses.

  steps:
    - id: analyze_sentiment
      agent: analyzer
      prompt: |
        Analyze sentiment of: "{{ inputs.text }}"
        
        Return JSON with:
        - sentiment: positive/negative/neutral
        - confidence: 0.0-1.0
        {% if inputs.include_emotions %}
        - emotions: {joy, sadness, anger, fear, surprise, disgust}
        {% endif %}
      outputs:
        result: object

outputs:
  sentiment: "{{ steps.analyze_sentiment.outputs.result.sentiment }}"
  confidence: "{{ steps.analyze_sentiment.outputs.result.confidence }}"
  emotions: "{{ steps.analyze_sentiment.outputs.result.emotions | default({}) }}"
```

### Block Versioning

Use semantic versioning for blocks:

```yaml
# Version 1.0.0 - Initial release
uses: lacquer/analyzer@v1.0.0

# Version 1.1.0 - Added new feature
uses: lacquer/analyzer@v1.1.0

# Version 2.0.0 - Breaking changes
uses: lacquer/analyzer@v2.0.0

# Latest v1.x version
uses: lacquer/analyzer@v1

# Specific commit
uses: github.com/user/blocks@abc123f
```

## Block Parameters

### Input Validation

Blocks can validate inputs:

```yaml
inputs:
  email:
    type: string
    pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
    description: Valid email address
  
  age:
    type: integer
    minimum: 18
    maximum: 120
  
  options:
    type: array
    min_items: 1
    max_items: 5
    items:
      type: string
      enum: ["option1", "option2", "option3"]
```

### Dynamic Inputs

Blocks can accept dynamic inputs:

```yaml
steps:
  - uses: lacquer/processor@v1
    with:
      # Static values
      mode: "advanced"
      
      # Dynamic values from previous steps
      data: "{{ steps.fetch_data.outputs.results }}"
      
      # Computed values
      threshold: "{{ inputs.base_threshold * 1.5 }}"
      
      # Conditional values
      verbose: "{{ env.DEBUG == 'true' }}"
```

## Block Composition

### Blocks Using Other Blocks

Blocks can compose other blocks:

```yaml
# block.laq.yaml
name: research-and-write
version: 1.0.0

workflow:
  steps:
    - id: research
      uses: lacquer/deep-research@v1
      with:
        topic: "{{ inputs.topic }}"
        depth: comprehensive
    
    - id: outline
      uses: lacquer/outline-generator@v1
      with:
        research: "{{ steps.research.outputs }}"
        style: "{{ inputs.writing_style }}"
    
    - id: write
      uses: lacquer/article-writer@v1
      with:
        outline: "{{ steps.outline.outputs }}"
        research: "{{ steps.research.outputs }}"
        word_count: "{{ inputs.target_words }}"
```

### Nested Block Calls

```yaml
steps:
  - id: complex_analysis
    uses: lacquer/analyzer@v1
    with:
      data: "{{ inputs.data }}"
      sub_blocks:
        - uses: lacquer/cleaner@v1
          with:
            data: "{{ parent.data }}"
        
        - uses: lacquer/transformer@v1
          with:
            cleaned_data: "{{ blocks[0].outputs }}"
```

## Block Testing

### Test Workflow Example

```yaml
# tests/test-sentiment.laq.yaml
version: "1.0"
metadata:
  name: test-sentiment-analyzer

workflow:
  steps:
    - id: test_positive
      uses: ../  # Parent directory block
      with:
        text: "I love this product! It's amazing!"
        include_emotions: true
    
    - id: verify_positive
      agent: validator
      prompt: |
        Verify this sentiment analysis:
        Result: {{ steps.test_positive.outputs }}
        
        Expected: positive sentiment with high confidence
```

## Block Best Practices

### 1. Clear Interfaces
```yaml
# Good - Clear, documented inputs
inputs:
  customer_id:
    type: string
    description: Unique customer identifier
    pattern: "^[A-Z]{2}[0-9]{6}$"
    required: true

# Avoid - Vague inputs
inputs:
  data: object  # What structure?
```

### 2. Semantic Versioning
- Use v1.0.0 for initial release
- Increment patch (v1.0.1) for bug fixes
- Increment minor (v1.1.0) for new features
- Increment major (v2.0.0) for breaking changes

### 3. Comprehensive Documentation
```yaml
# block.laq.yaml
name: data-enricher
version: 1.0.0
description: |
  Enriches customer data with external sources.
  
  Features:
  - Social media profile lookup
  - Company information
  - Email verification
  
  Rate limits: 100 requests/minute
```

### 4. Error Handling
```yaml
workflow:
  steps:
    - id: safe_operation
      agent: processor
      prompt: "Process: {{ inputs.data }}"
      on_error:
        - log: "Processing failed: {{ error.message }}"
        - output:
            success: false
            error: "{{ error.message }}"
```

### 5. Output Contracts
```yaml
outputs:
  result:
    description: Processing result
    schema:
      type: object
      properties:
        status: string
        data: array
        metadata: object
```

## Advanced Block Patterns

### Configurable Blocks
```yaml
# block.laq.yaml
inputs:
  config:
    type: object
    properties:
      mode:
        enum: ["fast", "balanced", "thorough"]
        default: "balanced"
      options:
        type: object
        default: {}

workflow:
  agents:
    processor:
      provider: openai
      model: "{{ inputs.config.mode == 'fast' ? 'gpt-3.5-turbo' : 'gpt-4' }}"
      temperature: "{{ inputs.config.mode == 'thorough' ? 0.2 : 0.5 }}"
```

### Multi-Strategy Blocks
```yaml
workflow:
  steps:
    - id: select_strategy
      switch:
        on: "{{ inputs.strategy }}"
        cases:
          aggressive:
            uses: ./strategies/aggressive
          
          conservative:
            uses: ./strategies/conservative
          
          balanced:
            uses: ./strategies/balanced
          
          default:
            agent: strategist
            prompt: "Determine best approach for: {{ inputs.data }}"
```

## Next Steps

- Learn about [Tool Integration](./tools.md) for extending block capabilities
- Explore [State Management](./state-management.md) for stateful blocks
- See [Examples](./examples/blocks/) for real-world block implementations