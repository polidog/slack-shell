package shell

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-tui/internal/slack"
)

var (
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	outputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	tailStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	newMsgStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

// Model is the Bubble Tea model for the shell UI
type Model struct {
	client         *slack.Client
	realtimeClient *slack.RealtimeClient
	executor       *Executor
	input          textinput.Model
	history        []string
	historyIndex   int
	commandHistory []string
	width          int
	height         int
	ready          bool
	tailMode       bool
}

// NewModel creates a new shell model
func NewModel(client *slack.Client) *Model {
	ti := textinput.New()
	ti.Prompt = promptStyle.Render("slack> ")
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 80

	return &Model{
		client:         client,
		executor:       NewExecutor(client),
		input:          ti,
		history:        []string{},
		historyIndex:   -1,
		commandHistory: []string{},
	}
}

// SetRealtimeClient sets the realtime client for receiving messages
func (m *Model) SetRealtimeClient(rc *slack.RealtimeClient) {
	m.realtimeClient = rc
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	// Show welcome message and run initial ls
	m.history = append(m.history, "Welcome to Slack TUI (Shell Mode)")
	m.history = append(m.history, "Type 'help' for available commands.\n")

	return textinput.Blink
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle tail mode key events
		if m.tailMode {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				m.tailMode = false
				m.history = append(m.history, tailStyle.Render("Stopped tailing."))
				m.input.Focus()
				return m, nil
			}
			switch msg.String() {
			case "q":
				m.tailMode = false
				m.history = append(m.history, tailStyle.Render("Stopped tailing."))
				m.input.Focus()
				return m, nil
			}
			return m, nil
		}

		// Normal mode key handling
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			return m.executeCommand()

		case tea.KeyUp:
			return m.navigateHistory(-1)

		case tea.KeyDown:
			return m.navigateHistory(1)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 10
		m.ready = true

	case IncomingMessageMsg:
		output := m.executor.HandleIncomingMessage(slack.IncomingMessage(msg))
		if output != "" {
			if m.tailMode {
				m.history = append(m.history, newMsgStyle.Render(output))
			} else {
				m.history = append(m.history, output)
			}
		}
		return m, nil
	}

	if !m.tailMode {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

func (m *Model) executeCommand() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.input.Value())

	// Add to history display
	m.history = append(m.history, m.executor.GetPrompt()+input)

	if input != "" {
		// Add to command history
		m.commandHistory = append(m.commandHistory, input)
		m.historyIndex = len(m.commandHistory)

		// Parse and execute
		cmd := ParseCommand(input)

		// Handle tail command specially
		if cmd.Type == CmdTail {
			return m.startTailMode(cmd)
		}

		result := m.executor.Execute(cmd)

		if result.Exit {
			return m, tea.Quit
		}

		if result.Error != nil {
			m.history = append(m.history, errorStyle.Render(FormatError(result.Error)))
		} else if result.Output != "" {
			m.history = append(m.history, outputStyle.Render(result.Output))
		}
	}

	// Update prompt
	m.input.SetValue("")
	m.input.Prompt = promptStyle.Render(m.executor.GetPrompt())

	return m, nil
}

func (m *Model) startTailMode(cmd Command) (tea.Model, tea.Cmd) {
	if m.executor.GetCurrentChannel() == nil {
		m.history = append(m.history, errorStyle.Render("Not in a channel. Use 'cd #channel' first."))
		m.input.SetValue("")
		return m, nil
	}

	if m.realtimeClient == nil {
		m.history = append(m.history, errorStyle.Render("Real-time connection not available. Set SLACK_APP_TOKEN to enable."))
		m.input.SetValue("")
		return m, nil
	}

	// Show initial messages first
	catCmd := Command{Type: CmdCat, Flags: map[string]string{}}
	if nFlag, ok := cmd.Flags["n"]; ok && nFlag != "" {
		catCmd.Flags["n"] = nFlag
	} else {
		catCmd.Flags["n"] = "10"
	}
	result := m.executor.Execute(catCmd)
	if result.Output != "" {
		m.history = append(m.history, outputStyle.Render(result.Output))
	}

	m.tailMode = true
	m.input.Blur()
	m.history = append(m.history, tailStyle.Render("Tailing messages... (press 'q' or Ctrl+C to stop)"))
	m.input.SetValue("")

	return m, nil
}

func (m *Model) navigateHistory(direction int) (tea.Model, tea.Cmd) {
	if len(m.commandHistory) == 0 {
		return m, nil
	}

	newIndex := m.historyIndex + direction

	if newIndex < 0 {
		newIndex = 0
	} else if newIndex >= len(m.commandHistory) {
		// Past the end - clear input
		m.historyIndex = len(m.commandHistory)
		m.input.SetValue("")
		return m, nil
	}

	m.historyIndex = newIndex
	m.input.SetValue(m.commandHistory[m.historyIndex])
	m.input.CursorEnd()

	return m, nil
}

// View renders the UI
func (m *Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	var sb strings.Builder

	// Calculate how many history lines we can show
	availableHeight := m.height - 2 // Reserve space for input and padding

	// Get the history lines to display
	historyLines := []string{}
	for _, h := range m.history {
		lines := strings.Split(h, "\n")
		historyLines = append(historyLines, lines...)
	}

	// Show only the last N lines that fit
	startIdx := 0
	if len(historyLines) > availableHeight {
		startIdx = len(historyLines) - availableHeight
	}

	for i := startIdx; i < len(historyLines); i++ {
		sb.WriteString(historyLines[i])
		sb.WriteString("\n")
	}

	// Add input line or tail mode indicator
	if m.tailMode {
		sb.WriteString(tailStyle.Render(">>> Watching for new messages (q to quit) <<<"))
	} else {
		sb.WriteString(m.input.View())
	}

	return sb.String()
}

// HandleRealtimeEvent handles events from the realtime client
func (m *Model) HandleRealtimeEvent(event interface{}) tea.Cmd {
	switch e := event.(type) {
	case slack.IncomingMessage:
		return func() tea.Msg {
			return IncomingMessageMsg(e)
		}
	case string:
		if e == "connected" {
			return func() tea.Msg {
				return ConnectionStatusMsg{Connected: true}
			}
		} else if e == "disconnected" {
			return func() tea.Msg {
				return ConnectionStatusMsg{Connected: false}
			}
		}
	}
	return nil
}

// IncomingMessageMsg is a message type for incoming Slack messages
type IncomingMessageMsg slack.IncomingMessage

// ConnectionStatusMsg is a message type for connection status changes
type ConnectionStatusMsg struct {
	Connected bool
}
