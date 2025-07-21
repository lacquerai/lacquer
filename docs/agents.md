# Agents

Agents are the core execution units in Lacquer workflows. They represent AI models configured with specific parameters, prompts, and tools to perform tasks.

## Table of Contents

- [Basic Agent Definition](#basic-agent-definition)
- [Agent Properties](#agent-properties)
- [Provider-Specific Configuration](#provider-specific-configuration)
- [Agent Output Parsing](#agent-output-parsing)
- [Best Practices](#best-practices)
- [Examples](#examples)

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

**Supported providers:**
- `openai` - OpenAI models (GPT-4, GPT-3.5, etc.)
- `anthropic` - Anthropic models (Claude family)
- `local` - Local models (currently only `claude-code`)

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

> **Note**: Available models are dynamically fetched from each provider's API. The models listed below are examples and may change over time. Lacquer automatically caches the available models list for 24 hours.

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
**Description**: Controls randomness in responses.

- **Lower values (0.0-0.5)**: More focused and deterministic
- **Higher values (0.5-2.0)**: More creative and varied

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
**Description**: Sets the agent's behavior, expertise, and constraints.

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
**Description**: Maximum number of tokens in the response. Useful for controlling response length and API costs.

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
**Description**: Alternative to temperature for controlling randomness using nucleus sampling.

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
**Description**: Tools available to the agent for extending capabilities.

For detailed tool configuration, see [Tool Integration](./tools.md).

```yaml
agents:
  researcher:
    provider: openai
    model: gpt-4
    tools:
      - name: web_search
        script: "go run scripts/web_search.go"
```

## Provider-Specific Configuration

Different providers may support different models and features. Here's what you need to know:

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

## Examples

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

## Best Practices

### 1. Use Descriptive Names

Choose agent names that clearly indicate their purpose:
- ✅ `legal_advisor`, `content_writer`, `data_analyzer`
- ❌ `agent1`, `helper`, `ai`

### 2. Set Appropriate Temperature

Match temperature to the task:
- **Factual tasks** (0.0-0.3): Research, analysis, fact-checking
- **Balanced tasks** (0.4-0.7): General assistance, summaries
- **Creative tasks** (0.8-2.0): Creative writing, brainstorming

### 3. Write Clear System Prompts

Be specific about:
- The agent's role and expertise
- Expected behavior and constraints
- Output format preferences
- Tone and style guidelines

### 4. Optimize Tool Usage

- Only provide tools the agent actually needs
- Document tool purposes clearly
- Test tool integration thoroughly

### 5. Manage Token Usage

- Set `max_tokens` to control costs
- Consider response length requirements
- Monitor usage for optimization

### 6. Test Different Models

- Different models excel at different tasks
- Benchmark performance for your use case
- Consider cost vs. capability trade-offs

## Related Documentation

- [Workflow Steps](./workflow-steps.md) - Use agents in your workflow
- [Tool Integration](./tools.md) - Extend agent capabilities
- [Variable Interpolation](./variables.md) - Dynamic agent configuration
- [Examples](./examples/) - See agents in action