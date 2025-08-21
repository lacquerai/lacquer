<p align="center">
<img width="1240" height="480" alt="lacquer-banner-stars" src="https://github.com/user-attachments/assets/42844a33-c8cb-404b-ba56-54b803615e03" />
 <img alt="GitHub Release" src="https://img.shields.io/github/v/release/lacquerai/lacquer">
<img alt="GitHub License" src="https://img.shields.io/github/license/lacquerai/lacquer">
<a href="https://lacquer.ai/docs">
<img alt="Static Badge" src="https://img.shields.io/badge/docs-latest-blue">
</a>
</p>
<p align="center">
Lacquer is a lightweight AI workflow engine that codifies repeatable engineering processes into reliable YAML workflows that never skip a step. Think GitHub Actions, but for AI-powered internal tools.
</p>

### See it in action


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
      You are a Kubernetes SRE expert. Analyze logs for: root causes, error patterns, 
      service impact, and specific remediation steps.

workflow:
  steps:
    - id: get_logs
      run: "kubectl logs '${{ inputs.pod_name }}' --tail=50 | grep -E 'ERROR|WARN|Exception'"

    - id: analyze_logs
      agent: assistant
      prompt: |
        Analyze these recent error logs and identify root causes and recommended fixes:
        ${{ steps.get_logs.output }}

  outputs:
    issues: ${{ steps.analyze_logs.output }}
```

```bash
laq run debug-pod.laq.yml --input pod_name=api-server-7d9c5
```

## The Problem

Building AI automation for internal tasks seems like a drag when current solutions are built for the no-code crowd:

- **Drag-and-drop UIs** when you live in the terminal
- **No version control** for auditing changes or rollbacks  
- **Vendor lock-in** making internal approval a nightmare
- **Black box systems** you can't debug, extend or embed

### Why Lacquer?

🔄 **GitOps Native** - Your workflows are just YAML files. Commit them, review them, version them like any other code.

💻 **Local-First Development** - Test everything on your laptop before deploying. No cloud account needed.

🏠 **Familiar DSL** - If you've used GitHub Actions, you'll feel right at home.

⚡ **Zero Dependencies** - Single static Go binary. No Python, no Node, no Docker required. Download and run.

🚀 **Production Ready** - Built-in HTTP server, health checks, metrics, and observability. Deploy to Kubernetes, serverless, or just a regular VM with ease.

## Features

Lacquer scales as you grow with all the features you need to build production workflows:

<details>
<summary>
🔌 <b>MCP support</b> - Use local or remote MCP servers to extend your agents with common integrations.
</summary>

```yaml
agents:
  incident_responder:
    provider: anthropic
    model: claude-sonnet-4
    system_prompt: |
      You are an SRE expert who:

      - Analyzes production incidents
      - Identifies root causes from logs and metrics
      - Creates runbooks for remediation
      - Documents post-mortems
    tools:
      - name: filesystem
        description: Access runbooks and configuration files
        mcp_server:
          type: local
          command: npx
          args:
            - "-y"
            - "@modelcontextprotocol/server-filesystem"
            - "/etc/kubernetes/manifests"
```
</details>
<details>
<summary>
🛠️ <b>Local tools</b> - Extend your agents automation abilities by building your own custom tools in any language.
</summary>

```yaml
agents:
  ops_assistant:
    provider: openai
    model: gpt-4
    temperature: 0.2
    system_prompt: You investigate production issues and query infrastructure state.
    tools:
      - name: query_metrics
        script: "python ./tools/prometheus_query.py"
        description: "Query Prometheus for system metrics"
        parameters:
          type: object
          properties:
            query:
              type: string
              description: "PromQL query to execute"
            timerange:
              type: string
              description: "Time range (e.g., '5m', '1h', '24h')"
```
</details>

<details>
<summary>
📦 <b>Script and container support</b> - Run steps with any language or container.
</summary>

```yaml
steps:
  - id: backup_database
    run: "python ./scripts/pg_backup.py --database ${{ inputs.db_name }}"
    with:
      retention_days: 30

  - id: run_migration
    container: migrate/migrate:latest
    command:
      - "migrate"
      - "-path=/migrations"
      - "-database=${{ secrets.DATABASE_URL }}"
      - "up"
```
</details>

<details>
<summary>
🔀 <b>Complex control flow</b> - Run steps conditionally based on the output of previous steps or break out steps into sub steps which run until a condition is met.
</summary>

```yaml
steps:
  - id: check_health
    agent: monitor
    prompt: "Check health status of service: ${{ inputs.service_name }}"
    outputs:
      healthy: 
        type: boolean
        description: "Whether the service is healthy"
      error_rate:
        type: float
        description: "The error rate of the service"

  # Conditionally execute steps
  - id: scale_up
    condition: ${{ steps.check_health.outputs.error_rate > 0.05 }}
    run: "kubectl scale deployment ${{ inputs.service_name }} --replicas=5"

  # Break out steps into sub steps and run until a condition is met
  - id: rolling_restart
    while: ${{ steps.rolling_restart.iteration < 3 && !steps.rolling_restart.outputs.healthy }}
    steps:
      - id: restart_pod
        run: |
          kubectl rollout restart deployment/${{ inputs.service_name }}
          kubectl rollout status deployment/${{ inputs.service_name }} --timeout=300s

      - id: verify_health
        agent: monitor
        prompt: |
          Verify service health after restart:
          - Check HTTP endpoints return 200
          - Verify error rate < 1%
          - Confirm all pods are ready

          Service: ${{ inputs.service_name }}
        outputs:
          healthy: 
            type: boolean
            description: "Whether the service is healthy"
          metrics: 
            type: object
            description: "The metrics of the service"
```

</details>

<details>
<summary>
📊 <b>Built in state management</b> - Lacquer keeps track of the state of your workflow and can be used to build complex workflows.
</summary>

```yaml
state:
  rollback_count: 0
  deployment_status: "pending"

workflow:
  steps:
    - id: deploy_service
      run: "helm upgrade --install ${{ inputs.service }} ./charts/${{ inputs.service }}"
      updates:
        deployment_status: "${{ steps.deploy_service.output ? 'deployed' : 'failed' }}"
        
    - id: rollback_if_needed
      condition: ${{ state.deployment_status == 'failed' }}
      run: "helm rollback ${{ inputs.service }}"
      updates:
        rollback_count: "${{ state.rollback_count + 1 }}"
```

</details>

<details>
<summary>
🧩 <b>Composable steps</b> - Build reusable workflow components that enforce consistent operational procedures across teams and environments.
</summary>

```yaml
steps:
  - id: security_scan
    uses: ./workflows/security/container-scan.laq.yml
    with:
      image: ${{ inputs.docker_image }}
      
  - id: deploy_to_k8s
    uses: github.com/lacquerai/workflows/k8s-deploy@v1
    with:
      manifest: ${{ steps.generate_manifest.outputs.yaml }}
      namespace: production
```

</details>

<details>
<summary>
🤖 <b>Multi-agent support</b> - Define multiple agents with different models, prompts, and tools to perform different tasks. Support out the box for OpenAI, Anthropic, and Claude Code models.
</summary>

```yaml
agents:
  architect:
    provider: local
    model: claude-code
    system_prompt: |
      You are a cloud architect who designs scalable infrastructure solutions
      and creates Terraform configurations for AWS deployments.
      
  security_auditor:
    provider: anthropic
    model: claude-sonnet-4
    system_prompt: |
      You are a security engineer who audits infrastructure for vulnerabilities,
      reviews IAM policies, and ensures compliance with security best practices.
```

</details>

<details>
<summary>
📤 <b>Output marshalling</b> - Constrain your agent steps to only return the data you need and then use it in later steps.
</summary>

```yaml
workflow:
  steps:
    - id: analyze_incident
      agent: sre_expert
      prompt: |
        Analyze this PagerDuty alert and provide structured incident data:
        
        ${{ inputs.alert_payload }}
      outputs:
        severity:
          type: string
          enum: ["low", "medium", "high", "critical"]
          description: "The severity of the incident"
        affected_services:
          type: array
          items:
            type: string
          description: "The affected services"
        remediation_steps:
          type: array
          items:
            type: string
          description: "The remediation steps"
        requires_escalation:
          type: boolean
          description: "Whether the incident requires escalation"

  outputs:
    incident_report:
      severity: ${{ steps.analyze_incident.outputs.severity }}
      services: ${{ steps.analyze_incident.outputs.affected_services }}
      next_steps: ${{ steps.analyze_incident.outputs.remediation_steps }}
```

</details>

<details>
<summary>
🌐 <b>HTTP server</b> - Once you're done prototyping your workflow, ship it to production and expose it to your team using a simple REST API.
</summary>

```bash
laq serve incident-response.laq.yml            # Serve single workflow
laq serve pr-review.laq.yml deploy.laq.yml    # Serve multiple workflows  
laq serve --workflow-dir ./ops/workflows      # Serve all workflows in directory
laq serve --port 8080 --host 0.0.0.0         # Custom host and port
```
</details>



## Get Started in 60 Seconds

**1. Install**
```bash
curl -sSL https://lacquer.ai/install.sh | sh
```

**2. Get AI to scaffold your first workflow**
```bash
laq init
? Project name: debug-pod
? Description: Analyze kubernetes pod logs and suggest fixes
? Model provider: anthropic
✓ Created workflow.laq.yml
```

**3. Run It**
```bash
laq run workflow.laq.yml --input pod_name=api-server-7d9c5
```

## Learn More

Please check our extensive [documentation](https://lacquer.ai/docs) for more details.

## Community & Contributing

Lacquer is built by developers, for developers. We'd love your help making it better!

- [Report bugs](https://github.com/lacquerai/lacquer/issues)
- [Request features](https://github.com/lacquerai/lacquer/discussions)
- [Improve docs](https://github.com/lacquerai/lacquer/tree/main/docs)
- [Star us on GitHub](https://github.com/lacquerai/lacquer)

## Project Status

> Lacquer is in early alpha but already powers production workflows. We're iterating quickly based on community feedback. Expect some breaking changes before v1.0.

## License

Apache 2.0 - Use it anywhere, modify it freely, ship it commercially.

---

<div align="center">

<sub>Built with ❤️ by developers who prefer terminals over GUIs</sub>

</div>