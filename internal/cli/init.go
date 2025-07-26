package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/style"
	"github.com/spf13/cobra"
)

var lacquerAPIBaseURL = os.Getenv("LACQUER_API")

func init() {
	if lacquerAPIBaseURL == "" {
		lacquerAPIBaseURL = "https://api.lacquer.ai"
	}
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Lacquer project with interactive setup",
	Long: `Initialize a new Lacquer project using an interactive setup wizard.

The wizard will guide you through:
- Choosing a project name
- Describing what you want to build
- Selecting model providers (Anthropic, OpenAI, Claude Code)
- Choosing your preferred scripting language
`,
	Example: `
  laq init    # Start interactive setup wizard`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runCtx := execcontext.RunContext{
			Context: cmd.Context(),
			StdOut:  cmd.OutOrStdout(),
			StdErr:  cmd.OutOrStderr(),
		}
		initializeProjectInteractive(runCtx)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

type Step int

const (
	StepProjectName Step = iota
	StepDescription
	StepModelProviders
	StepScriptLanguage
	StepSummary
	StepProcessing
	StepComplete
)

var (
	quitKey = key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("Ctrl+C", "Quit"),
	)
	enterKey = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("Enter", "Continue"),
	)
	tabKey = key.NewBinding(
		key.WithKeys("tab", "shift+tab"),
		key.WithHelp("Tab", "Next"),
	)
	spaceKey = key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("Space", "Select"),
	)
)

// Model represents the wizard state
type model struct {
	step             Step
	projectNameInput textinput.Model
	description      textinput.Model
	modelProviders   list.Model
	scriptLanguage   list.Model
	spinner          spinner.Model
	width            int
	height           int
	err              error

	answers struct {
		projectName    string
		description    string
		modelProviders []string
		scriptLanguage string
	}

	workflowID     string
	pollComplete   bool
	generatedFiles map[string]string
}

// Provider item for lists
type providerItem struct {
	name     string
	selected bool
}

func (i providerItem) FilterValue() string { return i.name }
func (i providerItem) Title() string       { return i.name }
func (i providerItem) Description() string {
	if i.selected {
		return "✓ Selected"
	}
	return "Not selected"
}

type languageItem struct {
	name        string
	description string
}

func (i languageItem) FilterValue() string { return i.name }
func (i languageItem) Title() string       { return i.name }
func (i languageItem) Description() string { return i.description }

// Initialize the model
func initialModel() model {
	// Project name input
	pni := textinput.New()
	pni.Placeholder = "Enter project name..."
	pni.SetValue("lacquer")
	pni.Focus()
	pni.CharLimit = 50
	pni.SetWidth(50)

	// Description input
	ti := textinput.New()
	ti.Placeholder = "Describe what you want to build..."
	ti.CharLimit = 200
	ti.SetWidth(50)

	providerItems := []list.Item{
		providerItem{name: "anthropic", selected: false},
		providerItem{name: "openai", selected: false},
		providerItem{name: "claude-code", selected: false},
	}

	providerList := list.New(providerItems, list.NewDefaultDelegate(), 50, 10)
	providerList.SetShowTitle(false)
	providerList.SetShowStatusBar(false)
	providerList.SetFilteringEnabled(false)
	providerList.SetShowHelp(false)
	providerList.Styles.Title = titleStyle

	mcpInput := textinput.New()
	mcpInput.Placeholder = "Enter MCP providers (comma-separated, or leave empty)"
	mcpInput.CharLimit = 200
	mcpInput.SetWidth(50)

	languageItems := []list.Item{
		languageItem{name: "node", description: "Node.js/JavaScript"},
		languageItem{name: "python", description: "Python"},
		languageItem{name: "go", description: "Go"},
	}

	languageList := list.New(languageItems, list.NewDefaultDelegate(), 50, 10)
	languageList.SetShowTitle(false)
	languageList.SetShowStatusBar(false)
	languageList.SetFilteringEnabled(false)
	languageList.SetShowHelp(false)
	languageList.Styles.Title = titleStyle

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = style.AccentStyle

	return model{
		step:             StepProjectName,
		projectNameInput: pni,
		description:      ti,
		modelProviders:   providerList,
		scriptLanguage:   languageList,
		spinner:          s,
		generatedFiles:   make(map[string]string),
	}
}

// Init implements tea.Model
func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

// Styles - using the standardized color themes from internal/style/output.go
var (
	titleStyle = style.TitleStyle.
			Background(style.SuccessColor).
			Foreground(style.PrimaryBgColor).
			Padding(1, 1)

	subtitleStyle = style.MutedStyle

	selectedStyle = style.SuccessStyle.Bold(true)

	errorStyle = style.ErrorStyle

	successStyle = style.SuccessStyle
)

// Update handles messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, quitKey):
			return m, tea.Quit
		case key.Matches(msg, enterKey):
			return m.handleEnter()
		case key.Matches(msg, tabKey):
			return m.handleTab()
		case key.Matches(msg, spaceKey):
			if m.step == StepModelProviders {
				return m.toggleProvider()
			}
		}

	case spinner.TickMsg:
		if m.step == StepProcessing {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, tea.Batch(cmd, m.pollWorkflow())
		}

	case initResponseMsg:
		m.workflowID = msg.workflowID
		return m, m.pollWorkflow()

	case pollResultMsg:
		return m.handlePollResult(msg)

	case errorMsg:
		m.err = msg.err
		return m, nil
	}

	switch m.step {
	case StepProjectName:
		m.projectNameInput, cmd = m.projectNameInput.Update(msg)
	case StepDescription:
		m.description, cmd = m.description.Update(msg)
	case StepModelProviders:
		m.modelProviders, cmd = m.modelProviders.Update(msg)
	case StepScriptLanguage:
		m.scriptLanguage, cmd = m.scriptLanguage.Update(msg)
	}

	return m, cmd
}

// Handle enter key
func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepProjectName:
		projectName := strings.TrimSpace(m.projectNameInput.Value())
		if projectName == "" {
			return m, nil
		}
		if !isValidProjectName(projectName) {
			return m, nil
		}
		if _, err := os.Stat(projectName); err == nil {
			return m, nil
		}
		m.answers.projectName = projectName
		m.step = StepDescription
		m.description.Focus()
		return m, nil

	case StepDescription:
		if strings.TrimSpace(m.description.Value()) == "" {
			return m, nil
		}
		m.answers.description = m.description.Value()
		m.step = StepModelProviders
		return m, nil

	case StepModelProviders:
		// Collect selected providers
		m.answers.modelProviders = []string{}
		for _, item := range m.modelProviders.Items() {
			if provider, ok := item.(providerItem); ok && provider.selected {
				m.answers.modelProviders = append(m.answers.modelProviders, provider.name)
			}
		}

		m.step = StepScriptLanguage
		return m, nil
	case StepScriptLanguage:
		if selectedItem, ok := m.scriptLanguage.SelectedItem().(languageItem); ok {
			m.answers.scriptLanguage = selectedItem.name
		}
		if m.answers.scriptLanguage == "" {
			return m, nil
		}
		m.step = StepSummary
		return m, nil

	case StepSummary:
		return m.startProcessing()

	case StepComplete:
		return m, tea.Quit
	}

	return m, nil
}

// Handle tab key
func (m model) handleTab() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepDescription:
		// Move to next step if description is filled
		if strings.TrimSpace(m.description.Value()) != "" {
			return m.handleEnter()
		}
	}
	return m, nil
}

// Toggle provider selection
func (m model) toggleProvider() (tea.Model, tea.Cmd) {
	if selectedItem, ok := m.modelProviders.SelectedItem().(providerItem); ok {
		selectedItem.selected = !selectedItem.selected

		// Update the item in the list
		index := m.modelProviders.Index()
		items := m.modelProviders.Items()
		items[index] = selectedItem
		m.modelProviders.SetItems(items)
	}
	return m, nil
}

// Start processing (API calls)
func (m model) startProcessing() (tea.Model, tea.Cmd) {
	m.step = StepProcessing
	return m, tea.Batch(
		m.spinner.Tick,
		m.callInitAPI(),
	)
}

// API message types
type initResponseMsg struct {
	workflowID string
}

type pollResultMsg struct {
	status   string
	workflow string
	scripts  []fileContent
	tools    []fileContent
}

type fileContent struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type errorMsg struct {
	err error
}

type initRequest struct {
	Description    string   `json:"description"`
	ModelProviders []string `json:"model_providers"`
	ScriptLanguage string   `json:"script_language"`
	ProjectName    string   `json:"project_name"`
	Version        string   `json:"version"`
}

// Call init API
func (m model) callInitAPI() tea.Cmd {
	return func() tea.Msg {
		requestData := initRequest{
			Description:    m.answers.description,
			ModelProviders: m.answers.modelProviders,
			ScriptLanguage: m.answers.scriptLanguage,
			ProjectName:    m.answers.projectName,
			Version:        getVersion(),
		}

		jsonData, err := json.Marshal(requestData)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to marshal request: %w", err)}
		}

		resp, err := http.Post(
			lacquerAPIBaseURL+"/v1/workflows/init",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to call init API: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return errorMsg{err: fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))}
		}

		var result struct {
			WorkflowID string `json:"workflow_id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return errorMsg{err: fmt.Errorf("failed to decode response: %w", err)}
		}

		return initResponseMsg{workflowID: result.WorkflowID}
	}
}

// Poll workflow results
func (m model) pollWorkflow() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		if m.workflowID == "" {
			return nil
		}

		resp, err := http.Get(lacquerAPIBaseURL + "/v1/workflows/workflow/" + m.workflowID + "/results")
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to poll workflow: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return errorMsg{err: fmt.Errorf("polling error %d", resp.StatusCode)}
		}

		var result struct {
			Status   string        `json:"status"`
			Workflow string        `json:"workflow"`
			Scripts  []fileContent `json:"scripts"`
			Tools    []fileContent `json:"tools"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return errorMsg{err: fmt.Errorf("failed to decode poll response: %w", err)}
		}

		return pollResultMsg{
			status:   result.Status,
			workflow: result.Workflow,
			scripts:  result.Scripts,
			tools:    result.Tools,
		}
	})
}

// Handle poll result
func (m model) handlePollResult(msg pollResultMsg) (tea.Model, tea.Cmd) {
	if msg.status == "done" {
		// Save files and complete
		if err := m.saveGeneratedFiles(msg); err != nil {
			m.err = err
			return m, nil
		}
		m.step = StepComplete
		m.pollComplete = true
		return m, nil
	}

	// Continue polling if not done
	return m, m.pollWorkflow()
}

// Save generated files
func (m model) saveGeneratedFiles(msg pollResultMsg) error {
	// Create project directory
	if err := os.MkdirAll(m.answers.projectName, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Save main workflow
	if msg.workflow != "" {
		workflowPath := filepath.Join(m.answers.projectName, "workflow.laq.yml")
		if err := os.WriteFile(workflowPath, []byte(msg.workflow), 0644); err != nil {
			return fmt.Errorf("failed to save workflow: %w", err)
		}
		m.generatedFiles["workflow.laq.yml"] = workflowPath
	}

	// Save scripts
	if len(msg.scripts) > 0 {
		scriptsDir := filepath.Join(m.answers.projectName, "scripts")
		if err := os.MkdirAll(scriptsDir, 0755); err != nil {
			return fmt.Errorf("failed to create scripts directory: %w", err)
		}

		for _, script := range msg.scripts {
			scriptPath := filepath.Join(scriptsDir, script.Name)
			if err := os.WriteFile(scriptPath, []byte(script.Content), 0755); err != nil {
				return fmt.Errorf("failed to save script %s: %w", script.Name, err)
			}
			m.generatedFiles["scripts/"+script.Name] = scriptPath
		}
	}

	// Save tools
	if len(msg.tools) > 0 {
		toolsDir := filepath.Join(m.answers.projectName, "tools")
		if err := os.MkdirAll(toolsDir, 0755); err != nil {
			return fmt.Errorf("failed to create tools directory: %w", err)
		}

		for _, tool := range msg.tools {
			toolPath := filepath.Join(toolsDir, tool.Name)
			if err := os.WriteFile(toolPath, []byte(tool.Content), 0755); err != nil {
				return fmt.Errorf("failed to save tool %s: %w", tool.Name, err)
			}
			m.generatedFiles["tools/"+tool.Name] = toolPath
		}
	}

	return nil
}

// View renders the current state
func (m model) View() string {
	if m.err != nil {
		return errorStyle.Render("Error: "+m.err.Error()) + "\n\nPress 'q' to quit."
	}

	box := lipgloss.NewStyle().
		Padding(1, 2)

	var out string
	switch m.step {
	case StepProjectName:
		out = m.renderProjectNameStep()
	case StepDescription:
		out = m.renderDescriptionStep()
	case StepModelProviders:
		out = m.renderModelProvidersStep()
	case StepScriptLanguage:
		out = m.renderScriptLanguageStep()
	case StepSummary:
		out = m.renderSummaryStep()
	case StepProcessing:
		out = m.renderProcessingStep()
	case StepComplete:
		out = m.renderCompleteStep()
	}

	return box.Render(out)
}

func (m model) renderProjectNameStep() string {
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\n%s",
		titleStyle.Render("Lacquer Project Setup"),
		subtitleStyle.Render("Give your Lacquer project a name:"),
		m.projectNameInput.View(),
		"Press Enter to continue, Ctrl+C to quit",
	)
}

func (m model) renderDescriptionStep() string {
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\n%s",
		titleStyle.Render("Lacquer Project Setup"),
		subtitleStyle.Render("What do you want to build?"),
		m.description.View(),
		"Press Enter to continue, Ctrl+C to quit",
	)
}

func (m model) renderModelProvidersStep() string {
	selected := []string{}
	for _, item := range m.modelProviders.Items() {
		if provider, ok := item.(providerItem); ok && provider.selected {
			selected = append(selected, provider.name)
		}
	}

	selectedText := "None selected"
	if len(selected) > 0 {
		selectedText = strings.Join(selected, ", ")
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\n%s\n\n%s",
		titleStyle.Render("Model Providers"),
		subtitleStyle.Render("Select the model providers you want to use or leave empty for lacquer to choose the best one for you (use Space to toggle, Enter to continue):"),
		m.modelProviders.View(),
		"Selected: "+selectedStyle.Render(selectedText),
		"Press Space to select/deselect, Enter to continue",
	)
}

func (m model) renderScriptLanguageStep() string {
	selected := "None selected"
	if selectedItem, ok := m.scriptLanguage.SelectedItem().(languageItem); ok {
		selected = selectedItem.name
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\n%s\n\n%s",
		titleStyle.Render("Script Language"),
		subtitleStyle.Render("Choose your preferred language for scripts and/or local tools to be written in:"),
		m.scriptLanguage.View(),
		"Selected: "+selectedStyle.Render(selected),
		"Press Enter to continue",
	)
}

func (m model) renderSummaryStep() string {
	var summaryBuilder strings.Builder

	// Project name
	summaryBuilder.WriteString(fmt.Sprintf("%s: %s\n",
		selectedStyle.Render("Project Name"),
		m.answers.projectName))

	// Description
	summaryBuilder.WriteString(fmt.Sprintf("%s: %s\n",
		selectedStyle.Render("Description"),
		m.answers.description))

	// Model providers
	providers := "None selected"
	if len(m.answers.modelProviders) > 0 {
		providers = strings.Join(m.answers.modelProviders, ", ")
	} else {
		providers = "Auto-select best provider"
	}
	summaryBuilder.WriteString(fmt.Sprintf("%s: %s\n",
		selectedStyle.Render("Model Providers"),
		providers))

	// Script language
	summaryBuilder.WriteString(fmt.Sprintf("%s: %s\n",
		selectedStyle.Render("Script Language"),
		m.answers.scriptLanguage))

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\n%s",
		titleStyle.Render("Project Summary"),
		subtitleStyle.Render("Please review your selections:"),
		summaryBuilder.String(),
		"Press Enter to generate your project, Ctrl+C to quit",
	)
}

func (m model) renderProcessingStep() string {
	return fmt.Sprintf(
		"%s\n\n%s %s\n\n%s",
		titleStyle.Render("Generating Project"),
		m.spinner.View(),
		"Generating your workflow...",
		"This may take a few moments. Please wait.",
	)
}

func (m model) renderCompleteStep() string {
	var filesList strings.Builder
	for relativePath := range m.generatedFiles {
		filesList.WriteString(fmt.Sprintf("  ✓ %s\n", relativePath))
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\n%s\n  cd %s\n  laq validate main.laq.yml\n  laq run main.laq.yml\n\nPress Enter to exit.",
		successStyle.Render("Project Created Successfully!"),
		"Generated files:",
		filesList.String(),
		"Next steps:",
		m.answers.projectName,
	)
}

// Main initialization function
func initializeProjectInteractive(runCtx execcontext.RunContext) {
	// Run the interactive wizard
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		style.Error(runCtx, fmt.Sprintf("Failed to run setup wizard: %v", err))
		os.Exit(1)
	}
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
