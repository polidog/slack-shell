package shell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-shell/internal/cache"
	"github.com/polidog/slack-shell/internal/config"
	"github.com/polidog/slack-shell/internal/notification"
	"github.com/polidog/slack-shell/internal/slack"
)

var (
	promptStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	outputStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	modeStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
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

	// Browse mode
	browseMode  bool
	browseModel *BrowseModel

	// Live mode
	liveMode  bool
	liveModel *LiveModel

	// Tab completion
	completionCandidates []string
	completionIndex      int
	completionActive     bool
	originalInput        string

	// Startup config
	startupConfig *config.StartupConfig
}

// NewModel creates a new shell model
func NewModel(client *slack.Client, notifyMgr *notification.Manager, promptConfig *config.PromptConfig, startupConfig *config.StartupConfig, hasAppToken bool) *Model {
	executor := NewExecutor(client, promptConfig, hasAppToken)

	ti := textinput.New()
	ti.Prompt = promptStyle.Render(executor.GetPrompt())
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 80

	return &Model{
		client:              client,
		notificationManager: notifyMgr,
		executor:            executor,
		input:               ti,
		history:             []string{},
		historyIndex:        -1,
		commandHistory:      []string{},
		startupConfig:       startupConfig,
	}
}

// SetRealtimeClient sets the realtime client for receiving messages
func (m *Model) SetRealtimeClient(rc *slack.RealtimeClient) {
	m.realtimeClient = rc
}

// SetUserCache sets the user cache for the executor
func (m *Model) SetUserCache(userCache *cache.UserCache) {
	m.executor.SetUserCache(userCache)
}

// SaveUserCache saves the user cache to disk
func (m *Model) SaveUserCache() error {
	return m.executor.SaveCache()
}

// GetExecutor returns the executor (for cache access)
func (m *Model) GetExecutor() *Executor {
	return m.executor
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	// Show startup banner or message
	workspaceName := m.executor.GetWorkspaceName()

	if m.startupConfig != nil && m.startupConfig.Banner != "" {
		// Show multi-line banner
		banner := strings.ReplaceAll(m.startupConfig.Banner, "{workspace}", workspaceName)
		for _, line := range strings.Split(strings.TrimSuffix(banner, "\n"), "\n") {
			m.history = append(m.history, line)
		}
	} else if m.startupConfig != nil && m.startupConfig.Message != "" {
		// Show single line message
		message := strings.ReplaceAll(m.startupConfig.Message, "{workspace}", workspaceName)
		m.history = append(m.history, message)
	} else {
		// Default message
		m.history = append(m.history, fmt.Sprintf("Welcome to Slack Shell - %s", workspaceName))
	}
	m.history = append(m.history, "Type 'help' for available commands.\n")

	// Execute init commands
	if m.startupConfig != nil && len(m.startupConfig.InitCommands) > 0 {
		for _, cmdStr := range m.startupConfig.InitCommands {
			m.history = append(m.history, promptStyle.Render(m.executor.GetPrompt())+cmdStr)
			pipeline := ParsePipeline(cmdStr)
			result := m.executor.ExecutePipeline(pipeline)
			if result.Output != "" {
				m.history = append(m.history, result.Output)
			}
			if result.Error != nil {
				m.history = append(m.history, errorStyle.Render(fmt.Sprintf("Error: %v", result.Error)))
			}
		}
		// Update prompt after init commands
		m.input.Prompt = promptStyle.Render(m.executor.GetPrompt())
	}

	return textinput.Blink
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle live mode key events
		if m.liveMode {
			// Check for exit condition first
			if m.liveModel.ShouldExit(msg) {
				m.liveMode = false
				m.liveModel = nil
				m.history = append(m.history, modeStyle.Render("Exited live mode."))
				m.input.Focus()
				return m, nil
			}
			m.liveModel, cmd = m.liveModel.Update(msg)
			return m, cmd
		}

		// Handle browse mode key events
		if m.browseMode {
			// Check for exit condition first
			if m.browseModel.ShouldExit(msg) {
				m.browseMode = false
				m.browseModel = nil
				m.history = append(m.history, modeStyle.Render("Exited browse mode."))
				m.input.Focus()
				return m, nil
			}
			m.browseModel, cmd = m.browseModel.Update(msg)
			return m, cmd
		}

		// Normal mode key handling
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyCtrlL:
			m.history = nil
			return m, tea.Batch(tea.ClearScreen, tea.WindowSize())

		case tea.KeyEnter:
			m.resetCompletion()
			return m.executeCommand()

		case tea.KeyUp:
			m.resetCompletion()
			return m.navigateHistory(-1)

		case tea.KeyDown:
			m.resetCompletion()
			return m.navigateHistory(1)

		case tea.KeyTab:
			return m.handleTabCompletion()

		default:
			// Reset completion on any other key
			if m.completionActive {
				m.resetCompletion()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 10
		m.ready = true
		// Update live model dimensions if active
		if m.liveMode && m.liveModel != nil {
			m.liveModel, cmd = m.liveModel.Update(msg)
			return m, cmd
		}
		// Update browse model dimensions if active
		if m.browseMode && m.browseModel != nil {
			m.browseModel, cmd = m.browseModel.Update(msg)
			return m, cmd
		}

	// Handle live mode messages
	case LiveMessagesLoadedMsg, LiveThreadLoadedMsg, LiveMessageSentMsg, LiveReplySentMsg, LiveOlderMessagesLoadedMsg:
		if m.liveMode && m.liveModel != nil {
			m.liveModel, cmd = m.liveModel.Update(msg)
			return m, cmd
		}

	// Handle browse mode messages
	case MessagesLoadedMsg, ThreadLoadedMsg, ReplySentMsg:
		if m.browseMode && m.browseModel != nil {
			m.browseModel, cmd = m.browseModel.Update(msg)
			return m, cmd
		}

	case IncomingMessageMsg:
		slackMsg := slack.IncomingMessage(msg)
		userName := m.executor.GetUserName(slackMsg.UserID)

		// Handle live mode - add message to live view
		if m.liveMode && m.liveModel != nil {
			m.liveModel.AddIncomingMessage(
				slackMsg.ChannelID,
				slackMsg.UserID,
				userName,
				slackMsg.Text,
				slackMsg.Timestamp,
				slackMsg.ThreadTS,
			)
		}

		// Handle browse mode - add message to browse view
		if m.browseMode && m.browseModel != nil {
			m.browseModel.AddIncomingMessage(
				slackMsg.ChannelID,
				slackMsg.UserID,
				userName,
				slackMsg.Text,
				slackMsg.Timestamp,
				slackMsg.ThreadTS,
			)
		}

		// Trigger notifications for messages from other channels (skip self messages)
		if m.notificationManager != nil && slackMsg.UserID != m.executor.GetCurrentUserID() {
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

			m.notificationManager.HandleMessage(notifyMsg, currentChannelID, m.browseMode || m.liveMode)
		}
		return m, nil
	}

	if !m.browseMode && !m.liveMode {
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

			// Handle browse command specially
			if parsedCmd.Type == CmdBrowse {
				return m.startBrowseMode(parsedCmd)
			}

			// Handle live command specially
			if parsedCmd.Type == CmdLive {
				return m.startLiveMode(parsedCmd)
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

func (m *Model) startBrowseMode(cmd Command) (tea.Model, tea.Cmd) {
	currentChannel := m.executor.GetCurrentChannel()
	if currentChannel == nil {
		m.history = append(m.history, errorStyle.Render("Not in a channel. Use 'cd #channel' first."))
		m.input.SetValue("")
		return m, nil
	}

	channelName := currentChannel.Name
	if currentChannel.IsIM {
		if name, ok := m.executor.userNames[currentChannel.UserID]; ok {
			channelName = name
		} else {
			channelName = currentChannel.UserID
		}
	}

	m.browseModel = NewBrowseModel(m.client, currentChannel.ID, channelName, m.executor.userNames)
	m.browseModel.width = m.width
	m.browseModel.height = m.height
	m.browseMode = true
	m.input.Blur()
	m.input.SetValue("")

	return m, m.browseModel.Init()
}

func (m *Model) startLiveMode(cmd Command) (tea.Model, tea.Cmd) {
	currentChannel := m.executor.GetCurrentChannel()
	if currentChannel == nil {
		m.history = append(m.history, errorStyle.Render("Not in a channel. Use 'cd #channel' first."))
		m.input.SetValue("")
		return m, nil
	}

	if m.realtimeClient == nil {
		m.history = append(m.history, errorStyle.Render("Real-time connection not available. Set SLACK_APP_TOKEN to enable."))
		m.input.SetValue("")
		return m, nil
	}

	channelName := currentChannel.Name
	if currentChannel.IsIM {
		if name, ok := m.executor.userNames[currentChannel.UserID]; ok {
			channelName = name
		} else {
			channelName = currentChannel.UserID
		}
	}

	m.liveModel = NewLiveModel(m.client, currentChannel.ID, channelName, m.executor.userNames)
	m.liveModel.width = m.width
	m.liveModel.height = m.height
	m.liveMode = true
	m.input.Blur()
	m.input.SetValue("")

	return m, m.liveModel.Init()
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

func (m *Model) handleTabCompletion() (tea.Model, tea.Cmd) {
	input := m.input.Value()

	// Parse input to determine completion type
	parts := strings.SplitN(input, " ", 2)
	cmdPart := parts[0]

	if !m.completionActive {
		// First Tab press - initialize completion
		m.originalInput = input

		if len(parts) == 1 && !strings.HasSuffix(input, " ") {
			// Complete command name
			m.completionCandidates = m.executor.GetCommandCompletions(cmdPart)
		} else if len(parts) >= 1 {
			// Complete argument
			argPrefix := ""
			if len(parts) == 2 {
				argPrefix = parts[1]
			}
			m.completionCandidates = m.executor.GetArgumentCompletions(cmdPart, argPrefix)
		}

		m.completionIndex = 0
		m.completionActive = true

		if len(m.completionCandidates) == 0 {
			m.resetCompletion()
			return m, nil
		}
	} else {
		// Subsequent Tab press - cycle to next candidate
		m.completionIndex = (m.completionIndex + 1) % len(m.completionCandidates)
	}

	// Apply current completion
	if len(m.completionCandidates) > 0 {
		candidate := m.completionCandidates[m.completionIndex]

		var completed string
		originalParts := strings.SplitN(m.originalInput, " ", 2)
		if len(originalParts) == 1 && !strings.HasSuffix(m.originalInput, " ") {
			// Command completion
			completed = candidate
		} else {
			// Argument completion
			completed = originalParts[0] + " " + candidate
		}

		m.input.SetValue(completed)
		m.input.CursorEnd()
	}

	return m, nil
}

func (m *Model) resetCompletion() {
	m.completionActive = false
	m.completionCandidates = nil
	m.completionIndex = 0
	m.originalInput = ""
}

// View renders the UI
func (m *Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Live mode takes over the entire screen
	if m.liveMode && m.liveModel != nil {
		return m.liveModel.View()
	}

	// Browse mode takes over the entire screen
	if m.browseMode && m.browseModel != nil {
		return m.browseModel.View()
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

	// Add input line
	sb.WriteString(m.input.View())

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
