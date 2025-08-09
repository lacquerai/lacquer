<div align="center">
<img width="1240" height="480" alt="lacquer-banner-stars" src="https://github.com/user-attachments/assets/42844a33-c8cb-404b-ba56-54b803615e03" />


[Quick Start](#-quick-start) • [Features](#-features) • [Documentation](https://lacquer.ai/docs)
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
  • Quantum computing ...
```

## 🚀 Quick Start

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


## Features
Lacquer comes with all the tools you need to build production-ready AI workflows

* **Script & Container Support** - Extend your workflows with custom scripts and containers
* **Agent Tools** - Extend your agents easily by defining local tools that the model can use
* **State Management** - Maintain workflow state across steps with automatic updates
* **Conditional Logic** - Build complex workflows with conditionals and loops
* **MCP Integration** - Connect your agents to Model Context Protocol servers for enhanced capabilities
* **Built in HTTP Server** - Lacquer comes with a built in HTTP server that you can use to expose your workflows as APIs
* **Seamless Integration with Claude Code** - Bullet proof your local development workflows with Claude Code and lacquer, no need to wrestle with your CLAUDE.md file to get claude to do what you want

Check out the [documentation](https://lacquer.ai/docs) for more details on each feature and how to get building your first workflow.

## 🤝 Contributing

We welcome contributions! Lacquer is in early alpha, and we're actively seeking feedback and help with:

- Bug fixes and performance optimizations
- Additional provider integrations
- DSL improvements

## 📄 License

Lacquer is Apache 2.0 licensed. See [LICENSE](LICENSE) for details.

## 🚦 Project Status

Lacquer is in **early alpha**, the core engine is functional but still being actively developed. Expect breaking changes as we iterate based on community feedback.

---

<div align="center">

**Where AI workflows get their shine** ✨

[Site](https://lacquer.ai) • [Documentation](https://lacquer.ai/docs)
</div>
