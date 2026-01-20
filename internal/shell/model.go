package shell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-tui/internal/notification"
	"github.com/polidog/slack-tui/internal/slack"
)

var (
	promptStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	outputStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	tailStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	newMsgStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	notificationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("4")).
				Padding(0, 1)
)

// Model is the Bubble Tea model for the shell UI
type Model struct {
	client              *slack.Client
	realtimeClient      *slack.RealtimeClient
	notificationManager *notification.Manager
	executor            *Executor
	input               textinput.Model
	history             []string
	historyIndex        int
	commandHistory      []string
	width               int
	height              int
	ready               bool
	tailMode            bool
}

// NewModel creates a new shell model
func NewModel(client *slack.Client, notifyMgr *notification.Manager) *Model {
	ti := textinput.New()
	ti.Prompt = promptStyle.Render("slack> ")
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 80

	return &Model{
		client:              client,
		notificationManager: notifyMgr,
		executor:            NewExecutor(client),
		input:               ti,
		history:             []string{},
		historyIndex:        -1,
		commandHistory:      []string{},
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
		slackMsg := slack.IncomingMessage(msg)
		output := m.executor.HandleIncomingMessage(slackMsg)
		if output != "" {
			if m.tailMode {
				m.history = append(m.history, newMsgStyle.Render(output))
			} else {
				m.history = append(m.history, output)
			}
		}

		// Trigger notifications for messages from other channels
		if m.notificationManager != nil {
			currentChannelID := ""
			currentChannel := m.executor.GetCurrentChannel()
			if currentChannel != nil {
				currentChannelID = currentChannel.ID
			}

			// Get channel and user info for notification
			channelName := m.executor.GetChannelName(slackMsg.ChannelID)
			userName := m.executor.GetUserName(slackMsg.UserID)
			isMention := m.executor.IsMentionedInMessage(slackMsg.Text)

			notifyMsg := notification.Message{
				ChannelID:   slackMsg.ChannelID,
				ChannelName: channelName,
				UserName:    userName,
				Text:        slackMsg.Text,
				IsMention:   isMention,
				IsIM:        m.executor.IsIMChannel(slackMsg.ChannelID),
			}

			m.notificationManager.HandleMessage(notifyMsg, currentChannelID, m.tailMode)
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

		var result ExecuteResult
		var parsedCmd Command

		// Check if this is a pipeline
		if IsPipeline(input) {
			pipeline := ParsePipeline(input)
			result = m.executor.ExecutePipeline(pipeline)
		} else {
			// Parse and execute single command
			parsedCmd = ParseCommand(input)

			// Handle tail command specially
			if parsedCmd.Type == CmdTail {
				return m.startTailMode(parsedCmd)
			}

			result = m.executor.Execute(parsedCmd)
		}

		if result.Exit {
			return m, tea.Quit
		}

		if result.Error != nil {
			m.history = append(m.history, errorStyle.Render(FormatError(result.Error)))
		} else if result.SwitchWorkspace != nil {
			// Handle workspace switch
			m.client = result.SwitchWorkspace.Client
			m.executor.SwitchClient(result.SwitchWorkspace.Client)
			m.history = append(m.history, outputStyle.Render(
				"Switched to workspace: "+result.SwitchWorkspace.TeamName))
		} else if result.Output != "" {
			m.history = append(m.history, outputStyle.Render(result.Output))

			// Clear unread notifications when entering a channel
			if parsedCmd.Type == CmdCd && m.notificationManager != nil {
				currentChannel := m.executor.GetCurrentChannel()
				if currentChannel != nil {
					m.notificationManager.ClearUnread(currentChannel.ID)
				}
			}
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

	// Render visual notifications at the top if any
	notificationArea := m.renderNotifications()
	notificationLines := 0
	if notificationArea != "" {
		sb.WriteString(notificationArea)
		sb.WriteString("\n")
		notificationLines = strings.Count(notificationArea, "\n") + 2
	}

	// Calculate how many history lines we can show
	availableHeight := m.height - 2 - notificationLines // Reserve space for input, padding and notifications

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

// renderNotifications renders the visual notification area
func (m *Model) renderNotifications() string {
	if m.notificationManager == nil {
		return ""
	}

	notifications := m.notificationManager.GetVisualNotifications()
	if len(notifications) == 0 {
		return ""
	}

	var lines []string
	for _, n := range notifications {
		var prefix string
		if n.IsIM {
			prefix = fmt.Sprintf("@%s", n.ChannelName)
		} else {
			prefix = fmt.Sprintf("#%s", n.ChannelName)
		}

		// Truncate message if too long
		text := n.Text
		if len(text) > 50 {
			text = text[:47] + "..."
		}

		line := fmt.Sprintf("%s | %s: %s", prefix, n.UserName, text)
		lines = append(lines, notificationStyle.Render(line))
	}

	return strings.Join(lines, "\n")
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
