# Agents

Agents are the core execution units in Lacquer workflows. They represent AI models configured with specific parameters, prompts, and tools to perform tasks.

## Basic Agent Definition

```yaml
agents:
  my_agent:
    provider: openai
    model: gpt-4
    temperature: 0.7
```

## Agent Properties

### provider
**Required**: Yes (when using a model)  
**Type**: String  
**Description**: The AI provider for this agent.

Supported providers:
- `openai` 
- `anthropic`
- `local`

```yaml
agents:
  gpt_agent:
    provider: openai
    model: gpt-4
  
  claude_agent:
    provider: anthropic
    model: claude-sonnet-4-20250514
  
  claude_code_agent:
    provider: local
    model: claude-code
```

### model
**Required**: Yes
**Type**: String  
**Description**: The AI model to use for this agent.

**Note**: Available models are dynamically fetched from each provider's API. The models listed below are examples and may change over time. Lacquer automatically caches the available models list for 24 hours.

Example models by provider:
- **OpenAI**: `gpt-4`, `gpt-4-turbo`, `gpt-3.5-turbo`, ...
- **Anthropic**: `claude-sonnet-4-20250514`, `claude-3-5-sonnet-20241022`, ...
- **Local**: only `claude-code` is supported

```yaml
agents:
  writer:
    provider: anthropic
    model: claude-sonnet-4-20250514
```

### temperature
**Required**: No  
**Type**: Float (0.0 - 2.0)  
**Default**: Model-specific default  
**Description**: Controls randomness in responses. Lower values (0.0-0.5) are more focused, higher values (0.5-2.0) are more creative.

```yaml
agents:
  creative_writer:
    provider: openai
    model: gpt-4
    temperature: 1.2  # More creative
  
  fact_checker:
    provider: openai
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
    provider: openai
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
    provider: openai
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
    provider: openai
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
    provider: openai
    model: gpt-4
    tools:
      - name: web_search
        script: "go run scripts/web_search.go"
```

## Agent Configuration

All agent properties must be explicitly defined:

```yaml
agents:
  environmental_researcher:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.5
    system_prompt: |
      You are a researcher focused on environmental topics.
      Provide evidence-based analysis with credible sources.
```

## Agent Examples

### Research Agent
```yaml
agents:
  researcher:
    provider: openai
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
        script: "go run scripts/web_search.go"
        parameters:
          type: object
          properties:
            query:
              type: string
```

### Creative Writer Agent
```yaml
agents:
  creative_writer:
    provider: anthropic
    model: claude-3-opus-20240229
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
    provider: openai
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
        script: "go run ./tools/code-analyzer.go"
        parameters:
          type: object
          properties:
            code:
              type: string
            language:
              type: string
```

### Data Analysis Agent
```yaml
agents:
  data_analyst:
    provider: openai
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
        script: "python3 ./tools/db_query.py"
        parameters:
          type: object
          properties:
            query:
              type: string
      - name: analyze_csv
        script: "python3 ./tools/csv_analyzer.py"
        parameters:
          type: object
          properties:
            file_path:
              type: string
```


## Dynamic Agent Configuration

Agent's system prompt can be configured using inputs:

```yaml
agents:
  configurable_agent:
    provider: openai
    model: gpt-4
    temperature: 0.7
    system_prompt: |
      You are an AI assistant.
      Environment: ${{ inputs.environment }}
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
      score:
        type: integer
      status:
        type: string
      items:
        type: array
        items:
          type: string
```

If the agent responds with:
```json
{"score": 85, "status": "success", "items": ["A", "B", "C"]}
```

You can access:
- `${{ steps.analyze.outputs.score }}` → 85
- `${{ steps.analyze.outputs.status }}` → "success"
- `${{ steps.analyze.outputs.items }}` → ["A", "B", "C"]

## Agent Best Practices

1. **Use descriptive names**: `legal_advisor` instead of `agent1`
2. **Set appropriate temperature**: Lower for factual tasks, higher for creative tasks
3. **Write clear system prompts**: Be specific about the agent's role and constraints
4. **Limit tools**: Only provide tools the agent actually needs
5. **Consider token limits**: Set `max_tokens` to control costs and response length
6. **Test different models**: Different models excel at different tasks

## Next Steps

- Learn about [Workflow Steps](./workflow-steps.md) to use agents in your workflow
- Explore [Tool Integration](./tools.md) to extend agent capabilities
- See [Examples](./examples/agents/) for more agent configurations