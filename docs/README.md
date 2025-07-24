# Lacquer

Welcome to the documentation for Lacquer's workflow DSL (Domain Specific Language). Lacquer provides a declarative, YAML-based syntax for orchestrating AI agent workflows, similar to how GitHub Actions works for CI/CD workflows.

Lacquer was built out of a desire to have a workflow language that provides:

* <span class="font-highlight">Code not Clicks</span> - Write AI workflows in YAML, not drag-and-drop. Version control, code review and debug like real code.
* <span class="font-highlight">Zero Dependencies</span> - No Python environments, no package conflicts, just a lightweight Go binary.
* <span class="font-highlight">Develop local, deploy global</span> - Prototype quickly on your laptop then ship to any cloud. No surprises between dev and production environments.
* <span class="font-highlight">Compose once, reuse everywhere</span> - Build modular workflows that snap together like LEGO blocks. Share common patterns across projects and teams without copy-paste hell.  
* <span class="font-highlight">Declarative > Imperative</span> - Describe what you want, not how to get it. Let Lacquer handle the orchestration complexity while you focus on business logic.
* <span class="font-highlight">Agents, not just prompts</span> - Configure specialized AI agents with custom tools and behaviors. Build sophisticated multi-agent systems without the boilerplate.

## Quick Start {docsify-ignore}

**Install `laq`**

```bash
curl -LsSf https://get.lacquer.ai | sh
```

**Initialize a Lacquer Workflow**

Run `laq init` to run through the interactive setup process for your first workflow.

```bash
laq init
```

...

<pre v-pre="" data-lang="bash"><code class="lang-bash"><span class="token highlight">Project Summary</span>

Please review your selections:

<span class="token string">Project Name:</span> greeter
<span class="token string">Description:</span> A simple hello world workflow that should greet a given person
<span class="token string">Model Providers:</span> anthropic
<span class="token string">Script Language:</span> go</code></pre>

Press `Enter` to generate your project, lacquer will create a new directory with a `workflow.laq.yaml` file, here's an exammple of what it might look like:

```yaml
version: "1.0"
metadata:
  name: hello-world
  description: A simple greeting workflow demonstrating basic Lacquer syntax

# Define a single agent using GPT-4
agents:
  greeter:
    provider: openai
    model: gpt-4
    temperature: 0.7
    system_prompt: You are a friendly assistant who gives warm greetings.

# Define input parameters for the workflow
inputs:
  name:
    type: string
    description: Name of the person to greet
    default: "World"

# Define a simple single step workflow
workflow:
  steps:
    - id: say_hello
      agent: greeter
      prompt: |
        Say hello to ${{ inputs.name }} in a creative and friendly way.
        Make it warm and welcoming!
  
  # Return the greeting as output
  outputs:
    greeting: ${{ steps.say_hello.output }}
```

**Run the Workflow**

Now run your workflow passing a name to greet `laq run --input name=lackey`

<pre v-pre="" data-lang="bash"><code class="lang-bash">Running <span class="token string">hello-world</span> workflow <span class="token punctuation">(</span><span class="token number">1</span> steps<span class="token punctuation">)</span>

<span class="token string">✓</span>  Running step say_hello <span class="token punctuation">(</span><span class="token number">1/1</span><span class="token punctuation">)</span>
   <span class="token string">✓</span> Say hello to lackey <span class="token keyword">in</span> a creative and friendly way.

<span class="token string">✓</span> Workflow completed successfully <span class="token punctuation">(</span><span class="token number">3.56s</span><span class="token punctuation">)</span>


<span class="token highlight small">Outputs</span>

<span class="token string">greeting:</span> Hey there, lackey<span class="token operator">!</span> *waves enthusiastically* 🌟 What an absolute joy to see your wonderful self here<span class="token operator">!</span></code></pre>

## Learn More {docsify-ignore}

* Dive into [writing your first workflow](guides/writing-your-first-workflow.md) which explores the core concepts of Lacquer.
* Review the main features of Lacquer in the [features](start/features.md) guide.
* Understand the DSL [concepts](concepts) that underpin Lacquer.