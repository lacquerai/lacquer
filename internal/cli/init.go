package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new Lacquer project",
	Long: `Initialize a new Lacquer project with a basic workflow structure and configuration.

This command creates:
- Project directory structure
- Example workflow file
- Configuration file (.lacquer/config.yaml)
- README with getting started instructions

Templates available:
- basic: Simple hello-world workflow
- research: Multi-step research workflow with web search
- enterprise: Advanced workflow with error handling and retries

Examples:
  laq init my-project                    # Create project with basic template
  laq init --template research my-bot    # Create with research template
  laq init --no-git my-project           # Skip git initialization`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := "lacquer-project"
		if len(args) > 0 {
			projectName = args[0]
		}
		initializeProject(projectName)
	},
}

var (
	templateName string
	noGit        bool
	force        bool
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&templateName, "template", "t", "basic", "project template (basic, research, enterprise)")
	initCmd.Flags().BoolVar(&noGit, "no-git", false, "skip git repository initialization")
	initCmd.Flags().BoolVar(&force, "force", false, "overwrite existing project directory")
}

// ProjectTemplate represents a project template
type ProjectTemplate struct {
	Name        string
	Description string
	Files       map[string]string
}

var templates = map[string]ProjectTemplate{
	"basic": {
		Name:        "Basic",
		Description: "Simple hello-world workflow",
		Files: map[string]string{
			"workflow.laq.yaml": basicWorkflow,
			"README.md":         basicReadme,
		},
	},
	"research": {
		Name:        "Research",
		Description: "Multi-step research workflow with web search",
		Files: map[string]string{
			"workflow.laq.yaml": researchWorkflow,
			"README.md":         researchReadme,
		},
	},
	"enterprise": {
		Name:        "Enterprise",
		Description: "Advanced workflow with error handling and retries",
		Files: map[string]string{
			"workflow.laq.yaml": enterpriseWorkflow,
			"README.md":         enterpriseReadme,
		},
	},
}

func initializeProject(projectName string) {
	// Validate project name
	if !isValidProjectName(projectName) {
		Error("Project name must contain only letters, numbers, hyphens, and underscores")
		os.Exit(1)
	}

	// Check if template exists
	template, exists := templates[templateName]
	if !exists {
		Error(fmt.Sprintf("Unknown template: %s", templateName))
		fmt.Println("Available templates:")
		for name, tmpl := range templates {
			fmt.Printf("  %s: %s\n", name, tmpl.Description)
		}
		os.Exit(1)
	}

	// Check if directory exists
	if _, err := os.Stat(projectName); err == nil && !force {
		Error(fmt.Sprintf("Directory %s already exists, use --force to overwrite", projectName))
		os.Exit(1)
	}

	Info(fmt.Sprintf("Creating new Lacquer project: %s", projectName))
	Info(fmt.Sprintf("Using template: %s", template.Name))

	// Create project directory
	if err := os.MkdirAll(projectName, 0755); err != nil {
		Error(fmt.Sprintf("Failed to create project directory: %v", err))
		os.Exit(1)
	}

	// Create .lacquer directory
	lacquerDir := filepath.Join(projectName, ".lacquer")
	if err := os.MkdirAll(lacquerDir, 0755); err != nil {
		Error(fmt.Sprintf("Failed to create .lacquer directory: %v", err))
		os.Exit(1)
	}

	// Create config file
	configPath := filepath.Join(lacquerDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		Error(fmt.Sprintf("Failed to create config file: %v", err))
		os.Exit(1)
	}

	// Create template files
	for filename, content := range template.Files {
		filePath := filepath.Join(projectName, filename)
		content = strings.ReplaceAll(content, "{{PROJECT_NAME}}", projectName)

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			Error(fmt.Sprintf("Failed to create %s: %v", filename, err))
			os.Exit(1)
		}
	}

	// Initialize git repository
	if !noGit {
		if err := initGitRepository(projectName); err != nil {
			Warning(fmt.Sprintf("Failed to initialize git repository: %v", err))
		} else {
			Info("Initialized git repository")
		}
	}

	Success(fmt.Sprintf("Project %s created successfully!", projectName))
	fmt.Println()
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Printf("  laq validate workflow.laq.yaml\n")
	fmt.Printf("  laq run workflow.laq.yaml\n")
	fmt.Println()
	fmt.Printf("Learn more at https://lacquer.ai/docs\n")
}

func isValidProjectName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}

	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}

	return true
}

func initGitRepository(projectDir string) error {
	// Create .gitignore
	gitignorePath := filepath.Join(projectDir, ".gitignore")
	gitignoreContent := `# Lacquer
.lacquer/state/
.lacquer/cache/
*.log

# Dependencies
node_modules/

# IDE
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
`

	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return err
	}

	// TODO: Initialize git repository when we have git command available
	// For now, just create .gitignore
	return nil
}

// Template content
const defaultConfig = `# Lacquer Configuration File
# This file contains default settings for your Lacquer project

# Logging configuration
log_level: info

# Output format (text, json, yaml)
output: text

# Default model configuration
defaults:
  model: gpt-4
  temperature: 0.7
  max_tokens: 2000

# Environment variables to load
env_files:
  - .env
  - .env.local

# API keys and secrets (use environment variables)
# openai_api_key: ${OPENAI_API_KEY}
# anthropic_api_key: ${ANTHROPIC_API_KEY}
`

const basicWorkflow = `version: "1.0"

metadata:
  name: "{{PROJECT_NAME}}"
  description: "A basic Lacquer workflow"
  author: "Your Name"

agents:
  assistant:
    model: gpt-4
    temperature: 0.7
    system_prompt: "You are a helpful AI assistant."

workflow:
  inputs:
    topic:
      type: string
      description: "The topic to discuss"
      required: true
      default: "AI and automation"

  steps:
    - id: greet
      agent: assistant
      prompt: "Hello! Let's talk about {{inputs.topic}}. Give me a brief overview."
      
    - id: elaborate
      agent: assistant
      prompt: "That's interesting! Can you elaborate on the key benefits of {{inputs.topic}}?"

  outputs:
    greeting: "{{steps.greet.response}}"
    elaboration: "{{steps.elaborate.response}}"
`

const basicReadme = `# {{PROJECT_NAME}}

A basic Lacquer workflow project.

## Getting Started

1. **Validate the workflow:**
` + "```bash" + `
   laq validate workflow.laq.yaml
` + "```" + `

2. **Run the workflow:**
` + "```bash" + `
   laq run workflow.laq.yaml --input topic="machine learning"
` + "```" + `

3. **Customize the workflow:**
   - Edit ` + "`workflow.laq.yaml`" + ` to modify the conversation
   - Add new agents with different models or configurations
   - Create additional steps for more complex interactions

## Configuration

The project configuration is stored in ` + "`.lacquer/config.yaml`" + `. You can customize:
- Default model settings
- API keys and credentials
- Logging preferences

## Learn More

- [Lacquer Documentation](https://lacquer.ai/docs)
- [DSL Reference](https://lacquer.ai/docs/dsl)
- [Examples](https://lacquer.ai/examples)
`

const researchWorkflow = `version: "1.0"

metadata:
  name: "{{PROJECT_NAME}}"
  description: "Research workflow with web search capabilities"
  author: "Your Name"

agents:
  researcher:
    model: gpt-4
    temperature: 0.3
    system_prompt: "You are a thorough researcher who provides accurate, well-sourced information."
    tools:
      - name: web_search
        uses: lacquer/web-search@v1

workflow:
  inputs:
    research_topic:
      type: string
      description: "Topic to research"
      required: true

  steps:
    - id: search
      agent: researcher
      prompt: "Search for recent information about {{inputs.research_topic}}"
      
    - id: analyze
      agent: researcher
      prompt: "Based on the search results, provide a comprehensive analysis of {{inputs.research_topic}}. Include key findings, trends, and implications."
      
    - id: summarize
      agent: researcher
      prompt: "Create a concise executive summary of the research on {{inputs.research_topic}}"

  outputs:
    search_results: "{{steps.search.response}}"
    analysis: "{{steps.analyze.response}}"
    summary: "{{steps.summarize.response}}"
`

const researchReadme = `# {{PROJECT_NAME}}

A research-focused Lacquer workflow with web search capabilities.

## Features

- Web search integration
- Multi-step research process
- Comprehensive analysis and summarization

## Usage

` + "```bash" + `
# Research a topic
laq run workflow.laq.yaml --input research_topic="artificial intelligence trends 2024"

# Validate before running
laq validate workflow.laq.yaml
` + "```" + `

## Requirements

This workflow uses the ` + "`lacquer/web-search@v1`" + ` block, which requires:
- Internet connection
- Search API configuration (see .lacquer/config.yaml)

## Customization

- Modify agent prompts for different research styles
- Add more analysis steps
- Include fact-checking or source verification steps
- Export results to different formats
`

const enterpriseWorkflow = `version: "1.0"

metadata:
  name: "{{PROJECT_NAME}}"
  description: "Enterprise workflow with error handling and retries"
  author: "Your Name"
  version: "1.0.0"

agents:
  analyst:
    model: gpt-4
    temperature: 0.2
    max_tokens: 2000
    system_prompt: "You are a professional business analyst."
    policies:
      max_retries: 3
      timeout: 5m
      cost_limit: "$1.00"

workflow:
  inputs:
    business_question:
      type: string
      description: "Business question to analyze"
      required: true
    
    priority:
      type: string
      description: "Analysis priority level"
      enum: ["low", "medium", "high", "critical"]
      default: "medium"

  state:
    analysis_context: {}
    retry_count: 0

  steps:
    - id: initial_analysis
      agent: analyst
      prompt: "Analyze this business question: {{inputs.business_question}}. Priority: {{inputs.priority}}"
      timeout: 2m
      retry:
        max_attempts: 3
        backoff: exponential
        initial_delay: 1s
        max_delay: 30s
      on_error:
        - log: "Initial analysis failed"
          fallback: fallback_analysis
    
    - id: deep_dive
      agent: analyst
      prompt: "Provide a deeper analysis based on the initial findings: {{steps.initial_analysis.response}}"
      condition: "{{steps.initial_analysis.success}}"
      
    - id: recommendations
      agent: analyst
      prompt: "Based on the analysis, provide actionable recommendations for: {{inputs.business_question}}"
      
    - id: fallback_analysis
      agent: analyst
      prompt: "Provide a simplified analysis for: {{inputs.business_question}}"
      skip_if: "{{steps.initial_analysis.success}}"

  outputs:
    analysis: "{{steps.initial_analysis.response || steps.fallback_analysis.response}}"
    deep_analysis: "{{steps.deep_dive.response}}"
    recommendations: "{{steps.recommendations.response}}"
    metadata:
      priority: "{{inputs.priority}}"
      completed_at: "{{workflow.completed_at}}"
      retry_count: "{{state.retry_count}}"
`

const enterpriseReadme = `# {{PROJECT_NAME}}

An enterprise-grade Lacquer workflow with advanced error handling, retries, and monitoring.

## Enterprise Features

- **Error Handling**: Automatic retries with exponential backoff
- **Cost Controls**: Budget limits and monitoring
- **Timeout Management**: Step-level timeouts
- **Conditional Logic**: Dynamic workflow execution
- **State Management**: Persistent workflow state
- **Fallback Strategies**: Graceful degradation

## Usage

` + "```bash" + `
# Run enterprise analysis
laq run workflow.laq.yaml \
  --input business_question="How can we improve customer retention?" \
  --input priority="high"

# Monitor execution
laq run workflow.laq.yaml --verbose --output json
` + "```" + `

## Configuration

Enterprise workflows require additional configuration:

1. **Cost Limits**: Set in agent policies
2. **Retry Policies**: Configured per step
3. **Monitoring**: Enable structured logging
4. **Secrets**: Use environment variables for API keys

## Monitoring

- View execution logs: ` + "`laq logs`" + `
- Check workflow status: ` + "`laq status`" + `
- Export metrics: ` + "`laq metrics --export`" + `

## Production Deployment

- Use ` + "`laq serve`" + ` for API deployment
- Configure monitoring and alerting
- Set up automated testing with ` + "`laq test`" + `
`
