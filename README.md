<div align="center">
<img width="1240" height="480" alt="lacquer-banner-stars" src="https://github.com/user-attachments/assets/42844a33-c8cb-404b-ba56-54b803615e03" />


[Installation](#-installation) • [Quick Start](#-quick-start) • [Documentation](https://lacquer.ai/docs) • [Examples](examples/) • [Community](https://discord.gg/lacquer)
</div>

---

## 🚀 What is Lacquer?

Lacquer (`laq`) is a blazing-fast, code-first orchestration engine for AI agent workflows. Write declarative YAML workflows that seamlessly coordinate multiple AI models and tools - all from a single binary.

```yaml
# hello-world.laq.yml
version: "1.0"
agents:
  assistant:
    provider: openai
    model: gpt-4
    temperature: 0.7

workflow:
  inputs:
    topic:
      type: string
      description: Topic to explore
  
  steps:
    - id: research
      agent: assistant
      prompt: "Tell me about ${{ inputs.topic }}"
    
    - id: summarize
      agent: assistant
      prompt: "Summarize this in 3 bullet points: ${{ steps.research.output }}"
  
  outputs:
    summary: "${{ steps.summarize.output }}"
```

```bash
$ laq run hello-world.laq.yml --input topic="quantum computing"

Running hello-world (2 steps)

✓ Step research completed (2.1s)
✓ Step summarize completed (1.3s)

✓ Workflow completed successfully

Outputs:

summary: "Quantum computing is a field of computing that uses quantum-mechanical phenomena, such as superposition and entanglement, to perform calculations."
```

## 🎯 Why Lacquer?

### For Engineers Who Want to Ship, Not Configure

- **Code-First**: Version control your workflows, ensure governance, bulletproof your AI features.
- **Single Binary**: Download and run. No Python environments, no dependency hell.
- **Extensible**: Build workflows with custom scripts, containers, and more
- **Lightning Fast**: <100ms validation, sub-second startup, efficient Go runtime
- **Provider Agnostic**: Works with OpenAI, Anthropic, Claude Code, and more coming soon

## 📦 Installation

### macOS/Linux (Coming Soon)
```bash
# Installation scripts are being prepared
# For now, build from source
```

### From Source
```bash
git clone https://github.com/lacquer/lacquer
cd lacquer
go build -o laq ./cmd/laq
```

### Docker (Coming Soon)
```bash
docker run -v ~/.laq:/root/.laq lacquer/laq:latest
```

## 🏃 Quick Start

### 1. Initialize a New Project
```bash
laq init my-agent
cd my-agent
```

### 2. Create Your First Workflow
```yaml
# analyze.laq.yml
version: "1.0"
metadata:
  name: data-analyzer
  description: Analyze data and provide insights

agents:
  analyst:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.3
    system_prompt: You are a data analyst expert.

workflow:
  inputs:
    data:
      type: string
      description: Data to analyze
    
  steps:
    - id: analyze
      agent: analyst
      prompt: |
        Analyze this data and provide key insights:
        ${{ inputs.data }}
        
    - id: format
      agent: analyst
      prompt: |
        Format these insights as a markdown report:
        ${{ steps.analyze.output }}
        
  outputs:
    report: "${{ steps.format.output }}"
```

### 3. Run It
```bash
laq run analyze.laq.yml --input data="Q4 sales increased 25% YoY..."
```

## 🌟 Current Features

### ✅ Core Functionality
- **YAML-based DSL** with complex logic and templating
- **Multi-provider support**: OpenAI, Anthropic, Claude Code
- **Complex workflow execution**: Sequential, parallel, conditional, and more
- **Serve your workflows as an HTTP API**

### ✅ CLI Commands
```bash
laq init       # Initialize new project with example workflow
laq validate   # Validate workflow syntax and semantics
laq run        # Execute workflow locally
laq serve      # Run as HTTP API server (with WebSocket support)
laq version    # Display version information
```

### ✅ Agent Configuration
```yaml
agents:
  researcher:
    provider: openai        # or anthropic, claude-code
    model: gpt-4           # or claude-3-5-sonnet-20241022
    temperature: 0.7
    max_tokens: 2000
    system_prompt: "You are a helpful research assistant"
```

### ✅ Script & Docker Steps
Execute custom code in any language using inline scripts or Docker containers:

**Go Script Steps:**
```yaml
steps:
  - id: process_data
    script: |
      package main
      import (
        "encoding/json"
        "os"
      )
      
      type Context struct {
        Inputs Inputs `json:"inputs"`
      }

      type Inputs struct {
        Text string `json:"text"`
      }
      
      func main() {
        var ctx Context
        json.NewDecoder(os.Stdin).Decode(&ctx)
        
        // Process inputs...
        result := ctx.Inputs.Text + " processed!"
        
        json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
          "outputs": map[string]interface{}{
            "result": result,
          },
        })
      }
    with:
      text: "${{ inputs.text }}"
```

**Docker Container Steps:**
```yaml
steps:
  - id: analyze_with_python
    container: ./.docker/python-analysis.dockerfile
    with:
      text: "${{ inputs.text }}"
```

Both approaches support full I/O via JSON, making it easy to integrate any tool or language.

### ✅ Server Mode
Run Lacquer as an HTTP API server:

```bash
laq serve --port 8080

# Execute workflows via REST API
curl -X POST http://localhost:8080/workflows/{workflow_id}/execute \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"data": "..."}}'
```

Features:
- REST API for workflow execution
- WebSocket support for streaming responses
- Prometheus metrics at `/metrics`
- Concurrent execution management
- CORS support for web integrations

### ✅ Tool Integration (Experimental)
- **MCP (Model Context Protocol)** client support
- **Script tools** for custom integrations
- Tool registry and execution framework

## 🛠️ Development

```bash
# Clone the repo
git clone https://github.com/lacquer/lacquer
cd lacquer

# Install dependencies
go mod download

# Build
go build -o laq ./cmd/laq

# Run tests
go test ./...

# Run with debug logging
./laq run workflow.laq.yml --log-level debug
```

## 🤝 Contributing

We welcome contributions! Lacquer is in early alpha, and we're actively seeking feedback and help with:

- Additional provider integrations
- More official blocks
- Documentation improvements
- Bug fixes and performance optimizations
- Example workflows

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📄 License

Lacquer is Apache 2.0 licensed. See [LICENSE](LICENSE) for details.

## 🚦 Project Status

Lacquer is in **early alpha** (v0.1.0). The core engine is functional and being actively developed. Expect breaking changes as we iterate based on community feedback.

---

<div align="center">

**Ready to try Lacquer?**

[Get Started](#-quick-start) • [Report Issues](https://github.com/lacquer/lacquer/issues) • [Star on GitHub](https://github.com/lacquer/lacquer)

</div>
