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
        description: Search the web for the given query
        parameters:
          type: object
          properties:
            query:
              type: string
              description: The query to search for
```

### Creative Writer Agent
```yaml
agents:
  creative_writer:
    provider: anthropic
    model: claude-sonnet-4-20250514
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
      - name: filesystem
        description: Filesystem access for code analysis
        mcp_server:
          type: local
          command: npx
          args:
            - "-y"
            - "@modelcontextprotocol/server-filesystem"
            - "/usr/src/app"
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

When an agent step defines outputs lacquer will generate a JSON schema which it will automatically pass to the agent in the prompt. Then it will attempt to parse the response into the specified output types. Here's an example:

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

Lacquer will automatically append the following JSON schema to the agent's prompt:
```json
{
  "score": {
    "type": "integer"
  },
  "status": {
    "type": "string"
  },
  "items": {
    "type": "array",
    "items": {
      "type": "string"
    }
  }
}
```

The agent responds with the following
```json
I've analyzed the data and here are the results:
\`\`\`json
{"score": 85, "status": "success", "items": ["A", "B", "C"]}
\`\`\`
```

Lacquer will then parse the response into the specified output types.

You can can then access the outputs in the workflow:
- `${{ steps.analyze.outputs.score }}` → 85
- `${{ steps.analyze.outputs.status }}` → "success"
- `${{ steps.analyze.outputs.items }}` → ["A", "B", "C"]
- `${{ steps.analyze.output }}` → "full response"

## Related Documentation

- [Workflow Steps](workflow-steps.md) - Use agents in your workflow
- [Tool Integration](tools.md) - Extend agent capabilities
- [Variable Interpolation](variables.md) - Dynamic agent configuration
- [Examples](examples/) - See agents in action