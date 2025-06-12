# Variable Interpolation and Outputs

Lacquer uses a Jinja2-style template syntax for variable interpolation, allowing dynamic values throughout your workflows. This document covers variable usage, output handling, and available functions.

## Variable Syntax

### Basic Interpolation

Use double curly braces for variable interpolation:

```yaml
steps:
  - id: greet
    agent: assistant
    prompt: "Hello {{ inputs.name }}, welcome to {{ inputs.location }}!"
```

### Accessing Nested Values

Use dot notation for nested objects:

```yaml
prompt: |
  Customer: {{ inputs.customer.name }}
  Email: {{ inputs.customer.contact.email }}
  Order ID: {{ inputs.order.id }}
  Total: ${{ inputs.order.total }}
```

### Array Access

Access array elements by index:

```yaml
prompt: |
  First item: {{ inputs.items[0] }}
  Last item: {{ inputs.items[-1] }}
  Second category: {{ inputs.categories[1].name }}
```

## Variable Contexts

### Available Contexts

Lacquer provides several contexts for variable access:

```yaml
steps:
  - id: example
    agent: assistant
    prompt: |
      # Workflow inputs
      Input value: {{ inputs.some_value }}
      
      # Step outputs
      Previous result: {{ steps.previous_step.output }}
      Specific output: {{ steps.analyzer.outputs.score }}
      
      # Workflow state
      Counter: {{ state.counter }}
      Status: {{ state.current_status }}
      
      # Environment variables
      API URL: {{ env.API_BASE_URL }}
      Environment: {{ env.ENVIRONMENT }}
      
      # Secrets
      API Key: {{ secrets.API_KEY }}
      
      # Metadata
      Workflow name: {{ metadata.name }}
      Author: {{ metadata.author }}
      
      # Current context
      Step index: {{ step_index }}
      Step ID: {{ step.id }}
      Timestamp: {{ now() }}
```

## Step Outputs

### Defining Outputs

Explicitly define step outputs:

```yaml
steps:
  - id: analyze_data
    agent: analyst
    prompt: "Analyze this dataset: {{ inputs.data }}"
    outputs:
      summary: string
      insights: array
      metrics:
        type: object
        properties:
          average: float
          total: integer
          categories: array
```

### Default Output

When no outputs are defined, access the default output:

```yaml
steps:
  - id: generate
    agent: generator
    prompt: "Generate content"
  
  - id: use_output
    agent: processor
    prompt: "Process: {{ steps.generate.output }}"
```

### Multiple Outputs

Access specific outputs:

```yaml
steps:
  - id: multi_output
    agent: analyzer
    prompt: "Analyze comprehensively"
    outputs:
      summary: string
      score: float
      tags: array
      metadata: object
  
  - id: use_outputs
    agent: reporter
    prompt: |
      Summary: {{ steps.multi_output.outputs.summary }}
      Score: {{ steps.multi_output.outputs.score }}
      Tags: {{ steps.multi_output.outputs.tags | join(', ') }}
      Created: {{ steps.multi_output.outputs.metadata.created_at }}
```

## Filters and Functions

### String Filters

```yaml
prompt: |
  # Case conversion
  Upper: {{ inputs.text | upper }}
  Lower: {{ inputs.text | lower }}
  Title: {{ inputs.text | title }}
  
  # String manipulation
  Trimmed: {{ inputs.text | trim }}
  No spaces: {{ inputs.text | replace(' ', '_') }}
  First 50 chars: {{ inputs.text | truncate(50) }}
  
  # String checks
  Starts with 'Hello': {{ inputs.text | startswith('Hello') }}
  Contains 'world': {{ 'world' in inputs.text }}
  Length: {{ inputs.text | length }}
```

### Array Filters

```yaml
prompt: |
  # Array operations
  Count: {{ inputs.items | length }}
  First: {{ inputs.items | first }}
  Last: {{ inputs.items | last }}
  
  # Filtering
  High scores: {{ inputs.scores | select('>', 80) }}
  Active users: {{ inputs.users | selectattr('status', 'active') }}
  
  # Transformation
  Sorted: {{ inputs.numbers | sort }}
  Reversed: {{ inputs.items | reverse }}
  Unique: {{ inputs.tags | unique }}
  
  # Aggregation
  Sum: {{ inputs.numbers | sum }}
  Min: {{ inputs.numbers | min }}
  Max: {{ inputs.numbers | max }}
  Average: {{ inputs.numbers | average }}
  
  # Joining
  CSV: {{ inputs.items | join(', ') }}
  Lines: {{ inputs.lines | join('\n') }}
```

### Object Filters

```yaml
prompt: |
  # Object operations
  Keys: {{ inputs.data | keys }}
  Values: {{ inputs.data | values }}
  Items: {{ inputs.data | items }}
  
  # Merging
  Merged: {{ defaults | merge(inputs.overrides) }}
  
  # JSON
  As JSON: {{ inputs.complex_data | json }}
  Pretty JSON: {{ inputs.complex_data | json(indent=2) }}
```

### Date and Time

```yaml
prompt: |
  # Current time
  Now: {{ now() }}
  Today: {{ today() }}
  
  # Formatting
  Formatted: {{ now() | strftime('%Y-%m-%d %H:%M:%S') }}
  Date only: {{ now() | date }}
  Time only: {{ now() | time }}
  
  # Date math
  Tomorrow: {{ now() | add_days(1) }}
  Last week: {{ now() | add_days(-7) }}
  In 2 hours: {{ now() | add_hours(2) }}
  
  # Comparisons
  Is past: {{ inputs.deadline < now() }}
  Days until: {{ (inputs.deadline - now()) | days }}
```

### Conditional Expressions

```yaml
prompt: |
  # Ternary operator
  Status: {{ 'Active' if inputs.is_active else 'Inactive' }}
  
  # Default values
  Name: {{ inputs.name | default('Anonymous') }}
  Count: {{ inputs.count | default(0) }}
  
  # Null checking
  Has email: {{ inputs.email is not none }}
  Empty list: {{ inputs.items | length == 0 }}
```

## Advanced Interpolation

### Complex Expressions

```yaml
steps:
  - id: calculate
    agent: calculator
    prompt: |
      # Math operations
      Total: {{ inputs.price * inputs.quantity }}
      With tax: {{ (inputs.price * inputs.quantity * 1.08) | round(2) }}
      Discount: {{ inputs.price * (inputs.discount_percent / 100) }}
      
      # Boolean logic
      Eligible: {{ inputs.age >= 18 and inputs.score > 70 }}
      Special case: {{ inputs.type == 'premium' or inputs.vip == true }}
      
      # List comprehension
      Doubled: {{ [item * 2 for item in inputs.numbers] }}
      Filtered: {{ [user.name for user in inputs.users if user.active] }}
```

### Template Blocks

Use template blocks for complex logic:

```yaml
prompt: |
  {% if inputs.type == 'detailed' %}
    Provide a comprehensive analysis including:
    - Historical data
    - Trend analysis
    - Future projections
  {% elif inputs.type == 'summary' %}
    Provide a brief summary with key points
  {% else %}
    Provide a standard analysis
  {% endif %}
  
  {% for item in inputs.items %}
    {{ loop.index }}. {{ item.name }} - ${{ item.price }}
  {% endfor %}
  
  Total items: {{ inputs.items | length }}
```

### Loops in Templates

```yaml
prompt: |
  # Simple loop
  {% for user in inputs.users %}
  - {{ user.name }} ({{ user.email }})
  {% endfor %}
  
  # Loop with index
  {% for item in inputs.items %}
  {{ loop.index }}. {{ item.title }}
     Priority: {{ item.priority }}
     {% if item.tags %}
     Tags: {{ item.tags | join(', ') }}
     {% endif %}
  {% endfor %}
  
  # Conditional loop
  Active users:
  {% for user in inputs.users if user.status == 'active' %}
  - {{ user.name }}
  {% else %}
  No active users found
  {% endfor %}
```

## Variable Scoping

### Global vs Local Variables

```yaml
workflow:
  # Global scope
  inputs:
    global_setting: string
  
  state:
    global_counter: 0
  
  steps:
    - id: parallel_tasks
      parallel:
        for_each: "{{ inputs.tasks }}"
        as: task  # Local to this parallel block
        index_as: idx  # Local index
        steps:
          - id: process
            agent: processor
            prompt: |
              Global setting: {{ inputs.global_setting }}
              Local task: {{ task.name }}
              Local index: {{ idx }}
              Global counter: {{ state.global_counter }}
```

### Variable Precedence

Variables are resolved in this order:
1. Local loop variables (task, idx)
2. Step outputs (steps.*.outputs)
3. Workflow state
4. Workflow inputs
5. Environment variables
6. Default values

## Output Formatting

### Structured Outputs

Define complex output structures:

```yaml
steps:
  - id: analyze
    agent: analyst
    prompt: "Perform analysis"
    outputs:
      report:
        type: object
        properties:
          summary: string
          sections:
            type: array
            items:
              type: object
              properties:
                title: string
                content: string
                score: float
      
      metrics:
        type: array
        items:
          type: object
          properties:
            name: string
            value: float
            unit: string
```

### Output Transformation

Transform outputs before use:

```yaml
steps:
  - id: get_data
    agent: fetcher
    prompt: "Fetch data"
    outputs:
      raw_data: string
  
  - id: transform
    agent: transformer
    prompt: |
      Transform this data:
      {{ steps.get_data.outputs.raw_data | parse_json | selectattr('active') | list }}
```

### Output Validation

Validate outputs match expected format:

```yaml
outputs:
  result:
    type: object
    required: ["status", "data"]
    properties:
      status:
        type: string
        enum: ["success", "failure", "partial"]
      data:
        type: array
        min_items: 1
      error:
        type: string
        required: false
```

## Workflow Outputs

### Defining Workflow Outputs

```yaml
workflow:
  # ... steps ...
  
  outputs:
    # Simple output
    summary: "{{ steps.summarize.output }}"
    
    # Computed output
    total_score: "{{ steps.scores.outputs.values | sum }}"
    
    # Conditional output
    status: |
      {{ 'success' if steps.validate.outputs.is_valid else 'failure' }}
    
    # Complex output
    report:
      title: "{{ inputs.title }}"
      date: "{{ now() | date }}"
      sections: "{{ steps.analyze.outputs.sections }}"
      metrics: |
        {{
          {
            'total': steps.process.outputs.total,
            'average': steps.process.outputs.total / steps.process.outputs.count,
            'categories': steps.categorize.outputs.categories
          }
        }}
```

### Output Best Practices

1. **Use descriptive names**
```yaml
outputs:
  analysis_report: "{{ steps.analyze.output }}"  # Good
  output1: "{{ steps.step1.output }}"  # Avoid
```

2. **Document output types**
```yaml
outputs:
  customer_ids:
    value: "{{ steps.fetch.outputs.ids }}"
    description: "List of customer IDs that were processed"
    type: array
```

3. **Provide defaults for optional outputs**
```yaml
outputs:
  error_message: "{{ steps.process.outputs.error | default('No errors') }}"
  results: "{{ steps.analyze.outputs.results | default([]) }}"
```

## Custom Functions

### Registering Custom Functions

Lacquer allows custom functions in templates:

```yaml
# Using custom functions
prompt: |
  Hashed value: {{ inputs.password | hash('sha256') }}
  Encoded: {{ inputs.data | base64_encode }}
  Parsed URL: {{ inputs.url | parse_url }}
  Distance: {{ calculate_distance(point1, point2) }}
```

### Built-in Utility Functions

```yaml
prompt: |
  # Type checking
  Is string: {{ inputs.value is string }}
  Is number: {{ inputs.value is number }}
  Is array: {{ inputs.value is list }}
  
  # Type conversion
  As integer: {{ inputs.string_number | int }}
  As float: {{ inputs.string_number | float }}
  As string: {{ inputs.number | string }}
  As boolean: {{ inputs.string_bool | bool }}
  
  # Utility
  Random number: {{ random(1, 100) }}
  UUID: {{ uuid() }}
  Hash: {{ inputs.text | hash('md5') }}
```

## Debugging Variables

### Debug Output

```yaml
steps:
  - id: debug_vars
    agent: debugger
    prompt: |
      === Debug Information ===
      All inputs: {{ inputs | json(indent=2) }}
      
      Current state: {{ state | json(indent=2) }}
      
      Available steps:
      {% for step_id in steps.keys() %}
      - {{ step_id }}: {{ steps[step_id].outputs.keys() | list }}
      {% endfor %}
      
      Environment: {{ env.keys() | list }}
```

## Best Practices

1. **Use meaningful variable names**
2. **Provide defaults for optional values**
3. **Validate variable types when needed**
4. **Document expected input/output formats**
5. **Use filters to ensure correct types**
6. **Handle null/undefined values gracefully**

## Next Steps

- Create [Examples](./examples/) to see variables in action
- Review [Best Practices](./best-practices.md) for production workflows
- Explore [Advanced Patterns](./advanced-patterns.md) for complex scenarios