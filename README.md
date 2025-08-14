<div align="center">
<img width="1240" height="480" alt="lacquer-banner-stars" src="https://github.com/user-attachments/assets/42844a33-c8cb-404b-ba56-54b803615e03" />


[Quick Start](#-quick-start) • [Features](#-features) • [Documentation](https://lacquer.ai/docs)
</div>

---


Lacquer is an open-source **AI orchestration engine** inspired by GitHub Actions. Define complex agent workflows in familiar YAML, test locally, and ship anywhere with a single binary. Built for developers who prefer **terminals** over drag-and-drop.

```yaml
version: "1.0"

agents:
  code_reviewer:
    provider: openai
    model: gpt-4
    temperature: 0.3
    system_prompt: You are an expert code reviewer who analyses pull requests.

inputs:
  pr_number:
    type: integer
    description: Pull request number to review
    required: true

workflow:
  steps:
    - id: fetch_pr
      run: node scripts/fetch_pr.js
      with:
        pr_number: ${{ inputs.pr_number }}

    - id: analyze_changes
      agent: code_reviewer
      prompt: |
        Please analyze this pull request and help me review it:
        
        ${{ steps.fetch_pr.outputs.diff }}
        
        Please provide:
        1. **Summary**: What does this PR do in simple terms?
        2. **Key Changes**: What are the main files/functions modified?
        3. **Potential Concerns**: Any issues or risks to be aware of?
        
        Keep explanations clear and accessible.

  outputs:
    pr_analysis: "${{ steps.analyze_changes.output }}"

```

```bash
$ laq run workflow.laq.yml pr.laq.yml --input pr_number=3442

Running pull_request_analysis (2 steps)

✓ Step fetch_pr completed (2.1s)
✓ Step analyze_changes completed (1.3s)

✓ Workflow completed successfully

Outputs:

summary: 
  • This PR adds the new laq function to the ...
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
