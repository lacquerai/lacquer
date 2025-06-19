<div align="center">
<img src="https://github.com/user-attachments/assets/3f60a020-402e-460e-a547-68f2521b2a5f">

[Installation](#-installation) • [Quick Start](#-quick-start) • [Documentation](https://docs.lacquer.ai) • [Examples](examples/) • [Community](https://discord.gg/lacquer)
</div>

---



## 🚀 What is Lacquer?

Lacquer (`laq`) is a blazing-fast, code-first orchestration engine for AI agent workflows. Write declarative YAML workflows that seamlessly coordinate multiple AI models, tools, and integrations - all from a single 10MB binary.

```yaml
# research.laq.yaml
agents:
  researcher:
    model: gpt-4
    tools: [lacquer/web-search@v1]

workflow:
  steps:
    - agent: researcher
      prompt: "Research the latest breakthroughs in {{ inputs.topic }}"
    
    - uses: lacquer/summarizer@v1
      with:
        content: "{{ steps[0].output }}"
        style: "technical blog post"
```

```bash
$ laq run research.laq.yaml --input topic="quantum computing"
✓ Researching latest breakthroughs... (2.1s)
✓ Generating summary... (1.3s)
✓ Workflow completed in 3.4s
```

## 🎯 Why Lacquer?

### For Engineers Who Want to Ship, Not Configure

- **Single Binary**: Download and run. No Python environments, no dependency hell.
- **Lightning Fast**: <100ms validation, sub-second startup, efficient Go runtime
- **Code-First**: Version control your workflows, review PRs, use your favorite IDE
- **Portable**: Run locally, in CI/CD, on Lambda, or Kubernetes - same workflow everywhere
- **Extensible**: Build custom blocks in Go, reuse workflows, share with the community

### Not Another No-Code Tool

While others focus on drag-and-drop interfaces that create productivity bottlenecks, Lacquer embraces what developers do best - write code. Define complex multi-agent workflows in YAML, version control them with Git, and deploy anywhere.

## 📦 Installation

### macOS/Linux
```bash
curl -sSL https://get.lacquer.ai | sh
```

### Homebrew
```bash
brew install lacquer/tap/laq
```

### Docker
```bash
docker run -v ~/.laq:/root/.laq lacquer/laq:latest
```

### From Source
```bash
go install github.com/lacquer/laq@latest
```

## 🏃 Quick Start

### 1. Initialize a New Project
```bash
laq init my-agent
cd my-agent
```

### 2. Create Your First Workflow
```yaml
# analyze.laq.yaml
version: "1.0"
agents:
  analyst:
    model: gpt-4
    temperature: 0.3

workflow:
  inputs:
    data: string
    
  steps:
    - id: analyze
      agent: analyst
      prompt: |
        Analyze this data and provide insights:
        {{ inputs.data }}
        
    - id: visualize
      uses: lacquer/chart-generator@v1
      with:
        data: "{{ steps.analyze.output }}"
        type: "insights-dashboard"
        
  outputs:
    insights: "{{ steps.analyze.output }}"
    chart_url: "{{ steps.visualize.output.url }}"
```

### 3. Run It
```bash
laq run analyze.laq.yaml --input data="Q4 sales increased 25% YoY..."
```

## 🌟 Key Features

### 🤖 Multi-Agent Orchestration
Coordinate multiple AI agents with different models and capabilities:

```yaml
agents:
  researcher:
    model: gpt-4
    tools: [lacquer/web-search@v1]
  
  critic:
    model: claude-3-opus
    temperature: 0.2
    
  writer:
    model: gpt-4-turbo
    temperature: 0.7
```

### 🔧 Rich Tool Ecosystem
Access pre-built integrations or build your own:

```yaml
tools:
  # Official Lacquer tools
  - lacquer/web-search@v1
  - lacquer/postgresql@v1
  - lacquer/github@v1
  - lacquer/slack@v1
  
  # Custom scripts
  - script: ./tools/analyzer.py
  
  # Enterprise integrations
  - mcp://salesforce.company.com
```

### 🔄 Advanced Control Flow
Build sophisticated workflows with parallel execution, conditions, and loops:

```yaml
steps:
  - id: validate
    agent: validator
    prompt: "Check if {{ inputs.data }} needs processing"
    
  - parallel:
      max_concurrency: 5
      for_each: "{{ steps.validate.output.items }}"
      steps:
        - agent: processor
          prompt: "Process {{ item }}"
          
  - condition: "{{ steps.validate.output.requires_review }}"
    agent: reviewer
    prompt: "Review the processed results"
```

### 🔍 Built-in Observability
Debug and monitor your workflows with structured logging and metrics:

```bash
$ laq run workflow.laq.yaml --debug
[2024-06-18 10:23:45] START workflow=analyze-customer-feedback
[2024-06-18 10:23:45] STEP id=sentiment-analysis model=gpt-4 tokens=1250
[2024-06-18 10:23:47] TOOL call=web-search query="customer satisfaction benchmarks"
[2024-06-18 10:23:48] COMPLETE duration=3.2s total_tokens=2150 cost=$0.086
```

## 💪 Real-World Example

Build a complete content pipeline that researches, writes, and publishes:

```yaml
# content-pipeline.laq.yaml
name: content-creation-pipeline
agents:
  researcher:
    model: gpt-4
    tools: [lacquer/web-search@v1, lacquer/pdf-reader@v1]
    
  writer:
    model: claude-3-opus
    temperature: 0.7
    
workflow:
  inputs:
    topic: string
    audience: string
    
  steps:
    # Research phase
    - id: research
      agent: researcher
      prompt: |
        Research "{{ inputs.topic }}" for {{ inputs.audience }} audience.
        Find recent sources, statistics, and expert opinions.
      retry:
        max_attempts: 3
        
    # Create content
    - id: outline
      uses: lacquer/content-outliner@v1
      with:
        research: "{{ steps.research.output }}"
        audience: "{{ inputs.audience }}"
        
    - id: write
      agent: writer
      prompt: |
        Write a comprehensive article based on:
        Outline: {{ steps.outline.output }}
        Research: {{ steps.research.output }}
        Target audience: {{ inputs.audience }}
        
    # Optimize and publish
    - id: optimize_seo
      uses: lacquer/seo-optimizer@v1
      with:
        content: "{{ steps.write.output }}"
        
    - id: create_social
      parallel:
        steps:
          - uses: lacquer/social-media@v1
            with:
              content: "{{ steps.write.output }}"
              platform: twitter
              
          - uses: lacquer/social-media@v1
            with:
              content: "{{ steps.write.output }}"
              platform: linkedin
              
    - id: publish
      uses: lacquer/wordpress@v1
      with:
        title: "{{ steps.outline.output.title }}"
        content: "{{ steps.optimize_seo.output }}"
        status: draft
        
  outputs:
    article_url: "{{ steps.publish.output.url }}"
    social_posts: "{{ steps.create_social.output }}"
```

## 🏗️ Building Custom Blocks

Extend Lacquer with your own reusable components:

```yaml
# blocks/code-reviewer/block.laq.yaml
name: code-reviewer
version: 1.0.0
runtime: go

inputs:
  file_path: string
  language: string

script: |
  package main
  
  import (
    "github.com/lacquer/laq/sdk"
  )
  
  func main() {
    code := sdk.ReadFile(inputs.GetString("file_path"))
    language := inputs.GetString("language")
    
    // Custom review logic
    issues := analyzeCode(code, language)
    
    outputs.Set("issues", issues)
    outputs.Set("score", calculateScore(issues))
  }
```

## 🚀 CLI Commands

```bash
laq init            # Initialize new project
laq validate        # Validate workflow syntax
laq run            # Execute workflow
laq test           # Run workflow tests
laq serve          # Run as HTTP API server
laq list           # List available agents/tools
```

## 🤝 Community & Ecosystem

### Official Blocks
- `lacquer/web-search@v1` - Web search integration
- `lacquer/summarizer@v1` - Text summarization
- `lacquer/code-reviewer@v1` - Code analysis
- `lacquer/postgresql@v1` - Database operations
- `lacquer/github@v1` - GitHub integration
- `lacquer/slack@v1` - Slack notifications

### Resources
- 📚 [Documentation](https://docs.lacquer.ai)
- 💬 [Discord Community](https://discord.gg/lacquer)
- 🎯 [Examples](examples/)
- 📝 [Blog](https://lacquer.ai/blog)
- 🐛 [Issue Tracker](https://github.com/lacquer/laq/issues)

## 🛠️ Development

```bash
# Clone the repo
git clone https://github.com/lacquer/laq
cd laq

# Build
make build

# Run tests
make test

# Install locally
make install
```

## 📈 Benchmarks

| Operation | Time | Memory |
|-----------|------|---------|
| Startup | <50ms | <5MB |
| Validate typical workflow | <100ms | <10MB |
| Execute 10-step workflow | <500ms | <20MB |
| Parallel execution (10 agents) | <1s | <50MB |

## 🤔 Why Go?

- **Performance**: Native concurrency with goroutines perfect for parallel agent execution
- **Distribution**: Single static binary, no runtime dependencies
- **Reliability**: Strong typing prevents runtime errors
- **Ecosystem**: Same language for core engine and custom blocks

## 📄 License

Lacquer is Apache 2.0 licensed. See [LICENSE](LICENSE) for details.

## 🌟 Star History

[![Star History Chart](https://api.star-history.com/svg?repos=lacquer/laq&type=Date)](https://star-history.com/#lacquer/laq&Date)

---

<div align="center">

**Ready to make your AI workflows shine?**

[Get Started](https://docs.lacquer.ai/quickstart) • [Join Discord](https://discord.gg/lacquer) • [Star on GitHub](https://github.com/lacquer/laq)

</div>
