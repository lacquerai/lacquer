# Control Flow

Lacquer provides powerful control flow mechanisms to create complex, dynamic workflows. This includes parallel execution, conditional logic, loops, and branching.

## Conditional Execution

### Basic Conditions

Use the `condition` field to conditionally execute steps:

```yaml
steps:
  - id: check_length
    agent: analyzer
    prompt: "Count words in: {{ inputs.text }}"
    outputs:
      word_count: integer
  
  - id: expand_text
    condition: "{{ steps.check_length.outputs.word_count < 100 }}"
    agent: writer
    prompt: "Expand this text to at least 100 words: {{ inputs.text }}"
```

### Condition Expressions

Conditions support various operators and functions:

```yaml
# Boolean operators
condition: "{{ steps.validate.outputs.is_valid and not steps.validate.outputs.has_errors }}"

# Comparison operators
condition: "{{ steps.score.outputs.value >= 8 }}"
condition: "{{ steps.count.outputs.total != 0 }}"

# String operations
condition: "{{ steps.classify.outputs.category == 'urgent' }}"
condition: "{{ 'error' in steps.check.outputs.message }}"

# List operations
condition: "{{ steps.analyze.outputs.risks | length > 0 }}"
condition: "{{ 'critical' in steps.analyze.outputs.risk_levels }}"

# Environment variables
condition: "{{ env.ENVIRONMENT == 'production' }}"

# Complex conditions
condition: "{{ (steps.score.outputs.value > 7 or steps.review.outputs.approved) and env.AUTO_PUBLISH == 'true' }}"
```

### Negation and Skip Conditions

```yaml
steps:
  - id: premium_feature
    condition: "{{ inputs.plan == 'premium' }}"
    agent: premium_processor
    prompt: "Process premium features"
  
  - id: basic_feature
    skip_if: "{{ inputs.plan == 'premium' }}"  # Alternative syntax
    agent: basic_processor
    prompt: "Process basic features"
```

## Parallel Execution

### Basic Parallel Steps

Execute multiple steps concurrently:

```yaml
steps:
  - id: parallel_analysis
    parallel:
      steps:
        - id: sentiment_analysis
          agent: sentiment_analyzer
          prompt: "Analyze sentiment: {{ inputs.text }}"
        
        - id: keyword_extraction
          agent: keyword_extractor
          prompt: "Extract keywords: {{ inputs.text }}"
        
        - id: summary_generation
          agent: summarizer
          prompt: "Summarize: {{ inputs.text }}"
```

### Parallel with Max Concurrency

Control the number of concurrent executions:

```yaml
steps:
  - id: process_batch
    parallel:
      max_concurrency: 3  # Only 3 tasks run at once
      steps:
        - id: task1
          agent: processor
          prompt: "Process item 1"
        
        - id: task2
          agent: processor
          prompt: "Process item 2"
        
        # ... more tasks
```

### Parallel For Each

Process collections in parallel:

```yaml
steps:
  - id: get_items
    agent: fetcher
    prompt: "Get all items to process"
    outputs:
      items: array
  
  - id: process_items
    parallel:
      for_each: "{{ steps.get_items.outputs.items }}"
      as: item
      max_concurrency: 5
      steps:
        - id: process_single_item
          agent: processor
          prompt: |
            Process this item:
            ID: {{ item.id }}
            Name: {{ item.name }}
            Data: {{ item.data }}
          outputs:
            result: object
```

### Map-Reduce Pattern

Parallel processing with aggregation:

```yaml
steps:
  - id: map_reduce_analysis
    parallel:
      map:
        over: "{{ inputs.documents }}"
        as: document
        steps:
          - id: analyze_document
            agent: analyzer
            prompt: |
              Extract key insights from:
              {{ document.content }}
            outputs:
              insights: array
              sentiment: string
      
      reduce:
        agent: synthesizer
        prompt: |
          Synthesize these insights into a coherent summary:
          {{ map_results | json }}
        outputs:
          summary: string
          overall_sentiment: string
```

## Loops

### While Loops

Execute steps while a condition is true:

```yaml
steps:
  - id: iterative_improvement
    while: "{{ state.quality_score < 8 and state.iterations < 5 }}"
    steps:
      - id: generate
        agent: creator
        prompt: |
          Improve this content (iteration {{ state.iterations + 1 }}):
          {{ state.current_content }}
      
      - id: evaluate
        agent: evaluator
        prompt: "Rate quality (1-10): {{ steps.generate.output }}"
        outputs:
          score: integer
      
      - id: update_state
        action: update_state
        updates:
          current_content: "{{ steps.generate.output }}"
          quality_score: "{{ steps.evaluate.outputs.score }}"
          iterations: "{{ state.iterations + 1 }}"
```

### For Loops

Iterate over a collection:

```yaml
steps:
  - id: process_each_category
    for_each: "{{ inputs.categories }}"
    as: category
    index_as: index
    steps:
      - id: analyze_category
        agent: analyst
        prompt: |
          Analyze category {{ index + 1 }} of {{ inputs.categories | length }}:
          Category: {{ category.name }}
          Items: {{ category.items | length }}
```

### Do-While Pattern

Execute at least once, then check condition:

```yaml
steps:
  - id: get_user_input
    action: human_input
    prompt: "Please provide your input"
    outputs:
      response: string
      satisfied: boolean
  
  - id: refine_until_satisfied
    while: "{{ not steps.get_user_input.outputs.satisfied }}"
    steps:
      - id: improve_response
        agent: assistant
        prompt: |
          Improve based on feedback:
          {{ steps.get_user_input.outputs.response }}
      
      - id: get_feedback
        action: human_input
        prompt: |
          Here's the improved version:
          {{ steps.improve_response.output }}
          
          Are you satisfied?
        outputs:
          satisfied: boolean
          feedback: string
```

## Switch Statements

Multi-way branching based on values:

```yaml
steps:
  - id: classify_request
    agent: classifier
    prompt: "Classify this request: {{ inputs.request }}"
    outputs:
      category: string
      confidence: float
  
  - id: route_by_category
    switch:
      on: "{{ steps.classify_request.outputs.category }}"
      cases:
        technical:
          agent: tech_support
          prompt: "Handle technical issue: {{ inputs.request }}"
        
        billing:
          agent: billing_support
          prompt: "Handle billing inquiry: {{ inputs.request }}"
        
        sales:
          agent: sales_rep
          prompt: "Handle sales inquiry: {{ inputs.request }}"
        
        default:
          agent: general_support
          prompt: "Handle general inquiry: {{ inputs.request }}"
```

### Switch with Complex Cases

```yaml
steps:
  - id: advanced_routing
    switch:
      on: "{{ steps.analyze.outputs }}"
      cases:
        - condition: "{{ on.priority == 'urgent' and on.type == 'bug' }}"
          steps:
            - id: urgent_bug_fix
              agent: senior_developer
              prompt: "Fix urgent bug: {{ on.description }}"
        
        - condition: "{{ on.priority == 'high' }}"
          steps:
            - id: high_priority
              agent: experienced_handler
              prompt: "Handle high priority: {{ on.description }}"
        
        - condition: "{{ on.estimated_hours > 40 }}"
          steps:
            - id: large_project
              uses: lacquer/project-planner@v1
              with:
                description: "{{ on.description }}"
                hours: "{{ on.estimated_hours }}"
        
        default:
          steps:
            - id: standard_handling
              agent: handler
              prompt: "Process: {{ on.description }}"
```

## Nested Control Flow

Combine different control flow mechanisms:

```yaml
steps:
  - id: complex_workflow
    parallel:
      for_each: "{{ inputs.regions }}"
      as: region
      max_concurrency: 3
      steps:
        - id: process_region
          steps:
            - id: check_region_data
              agent: validator
              prompt: "Validate data for {{ region.name }}"
              outputs:
                is_valid: boolean
                issues: array
            
            - id: process_if_valid
              condition: "{{ steps.check_region_data.outputs.is_valid }}"
              parallel:
                steps:
                  - id: analyze_sales
                    agent: sales_analyst
                    prompt: "Analyze sales for {{ region.name }}"
                  
                  - id: analyze_inventory
                    agent: inventory_analyst
                    prompt: "Check inventory for {{ region.name }}"
            
            - id: fix_issues
              condition: "{{ not steps.check_region_data.outputs.is_valid }}"
              for_each: "{{ steps.check_region_data.outputs.issues }}"
              as: issue
              steps:
                - id: resolve_issue
                  agent: resolver
                  prompt: "Fix issue: {{ issue.description }}"
```

## Control Flow Best Practices

### 1. Keep Conditions Simple
```yaml
# Good
condition: "{{ steps.validate.outputs.is_valid }}"

# Avoid overly complex conditions
condition: "{{ (steps.a.output and steps.b.output) or (steps.c.output and not steps.d.output) or steps.e.output }}"
```

### 2. Use Meaningful Variable Names
```yaml
# Good
for_each: "{{ inputs.customers }}"
as: customer

# Less clear
for_each: "{{ inputs.data }}"
as: item
```

### 3. Limit Nesting Depth
Keep control flow structures shallow for readability:
```yaml
# Prefer breaking into separate named steps
steps:
  - id: validate_all
    uses: lacquer/batch-validator@v1
    with:
      items: "{{ inputs.items }}"
  
  - id: process_valid_items
    parallel:
      for_each: "{{ steps.validate_all.outputs.valid_items }}"
      steps:
        # Process each item
```

### 4. Handle Edge Cases
```yaml
steps:
  - id: safe_processing
    condition: "{{ inputs.items | length > 0 }}"
    parallel:
      for_each: "{{ inputs.items }}"
      steps:
        # Process items
  
  - id: handle_empty
    condition: "{{ inputs.items | length == 0 }}"
    agent: notifier
    prompt: "No items to process"
```

## Performance Considerations

### Parallel Execution Limits
```yaml
steps:
  - id: batch_process
    parallel:
      max_concurrency: 10  # Prevent API rate limiting
      for_each: "{{ inputs.large_dataset }}"
      steps:
        - id: process_item
          agent: processor
          prompt: "Process {{ item }}"
          retry:
            max_attempts: 3
            backoff: exponential
```

### Early Termination
```yaml
steps:
  - id: search_until_found
    while: "{{ not state.found and state.attempts < 10 }}"
    steps:
      - id: search
        agent: searcher
        prompt: "Search for: {{ inputs.target }}"
        outputs:
          found: boolean
          result: object
      
      - id: update_search_state
        action: update_state
        updates:
          found: "{{ steps.search.outputs.found }}"
          attempts: "{{ state.attempts + 1 }}"
          result: "{{ steps.search.outputs.result }}"
      
      - id: early_exit
        condition: "{{ state.found }}"
        break: true  # Exit the loop early
```

## Next Steps

- Learn about [Blocks](./blocks.md) for reusable workflow components
- Explore [State Management](./state-management.md) for complex workflows
- Understand [Error Handling](./error-handling.md) for robust control flow