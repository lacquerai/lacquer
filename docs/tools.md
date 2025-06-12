# Tool Integration

Tools extend agent capabilities by providing access to external services, APIs, and custom functionality. Lacquer supports three primary tool integration methods: Official Lacquer Tools, Local Scripts, and MCP (Model Context Protocol) Servers.

## Overview

Tools are functions that agents can call during execution:

```yaml
agents:
  researcher:
    model: gpt-4
    tools:
      - name: search_web
        uses: lacquer/web-search@v1
      
      - name: analyze_data
        script: ./tools/analyzer.py
      
      - name: crm_access
        mcp_server: enterprise-crm
```

## Tool Integration Methods

### 1. Official Lacquer Tools

Pre-built, maintained tools with stability guarantees:

```yaml
agents:
  data_agent:
    model: gpt-4
    tools:
      # Web search tool
      - name: search
        uses: lacquer/web-search@v1
        config:
          max_results: 5
          timeout: 30s
          regions: ["us", "eu"]
      
      # HTTP request tool
      - name: api_call
        uses: lacquer/http-request@v1
        config:
          base_url: "https://api.example.com"
          auth_header: "Bearer {{ secrets.API_TOKEN }}"
          timeout: 60s
      
      # Database tool
      - name: query_db
        uses: lacquer/postgresql@v1
        config:
          connection_string: "{{ env.DATABASE_URL }}"
          read_only: true
          max_rows: 1000
```

### 2. Local Scripts

Custom tools as executable scripts:

```yaml
agents:
  processor:
    model: gpt-4
    tools:
      # Python script
      - name: ml_predict
        script: ./tools/predict.py
        config:
          interpreter: python3
          requirements: ./tools/requirements.txt
          timeout: 120s
          env:
            MODEL_PATH: "/models/latest"
      
      # Shell script
      - name: file_processor
        script: |
          #!/bin/bash
          input_file="$1"
          output_format="$2"
          
          # Process file based on format
          case "$output_format" in
            "json") jq '.' "$input_file" ;;
            "yaml") yq eval '.' "$input_file" ;;
            "csv") python -c "import pandas as pd; pd.read_json('$input_file').to_csv()" ;;
          esac
        config:
          interpreter: bash
      
      # Go script
      - name: fast_calc
        script: ./tools/calculator.go
        config:
          interpreter: go
          compile: true  # Pre-compile for performance
```

### 3. MCP Servers

Enterprise integrations via Model Context Protocol:

```yaml
agents:
  enterprise_agent:
    model: gpt-4
    tools:
      # Connect to running MCP server
      - name: salesforce
        mcp_server: salesforce-mcp
        config:
          endpoint: "mcp://sf-mcp.company.com:8080"
          auth: "{{ secrets.MCP_SF_TOKEN }}"
          tools: ["create_lead", "update_opportunity", "search_accounts"]
      
      # Multiple tools from same server
      - name: github_enterprise
        mcp_server: github-mcp
        config:
          endpoint: "mcp://github-mcp.internal:443"
          tls: true
          client_cert: "{{ secrets.GITHUB_CERT }}"
          tools: ["*"]  # All available tools

# MCP server configuration
mcp_servers:
  salesforce-mcp:
    image: company/salesforce-mcp:v2.1
    port: 8080
    health_check: "/health"
    env:
      SF_INSTANCE: "{{ env.SALESFORCE_URL }}"
  
  github-mcp:
    endpoint: "github-mcp.internal:443"
    tls:
      enabled: true
      verify: true
      ca_cert: "{{ secrets.CA_CERT }}"
```

## Official Lacquer Tools

### Web Search
```yaml
tools:
  - name: search_web
    uses: lacquer/web-search@v1
    config:
      max_results: 10
      search_engine: "google"  # google, bing, ddg
      safe_search: true
      language: "en"
      regions: ["us", "uk"]
```

Usage in prompts:
```yaml
prompt: |
  Research recent developments in quantum computing.
  Use the search_web tool to find articles from the last 6 months.
  Focus on practical applications.
```

### HTTP Requests
```yaml
tools:
  - name: api
    uses: lacquer/http-request@v1
    config:
      base_url: "https://api.service.com/v2"
      default_headers:
        Accept: "application/json"
        User-Agent: "Lacquer/1.0"
      auth:
        type: "bearer"
        token: "{{ secrets.API_TOKEN }}"
      retry:
        max_attempts: 3
        backoff: exponential
```

### Database Operations
```yaml
tools:
  - name: database
    uses: lacquer/postgresql@v1
    config:
      connection_string: "{{ env.DATABASE_URL }}"
      connection_pool:
        min: 2
        max: 10
      query_timeout: 30s
      read_only: false
```

### File Operations
```yaml
tools:
  - name: files
    uses: lacquer/file-system@v1
    config:
      base_path: "/data"
      allowed_operations: ["read", "write", "list"]
      max_file_size: "10MB"
      allowed_extensions: [".txt", ".json", ".csv"]
```

## Local Script Tools

### Script Configuration

```yaml
tools:
  - name: custom_analyzer
    script: ./tools/analyzer.py
    config:
      interpreter: python3
      working_dir: ./tools
      timeout: 60s
      max_memory: "512MB"
      env:
        PYTHONPATH: "./lib"
        DATA_DIR: "/tmp/data"
```

### Script Interface

Scripts receive input via stdin and return output via stdout:

```python
#!/usr/bin/env python3
# tools/analyzer.py
import json
import sys

def main():
    # Read input from stdin
    input_data = json.loads(sys.stdin.read())
    
    # Process data
    result = analyze(input_data)
    
    # Return output to stdout
    print(json.dumps(result))

def analyze(data):
    # Custom analysis logic
    return {
        "status": "success",
        "insights": ["insight1", "insight2"],
        "metrics": {"score": 0.95}
    }

if __name__ == "__main__":
    main()
```

### Script Languages

#### Python Scripts
```yaml
tools:
  - name: ml_tool
    script: ./tools/ml_predict.py
    config:
      interpreter: python3
      virtualenv: ./tools/venv
      requirements: ./tools/requirements.txt
```

#### Node.js Scripts
```yaml
tools:
  - name: web_scraper
    script: ./tools/scraper.js
    config:
      interpreter: node
      npm_install: true
      package_json: ./tools/package.json
```

#### Go Scripts
```yaml
tools:
  - name: performance_tool
    script: ./tools/optimizer.go
    config:
      interpreter: go
      compile: true
      build_flags: ["-ldflags", "-s -w"]
```

## MCP Server Integration

### MCP Configuration

```yaml
# Global MCP configuration
mcp_servers:
  crm-server:
    image: company/crm-mcp:latest
    port: 9000
    startup_timeout: 60s
    env:
      CRM_API_URL: "{{ env.CRM_URL }}"
      CRM_API_KEY: "{{ secrets.CRM_KEY }}"
    health_check:
      endpoint: "/health"
      interval: 30s
      timeout: 5s
```

### Using MCP Tools

```yaml
agents:
  sales_agent:
    model: gpt-4
    tools:
      - name: crm
        mcp_server: crm-server
        config:
          namespace: "sales"
          tools: 
            - "create_opportunity"
            - "update_lead"
            - "search_contacts"
          rate_limit:
            requests_per_minute: 60
```

### MCP Tool Discovery

```yaml
steps:
  - id: discover_tools
    agent: assistant
    prompt: |
      List available CRM tools using the crm.list_tools() function.
      Show their descriptions and parameters.
```

## Tool Policies and Controls

### Rate Limiting
```yaml
agents:
  api_agent:
    model: gpt-4
    tools:
      - name: external_api
        uses: lacquer/http-request@v1
    
    tool_policy:
      rate_limits:
        external_api:
          requests_per_minute: 30
          burst: 50
          strategy: "sliding_window"
```

### Access Control
```yaml
tool_policy:
  require_approval: ["delete_records", "send_email"]
  audit_log: true
  allowed_hours:
    start: "09:00"
    end: "17:00"
    timezone: "America/New_York"
```

### Cost Controls
```yaml
tool_policy:
  cost_limits:
    per_tool:
      expensive_api: "$1.00"
    total_per_run: "$5.00"
  
  usage_tracking:
    enabled: true
    report_to: "usage-metrics"
```

## Tool Error Handling

### Retry Configuration
```yaml
tools:
  - name: flaky_api
    uses: lacquer/http-request@v1
    config:
      retry:
        max_attempts: 3
        backoff: exponential
        initial_delay: 1s
        max_delay: 30s
        retry_on: [500, 502, 503, 504]
```

### Fallback Tools
```yaml
agents:
  resilient_agent:
    model: gpt-4
    tools:
      - name: primary_search
        uses: lacquer/web-search@v1
        on_error:
          fallback: backup_search
      
      - name: backup_search
        uses: lacquer/alternative-search@v1
        on_error:
          return:
            error: "Search unavailable"
            results: []
```

## Tool Development Guidelines

### 1. Input Validation
```python
def validate_input(data):
    required_fields = ["query", "limit"]
    for field in required_fields:
        if field not in data:
            raise ValueError(f"Missing required field: {field}")
    
    if not isinstance(data["limit"], int) or data["limit"] < 1:
        raise ValueError("Limit must be a positive integer")
```

### 2. Error Handling
```python
def safe_api_call(endpoint, params):
    try:
        response = requests.get(endpoint, params=params, timeout=30)
        response.raise_for_status()
        return response.json()
    except requests.exceptions.Timeout:
        return {"error": "Request timed out", "code": "TIMEOUT"}
    except requests.exceptions.RequestException as e:
        return {"error": str(e), "code": "API_ERROR"}
```

### 3. Structured Output
```python
def format_output(results):
    return {
        "status": "success",
        "data": results,
        "metadata": {
            "timestamp": datetime.now().isoformat(),
            "count": len(results),
            "version": "1.0"
        }
    }
```

## Advanced Tool Patterns

### Dynamic Tool Selection
```yaml
steps:
  - id: select_tool
    agent: planner
    prompt: |
      Determine which tool to use:
      - search_web: For general information
      - query_db: For customer data
      - api_call: For real-time data
      
      Query: {{ inputs.query }}
    outputs:
      selected_tool: string
  
  - id: execute_tool
    agent: executor
    prompt: |
      Use the {{ steps.select_tool.outputs.selected_tool }} tool
      to answer: {{ inputs.query }}
```

### Tool Chaining
```yaml
agents:
  analyst:
    model: gpt-4
    tools:
      - name: fetch_data
        uses: lacquer/http-request@v1
      
      - name: process_data
        script: ./tools/processor.py
      
      - name: visualize
        script: ./tools/chart_generator.py

steps:
  - id: analyze
    agent: analyst
    prompt: |
      1. Use fetch_data to get sales data from /api/sales
      2. Use process_data to calculate trends
      3. Use visualize to create a chart
      4. Summarize the findings
```

### Conditional Tool Usage
```yaml
agents:
  smart_agent:
    model: gpt-4
    tools:
      - name: cache_lookup
        script: ./tools/cache.py
      
      - name: expensive_api
        uses: lacquer/premium-data@v1
    
    tool_policy:
      tool_selection: |
        Always try cache_lookup first.
        Only use expensive_api if cache miss
        and user explicitly requests fresh data.
```

## Tool Best Practices

1. **Clear Tool Names**: Use descriptive names that indicate function
2. **Comprehensive Documentation**: Document all parameters and outputs
3. **Graceful Degradation**: Always handle errors and provide fallbacks
4. **Security First**: Validate inputs, sanitize outputs, use authentication
5. **Performance Optimization**: Cache results, batch operations when possible
6. **Versioning**: Version your tools and maintain backwards compatibility

## Next Steps

- Learn about [State Management](./state-management.md) for stateful tools
- Explore [Error Handling](./error-handling.md) for robust tool usage
- See [Examples](./examples/tools/) for real-world tool implementations