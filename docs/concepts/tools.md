# Tool Integration

Tools extend agent capabilities by providing access to external services, APIs, and custom functionality. Lacquer supports two primary tool integration methods: Script Tools and MCP (Model Context Protocol) Servers.

## Basic Tool Structure

```yaml
agents:
  researcher:
    provider: openai
    model: gpt-4
    temperature: 0.2
    system_prompt: You are a helpful researcher that answers questions about a given topic.
    tools:
      - name: search_web
        script: "go run ./scripts/web_search.go"
        description: "Search the web for information"
        parameters:
          type: object
          properties:
            query:
              type: string
              description: "The query to search for"
      - name: time_converter
        description: Convert and analyze time across different timezones using MCP server
        mcp_server:
          type: local
          command: uvx
          args: ["mcp-server-time"]
          timeout: 1m
```

## Tool Properties

### name

**Required**: Yes  
**Type**: String  
**Description**: Unique identifier for the tool within the agent.

### description

**Required**: No (but highly recommended)  
**Type**: String  
**Description**: Describes the tool's purpose and capabilities to the agent.

> **Important**: Make descriptions detailed and specific. Agents use this information to decide when and how to use the tool.

### parameters

**Required**: Yes (for script tools)  
**Type**: Object  
**Description**: Defines the input parameters that the agent will provide to the tool.

Each parameter must include:
- `type`: The data type (string, integer, boolean, array, object)
- `description`: Clear description of the parameter's purpose

### mcp_server

**Required**: Yes (for MCP tools)  
**Type**: Object  
**Description**: Configuration for Model Context Protocol servers.

```yaml
tools:
  - name: time_converter
    description: Convert and analyze time across different timezones using MCP server
    mcp_server:
      type: local
      command: uvx
      args: ["mcp-server-time"]
      timeout: 1m
```

## Script Tools

Script tools allow you to integrate custom functionality through executable scripts in any language.

### Defining Script Tools

```yaml
tools:
  - name: analyze_data
    script: python3 ./tools/analyzer.py
    parameters:
      type: object
      description: Analyze the given data and return the results
      properties:
        data:
          type: string
          description: The data to analyze
        limit:
          type: integer
          description: The number of results to return
```

When the agent calls this tool, Lacquer passes a JSON object to the script via stdin:

```json
{
  "inputs": {
    "data": "some data",
    "limit": 10
  }
}
```

### Example Python Script

Here's a complete example of a Python script that handles tool input and output:

```python
#!/usr/bin/env python3
import json
import sys

def main():
    # Read input from stdin
    try:
        input_data = sys.stdin.read()
        if input_data:
            inputs = json.loads(input_data)
        else:
            inputs = {}
    except json.JSONDecodeError:
        inputs = {}
    
    # Extract any test parameter
    data = inputs.get('inputs', {}).get('data', 'default_value')
    limit = inputs.get('inputs', {}).get('limit', 10)
    
    # Do something with the data and limit

    result = {
      'results': f'Analyzed {data} with limit {limit}'
    }
    
    print(json.dumps({'outputs': result}))

if __name__ == '__main__':
    main()
```

### Script Requirements

1. **Read from stdin**: Scripts must read input from standard input
2. **Output to stdout**: Results must be printed to standard output as JSON
3. **Handle errors gracefully**: Return error messages in the output
4. **Be executable**: Ensure proper permissions and shebang lines

## MCP Servers

MCP (Model Context Protocol) servers provide a standardized way to extend agent capabilities with complex tools.

### Local MCP Servers

Run MCP servers locally for development or private tools:

```yaml
agents:
  mcp_agent:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0
    system_prompt: You are a helpful assistant that can use MCP tools to help answer questions about time and timezone operations.
    tools:
      - name: time_converter
        description: Convert and analyze time across different timezones using MCP server
        mcp_server:
          type: local
          command: uvx
          args: ["mcp-server-time"]
          timeout: 1m
```

### Remote MCP Servers

Connect to hosted MCP servers for shared functionality:

```yaml

agents:
  assistant:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0
    system_prompt: You are a helpful assistant that fetches information about a github repository.
    tools:
      - name: github_tools
        description: Tools for interacting with GitHub repositories via MCP
        mcp_server:
          type: remote
          url: "https://api.githubcopilot.com/mcp/"
          auth:
            type: oauth2
            client_id: "${GITHUB_CLIENT_ID}"
            client_secret: "${GITHUB_CLIENT_SECRET}"
            token_url: "https://api.github.com/oauth/token"
            scopes: "repo"
          timeout: 30s
```

## Tool Communication

### Input Format

Tools receive input as JSON with this structure:

```json
{
  "inputs": {
    "parameter1": "value1",
    "parameter2": "value2"
  }
}
```

### Output Format

Tools must return JSON with this structure:

```json
{
  "outputs": {
    "result": "processed value",
    "status": "success",
    "metadata": {}
  }
}
```

## Related Documentation

- [Agents](agents.md) - Configure agents with tools
- [Workflow Steps](workflow-steps.md) - Use tools in workflows
- [Examples](examples/) - See tools in action