<div align="center">
<img width="1240" height="480" alt="lacquer-banner-stars" src="https://github.com/user-attachments/assets/42844a33-c8cb-404b-ba56-54b803615e03" />

[Get Started](#-get-started-in-60-seconds) • [Why Lacquer?](#-why-lacquer) • [Examples](https://github.com/lacquerai/examples) • [Documentation](https://lacquer.ai/docs)

</div>

---

## What is Lacquer?

**Lacquer** (`laq`) is a lightweight AI workflow engine that lets you orchestrate complex AI agent interactions using simple YAML files—just like GitHub Actions, but for AI. 

**No Python environments. No dependency hell. Just a single Go binary that runs anywhere.**

```yaml
version: "1.0"

inputs:
  alert_id:
    type: string
    required: true

agents:
  investigator:
    provider: anthropic
    model: claude-4-sonnet-20240229
    temperature: 0.1
    system_prompt: You are an SRE expert who investigates production issues.
    tools:
      - name: query_logs
        script: "node scripts/log_search.js"
        parameters:
          query:
            type: string
            description: "CloudWatch Insights query to search and filter application logs"

  fixer:
    provider: local  
    model: claude-code
    system_prompt: You are a DevOps engineer who writes production-ready fixes and runbooks.

workflow:
  steps:
    - id: get_alert
      run: "pd incident get ${{ inputs.alert_id }}"
      
    - id: investigate
      agent: investigator
      prompt: |
        Investigate this alert: ${{ steps.get_alert.output }}

        Find the root cause using log search.
      outputs:
        root_cause:
          type: string
          description: "The root cause of the alert"
        
    - id: generate_fix
      agent: fixer
      prompt: |
        Create a fix for: ${{ steps.investigate.outputs.root_cause }}

        Generate both code changes and a runbook.
      outputs:
        commit_message:
          type: string
          description: "The commit message for the fix that you have generated"
        
    - id: create_pr
      run: |
        node scripts/create_pr.js \
          --alert_id ${{ inputs.alert_id }} \
          --commit_message ${{ steps.generate_fix.outputs.commit_message }}
          
  outputs:
    pr_url: ${{ steps.create_pr.output }}
```

## The Problem

**Tired of "no-code" AI tools that make everything harder?**

- 🖱️ **Drag-and-drop interfaces** that slow you down
- 🔒 **No version control** for your AI workflows  
- 🌐 **Vendor lock-in** with proprietary platforms
- 💸 **Expensive cloud platforms** for simple workflows
- 🐛 **Black box systems** you can't debug

## ✨ Enter Lacquer

```bash
# Install in seconds
curl -sSL https://lacquer.ai/install.sh | sh

# Create your first workflow with AI assistance
laq init

# Run locally, test, iterate  
laq run workflow.laq.yml --input code_changes="$(git diff HEAD~1)" --input project_type=web

# Ship anywhere - it's just a binary + YAML
scp workflow.laq.yml prod-server:/usr/local/bin/
```

## Why Lacquer?

### **1. GitOps Native**
Your workflows are just YAML files. Commit them, review them, version them like any other code.

### **2. Local-First Development**
Test everything on your laptop before deploying. No cloud account needed.

### **3. Zero Dependencies**
Single static Go binary. No Python, no Node, no Docker required. Download and run.

### **4. Composable & Reusable**
```yaml
steps:
  - uses: ./workflows/analyze-sentiment.laq.yml
  - uses: github.com/lacquerai/workflows/summarize@v1
```

### **5. Any Language, Any Tool**
```yaml
steps:
  - run: python scripts/analyze.py
  - run: node tools/fetch-data.js  
  - run: ./bin/custom-processor
  - container: postgres:15
```

### **6. Production Ready**
Built-in HTTP server, health checks, metrics, and observability. Deploy to Kubernetes, serverless, or bare metal.

## 🚀 Get Started in 60 Seconds

### 1. Install
```bash
# macOS/Linux
curl -sSL https://lacquer.ai/install.sh | sh

# or via Go
go install github.com/lacquerai/lacquer/cmd/laq@latest
```

### 2. Create Your First Workflow
```bash
laq init
? Project name: my-assistant
? Description: An AI documentation assistant who writes documentation for a given file
? Model provider: anthropic
✓ Created workflow.laq.yml
```

### 3. Run It
```bash
laq run workflow.laq.yml --input file_path="rocket.go"
```

## Examples of what you can do with Lacquer

- 📚 Documentation
- 🐛 Bug fixes
- 🔄 CI/CD
- 📦 Package management
- 🔄 CI/CD

## 📚 Learn More

Please check our extensive [documentation](https://lacquer.ai/docs) for more details.

## 🤝 Community & Contributing

Lacquer is built by developers, for developers. We'd love your help making it better!

- 🐛 [Report bugs](https://github.com/lacquerai/lacquer/issues)
- 💡 [Request features](https://github.com/lacquerai/lacquer/discussions)
- 📖 [Improve docs](https://github.com/lacquerai/lacquer/tree/main/docs)
- ⭐ [Star us on GitHub](https://github.com/lacquerai/lacquer)

## 🚦 Project Status

> Lacquer is in early alpha but already powers production workflows. We're iterating quickly based on community feedback. Expect some breaking changes before v1.0.

## 📄 License

Apache 2.0 - Use it anywhere, modify it freely, ship it commercially.

---

<div align="center">

### Ready to ditch the drag-and-drop?

<b>

[⚡ Get Started](https://lacquer.ai) • [📖 Read the Docs](https://lacquer.ai/docs) • [⭐ Star on GitHub](https://github.com/lacquerai/lacquer)

</b>

<sub>Built with ❤️ by developers who prefer terminals over GUIs</sub>

</div>