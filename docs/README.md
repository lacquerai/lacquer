# Lacquer

Welcome to the documentation for Lacquer - a lightweight AI workflow engine built for platform engineering teams. Lacquer lets you codify repeatable engineering processes into reliable YAML workflows that never skip a step. Think GitHub Actions, but for AI-powered internal tools.

Lacquer was built by engineers who got tired of:

* <span class="font-highlight">Drag-and-drop UIs</span> - When you live in the terminal and need version control, not click-ops
* <span class="font-highlight">Black box systems</span> - You can't debug, extend or embed closed platforms into your infrastructure  
* <span class="font-highlight">Vendor lock-in</span> - Making internal approval a nightmare and migration impossible
* <span class="font-highlight">Python dependency hell</span> - Just a single Go binary that works everywhere, no virtual environments needed
* <span class="font-highlight">Cloud-only solutions</span> - Test everything locally before deploying, no surprises in production
* <span class="font-highlight">Copy-paste runbooks</span> - Build modular, reusable workflows that enforce consistent operational procedures

## Quick Start {docsify-ignore}

**Install `laq`**

```bash
curl -LsSf https://lacquer.ai/install.sh | sh
```

**Initialize a Lacquer Workflow**

Run `laq init` to scaffold your first operational workflow.

```bash
laq init
```

...

<pre v-pre="" data-lang="bash"><code class="lang-bash"><span class="token highlight">Project Summary</span>

Please review your selections:

<span class="token string">Project Name:</span> debug-pod
<span class="token string">Description:</span> Analyze kubernetes pod logs and suggest fixes
<span class="token string">Model Providers:</span> anthropic
</pre>

Press `Enter` to generate your project. Lacquer will create a new directory with a `workflow.laq.yaml` file. Here's an example for debugging Kubernetes pods:

```yaml
version: "1.0"

inputs:
  pod_name:
    type: string
    required: true

agents:
  assistant:
    provider: anthropic
    model: claude-sonnet-4
    system_prompt: |
      You are a Kubernetes SRE expert. Analyze logs for: 
      root causes, error patterns, service impact, 
      and specific remediation steps.

workflow:
  steps:
    - id: get_logs
      run: "kubectl logs '${{ inputs.pod_name }}' --tail=50 | grep -E 'ERROR|WARN|Exception'"

    - id: analyze_logs
      agent: assistant
      prompt: |
        Analyze these recent error logs and identify root 
        causes and recommended fixes:
        
        ${{ steps.get_logs.output }}
      outputs:
        root_cause:
          type: string
          description: The identified root cause
        remediation_steps:
          type: array
          items:
            type: string
          description: List of remediation actions

  outputs:
    root_cause: ${{ steps.analyze_issue.outputs.root_cause }}
    actions: ${{ steps.analyze_issue.outputs.remediation_steps }}
```

**Run the Workflow**

Now run your workflow to debug a problematic pod:

<pre v-pre="" data-lang="bash"><code class="lang-bash">laq run workflow.laq.yaml --input pod_name=api-server-7d9c5 --input namespace=production

Running <span class="token string">debug-pod</span> workflow <span class="token punctuation">(</span><span class="token number">3</span> steps<span class="token punctuation">)</span>

<span class="token string">✓</span>  Running step get_pod_logs <span class="token punctuation">(</span><span class="token number">1/3</span><span class="token punctuation">)</span>
<span class="token string">✓</span>  Running step get_pod_status <span class="token punctuation">(</span><span class="token number">2/3</span><span class="token punctuation">)</span>
<span class="token string">✓</span>  Running step analyze_issue <span class="token punctuation">(</span><span class="token number">3/3</span><span class="token punctuation">)</span>

<span class="token string">✓</span> Workflow completed successfully <span class="token punctuation">(</span><span class="token number">5.23s</span><span class="token punctuation">)</span>


<span class="token highlight small">Outputs</span>

<span class="token string">root_cause:</span> OOMKilled - Container memory limit of 512Mi exceeded due to memory leak in connection pooling

<span class="token string">actions:</span> 
  <span class="token operator">-</span> kubectl set resources deployment/api-server -c=api --limits=memory=1Gi -n production
  <span class="token operator">-</span> kubectl rollout restart deployment/api-server -n production
  <span class="token operator">-</span> Review application code for connection pool cleanup in database client</code></pre>

## Learn More {docsify-ignore}

* Build your first operational workflow in the [writing your first workflow](start/writing-your-first-workflow.md) guide
* Explore platform engineering features in the [features](start/features.md) guide  
* Master the DSL [concepts](concepts) for building production-grade workflows