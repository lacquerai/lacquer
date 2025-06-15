# Agents

Agents are the core execution units in Lacquer workflows. They represent AI models configured with specific parameters, prompts, and tools to perform tasks.

## Basic Agent Definition

```yaml
agents:
  my_agent:
    model: gpt-4
    temperature: 0.7
```

## Agent Properties

### model
**Required**: Yes  
**Type**: String  
**Description**: The AI model to use for this agent.

Supported models include:
- OpenAI: `gpt-4`, `gpt-4-turbo`, `gpt-3.5-turbo`
- Anthropic: `claude-3-opus`, `claude-3-sonnet`, `claude-3-haiku`
- Google: `gemini-pro`, `gemini-pro-vision`
- Open source: Various models via providers

```yaml
agents:
  writer:
    model: claude-3-opus
```

### temperature
**Required**: No  
**Type**: Float (0.0 - 2.0)  
**Default**: Model-specific default  
**Description**: Controls randomness in responses. Lower values (0.0-0.5) are more focused, higher values (0.5-2.0) are more creative.

```yaml
agents:
  creative_writer:
    model: gpt-4
    temperature: 1.2  # More creative
  
  fact_checker:
    model: gpt-4
    temperature: 0.1  # More deterministic
```

### system_prompt
**Required**: No  
**Type**: String  
**Description**: Sets the agent's behavior and expertise.

```yaml
agents:
  legal_expert:
    model: gpt-4
    temperature: 0.2
    system_prompt: |
      You are a legal expert specializing in contract law.
      Always cite relevant statutes and precedents.
      Provide balanced analysis of legal issues.
```

### max_tokens
**Required**: No  
**Type**: Integer  
**Description**: Maximum tokens in the response.

```yaml
agents:
  summarizer:
    model: gpt-4
    max_tokens: 500  # Keep summaries concise
```

### top_p
**Required**: No  
**Type**: Float (0.0 - 1.0)  
**Description**: Alternative to temperature for controlling randomness.

```yaml
agents:
  analyst:
    model: gpt-4
    top_p: 0.1  # Very focused responses
```

### tools
**Required**: No  
**Type**: Array  
**Description**: Tools available to the agent. See [Tool Integration](./tools.md) for details.

```yaml
agents:
  researcher:
    model: gpt-4
    tools:
      - name: web_search
        uses: lacquer/web-search@v1
      - name: calculate
        script: ./tools/calculator.py
```

## Using Pre-built Agents

Lacquer provides pre-built agent configurations through the `uses` syntax:

```yaml
agents:
  researcher:
    uses: lacquer/researcher@v1
    with:
      model: gpt-4
      temperature: 0.3
      focus_areas:
        - technology
        - science
```

### Overriding Pre-built Agent Properties

You can override specific properties of pre-built agents:

```yaml
agents:
  custom_researcher:
    uses: lacquer/researcher@v1
    with:
      temperature: 0.5  # Override default temperature
    system_prompt: |
      You are a researcher focused on environmental topics.
      # This overrides the default system prompt
```

## Agent Examples

### Research Agent
```yaml
agents:
  researcher:
    model: gpt-4
    temperature: 0.3
    system_prompt: |
      You are a meticulous researcher who:
      - Finds credible, recent sources
      - Cross-references information
      - Identifies conflicting viewpoints
      - Cites all sources properly
    tools:
      - name: search
        uses: lacquer/web-search@v1
      - name: read_pdf
        uses: lacquer/pdf-reader@v1
```

### Creative Writer Agent
```yaml
agents:
  creative_writer:
    model: claude-3-opus
    temperature: 0.9
    max_tokens: 2000
    system_prompt: |
      You are a creative writer who:
      - Uses vivid, engaging language
      - Creates compelling narratives
      - Maintains consistent tone and style
      - Adapts to different genres and formats
```

### Code Review Agent
```yaml
agents:
  code_reviewer:
    model: gpt-4
    temperature: 0.2
    system_prompt: |
      You are an expert code reviewer who:
      - Identifies bugs and security issues
      - Suggests performance improvements
      - Ensures code follows best practices
      - Provides constructive feedback
    tools:
      - name: analyze_code
        uses: lacquer/code-analyzer@v1
```

### Data Analysis Agent
```yaml
agents:
  data_analyst:
    model: gpt-4
    temperature: 0.1
    system_prompt: |
      You are a data analyst who:
      - Performs statistical analysis
      - Creates clear visualizations
      - Identifies trends and patterns
      - Provides actionable insights
    tools:
      - name: query_db
        uses: lacquer/postgresql@v1
      - name: analyze_csv
        uses: lacquer/csv-processor@v1
```

## Agent Inheritance

Agents can inherit from other agents:

```yaml
agents:
  base_analyst:
    model: gpt-4
    temperature: 0.2
    system_prompt: You are a thorough analyst
  
  financial_analyst:
    extends: base_analyst
    system_prompt: |
      You are a financial analyst specializing in:
      - Market analysis
      - Financial modeling
      - Risk assessment
    tools:
      - name: market_data
        uses: lacquer/financial-data@v1
```

## Dynamic Agent Configuration

Agents can be configured dynamically using environment variables:

```yaml
agents:
  configurable_agent:
    model: "{{ env.AI_MODEL | default('gpt-4') }}"
    temperature: "{{ env.AI_TEMPERATURE | default(0.7) | float }}"
    system_prompt: |
      You are an AI assistant.
      Environment: {{ env.ENVIRONMENT | default('production') }}
```

## Multi-Model Agents

Define agents that can switch between models based on task requirements:

```yaml
agents:
  adaptive_agent:
    model: "{{ workflow.state.selected_model | default('gpt-4') }}"
    temperature: 0.5
    system_prompt: |
      Adapt your expertise based on the task at hand.
```

## Agent Output Parsing

Lacquer automatically parses agent responses to extract structured data based on output definitions in workflow steps. This allows you to work with specific fields from agent responses rather than raw text.

### How Output Parsing Works

When an agent step defines outputs, Lacquer attempts to:
1. Parse JSON responses automatically
2. Extract structured data from natural language
3. Convert values to the specified types
4. Fall back to raw response if parsing fails

### Output Type Support

Supported output types:
- `string` - Text values
- `integer` - Whole numbers
- `float` / `number` - Decimal numbers
- `boolean` - True/false values
- `array` - Lists of items
- `object` - Complex data structures

### Example: Parsing JSON Responses

```yaml
steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze this data and return results as JSON"
    outputs:
      score: integer
      status: string
      items: array
```

If the agent responds with:
```json
{"score": 85, "status": "success", "items": ["A", "B", "C"]}
```

You can access:
- `{{ steps.analyze.outputs.score }}` → 85
- `{{ steps.analyze.outputs.status }}` → "success"
- `{{ steps.analyze.outputs.items }}` → ["A", "B", "C"]

### Example: Natural Language Extraction

```yaml
steps:
  - id: review
    agent: reviewer
    prompt: "Review this document"
    outputs:
      rating: integer
      approved: boolean
      issues: array
```

If the agent responds with:
```
After reviewing the document:
Rating: 8/10
Approved: yes
Issues:
- Missing references
- Unclear conclusion
- Grammar errors
```

Lacquer extracts:
- `{{ steps.review.outputs.rating }}` → 8
- `{{ steps.review.outputs.approved }}` → true
- `{{ steps.review.outputs.issues }}` → ["Missing references", "Unclear conclusion", "Grammar errors"]

### Example: Complex Output Structures

```yaml
steps:
  - id: process
    agent: processor
    prompt: "Process and categorize items"
    outputs:
      summary: string
      categories: object
      total_count: integer
```

### Accessing Outputs

Always available:
- `{{ steps.step_id.response }}` - Raw agent response
- `{{ steps.step_id.output }}` - Default output (same as response if no outputs defined)

With defined outputs:
- `{{ steps.step_id.outputs.field_name }}` - Specific parsed field

### Schema-Guided Output Parsing

Lacquer automatically generates JSON schemas from your output definitions and includes them in the agent prompt to ensure more deterministic parsing. This dramatically improves the reliability of structured output extraction.

#### How Schema Guidance Works

When you define outputs, Lacquer:
1. Generates a JSON schema from your output definitions
2. Adds schema instructions to the agent prompt
3. Prioritizes JSON parsing with auto-correction for common issues
4. Falls back to natural language extraction if needed

#### Example: Schema-Guided Response

```yaml
steps:
  - id: analyze_document
    agent: analyzer
    prompt: "Analyze this document for key insights"
    outputs:
      summary: string
      confidence: 
        type: number
        minimum: 0
        maximum: 1
      recommendations:
        type: array
        items: string
      actionable: boolean
```

**Generated Schema Instructions:**
```
IMPORTANT: You must respond with a valid JSON object that matches this exact schema:

{
  "type": "object",
  "properties": {
    "summary": {"type": "string"},
    "confidence": {"type": "number", "minimum": 0, "maximum": 1},
    "recommendations": {"type": "array", "items": {"type": "string"}},
    "actionable": {"type": "boolean"}
  },
  "required": ["summary", "confidence", "recommendations", "actionable"]
}
```

#### Advanced Output Definitions

```yaml
outputs:
  # Simple types
  name: string
  count: integer
  score: float
  active: boolean
  tags: array
  
  # Complex types with validation
  user_info:
    type: object
    description: "User profile information"
    properties:
      name: string
      age: integer
  
  status:
    type: string
    enum: ["pending", "completed", "failed"]
  
  # Array with specific item types
  scores:
    type: array
    items: number
    minItems: 1
    maxItems: 10
  
  # String with constraints
  description:
    type: string
    minLength: 10
    maxLength: 500
    
  # Optional fields
  notes:
    type: string
    optional: true
```

#### Automatic JSON Correction

Lacquer automatically fixes common JSON formatting issues:
- Trailing commas: `{"key": "value",}` → `{"key": "value"}`
- Single quotes: `{'key': 'value'}` → `{"key": "value"}`
- Unquoted keys: `{key: "value"}` → `{"key": "value"}`

### Tips for Reliable Output Parsing

1. **Define clear output schemas**: Use specific types and constraints
   ```yaml
   outputs:
     score:
       type: integer
       minimum: 0
       maximum: 100
     status:
       type: string
       enum: ["pass", "fail"]
   ```

2. **Use descriptive field names**: Clear names improve agent understanding
   ```yaml
   outputs:
     confidence_score: number    # Better than "score"
     is_valid: boolean          # Better than "valid"
     error_messages: array      # Better than "errors"
   ```

3. **Include validation constraints**: Help guide agent responses
   ```yaml
   outputs:
     summary:
       type: string
       minLength: 50
       maxLength: 200
     priority:
       type: string
       enum: ["low", "medium", "high", "critical"]
   ```

4. **Handle optional fields**: Mark non-required outputs as optional
   ```yaml
   outputs:
     result: string
     details:
       type: string
       optional: true
   ```

5. **Test with edge cases**: Verify parsing works with various response formats
   ```yaml
   condition: "{{ steps.analyze.outputs.score is defined and steps.analyze.outputs.score > 70 }}"
   ```

## Agent Best Practices

1. **Use descriptive names**: `legal_advisor` instead of `agent1`
2. **Set appropriate temperature**: Lower for factual tasks, higher for creative tasks
3. **Write clear system prompts**: Be specific about the agent's role and constraints
4. **Limit tools**: Only provide tools the agent actually needs
5. **Consider token limits**: Set `max_tokens` to control costs and response length
6. **Test different models**: Different models excel at different tasks

## Agent Policies

Configure policies for agent behavior:

```yaml
agents:
  restricted_agent:
    model: gpt-4
    temperature: 0.3
    policies:
      max_retries: 3
      timeout: 30s
      require_human_approval: true
      cost_limit: "$0.50"
```

## Next Steps

- Learn about [Workflow Steps](./workflow-steps.md) to use agents in your workflow
- Explore [Tool Integration](./tools.md) to extend agent capabilities
- See [Examples](./examples/agents/) for more agent configurations