package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"universal-controller/internal/ai"
	"universal-controller/internal/bootstrap"
	"universal-controller/internal/config"
	"universal-controller/internal/doctor"
	"universal-controller/internal/executor"
	"universal-controller/internal/health"
	"universal-controller/internal/receiver"
	"universal-controller/internal/tools"
)

type screen string

const (
	screenDashboard screen = "dashboard"
	screenSettings  screen = "settings"
	screenLogs      screen = "logs"
	screenHelp      screen = "help"
)

type menuAction struct {
	ID          string
	Title       string
	Description string
}

type formFieldSpec struct {
	Key         string
	Label       string
	Value       string
	Placeholder string
	Secret      bool
	Options     []string
}

type formField struct {
	Key         string
	Label       string
	Input       textinput.Model
	Options     []string
	OptionIndex int
}

type formState struct {
	Kind        string
	Title       string
	Description string
	Fields      []formField
	Focus       int
}

type taskFinishedMsg struct {
	Title string
	Body  string
	Err   error
}

type model struct {
	cfg             config.Config
	report          bootstrap.Report
	planner         ai.Planner
	local           executor.LocalExecutor
	client          executor.Client
	screen          screen
	menuCursor      int
	setupCursor     int
	form            *formState
	width           int
	height          int
	status          string
	logs            []string
	accessMode      string
	rootPassword    string
	receiverServer  *receiver.Server
	receiverRunning bool
}

func Run(cfg config.Config, report bootstrap.Report) error {
	m := newModel(cfg, report)
	program := tea.NewProgram(&m, tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func newModel(cfg config.Config, report bootstrap.Report) model {
	cfg.UI.Layout = normalizeLayout(cfg.UI.Layout)
	local := executor.LocalExecutor{Shell: cfg.Shell.Default, Name: "local-device"}
	m := model{
		cfg:        cfg,
		report:     report,
		planner:    ai.Planner{Config: cfg},
		local:      local,
		screen:     screenDashboard,
		status:     "Welcome. Start with 'Start Here (Guided)' and press Enter.",
		logs:       []string{},
		accessMode: "normal",
	}
	if cfg.Role == "self-use" || cfg.Role == "receiver" {
		m.client = local
	}
	if report.Created {
		m.logs = append(m.logs, timeStamped("First-run configuration created automatically."))
	}
	for _, note := range report.Notes {
		m.logs = append(m.logs, timeStamped("Repair: "+note))
	}
	return m
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case taskFinishedMsg:
		if msg.Err != nil {
			m.status = "Action failed: " + msg.Err.Error()
			m.logs = append(m.logs, timeStamped("ERROR "+msg.Title+": "+msg.Err.Error()))
			return m, nil
		}
		m.status = msg.Title + " completed"
		m.logs = append(m.logs, timeStamped(msg.Title))
		for _, line := range strings.Split(strings.TrimSpace(msg.Body), "\n") {
			if strings.TrimSpace(line) != "" {
				m.logs = append(m.logs, timeStamped(line))
			}
		}
		return m, nil
	case tea.KeyMsg:
		if m.form != nil {
			return m.handleFormKey(msg)
		}
		if m.cfg.Role == "" {
			return m.handleSetupKey(msg)
		}
		return m.handleMainKey(msg)
	}
	return m, nil
}

func (m *model) View() string {
	if m.cfg.Role == "" {
		return m.setupView()
	}

	contentWidth := m.width
	if contentWidth <= 0 {
		contentWidth = 110
	}
	if contentWidth < 60 {
		contentWidth = 60
	}
	orientation := "landscape"
	if m.height > 0 && m.height > contentWidth {
		orientation = "portrait"
	}

	header := headerStyle().Render(fmt.Sprintf(
		"Universal Controller  |  mode: %s  |  target: %s  |  access: %s  |  ai: %s  |  view: %s",
		renderRole(m.cfg.Role), m.currentTarget(), m.accessMode, strings.ToUpper(m.cfg.AI.Active), orientation,
	))
	header = lipgloss.NewStyle().Width(contentWidth).Render(header)
	footer := lipgloss.NewStyle().Width(contentWidth).Render(footerStyle().Render(m.status))

	availableHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if availableHeight <= 0 {
		availableHeight = 26
	}

	formHeight := 0
	if m.form != nil {
		formHeight = clamp(availableHeight/3, 9, 14)
		if availableHeight-formHeight < 8 {
			formHeight = 0
		}
	}
	mainHeight := availableHeight - formHeight
	if mainHeight < 10 {
		mainHeight = availableHeight
	}

	panelTotalWidth := max(56, contentWidth-2)
	logHeight := clamp(mainHeight/3, 6, 10)
	topHeight := max(8, mainHeight-logHeight)

	stacked := m.cfg.UI.Layout == "vertical" || contentWidth < 110 || orientation == "portrait"

	logPanel := panelStyle(panelTotalWidth).Height(logHeight).Render(m.renderLogs(logHeight-2, panelTotalWidth-6))

	var body string
	if stacked {
		menuHeight := clamp(topHeight/3, 6, 10)
		detailHeight := max(6, topHeight-menuHeight)
		menu := m.renderMenu(panelTotalWidth, menuHeight)
		detail := m.renderDetail(panelTotalWidth, detailHeight)
		body = lipgloss.JoinVertical(lipgloss.Left, menu, detail, logPanel)
	} else {
		menuWidth := clamp(panelTotalWidth/3, 30, 46)
		detailWidth := max(30, panelTotalWidth-menuWidth-1)
		menu := m.renderMenu(menuWidth, topHeight)
		detail := m.renderDetail(detailWidth, topHeight)
		top := lipgloss.JoinHorizontal(lipgloss.Top, menu, detail)
		body = lipgloss.JoinVertical(lipgloss.Left, top, logPanel)
	}

	if m.form != nil {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.renderForm(panelTotalWidth, formHeight))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *model) handleSetupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.setupCursor > 0 {
			m.setupCursor--
		}
	case "down", "j":
		if m.setupCursor < 2 {
			m.setupCursor++
		}
	case "enter":
		roles := []string{"controller", "receiver", "self-use"}
		m.cfg.Role = roles[m.setupCursor]
		if m.cfg.Role == "receiver" {
			m.cfg.Receiver.Enabled = true
			m.client = m.local
		}
		if m.cfg.Role == "self-use" {
			m.client = m.local
		}
		if m.cfg.Role == "controller" {
			m.client = nil
		}
		_ = config.Save(m.cfg)
		m.status = "Startup mode saved."
		m.logs = append(m.logs, timeStamped("Mode selected: "+m.cfg.Role))
	}
	return m, nil
}

func (m *model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.stopEmbeddedReceiver()
		return m, tea.Quit
	case "tab":
		m.screen = nextScreen(m.screen)
		m.menuCursor = 0
		return m, nil
	case "shift+tab":
		m.screen = previousScreen(m.screen)
		m.menuCursor = 0
		return m, nil
	case "esc":
		m.screen = screenDashboard
		m.menuCursor = 0
		return m, nil
	case "ctrl+h":
		m.cfg.UI.Layout = "horizontal"
		_ = config.Save(m.cfg)
		m.status = "Layout switched to horizontal."
		return m, nil
	case "ctrl+v":
		m.cfg.UI.Layout = "vertical"
		_ = config.Save(m.cfg)
		m.status = "Layout switched to vertical."
		return m, nil
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
		return m, nil
	case "down", "j":
		if m.menuCursor < len(m.actions())-1 {
			m.menuCursor++
		}
		return m, nil
	case "enter":
		return m, m.executeAction()
	}
	return m, nil
}

func (m *model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.stopEmbeddedReceiver()
		return m, tea.Quit
	case "esc":
		m.form = nil
		m.status = "Dialog cancelled."
		return m, nil
	case "up", "shift+tab":
		if m.form.Focus > 0 {
			m.form.Focus--
		}
		m.syncFormFocus()
		return m, nil
	case "down", "tab":
		if m.form.Focus < len(m.form.Fields)-1 {
			m.form.Focus++
		}
		m.syncFormFocus()
		return m, nil
	case "left":
		m.cycleOption(-1)
		return m, nil
	case "right":
		m.cycleOption(1)
		return m, nil
	case "enter":
		if m.form.Focus == len(m.form.Fields)-1 {
			return m, m.submitForm()
		}
		m.form.Focus++
		m.syncFormFocus()
		return m, nil
	}
	cmds := make([]tea.Cmd, 0, len(m.form.Fields))
	for i := range m.form.Fields {
		fieldModel, cmd := m.form.Fields[i].Input.Update(msg)
		m.form.Fields[i].Input = fieldModel
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *model) cycleOption(delta int) {
	field := &m.form.Fields[m.form.Focus]
	if len(field.Options) == 0 {
		return
	}
	field.OptionIndex += delta
	if field.OptionIndex < 0 {
		field.OptionIndex = len(field.Options) - 1
	}
	if field.OptionIndex >= len(field.Options) {
		field.OptionIndex = 0
	}
	field.Input.SetValue(field.Options[field.OptionIndex])
}

func (m *model) executeAction() tea.Cmd {
	action := m.actions()[m.menuCursor]
	switch action.ID {
	case "quick_start":
		return m.runSyncAction("Guided workflow opened", func() string {
			m.screen = screenHelp
			m.menuCursor = 0
			return "Open the beginner guide and follow the steps in order."
		})
	case "set_mode":
		return m.openForm("set_mode", "Choose Device Mode", "Pick how this app should behave on this machine.", []formFieldSpec{
			{Key: "mode", Label: "Mode", Value: m.cfg.Role, Options: []string{"self-use", "controller", "receiver"}},
		})
	case "mode_self":
		return m.runSyncAction("Switched to self mode", func() string {
			m.cfg.Role = "self-use"
			m.client = m.local
			_ = config.Save(m.cfg)
			return "Local execution is now the default target."
		})
	case "mode_controller":
		return m.runSyncAction("Switched to controller mode", func() string {
			m.cfg.Role = "controller"
			m.client = nil
			_ = config.Save(m.cfg)
			return "Connect a receiver to start remote execution."
		})
	case "mode_receiver":
		return m.runSyncAction("Switched to receiver mode", func() string {
			m.cfg.Role = "receiver"
			m.cfg.Receiver.Enabled = true
			m.client = m.local
			_ = config.Save(m.cfg)
			return "This device can now run locally and act as a background agent."
		})
	case "connect_local":
		return m.runSyncAction("Local device selected", func() string {
			m.client = m.local
			return "All actions will execute on this device."
		})
	case "connect_receiver":
		return m.openForm("connect_receiver", "Connect To Receiver", "Provide the receiver address and token.", []formFieldSpec{
			{Key: "address", Label: "Receiver URL", Value: m.lastReceiverAddress(), Placeholder: "http://192.168.1.5:8080"},
			{Key: "token", Label: "Receiver Token", Value: m.lastReceiverToken(), Placeholder: "pairing token"},
		})
	case "run_command":
		return m.openForm("run_command", "Run Command", "Type exactly what you want to run. Example: ls -la", []formFieldSpec{
			{Key: "command", Label: "Command", Placeholder: "pwd"},
			{Key: "mode", Label: "Access Mode", Value: m.accessMode, Options: []string{"normal", "root"}},
			{Key: "timeout", Label: "Timeout Seconds", Value: "120", Placeholder: "120"},
		})
	case "disconnect_remote":
		return m.runSyncAction("Receiver disconnected", func() string {
			if m.cfg.Role == "self-use" || m.cfg.Role == "receiver" {
				m.client = m.local
				return "Returned to local execution."
			}
			m.client = nil
			return "No active target."
		})
	case "receiver_toggle":
		if m.receiverRunning {
			m.stopEmbeddedReceiver()
			return task("Embedded receiver stopped", func() (string, error) {
				return "The in-app receiver listener has been shut down.", nil
			})
		}
		server := receiver.New(m.cfg, m.local)
		m.receiverServer = server
		m.receiverRunning = true
		go func() {
			_ = server.Start()
		}()
		return task("Embedded receiver started", func() (string, error) {
			return fmt.Sprintf("Listening on %s:%d with token %s", m.cfg.Receiver.Host, m.cfg.Receiver.Port, m.cfg.Receiver.Token), nil
		})
	case "health":
		return m.healthAction()
	case "ai_task":
		return m.openForm("ai_task", "AI Assistant", "Describe the task in plain language. No shell syntax is needed.", []formFieldSpec{
			{Key: "provider", Label: "Provider", Value: m.cfg.AI.Active, Options: []string{"chatgpt", "gemini", "claude"}},
			{Key: "task", Label: "Task", Placeholder: "Example: Check server health and summarize the issues"},
		})
	case "tool_catalog":
		return task("Tool catalog", func() (string, error) {
			return tools.Render(tools.CuratedCatalog()), nil
		})
	case "toggle_layout":
		return m.runSyncAction("Layout toggled", func() string {
			if m.cfg.UI.Layout == "horizontal" {
				m.cfg.UI.Layout = "vertical"
			} else {
				m.cfg.UI.Layout = "horizontal"
			}
			_ = config.Save(m.cfg)
			return "New layout: " + m.cfg.UI.Layout
		})
	case "toggle_access":
		if m.accessMode == "root" {
			return m.runSyncAction("Access changed", func() string {
				m.accessMode = "normal"
				m.rootPassword = ""
				return "Switched back to normal access."
			})
		}
		return m.openForm("root_password", "Root Access", "Enter your password once to enable elevated mode for the current session.", []formFieldSpec{
			{Key: "password", Label: "Password", Secret: true, Placeholder: "sudo password"},
		})
	case "settings":
		return m.runSyncAction("Settings opened", func() string {
			m.screen = screenSettings
			m.menuCursor = 0
			return "Use the settings actions on the left."
		})
	case "logs":
		return m.runSyncAction("Logs opened", func() string {
			m.screen = screenLogs
			m.menuCursor = 0
			return "Recent activity is visible in the log panel."
		})
	case "help":
		return m.runSyncAction("Help opened", func() string {
			m.screen = screenHelp
			m.menuCursor = 0
			return "Beginner guide opened."
		})
	case "active_ai":
		return m.openForm("active_ai", "Active AI Provider", "Choose the default assistant used for UI actions.", []formFieldSpec{
			{Key: "provider", Label: "Provider", Value: m.cfg.AI.Active, Options: []string{"chatgpt", "gemini", "claude"}},
		})
	case "openai":
		return m.openForm("openai", "OpenAI Settings", "Configure ChatGPT access.", []formFieldSpec{
			{Key: "api_key", Label: "API Key", Value: m.cfg.AI.OpenAI.APIKey, Secret: true, Placeholder: "sk-..."},
			{Key: "model", Label: "Model", Value: m.cfg.AI.OpenAI.Model, Placeholder: "gpt-4.1-mini"},
			{Key: "base_url", Label: "Base URL", Value: m.cfg.AI.OpenAI.BaseURL, Placeholder: "https://api.openai.com/v1/responses"},
		})
	case "gemini":
		return m.openForm("gemini", "Gemini Settings", "Configure Gemini API or Gemini CLI.", []formFieldSpec{
			{Key: "api_key", Label: "API Key", Value: m.cfg.AI.Gemini.APIKey, Secret: true, Placeholder: "AIza..."},
			{Key: "model", Label: "Model", Value: m.cfg.AI.Gemini.Model, Placeholder: "gemini-2.5-flash"},
			{Key: "cli", Label: "CLI Command", Value: m.cfg.AI.Gemini.CLICommand, Placeholder: "gemini --prompt %TASK%"},
		})
	case "claude":
		return m.openForm("claude", "Claude Settings", "Configure Claude access.", []formFieldSpec{
			{Key: "api_key", Label: "API Key", Value: m.cfg.AI.Claude.APIKey, Secret: true, Placeholder: "sk-ant-..."},
			{Key: "model", Label: "Model", Value: m.cfg.AI.Claude.Model, Placeholder: "claude-sonnet-4-20250514"},
			{Key: "base_url", Label: "Base URL", Value: m.cfg.AI.Claude.BaseURL, Placeholder: "https://api.anthropic.com/v1/messages"},
		})
	case "receiver_boot":
		return m.runSyncAction("Receiver boot flag updated", func() string {
			m.cfg.Receiver.AutoStart = !m.cfg.Receiver.AutoStart
			_ = config.Save(m.cfg)
			if m.cfg.Receiver.AutoStart {
				return "Auto-start flag enabled. Use the installer or `receiver install-service` to bind it to systemd."
			}
			return "Auto-start flag disabled."
		})
	case "doctor":
		return task("Environment doctor", func() (string, error) {
			checks := doctor.Run(m.cfg)
			return doctor.Render(checks), nil
		})
	case "back":
		return m.runSyncAction("Returned to dashboard", func() string {
			m.screen = screenDashboard
			m.menuCursor = 0
			return "Dashboard active."
		})
	case "clear_logs":
		return m.runSyncAction("Logs cleared", func() string {
			m.logs = []string{timeStamped("Activity log cleared.")}
			return "Log buffer reset."
		})
	case "quit":
		m.stopEmbeddedReceiver()
		return tea.Quit
	}
	return nil
}

func (m *model) healthAction() tea.Cmd {
	client := m.selectedClient()
	if receiverClient, ok := client.(executor.ReceiverClient); ok && receiverClient.BaseURL != "" && receiverClient.BaseURL != "local" {
		return task("Remote health check", func() (string, error) {
			payload, err := receiverClient.Health(context.Background())
			if err != nil {
				return "", err
			}
			raw, _ := json.MarshalIndent(payload, "", "  ")
			return string(raw), nil
		})
	}
	return task("Local health check", func() (string, error) {
		report, err := health.Collect()
		if err != nil {
			return "", err
		}
		return report.Render(), nil
	})
}

func (m *model) openForm(kind, title, description string, specs []formFieldSpec) tea.Cmd {
	fields := make([]formField, 0, len(specs))
	for _, spec := range specs {
		input := textinput.New()
		input.Placeholder = spec.Placeholder
		input.SetValue(spec.Value)
		input.CharLimit = 512
		if spec.Secret {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '•'
		}
		field := formField{
			Key:     spec.Key,
			Label:   spec.Label,
			Input:   input,
			Options: spec.Options,
		}
		if len(spec.Options) > 0 {
			field.OptionIndex = indexOf(spec.Options, spec.Value)
			if field.OptionIndex < 0 {
				field.OptionIndex = 0
				field.Input.SetValue(spec.Options[0])
			}
		}
		fields = append(fields, field)
	}
	m.form = &formState{
		Kind:        kind,
		Title:       title,
		Description: description,
		Fields:      fields,
		Focus:       0,
	}
	m.syncFormFocus()
	m.status = title + " opened."
	return nil
}

func (m *model) submitForm() tea.Cmd {
	values := map[string]string{}
	for _, field := range m.form.Fields {
		values[field.Key] = strings.TrimSpace(field.Input.Value())
	}
	kind := m.form.Kind
	m.form = nil
	switch kind {
	case "set_mode":
		return m.runSyncAction("Mode updated", func() string {
			mode := values["mode"]
			switch mode {
			case "self-use":
				m.cfg.Role = "self-use"
				m.client = m.local
			case "controller":
				m.cfg.Role = "controller"
				m.client = nil
			case "receiver":
				m.cfg.Role = "receiver"
				m.cfg.Receiver.Enabled = true
				m.client = m.local
			default:
				return "Unknown mode ignored."
			}
			_ = config.Save(m.cfg)
			return "Device mode switched to " + mode + "."
		})
	case "connect_receiver":
		address := values["address"]
		token := values["token"]
		client := executor.ReceiverClient{
			BaseURL: address,
			Token:   token,
			Name:    "receiver",
		}
		m.cfg.Profiles["last-receiver"] = config.Profile{
			Type:        "receiver",
			Address:     address,
			Token:       token,
			Description: "Last receiver used from the UI",
		}
		_ = config.Save(m.cfg)
		m.client = client
		return task("Receiver connection", func() (string, error) {
			payload, err := client.Health(context.Background())
			if err != nil {
				return "", err
			}
			raw, _ := json.MarshalIndent(payload, "", "  ")
			return "Connected.\n" + string(raw), nil
		})
	case "run_command":
		return m.manualCommand(values["command"], values["mode"], values["timeout"])
	case "ai_task":
		return m.aiTask(values["provider"], values["task"])
	case "root_password":
		return m.runSyncAction("Elevated access enabled", func() string {
			m.accessMode = "root"
			m.rootPassword = values["password"]
			return "Root access is active for this session."
		})
	case "active_ai":
		return m.runSyncAction("Active AI updated", func() string {
			m.cfg.AI.Active = values["provider"]
			m.planner.Config = m.cfg
			_ = config.Save(m.cfg)
			return "Default provider set to " + values["provider"]
		})
	case "openai":
		return m.runSyncAction("OpenAI settings saved", func() string {
			m.cfg.AI.OpenAI.APIKey = values["api_key"]
			m.cfg.AI.OpenAI.Model = fallback(values["model"], "gpt-4.1-mini")
			m.cfg.AI.OpenAI.BaseURL = values["base_url"]
			m.planner.Config = m.cfg
			_ = config.Save(m.cfg)
			return "ChatGPT configuration updated."
		})
	case "gemini":
		return m.runSyncAction("Gemini settings saved", func() string {
			m.cfg.AI.Gemini.APIKey = values["api_key"]
			m.cfg.AI.Gemini.Model = fallback(values["model"], "gemini-2.5-flash")
			m.cfg.AI.Gemini.CLICommand = values["cli"]
			m.cfg.AI.Gemini.UseCLI = values["cli"] != ""
			m.planner.Config = m.cfg
			_ = config.Save(m.cfg)
			return "Gemini configuration updated."
		})
	case "claude":
		return m.runSyncAction("Claude settings saved", func() string {
			m.cfg.AI.Claude.APIKey = values["api_key"]
			m.cfg.AI.Claude.Model = fallback(values["model"], "claude-sonnet-4-20250514")
			m.cfg.AI.Claude.BaseURL = values["base_url"]
			m.planner.Config = m.cfg
			_ = config.Save(m.cfg)
			return "Claude configuration updated."
		})
	}
	return nil
}

func (m *model) aiTask(provider, taskText string) tea.Cmd {
	return task("AI task", func() (string, error) {
		client := m.selectedClient()
		if client == nil {
			return "", fmt.Errorf("no execution target selected")
		}
		plan, err := m.planner.CreatePlan(context.Background(), provider, taskText, m.currentTarget())
		if err != nil {
			return "", err
		}
		lines := []string{
			"AI TASK REPORT",
			"",
			"Workflow:",
			"1) Send your natural-language goal to the selected AI provider.",
			"2) Receive a plan with explanation, warnings, and shell commands.",
			"3) Validate that a target is selected before command execution.",
			"4) Execute commands one by one and capture output for each step.",
			"",
			"Planner details:",
			"Provider: " + plan.Provider,
			"Explanation: " + plan.Explanation,
		}
		if len(plan.Warnings) > 0 {
			lines = append(lines, "Warnings:")
			for _, warning := range plan.Warnings {
				lines = append(lines, "- "+warning)
			}
		}
		if plan.Blocked || len(plan.Commands) == 0 {
			return strings.Join(lines, "\n"), nil
		}
		lines = append(lines, "")
		lines = append(lines, "Command execution:")
		for _, command := range plan.Commands {
			lines = append(lines, "> "+command)
			result, execErr := client.Execute(context.Background(), executor.Request{
				Command:      command,
				Mode:         m.accessMode,
				RootPassword: m.rootPassword,
				Timeout:      2 * time.Minute,
			})
			if execErr != nil {
				return strings.Join(lines, "\n"), execErr
			}
			lines = append(lines, result.CombinedOutput())
		}
		return strings.Join(lines, "\n"), nil
	})
}

func (m *model) manualCommand(commandText, modeValue, timeoutValue string) tea.Cmd {
	return task("Manual command", func() (string, error) {
		client := m.selectedClient()
		if client == nil {
			return "", fmt.Errorf("no execution target selected")
		}
		commandText = strings.TrimSpace(commandText)
		if commandText == "" {
			return "", fmt.Errorf("command is required")
		}

		mode := fallback(strings.TrimSpace(modeValue), "normal")
		if mode != "normal" && mode != "root" {
			mode = "normal"
		}
		timeoutSecs := 120
		if raw := strings.TrimSpace(timeoutValue); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err == nil && parsed > 0 {
				timeoutSecs = parsed
			}
		}
		if timeoutSecs > 900 {
			timeoutSecs = 900
		}

		lines := []string{
			"MANUAL COMMAND REPORT",
			"",
			"Workflow:",
			"1) Read your command exactly as typed.",
			"2) Resolve active target from current mode and connection.",
			"3) Build an execution request (command, access mode, timeout).",
		}
		switch client.(type) {
		case executor.LocalExecutor:
			lines = append(lines, "4) Local execution path selected.")
			if mode == "root" {
				lines = append(lines, "5) Run via sudo -S using your in-session password.")
			} else {
				lines = append(lines, "5) Run using configured shell on this device.")
			}
		case executor.ReceiverClient:
			if mode == "root" {
				mode = "normal"
				lines = append(lines, "4) Remote target does not allow root mode; forced to normal.")
			} else {
				lines = append(lines, "4) Remote execution path selected.")
			}
			lines = append(lines, "5) Send HTTPS/HTTP request to receiver /api/v1/exec with bearer token.")
		default:
			lines = append(lines, "4) Generic execution client selected.")
		}
		lines = append(lines, "6) Capture stdout, stderr, exit code, and duration.")
		lines = append(lines, "")
		lines = append(lines, "Request details:")
		lines = append(lines, "Target: "+client.Label())
		lines = append(lines, "Mode: "+mode)
		lines = append(lines, fmt.Sprintf("Timeout: %ds", timeoutSecs))
		lines = append(lines, "Command: "+commandText)

		result, err := client.Execute(context.Background(), executor.Request{
			Command:      commandText,
			Mode:         mode,
			RootPassword: m.rootPassword,
			Timeout:      time.Duration(timeoutSecs) * time.Second,
		})
		if err != nil {
			return strings.Join(lines, "\n"), err
		}

		lines = append(lines, "")
		lines = append(lines, "Result:")
		lines = append(lines, fmt.Sprintf("Exit code: %d", result.ExitCode))
		lines = append(lines, fmt.Sprintf("Duration: %s", result.Duration))
		if strings.TrimSpace(result.Stdout) != "" {
			lines = append(lines, "")
			lines = append(lines, "STDOUT:")
			lines = append(lines, result.Stdout)
		}
		if strings.TrimSpace(result.Stderr) != "" {
			lines = append(lines, "")
			lines = append(lines, "STDERR:")
			lines = append(lines, result.Stderr)
		}
		if strings.TrimSpace(result.Stdout) == "" && strings.TrimSpace(result.Stderr) == "" {
			lines = append(lines, "")
			lines = append(lines, "(No output returned)")
		}
		return strings.Join(lines, "\n"), nil
	})
}

func (m *model) syncFormFocus() {
	for i := range m.form.Fields {
		if i == m.form.Focus {
			m.form.Fields[i].Input.Focus()
		} else {
			m.form.Fields[i].Input.Blur()
		}
	}
}

func (m *model) stopEmbeddedReceiver() {
	if !m.receiverRunning || m.receiverServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = m.receiverServer.Shutdown(ctx)
	m.receiverRunning = false
	m.receiverServer = nil
}

func (m *model) actions() []menuAction {
	switch m.screen {
	case screenSettings:
		return []menuAction{
			{ID: "active_ai", Title: "Choose Default AI", Description: "Set the assistant used by app actions."},
			{ID: "openai", Title: "ChatGPT Settings", Description: "Save OpenAI API key, model and endpoint."},
			{ID: "gemini", Title: "Gemini Settings", Description: "Save Gemini API key or Gemini CLI command."},
			{ID: "claude", Title: "Claude Settings", Description: "Save Claude API key, model and endpoint."},
			{ID: "receiver_boot", Title: "Receiver Boot Flag", Description: "Mark this device for receiver auto-start."},
			{ID: "receiver_toggle", Title: "Start/Stop Embedded Receiver", Description: "Run receiver listener from inside UI."},
			{ID: "toggle_access", Title: "Toggle Root Access", Description: "Switch between normal and root mode."},
			{ID: "toggle_layout", Title: "Toggle Layout", Description: "Switch between horizontal and vertical panels."},
			{ID: "doctor", Title: "Run Doctor", Description: "Check dependencies and environment health."},
			{ID: "back", Title: "Back To Home", Description: "Return to the simplified dashboard."},
		}
	case screenLogs:
		return []menuAction{
			{ID: "clear_logs", Title: "Clear Activity Log", Description: "Reset the visible log buffer."},
			{ID: "back", Title: "Back To Home", Description: "Return to the simplified dashboard."},
		}
	case screenHelp:
		return []menuAction{
			{ID: "back", Title: "Back To Home", Description: "Return to the simplified dashboard."},
		}
	default:
		return []menuAction{
			{ID: "quick_start", Title: "Start Here (Guided)", Description: "Open beginner workflow and full usage guide."},
			{ID: "set_mode", Title: "Choose Device Mode", Description: "Pick self-use, controller, or receiver."},
			{ID: "connect_local", Title: "Use This Device", Description: "Execute commands on this machine."},
			{ID: "connect_receiver", Title: "Connect Remote Receiver", Description: "Connect to another device by URL + token."},
			{ID: "disconnect_remote", Title: "Disconnect Remote Target", Description: "Stop sending commands to receiver."},
			{ID: "run_command", Title: "Run Command", Description: "Type a command and see detailed execution report."},
			{ID: "ai_task", Title: "Ask AI To Do A Task", Description: "Give a plain-English goal and run planned commands."},
			{ID: "health", Title: "Check Device Health", Description: "See CPU, memory, disk, uptime and status."},
			{ID: "tool_catalog", Title: "Show Tool Catalog", Description: "List available tools like git, docker, nmap."},
			{ID: "settings", Title: "Open Settings", Description: "Manage AI providers and receiver startup."},
			{ID: "logs", Title: "Open Logs", Description: "Inspect activity and execution history."},
			{ID: "help", Title: "Open Guide", Description: "Read copy/paste basics through nmap workflows."},
			{ID: "quit", Title: "Quit", Description: "Exit the application."},
		}
	}
}

func (m *model) renderMenu(width, height int) string {
	actions := m.actions()
	rows := make([][]string, 0, len(actions))
	for i, action := range actions {
		marker := " "
		if i == m.menuCursor {
			marker = ">"
		}
		rows = append(rows, []string{marker + " " + action.Title, action.Description})
	}
	inner := max(20, width-6)
	usable := max(14, inner-7)
	actionCol, descCol := splitColumns(usable, 10, 22)
	table := renderSheet([]string{"Action", "Description"}, rows, []int{actionCol, descCol})
	content := fitBlock("ACTIONS\n"+table, max(20, width-6), max(4, height-2))
	return panelStyle(width).Height(height).Render(content)
}

func (m *model) renderDetail(width, height int) string {
	var body string
	innerWidth := max(20, width-6)
	switch m.screen {
	case screenSettings:
		body = m.settingsView(innerWidth)
	case screenLogs:
		body = m.renderLogs(height-2, innerWidth)
	case screenHelp:
		body = m.helpView(innerWidth)
	default:
		body = m.dashboardView(innerWidth)
	}
	content := fitBlock(body, innerWidth, max(4, height-2))
	return panelStyle(width).Height(height).Render(content)
}

func (m *model) dashboardView(innerWidth int) string {
	usable := max(20, innerWidth-7)
	flowStepCol, flowActionCol := splitColumns(usable, 6, 10)
	stateFieldCol, stateValueCol := splitColumns(usable, 14, 24)
	cmdFieldCol, cmdValueCol := splitColumns(usable, 14, 24)

	flowTable := renderSheet(
		[]string{"Step", "Action"},
		[][]string{
			{"1", "Start Here (Guided)"},
			{"2", "Choose Device Mode = self-use"},
			{"3", "Run Command: pwd"},
			{"4", "Check Device Health"},
			{"5", "Ask AI To Do A Task"},
			{"6", "Open Logs"},
		},
		[]int{flowStepCol, flowActionCol},
	)
	stateTable := renderSheet(
		[]string{"Field", "Value"},
		[][]string{
			{"Mode", renderRole(m.cfg.Role)},
			{"Target", m.currentTarget()},
			{"AI", strings.ToUpper(m.cfg.AI.Active)},
			{"Access", m.accessMode},
			{"Receiver", fmt.Sprintf("%s:%d", m.cfg.Receiver.Host, m.cfg.Receiver.Port)},
			{"Receiver Running", fmt.Sprintf("%t", m.receiverRunning)},
		},
		[]int{stateFieldCol, stateValueCol},
	)
	commandsTable := renderSheet(
		[]string{"Starter Command", "Purpose"},
		[][]string{
			{"pwd", "Show current folder"},
			{"ls -la", "List files and permissions"},
			{"whoami", "Show current user"},
			{"ip a", "Show local network interfaces"},
		},
		[]int{cmdFieldCol, cmdValueCol},
	)
	lines := []string{
		"HOME",
		"Beginner-first control panel. Follow the workflow from top to bottom.",
		"",
		"QUICK START FLOW",
		flowTable,
		"",
		"CURRENT STATE",
		stateTable,
		"",
		"STARTER COMMANDS",
		commandsTable,
		"",
		"Note: Use network tools like nmap only on systems/networks you own or are explicitly authorized to test.",
	}
	return strings.Join(lines, "\n")
}

func (m *model) settingsView(innerWidth int) string {
	usable := max(20, innerWidth-7)
	nameCol, valueCol := splitColumns(usable, 18, 28)
	settingsTable := renderSheet(
		[]string{"Setting", "Value"},
		[][]string{
			{"Default AI provider", strings.ToUpper(m.cfg.AI.Active)},
			{"OpenAI configured", fmt.Sprintf("%t", m.cfg.AI.OpenAI.APIKey != "")},
			{"Gemini configured", fmt.Sprintf("%t", m.cfg.AI.Gemini.APIKey != "" || m.cfg.AI.Gemini.CLICommand != "")},
			{"Claude configured", fmt.Sprintf("%t", m.cfg.AI.Claude.APIKey != "")},
			{"Gemini CLI command", fallback(m.cfg.AI.Gemini.CLICommand, "not set")},
			{"Receiver boot flag", fmt.Sprintf("%t", m.cfg.Receiver.AutoStart)},
			{"Receiver token set", fmt.Sprintf("%t", m.cfg.Receiver.Token != "")},
		},
		[]int{nameCol, valueCol},
	)
	lines := []string{
		"SETTINGS",
		"Everything here is optional. Start using the app before tuning advanced settings.",
		"",
		settingsTable,
		"",
		"NOTES",
		"Use left menu actions to edit credentials and startup behavior.",
		"Root mode is local-only in this build; remote root is blocked by design.",
		"API keys are stored in local config on this workstation.",
	}
	return strings.Join(lines, "\n")
}

func (m *model) helpView(_ int) string {
	lines := []string{
		"BEGINNER GUIDE (BASIC TO ADVANCED)",
		"",
		"A) BASICS: KEYBOARD + COPY/PASTE",
		"1) Move selection: Up/Down arrows (or J/K).",
		"2) Run highlighted action: Enter.",
		"3) Switch screen: Tab / Shift+Tab.",
		"4) Close dialog or return home: Esc.",
		"5) Quit app: Q or Ctrl+C.",
		"6) Copy text from terminal: mouse-select then Ctrl+Shift+C (or terminal copy shortcut).",
		"7) Paste text into dialog fields: Ctrl+Shift+V (or terminal paste shortcut).",
		"",
		"B) FIRST 10 MINUTES WORKFLOW",
		"1) Choose 'Choose Device Mode' -> select 'self-use'.",
		"2) Select 'Use This Device'.",
		"3) Open 'Run Command' and run: pwd",
		"4) Run again with: ls -la",
		"5) Open 'Check Device Health'.",
		"6) Open Logs and confirm each operation result.",
		"",
		"C) RUNNING COMMANDS FROM THE APP",
		"Use action: Run Command",
		"Field 1 Command: exact shell command, for example whoami",
		"Field 2 Access Mode: normal (recommended) or root (local only).",
		"Field 3 Timeout: max runtime in seconds.",
		"After Enter, the app prints a full execution report:",
		"- selected target",
		"- request details",
		"- internal execution path",
		"- stdout/stderr",
		"- exit code and duration",
		"",
		"D) HOW OPERATIONS WORK INTERNALLY",
		"Use This Device:",
		"Selects local executor and routes future requests to local shell.",
		"Connect Remote Receiver:",
		"Saves receiver URL/token profile, performs health check, then routes commands to receiver API.",
		"Run Command:",
		"Builds executor request {command, mode, timeout}.",
		"Local path runs shell command directly (or sudo for root mode).",
		"Remote path sends POST /api/v1/exec with bearer token.",
		"Ask AI To Do A Task:",
		"Sends natural language prompt to configured provider, receives command plan, executes commands sequentially, stores output in logs.",
		"Check Device Health:",
		"Runs local health collectors or remote health endpoint based on target.",
		"",
		"E) AUTHORIZED NMAP WORKFLOW (DEFENSIVE USE)",
		"Important: scan only systems/networks you own or have explicit permission to test.",
		"Start with discovery on your own LAN:",
		"nmap -sn 192.168.1.0/24",
		"Service/version scan on approved host:",
		"nmap -sV 192.168.1.10",
		"Safe local scan example:",
		"nmap -sV 127.0.0.1",
		"In this app: open 'Run Command', paste the nmap command, run in normal mode, read report and logs.",
		"",
		"F) MODE DEFINITIONS",
		"self-use: commands run on this machine.",
		"controller: connect and manage remote receiver only.",
		"receiver: this machine can act as a remote execution target.",
	}
	return strings.Join(lines, "\n")
}

func (m *model) renderLogs(maxRows, width int) string {
	if maxRows < 2 {
		maxRows = 2
	}
	lines := append([]string{"ACTIVITY LOG"}, tail(m.logs, maxRows-1)...)
	if len(lines) == 1 {
		lines = append(lines, "No activity yet.")
	}
	return fitBlock(strings.Join(lines, "\n"), max(20, width), maxRows)
}

func (m *model) renderForm(width, height int) string {
	lines := []string{
		strings.ToUpper(m.form.Title),
		m.form.Description,
	}
	for i, field := range m.form.Fields {
		label := field.Label
		if i == m.form.Focus {
			label = focusStyle().Render(label)
		}
		if len(field.Options) > 0 {
			label += " (left/right to change)"
		}
		lines = append(lines, label)
		lines = append(lines, field.Input.View())
	}
	lines = append(lines, "Enter submits the last field. Esc cancels.")
	content := fitBlock(strings.Join(lines, "\n"), max(20, width-6), max(4, height-2))
	return panelStyle(width).Height(height).Render(content)
}

func (m *model) setupView() string {
	options := []string{"Controller", "Receiver", "Self Use"}
	lines := []string{
		titleStyle().Render("UNIVERSAL CONTROLLER"),
		"Choose how this device should behave on first launch.",
		"",
	}
	for i, option := range options {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.setupCursor {
			prefix = "› "
			style = style.Foreground(lipgloss.Color("205")).Bold(true)
		}
		lines = append(lines, style.Render(prefix+option))
	}
	lines = append(lines, "", mutedStyle().Render("Receiver mode is designed for boot-time service startup."))
	return centeredStyle().Render(strings.Join(lines, "\n"))
}

func (m *model) runSyncAction(title string, apply func() string) tea.Cmd {
	body := apply()
	return task(title, func() (string, error) { return body, nil })
}

func (m *model) selectedClient() executor.Client {
	if m.client != nil {
		return m.client
	}
	if m.cfg.Role == "self-use" || m.cfg.Role == "receiver" {
		return m.local
	}
	return nil
}

func (m *model) currentTarget() string {
	client := m.selectedClient()
	if client == nil {
		return "none"
	}
	return client.Label()
}

func (m *model) lastReceiverAddress() string {
	if profile, ok := m.cfg.Profiles["last-receiver"]; ok {
		return profile.Address
	}
	return config.ReceiverAddress(m.cfg)
}

func (m *model) lastReceiverToken() string {
	if profile, ok := m.cfg.Profiles["last-receiver"]; ok {
		return profile.Token
	}
	return m.cfg.Receiver.Token
}

func (m *model) panelWidth() int {
	if m.width <= 0 {
		return 80
	}
	if m.cfg.UI.Layout == "vertical" {
		return max(60, m.width-4)
	}
	return max(60, (m.width/2)-3)
}

func nextScreen(current screen) screen {
	order := []screen{screenDashboard, screenSettings, screenLogs, screenHelp}
	for i, item := range order {
		if item == current {
			return order[(i+1)%len(order)]
		}
	}
	return screenDashboard
}

func previousScreen(current screen) screen {
	order := []screen{screenDashboard, screenSettings, screenLogs, screenHelp}
	for i, item := range order {
		if item == current {
			index := i - 1
			if index < 0 {
				index = len(order) - 1
			}
			return order[index]
		}
	}
	return screenDashboard
}

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
}

func mutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
}

func focusStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
}

func headerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24"))
}

func footerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("238"))
}

func panelStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Width(width)
}

func centeredStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(2, 4)
}

func normalizeLayout(value string) string {
	if value != "vertical" {
		return "horizontal"
	}
	return value
}

func timeStamped(line string) string {
	return time.Now().Format("15:04:05") + "  " + line
}

func task(title string, fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		body, err := fn()
		return taskFinishedMsg{Title: title, Body: body, Err: err}
	}
}

func tail(lines []string, size int) []string {
	if len(lines) <= size {
		return lines
	}
	return lines[len(lines)-size:]
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

func renderSheet(headers []string, rows [][]string, widths []int) string {
	if len(headers) == 0 {
		return ""
	}
	normalized := make([]int, len(headers))
	for i := range headers {
		w := 12
		if i < len(widths) && widths[i] > 0 {
			w = widths[i]
		}
		normalized[i] = max(6, w)
	}

	border := sheetBorder(normalized)
	lines := []string{border, sheetRow(headers, normalized), border}
	for _, row := range rows {
		lines = append(lines, sheetRow(row, normalized))
	}
	lines = append(lines, border)
	return strings.Join(lines, "\n")
}

func sheetBorder(widths []int) string {
	parts := make([]string, len(widths))
	for i, width := range widths {
		parts[i] = strings.Repeat("-", width+2)
	}
	return "+" + strings.Join(parts, "+") + "+"
}

func sheetRow(cells []string, widths []int) string {
	parts := make([]string, len(widths))
	for i, width := range widths {
		value := ""
		if i < len(cells) {
			value = cells[i]
		}
		parts[i] = " " + padOrTrim(value, width) + " "
	}
	return "|" + strings.Join(parts, "|") + "|"
}

func fitBlock(text string, width, height int) string {
	if width < 4 || height < 2 {
		return text
	}

	lines := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		lines = append(lines, wrapLine(line, width)...)
	}

	clipped := false
	if len(lines) > height {
		clipped = true
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = padOrTrim(lines[i], width)
	}
	if clipped && len(lines) > 0 {
		lines[len(lines)-1] = padOrTrim(lines[len(lines)-1], max(3, width-3)) + "..."
	}
	return strings.Join(lines, "\n")
}

func wrapLine(line string, width int) []string {
	if width < 2 {
		return []string{line}
	}
	if strings.TrimSpace(line) == "" {
		return []string{""}
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}

	out := []string{}
	current := words[0]
	for _, word := range words[1:] {
		if runeLen(current)+1+runeLen(word) <= width {
			current += " " + word
			continue
		}
		out = append(out, current)
		current = word
	}
	out = append(out, current)

	expanded := []string{}
	for _, row := range out {
		runes := []rune(row)
		if len(runes) <= width {
			expanded = append(expanded, row)
			continue
		}
		for len(runes) > width {
			expanded = append(expanded, string(runes[:width]))
			runes = runes[width:]
		}
		if len(runes) > 0 {
			expanded = append(expanded, string(runes))
		}
	}
	return expanded
}

func padOrTrim(value string, width int) string {
	if width <= 0 {
		return ""
	}
	clean := strings.ReplaceAll(value, "\t", " ")
	runes := []rune(clean)
	if len(runes) > width {
		if width <= 3 {
			return string(runes[:width])
		}
		return string(runes[:width-3]) + "..."
	}
	if len(runes) < width {
		return clean + strings.Repeat(" ", width-len(runes))
	}
	return clean
}

func runeLen(text string) int {
	return len([]rune(text))
}

func renderRole(role string) string {
	switch role {
	case "self-use":
		return "self"
	case "controller":
		return "controller"
	case "receiver":
		return "receiver"
	default:
		return "unconfigured"
	}
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func splitColumns(total, leftMin, leftMax int) (int, int) {
	if total < 2 {
		return 1, 1
	}
	left := clamp(total/3, leftMin, leftMax)
	right := total - left
	if right < 6 {
		right = 6
		left = max(1, total-right)
	}
	return left, right
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
