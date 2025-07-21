# Tool Integration

Tools extend agent capabilities by providing access to external services, APIs, and custom functionality. Lacquer currently supports two primary tool integration methods: Script Tools and MCP (Model Context Protocol) Servers.

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
**Required**: No  
**Type**: String  
**Description**: Description of the tool, this is used to describe the tool to the agent. Make it as detailed as possible so that the agent can use the tool correctly.

### parameters
**Required**: Yes (if tool is a local script) 
**Type**: Object  
**Description**: Parameters for the tool.

These are the parameters that will be passed to the tool when it is called by the agent. Each parameter has must define a type and a description. It is recommended to make the description as detailed as possible so that the agent can use the tool correctly.

### mcp_server
**Required**: Yes (if tool is a MCP server)
**Type**: Object
**Description**: MCP server configuration.

This is the configuration for the MCP server that will be used to call the tool.

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

## Tool Integration Methods

### 1. Local Script Tools

When using a local script tool it is important to define the parameters for the tool. This is done by defining the parameters object in the tool configuration. These parameters will be passed to the script when it is called by the agent. For example:

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

This will pass a json object to the script with the following structure:

```json
{
  "inputs": {
    "data": "some data",
    "limit": 10
  }
}
```

Here is an example python script that will print the input data and limit:

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

### 2. MCP Servers

Use local or remote MCP servers to extend agent capabilities.

#### Local MCP Servers

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

#### Remote MCP Servers

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

## Tool Best Practices

1. **Clear Tool Names**: Use descriptive names that indicate function
2. **Comprehensive Documentation**: Document all parameters and outputs using the description property

## Next Steps

- Learn about [State Management](./state-management.md) for stateful tools
- See [Examples](./examples/tools/) for real-world tool implementations