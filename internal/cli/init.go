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

You can skip steps by providing flags:
`,
	Example: `
  laq init                                                    # Start interactive setup wizard
  laq init --name myproject --description "My awesome app"    # Skip name and description steps
  laq init --providers anthropic,openai                       # Skip providers selection
  laq init --name myproject --description "CLI tool" --providers anthropic --non-interactive  # Full non-interactive setup`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runCtx := execcontext.RunContext{
			Context: cmd.Context(),
			StdOut:  cmd.OutOrStdout(),
			StdErr:  cmd.OutOrStderr(),
		}

		// Get flag values
		projectName, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		providers, _ := cmd.Flags().GetStringSlice("providers")
		nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

		initializeProjectInteractive(runCtx, InitFlags{
			ProjectName:    projectName,
			Description:    description,
			ModelProviders: providers,
			NonInteractive: nonInteractive,
		})
	},
}

// InitFlags represents the command line flags for init
type InitFlags struct {
	ProjectName    string
	Description    string
	ModelProviders []string
	NonInteractive bool
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Add flags
	initCmd.Flags().StringP("name", "n", "", "Project name")
	initCmd.Flags().StringP("description", "d", "", "Project description")
	initCmd.Flags().StringSliceP("providers", "p", []string{}, "Model providers (anthropic, openai, claude-code)")
	initCmd.Flags().Bool("non-interactive", false, "Run in non-interactive mode (requires all other flags)")
}

type Step int

const (
	StepProjectName Step = iota
	StepDescription
	StepModelProviders
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

// WorkflowManager handles workflow creation and polling
type WorkflowManager struct {
	answers ProjectAnswers
	out     io.Writer
}

type ProjectAnswers struct {
	projectName    string
	description    string
	modelProviders []string
}

// createWorkflow makes the initial API call to create a workflow
func (wm *WorkflowManager) createWorkflow() (string, error) {
	requestData := initRequest{
		Description:    wm.answers.description,
		ModelProviders: wm.answers.modelProviders,
		ProjectName:    wm.answers.projectName,
		Version:        getVersion(),
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(
		lacquerAPIBaseURL+"/v1/workflows/init",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to call init API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.NewDecoder(bytes.NewBuffer(body)).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ID == "" {
		return "", fmt.Errorf("failed to get workflow ID: %s", string(body))
	}

	return result.ID, nil
}

// pollWorkflowResults polls the workflow until completion and returns the results
func (wm *WorkflowManager) pollWorkflowResults(workflowID string) (pollResultMsg, error) {
	for {
		time.Sleep(3 * time.Second)

		resp, err := http.Get(lacquerAPIBaseURL + "/v1/workflows/" + workflowID + "/results")
		if err != nil {
			return pollResultMsg{}, fmt.Errorf("failed to poll workflow: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return pollResultMsg{}, fmt.Errorf("polling error %d", resp.StatusCode)
		}

		var pollResult struct {
			Status   string        `json:"status"`
			Workflow string        `json:"workflow_contents"`
			Scripts  []fileContent `json:"script_contents"`
			Error    string        `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&pollResult); err != nil {
			resp.Body.Close()
			return pollResultMsg{}, fmt.Errorf("failed to decode poll response: %w", err)
		}
		resp.Body.Close()

		if pollResult.Status == "completed" {
			return pollResultMsg{
				status:   pollResult.Status,
				workflow: pollResult.Workflow,
				scripts:  pollResult.Scripts,
			}, nil
		}

		if pollResult.Status == "failed" || pollResult.Error != "" {
			return pollResultMsg{}, fmt.Errorf("workflow failed: %s", pollResult.Error)
		}

		if wm.out != nil {
			fmt.Fprintf(wm.out, "Status: %s, continuing...\n", pollResult.Status)
		}
	}
}

// saveGeneratedFiles saves the workflow and associated files to disk
func (wm *WorkflowManager) saveGeneratedFiles(result pollResultMsg) (map[string]string, error) {
	generatedFiles := make(map[string]string)

	// Create project directory
	if err := os.MkdirAll(wm.answers.projectName, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	// Save main workflow
	if result.workflow != "" {
		workflowPath := filepath.Join(wm.answers.projectName, "workflow.laq.yml")
		if err := os.WriteFile(workflowPath, []byte(result.workflow), 0644); err != nil {
			return nil, fmt.Errorf("failed to save workflow: %w", err)
		}
		generatedFiles["workflow.laq.yml"] = workflowPath
	}

	// Save scripts
	if len(result.scripts) > 0 {
		scriptsDir := filepath.Join(wm.answers.projectName, "scripts")
		if err := os.MkdirAll(scriptsDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create scripts directory: %w", err)
		}

		for _, script := range result.scripts {
			scriptPath := filepath.Join(scriptsDir, script.Name)
			if err := os.WriteFile(scriptPath, []byte(script.Content), 0755); err != nil {
				return nil, fmt.Errorf("failed to save script %s: %w", script.Name, err)
			}
			generatedFiles["scripts/"+script.Name] = scriptPath
		}
	}

	return generatedFiles, nil
}

// runWorkflowProcess handles the complete workflow creation, polling, and file saving
func (wm *WorkflowManager) runWorkflowProcess() (map[string]string, error) {
	// Create workflow
	workflowID, err := wm.createWorkflow()
	if err != nil {
		return nil, err
	}

	if wm.out != nil {
		fmt.Fprintf(wm.out, "Generating workflow (ID: %s)...\n", workflowID)
	}

	// Poll for results
	result, err := wm.pollWorkflowResults(workflowID)
	if err != nil {
		return nil, err
	}

	// Save generated files
	generatedFiles, err := wm.saveGeneratedFiles(result)
	if err != nil {
		return nil, err
	}

	return generatedFiles, nil
}

// Model represents the wizard state
type model struct {
	step             Step
	projectNameInput textinput.Model
	description      textinput.Model
	modelProviders   list.Model
	spinner          spinner.Model
	width            int
	height           int
	err              error

	answers        ProjectAnswers
	workflowMgr    *WorkflowManager
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

// validateNonInteractiveFlags validates that all required flags are provided for non-interactive mode
func validateNonInteractiveFlags(flags InitFlags) error {
	if flags.ProjectName == "" {
		return fmt.Errorf("project name is required (--name)")
	}
	if flags.Description == "" {
		return fmt.Errorf("description is required (--description)")
	}

	// Validate project name
	if !isValidProjectName(flags.ProjectName) {
		return fmt.Errorf("invalid project name: %s", flags.ProjectName)
	}

	// Check if project directory already exists
	if _, err := os.Stat(flags.ProjectName); err == nil {
		return fmt.Errorf("directory %s already exists", flags.ProjectName)
	}

	// Validate model providers if provided
	if len(flags.ModelProviders) > 0 {
		validProviders := []string{"anthropic", "openai", "claude-code"}
		for _, provider := range flags.ModelProviders {
			validProvider := false
			for _, valid := range validProviders {
				if provider == valid {
					validProvider = true
					break
				}
			}
			if !validProvider {
				return fmt.Errorf("invalid model provider: %s (valid options: %s)", provider, strings.Join(validProviders, ", "))
			}
		}
	}

	return nil
}

// runNonInteractiveInit runs the initialization without the interactive wizard
func runNonInteractiveInit(runCtx execcontext.RunContext, flags InitFlags) error {
	fmt.Fprintf(runCtx.StdOut, "Initializing project %s...\n", flags.ProjectName)

	wm := &WorkflowManager{
		answers: ProjectAnswers{
			projectName:    flags.ProjectName,
			description:    flags.Description,
			modelProviders: flags.ModelProviders,
		},
		out: runCtx.StdOut,
	}

	generatedFiles, err := wm.runWorkflowProcess()
	if err != nil {
		return fmt.Errorf("failed to generate workflow: %w", err)
	}

	fmt.Fprint(wm.out, renderCompleteStep(flags.ProjectName, generatedFiles))

	return nil
}

// initialModelWithFlags creates the initial model with pre-filled values from flags
func initialModelWithFlags(flags InitFlags) model {
	m := initialModel()

	// Pre-fill values from flags and determine starting step
	if flags.ProjectName != "" {
		m.projectNameInput.SetValue(flags.ProjectName)
		m.answers.projectName = flags.ProjectName

		if flags.Description != "" {
			m.description.SetValue(flags.Description)
			m.answers.description = flags.Description

			if len(flags.ModelProviders) > 0 {
				// Pre-select model providers if provided
				if len(flags.ModelProviders) > 0 {
					items := m.modelProviders.Items()
					for i, item := range items {
						if provider, ok := item.(providerItem); ok {
							for _, selectedProvider := range flags.ModelProviders {
								if provider.name == selectedProvider {
									provider.selected = true
									items[i] = provider
								}
							}
						}
					}
					m.modelProviders.SetItems(items)
					m.answers.modelProviders = flags.ModelProviders
				}

				// If providers are set, go to summary
				if len(flags.ModelProviders) > 0 {
					m.step = StepSummary
				}
			} else {
				// Only name and description set, go to providers
				m.step = StepModelProviders
			}
		} else {
			// Only name set, go to description
			m.step = StepDescription
			m.description.Focus()
		}
	}

	return m
}

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

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = style.AccentStyle

	return model{
		step:             StepProjectName,
		projectNameInput: pni,
		description:      ti,
		modelProviders:   providerList,
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
			return m, tea.Batch(cmd)
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
		// If model providers are already set, advance to summary
		if len(m.answers.modelProviders) > 0 {
			m.step = StepSummary
		}
		return m, nil

	case StepModelProviders:
		// Collect selected providers
		m.answers.modelProviders = []string{}
		for _, item := range m.modelProviders.Items() {
			if provider, ok := item.(providerItem); ok && provider.selected {
				m.answers.modelProviders = append(m.answers.modelProviders, provider.name)
			}
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
	m.workflowMgr = &WorkflowManager{
		answers: m.answers,
	}
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
	ProjectName    string   `json:"project_name"`
	Version        string   `json:"version"`
}

// Call init API
func (m model) callInitAPI() tea.Cmd {
	return func() tea.Msg {
		workflowID, err := m.workflowMgr.createWorkflow()
		if err != nil {
			return errorMsg{err: err}
		}

		return initResponseMsg{workflowID: workflowID}
	}
}

// Poll workflow results
func (m model) pollWorkflow() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		result, err := m.workflowMgr.pollWorkflowResults(m.workflowID)
		if err != nil {
			return errorMsg{err: err}
		}

		return result
	})
}

// Handle poll result
func (m model) handlePollResult(msg pollResultMsg) (tea.Model, tea.Cmd) {
	if msg.status == "completed" {
		// Save files and complete
		generatedFiles, err := m.workflowMgr.saveGeneratedFiles(msg)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.generatedFiles = generatedFiles
		m.step = StepComplete
		m.pollComplete = true
		return m, nil
	}

	// Continue polling if not done
	return m, m.pollWorkflow()
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
	return renderCompleteStep(m.answers.projectName, m.generatedFiles)
}

func renderCompleteStep(projectName string, generatedFiles map[string]string) string {
	var filesList strings.Builder
	for relativePath := range generatedFiles {
		filesList.WriteString(fmt.Sprintf("  %s %s\n", style.SuccessIcon(), relativePath))
	}

	nextSteps := fmt.Sprintf(
		"cd %s && laq validate workflow.laq.yml && laq run workflow.laq.yml",
		projectName,
	)

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n%s\n\n%s\n\nPress Enter to exit.",
		titleStyle.Render("Project Created Successfully!"),
		"Generated files:",
		filesList.String(),
		"Next steps:",
		style.CodeStyle.Render(nextSteps),
	)
}

// Main initialization function
func initializeProjectInteractive(runCtx execcontext.RunContext, flags InitFlags) {
	// If non-interactive mode, validate all required flags are provided
	if flags.NonInteractive {
		if err := validateNonInteractiveFlags(flags); err != nil {
			style.Error(runCtx, fmt.Sprintf("Non-interactive mode error: %v", err))
			os.Exit(1)
		}

		// Run non-interactive initialization
		if err := runNonInteractiveInit(runCtx, flags); err != nil {
			style.Error(runCtx, fmt.Sprintf("Failed to initialize project: %v", err))
			os.Exit(1)
		}
		return
	}

	// Run the interactive wizard with pre-filled values
	p := tea.NewProgram(initialModelWithFlags(flags), tea.WithAltScreen())
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
