package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-tui/internal/slack"
	"github.com/polidog/slack-tui/internal/ui/styles"
	"github.com/polidog/slack-tui/internal/ui/views"
)

type Focus int

const (
	FocusSidebar Focus = iota
	FocusMessages
	FocusInput
	FocusThread
)

type Model struct {
	slackClient    *slack.Client
	realtimeClient *slack.RealtimeClient

	sidebar  views.SidebarModel
	messages views.MessagesModel
	input    views.InputModel
	thread   views.ThreadModel

	focus Focus

	width  int
	height int

	currentChannelID string
	userCache        map[string]string

	statusMessage string
	connected     bool
	err           error
}

// Messages for async operations
type ChannelsLoadedMsg struct {
	Channels []slack.Channel
	DMs      []slack.Channel
}

type MessagesLoadedMsg struct {
	Messages  []slack.Message
	ChannelID string
}

type ThreadLoadedMsg struct {
	Messages []slack.Message
	ThreadTS string
}

type MessageSentMsg struct {
	Timestamp string
}

type UsersCachedMsg struct {
	Cache map[string]string
}

type IncomingMessageMsg struct {
	Message slack.IncomingMessage
}

type ConnectionStatusMsg struct {
	Connected bool
}

type ErrorMsg struct {
	Err error
}

func NewModel(client *slack.Client) Model {
	return Model{
		slackClient: client,
		sidebar:     views.NewSidebarModel(),
		messages:    views.NewMessagesModel(),
		input:       views.NewInputModel(),
		thread:      views.NewThreadModel(),
		focus:       FocusSidebar,
		userCache:   make(map[string]string),
		connected:   true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadChannels(),
		tea.EnterAltScreen,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.focus != FocusInput {
				return m, tea.Quit
			}

		case "tab":
			if m.focus != FocusInput {
				m.cycleFocus()
			}

		case "enter":
			if m.focus == FocusInput && m.input.Value() != "" {
				cmds = append(cmds, m.sendMessage())
			} else if m.focus == FocusMessages {
				selectedMsg := m.messages.GetSelectedMessage()
				if selectedMsg != nil && (selectedMsg.ReplyCount > 0 || selectedMsg.ThreadTS != "") {
					cmds = append(cmds, m.loadThread(selectedMsg))
				}
			} else if m.focus == FocusSidebar {
				selectedChannel := m.sidebar.GetSelectedChannel()
				if selectedChannel != nil {
					m.currentChannelID = selectedChannel.ID
					m.messages.SetChannelName(selectedChannel.Name)
					cmds = append(cmds, m.loadMessages(selectedChannel.ID))
					m.focus = FocusMessages
					m.updateFocus()
				}
			}

		case "esc":
			if m.thread.IsVisible() {
				m.thread.Hide()
				m.focus = FocusMessages
				m.updateFocus()
			} else if m.focus == FocusInput {
				m.focus = FocusMessages
				m.updateFocus()
			}

		case "i":
			if m.focus != FocusInput {
				m.focus = FocusInput
				m.updateFocus()
			}

		case "r":
			if m.focus == FocusMessages {
				selectedMsg := m.messages.GetSelectedMessage()
				if selectedMsg != nil {
					cmds = append(cmds, m.loadThread(selectedMsg))
					m.input.SetPlaceholder("Reply in thread...")
				}
			}
		}

	case ChannelsLoadedMsg:
		m.sidebar.SetChannels(msg.Channels)
		m.sidebar.SetDMs(msg.DMs)

		// Collect user IDs from DMs to fetch names
		var userIDs []string
		for _, dm := range msg.DMs {
			if dm.UserID != "" {
				userIDs = append(userIDs, dm.UserID)
			}
		}
		if len(userIDs) > 0 {
			cmds = append(cmds, m.fetchUserNames(userIDs))
		}

		m.statusMessage = fmt.Sprintf("Loaded %d channels, %d DMs", len(msg.Channels), len(msg.DMs))

	case MessagesLoadedMsg:
		if msg.ChannelID == m.currentChannelID {
			m.messages.SetMessages(msg.Messages)
			m.messages.ScrollToBottom()

			// Collect user IDs to fetch names
			var userIDs []string
			seen := make(map[string]bool)
			for _, msg := range msg.Messages {
				if msg.User != "" && !seen[msg.User] {
					userIDs = append(userIDs, msg.User)
					seen[msg.User] = true
				}
			}
			if len(userIDs) > 0 {
				cmds = append(cmds, m.fetchUserNames(userIDs))
			}
		}

	case ThreadLoadedMsg:
		m.thread.SetMessages(msg.Messages)
		m.thread.SetVisible(true)
		m.focus = FocusThread
		m.updateFocus()

	case MessageSentMsg:
		m.input.Reset()
		if m.focus == FocusInput {
			m.focus = FocusMessages
			m.updateFocus()
		}

	case UsersCachedMsg:
		for k, v := range msg.Cache {
			m.userCache[k] = v
		}
		m.sidebar.SetUserCache(m.userCache)
		m.messages.SetUserCache(m.userCache)
		m.thread.SetUserCache(m.userCache)

	case IncomingMessageMsg:
		if msg.Message.ChannelID == m.currentChannelID {
			newMsg := slack.Message{
				Timestamp: msg.Message.Timestamp,
				User:      msg.Message.UserID,
				Text:      msg.Message.Text,
				ThreadTS:  msg.Message.ThreadTS,
			}

			if msg.Message.ThreadTS != "" && m.thread.IsVisible() {
				threadTS := m.thread.GetThreadTS()
				if msg.Message.ThreadTS == threadTS {
					m.thread.AppendMessage(newMsg)
				}
			} else if msg.Message.ThreadTS == "" {
				m.messages.AppendMessage(newMsg)
				m.messages.ScrollToBottom()
			}
		}

	case ConnectionStatusMsg:
		m.connected = msg.Connected
		if msg.Connected {
			m.statusMessage = "Connected"
		} else {
			m.statusMessage = "Disconnected"
		}

	case ErrorMsg:
		m.err = msg.Err
		m.statusMessage = fmt.Sprintf("Error: %v", msg.Err)
	}

	// Update sub-models
	var cmd tea.Cmd
	m.sidebar, cmd = m.sidebar.Update(msg)
	cmds = append(cmds, cmd)

	m.messages, cmd = m.messages.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.thread, cmd = m.thread.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Main content area (sidebar + messages + optional thread)
	sidebarView := m.sidebar.View()

	var mainArea string
	if m.thread.IsVisible() {
		messagesView := m.messages.View()
		threadView := m.thread.View()
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, messagesView, threadView)
	} else {
		mainArea = m.messages.View()
	}

	contentArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, mainArea)

	// Input area
	inputView := m.input.View()

	// Status bar
	statusBar := m.renderStatusBar()

	// Combine all sections
	return lipgloss.JoinVertical(
		lipgloss.Left,
		contentArea,
		inputView,
		statusBar,
	)
}

func (m *Model) updateLayout() {
	sidebarWidth := m.width / 5
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	if sidebarWidth > 40 {
		sidebarWidth = 40
	}

	inputHeight := 5
	statusHeight := 1
	contentHeight := m.height - inputHeight - statusHeight

	var messagesWidth int
	if m.thread.IsVisible() {
		threadWidth := m.width / 3
		if threadWidth < 30 {
			threadWidth = 30
		}
		messagesWidth = m.width - sidebarWidth - threadWidth
		m.thread.SetSize(threadWidth, contentHeight)
	} else {
		messagesWidth = m.width - sidebarWidth
	}

	m.sidebar.SetSize(sidebarWidth, contentHeight)
	m.messages.SetSize(messagesWidth, contentHeight)
	m.input.SetSize(m.width, inputHeight)
}

func (m *Model) updateFocus() {
	m.sidebar.SetFocused(m.focus == FocusSidebar)
	m.messages.SetFocused(m.focus == FocusMessages)
	m.input.SetFocused(m.focus == FocusInput)
	m.thread.SetFocused(m.focus == FocusThread)
}

func (m *Model) cycleFocus() {
	if m.thread.IsVisible() {
		switch m.focus {
		case FocusSidebar:
			m.focus = FocusMessages
		case FocusMessages:
			m.focus = FocusThread
		case FocusThread:
			m.focus = FocusSidebar
		}
	} else {
		switch m.focus {
		case FocusSidebar:
			m.focus = FocusMessages
		case FocusMessages:
			m.focus = FocusSidebar
		}
	}
	m.updateFocus()
}

func (m Model) renderStatusBar() string {
	statusStyle := styles.StatusConnectedStyle
	statusIcon := "●"
	if !m.connected {
		statusStyle = styles.StatusDisconnectedStyle
	}

	status := statusStyle.Render(statusIcon) + " " + m.statusMessage

	help := styles.HelpKeyStyle.Render("tab") + styles.HelpStyle.Render(":switch") + " " +
		styles.HelpKeyStyle.Render("enter") + styles.HelpStyle.Render(":select") + " " +
		styles.HelpKeyStyle.Render("i") + styles.HelpStyle.Render(":input") + " " +
		styles.HelpKeyStyle.Render("r") + styles.HelpStyle.Render(":reply") + " " +
		styles.HelpKeyStyle.Render("q") + styles.HelpStyle.Render(":quit")

	gap := m.width - lipgloss.Width(status) - lipgloss.Width(help) - 2
	if gap < 0 {
		gap = 0
	}

	return styles.StatusBarStyle.Width(m.width).Render(
		status + lipgloss.NewStyle().Width(gap).Render("") + help,
	)
}

// Async commands
func (m Model) loadChannels() tea.Cmd {
	return func() tea.Msg {
		channels, err := m.slackClient.GetChannels()
		if err != nil {
			return ErrorMsg{Err: err}
		}

		dms, err := m.slackClient.GetDMs()
		if err != nil {
			return ErrorMsg{Err: err}
		}

		return ChannelsLoadedMsg{
			Channels: channels,
			DMs:      dms,
		}
	}
}

func (m Model) loadMessages(channelID string) tea.Cmd {
	return func() tea.Msg {
		messages, err := m.slackClient.GetMessages(channelID, 100)
		if err != nil {
			return ErrorMsg{Err: err}
		}

		return MessagesLoadedMsg{
			Messages:  messages,
			ChannelID: channelID,
		}
	}
}

func (m Model) loadThread(parentMsg *slack.Message) tea.Cmd {
	return func() tea.Msg {
		threadTS := parentMsg.ThreadTS
		if threadTS == "" {
			threadTS = parentMsg.Timestamp
		}

		replies, err := m.slackClient.GetThreadReplies(m.currentChannelID, threadTS)
		if err != nil {
			return ErrorMsg{Err: err}
		}

		return ThreadLoadedMsg{
			Messages: replies,
			ThreadTS: threadTS,
		}
	}
}

func (m Model) sendMessage() tea.Cmd {
	text := m.input.Value()
	channelID := m.currentChannelID

	return func() tea.Msg {
		var ts string
		var err error

		if m.thread.IsVisible() {
			threadTS := m.thread.GetThreadTS()
			ts, err = m.slackClient.PostThreadReply(channelID, threadTS, text)
		} else {
			ts, err = m.slackClient.PostMessage(channelID, text)
		}

		if err != nil {
			return ErrorMsg{Err: err}
		}

		return MessageSentMsg{Timestamp: ts}
	}
}

func (m Model) fetchUserNames(userIDs []string) tea.Cmd {
	return func() tea.Msg {
		cache := make(map[string]string)

		users, err := m.slackClient.GetUsersInfo(userIDs)
		if err != nil {
			return ErrorMsg{Err: err}
		}

		if users != nil {
			for _, user := range *users {
				displayName := user.Profile.DisplayName
				if displayName == "" {
					displayName = user.RealName
				}
				if displayName == "" {
					displayName = user.Name
				}
				cache[user.ID] = displayName
			}
		}

		return UsersCachedMsg{Cache: cache}
	}
}

func (m *Model) SetRealtimeClient(client *slack.RealtimeClient) {
	m.realtimeClient = client
}

func (m *Model) HandleRealtimeEvent(event interface{}) tea.Cmd {
	switch e := event.(type) {
	case slack.IncomingMessage:
		return func() tea.Msg {
			return IncomingMessageMsg{Message: e}
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
