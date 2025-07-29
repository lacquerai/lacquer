<div align="center">
<img width="1240" height="480" alt="lacquer-banner-stars" src="https://github.com/user-attachments/assets/42844a33-c8cb-404b-ba56-54b803615e03" />


[Quick Start](#-quick-start) • [Examples](#-examples) • [Documentation](https://lacquer.ai/docs)
</div>

---

## Orchestrate AI Agents with Code, Not Clicks

Lacquer is a blazing-fast, code-first orchestration engine for AI agent workflows. Bring **GitHub Actions-style workflows** to AI and define complex multi-agent systems in simple YAML.

Built for artisans who prefer **terminals** over drag-and-drop.

```yaml
version: "1.0"

agents:
  assistant:
    provider: openai
    model: gpt-4
    temperature: 0.7

inputs:
  topic:
    type: string
    description: Topic to explore
  
workflow:
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
$ laq run workflow.laq.yml --input topic="quantum computing"

Running research_workflow (2 steps)

✓ Step research completed (2.1s)
✓ Step summarize completed (1.3s)

✓ Workflow completed successfully

Outputs:

summary: 
  • Quantum computing uses quantum-mechanical phenomena like superposition and entanglement for calculations
  • It represents a revolutionary approach to processing information beyond classical computing limitations
  • Current applications include cryptography, optimization, and simulation of quantum systems
```

## Quick Start

Install `laq` with a single command:

```bash
curl -sSL https://lacquer.ai/install.sh | sh
```

Use the `laq init` wizard to create a new workflow using AI.

```bash
laq init
```

Execute it and see the magic.

```bash
laq run workflow.laq.yml
```

Now explore the [documentation](https://lacquer.ai/docs) to find out how to improve your workflow.


## Examples

### 🔧 Agent Tools

Extend your agents with custom tools for web search, file operations, and more:

```yaml
agents:
  researcher:
    provider: openai
    model: gpt-4
    temperature: 0.2
    system_prompt: You are a helpful researcher that answers questions about a given topic.
    tools:
      - name: search_web
        script: "node ./scripts/web_search.js"
        description: |
          the search_web tool provides a easy way to search the web for information.
          Use this tool to get the latest information on a given topic, it will provide
          a short summary of the latest information on the given topic.
        parameters:
          type: object
          properties:
            query:
              type: string
              description: |
                The given topic that you want to search for.
```

### 📊 State Management

Maintain workflow state across steps with automatic updates:

```yaml
state:
  counter: 0
  status: "pending"

workflow:
  steps:
    - id: process_item
      agent: processor
      prompt: "Process: ${{ inputs.item }}"
      updates:
        counter: "${{ state.counter + 1 }}"
        status: "${{ steps.process_item.outputs.success ? 'processing' : 'error' }}"
```

### 🔀 Conditional Logic

Build complex workflows with conditionals and loops:

```yaml
steps:
  - id: check_length
    agent: analyzer
    prompt: "Count words in: ${{ inputs.text }}"
    outputs:
      word_count: 
        type: integer

  # Conditionally execute steps
  - id: expand_text
    condition: ${{ steps.check_length.outputs.word_count < 100 }}
    agent: writer
    prompt: "Expand this text to at least 100 words: ${{ inputs.text }}"

  # Iterative refinement with while loops
  - id: refine_text
    while: ${{ steps.refine_text.iteration < 3 && steps.refine_text.outputs.quality_score < 8 }}
    steps:
      - id: analyze_quality
        agent: analyzer
        prompt: |
          Analyze the quality of this text on a scale of 1-10:
          ${{ steps.expand_text.outputs.expanded_text || inputs.text }}
        outputs:
          quality_score: integer
          improvement_suggestions: string

      - id: apply_improvements
        condition: ${{ steps.refine_text.steps.analyze_quality.outputs.quality_score < 8 }}
        agent: editor
        prompt: |
          Improve this text based on these suggestions:
          Text: ${{ steps.expand_text.outputs.expanded_text || inputs.text }}
          Suggestions: ${{ steps.refine_text.steps.analyze_quality.outputs.improvement_suggestions }}
```

### 🛠️ Custom Scripts & Containers

Execute custom code alongside AI agents:

```yaml
requirements:
  runtimes:
    - name: python
      version: "3.9"

workflow:
  steps:
    - id: create_fix
      agent: fixer
      prompt: |
        We've encountered the following error in production
        ${{ inputs.error }}

        Please create a fix for the error in the following code:
        ${{ inputs.code }}

    # Execute custom Python scripts
    - id: validate_fix
      run: "python3 scripts/validate.py"
      with:
        patch: ${{ steps.create_fix.outputs.patch }}
        code: ${{ inputs.code }}

    # Or run in isolated containers
    - id: validate_fix_container
      container: ./validate/Dockerfile
      command:
        - scripts/validate.py
        - ${{ steps.create_fix.outputs.patch }}
        - ${{ inputs.code }}
```

### 🔌 MCP Integration

Connect to Model Context Protocol servers for enhanced capabilities:

```yaml
agents:
  fixer:
    provider: anthropic
    model: claude-4-sonnet-20240229
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

## 🤝 Contributing

We welcome contributions! Lacquer is in early alpha, and we're actively seeking feedback and help with:

- Bug fixes and performance optimizations
- Additional provider integrations
- DSL improvements
- Documentation improvements  
- Example workflows

## 📄 License

Lacquer is Apache 2.0 licensed. See [LICENSE](LICENSE) for details.

## 🚦 Project Status

Lacquer is in **early alpha** (v0.1.0). The core engine is functional and being actively developed. Expect breaking changes as we iterate based on community feedback.

---

<div align="center">

**Where AI workflows get their shine** ✨

[Site](https://lacquer.ai) • [Documentation](https://lacquer.ai/docs)
</div>
