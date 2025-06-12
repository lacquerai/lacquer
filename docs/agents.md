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