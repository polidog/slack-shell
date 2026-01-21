package shell

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-shell/internal/slack"
)

var (
	liveSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("4")).
				Foreground(lipgloss.Color("15"))
	liveNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
	liveThreadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			PaddingLeft(2)
	liveHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true)
	liveHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
	liveNewMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
)

// InputMode represents the type of input in live mode
type InputMode int

const (
	InputModeNone InputMode = iota
	InputModeNewMessage
	InputModeReply
)

// LiveModel represents the live mode UI with real-time updates and message sending
type LiveModel struct {
	client        *slack.Client
	messages      []slack.Message
	selectedIndex int
	scrollOffset  int
	width, height int
	userCache     map[string]string

	// Thread display
	threadMessages []slack.Message
	threadVisible  bool
	threadTS       string

	// Input mode
	inputMode InputMode
	inputText textinput.Model

	channelID   string
	channelName string

	// Loading state
	loading    bool
	loadingErr error
}

// NewLiveModel creates a new LiveModel
func NewLiveModel(client *slack.Client, channelID, channelName string, userCache map[string]string) *LiveModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 1000
	ti.Width = 60

	return &LiveModel{
		client:      client,
		channelID:   channelID,
		channelName: channelName,
		userCache:   userCache,
		inputText:   ti,
		loading:     true,
	}
}

// Init initializes the live model
func (m *LiveModel) Init() tea.Cmd {
	return m.loadMessages()
}

// LiveMessagesLoadedMsg is sent when messages are loaded in live mode
type LiveMessagesLoadedMsg struct {
	Messages []slack.Message
	Err      error
}

// LiveThreadLoadedMsg is sent when thread is loaded in live mode
type LiveThreadLoadedMsg struct {
	Messages []slack.Message
	Err      error
}

// LiveMessageSentMsg is sent when a message is sent in live mode
type LiveMessageSentMsg struct {
	Err error
}

// LiveReplySentMsg is sent when a reply is sent in live mode
type LiveReplySentMsg struct {
	Err error
}

func (m *LiveModel) loadMessages() tea.Cmd {
	return func() tea.Msg {
		messages, err := m.client.GetMessages(m.channelID, 50)
		return LiveMessagesLoadedMsg{Messages: messages, Err: err}
	}
}

func (m *LiveModel) loadThread(threadTS string) tea.Cmd {
	return func() tea.Msg {
		messages, err := m.client.GetThreadReplies(m.channelID, threadTS)
		return LiveThreadLoadedMsg{Messages: messages, Err: err}
	}
}

func (m *LiveModel) sendMessage(text string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.PostMessage(m.channelID, text)
		return LiveMessageSentMsg{Err: err}
	}
}

func (m *LiveModel) sendReply(threadTS, text string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.PostThreadReply(m.channelID, threadTS, text)
		return LiveReplySentMsg{Err: err}
	}
}

// Update handles messages
func (m *LiveModel) Update(msg tea.Msg) (*LiveModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case LiveMessagesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.loadingErr = msg.Err
		} else {
			m.messages = msg.Messages
			// Select the last (newest) message by default
			if len(m.messages) > 0 {
				m.selectedIndex = len(m.messages) - 1
				m.ensureVisible()
			}
		}
		return m, nil

	case LiveThreadLoadedMsg:
		if msg.Err != nil {
			m.loadingErr = msg.Err
			m.threadVisible = false
		} else {
			m.threadMessages = msg.Messages
			m.threadVisible = true
		}
		return m, nil

	case LiveMessageSentMsg:
		if msg.Err != nil {
			m.loadingErr = msg.Err
		}
		// Message will appear via real-time events
		return m, nil

	case LiveReplySentMsg:
		if msg.Err != nil {
			m.loadingErr = msg.Err
		} else {
			// Reload thread to show the new reply
			return m, m.loadThread(m.threadTS)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.inputText.Width = msg.Width - 20
		return m, nil

	case tea.KeyMsg:
		// Handle input mode
		if m.inputMode != InputModeNone {
			switch msg.Type {
			case tea.KeyEsc:
				m.inputMode = InputModeNone
				m.inputText.Blur()
				m.inputText.SetValue("")
				return m, nil
			case tea.KeyEnter:
				text := strings.TrimSpace(m.inputText.Value())
				if text != "" {
					currentMode := m.inputMode
					m.inputMode = InputModeNone
					m.inputText.Blur()
					m.inputText.SetValue("")

					if currentMode == InputModeNewMessage {
						return m, m.sendMessage(text)
					} else if currentMode == InputModeReply {
						return m, m.sendReply(m.threadTS, text)
					}
				}
				return m, nil
			default:
				m.inputText, cmd = m.inputText.Update(msg)
				return m, cmd
			}
		}

		// Handle thread view
		if m.threadVisible {
			switch msg.String() {
			case "q", "esc":
				m.threadVisible = false
				m.threadMessages = nil
				m.threadTS = ""
				return m, nil
			case "r":
				if m.threadTS != "" {
					m.inputMode = InputModeReply
					m.inputText.Placeholder = "Type your reply..."
					m.inputText.Focus()
					return m, textinput.Blink
				}
				return m, nil
			}
			return m, nil
		}

		// Handle main list view
		switch msg.String() {
		case "q":
			// Signal to exit live mode (handled by parent)
			return m, nil
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
				m.ensureVisible()
			}
			return m, nil
		case "down", "j":
			if m.selectedIndex < len(m.messages)-1 {
				m.selectedIndex++
				m.ensureVisible()
			}
			return m, nil
		case "enter":
			if len(m.messages) > 0 && m.selectedIndex < len(m.messages) {
				selectedMsg := m.messages[m.selectedIndex]
				// Use the message timestamp as thread_ts
				threadTS := selectedMsg.Timestamp
				if selectedMsg.ThreadTS != "" {
					threadTS = selectedMsg.ThreadTS
				}
				m.threadTS = threadTS
				return m, m.loadThread(threadTS)
			}
			return m, nil
		case "i":
			// New message input mode
			m.inputMode = InputModeNewMessage
			m.inputText.Placeholder = "Type a message..."
			m.inputText.Focus()
			return m, textinput.Blink
		case "r":
			// Reply to selected message directly (create thread or reply in existing thread)
			if len(m.messages) > 0 && m.selectedIndex < len(m.messages) {
				selectedMsg := m.messages[m.selectedIndex]
				threadTS := selectedMsg.Timestamp
				if selectedMsg.ThreadTS != "" {
					threadTS = selectedMsg.ThreadTS
				}
				m.threadTS = threadTS
				m.inputMode = InputModeReply
				m.inputText.Placeholder = "Type your reply..."
				m.inputText.Focus()
				return m, textinput.Blink
			}
			return m, nil
		case "R":
			// Reload messages
			m.loading = true
			m.loadingErr = nil
			return m, m.loadMessages()
		}
	}

	return m, nil
}

func (m *LiveModel) ensureVisible() {
	visibleLines := m.getVisibleLines()
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	} else if m.selectedIndex >= m.scrollOffset+visibleLines {
		m.scrollOffset = m.selectedIndex - visibleLines + 1
	}
}

func (m *LiveModel) getVisibleLines() int {
	// Reserve space for header (2 lines), input area (2 lines), and help (2 lines)
	available := m.height - 6
	if available < 1 {
		return 1
	}
	return available
}

// View renders the live UI
func (m *LiveModel) View() string {
	var sb strings.Builder

	// Header
	header := fmt.Sprintf("Live #%s", m.channelName)
	if m.threadVisible {
		header += " (Thread View)"
	}
	sb.WriteString(liveHeaderStyle.Render(header))
	sb.WriteString("\n")

	if m.loading {
		sb.WriteString("\nLoading messages...\n")
		sb.WriteString(m.renderHelp())
		return sb.String()
	}

	if m.loadingErr != nil {
		sb.WriteString(fmt.Sprintf("\nError: %v\n", m.loadingErr))
		sb.WriteString(m.renderHelp())
		return sb.String()
	}

	if len(m.messages) == 0 {
		sb.WriteString("\nNo messages found.\n")
		sb.WriteString(m.renderHelp())
		return sb.String()
	}

	// Thread view
	if m.threadVisible {
		sb.WriteString(m.renderThread())
	} else {
		// Main message list
		sb.WriteString(m.renderMessageList())
	}

	// Input mode
	if m.inputMode != InputModeNone {
		sb.WriteString("\n")
		if m.inputMode == InputModeNewMessage {
			sb.WriteString("Message: ")
		} else {
			sb.WriteString("Reply: ")
		}
		sb.WriteString(m.inputText.View())
		sb.WriteString("\n")
	}

	sb.WriteString(m.renderHelp())

	return sb.String()
}

func (m *LiveModel) renderMessageList() string {
	var sb strings.Builder

	visibleLines := m.getVisibleLines()
	endIdx := m.scrollOffset + visibleLines
	if endIdx > len(m.messages) {
		endIdx = len(m.messages)
	}

	for i := m.scrollOffset; i < endIdx; i++ {
		msg := m.messages[i]
		line := m.formatMessageLine(msg, i)

		if i == m.selectedIndex {
			sb.WriteString(liveSelectedStyle.Render(line))
		} else {
			sb.WriteString(liveNormalStyle.Render(line))
		}
		sb.WriteString("\n")
	}

	// Scroll indicator
	if len(m.messages) > visibleLines {
		sb.WriteString(fmt.Sprintf("\n[%d-%d of %d messages]",
			m.scrollOffset+1, endIdx, len(m.messages)))
	}

	return sb.String()
}

func (m *LiveModel) renderThread() string {
	var sb strings.Builder

	if len(m.threadMessages) == 0 {
		sb.WriteString("\nNo replies in this thread.\n")
		return sb.String()
	}

	sb.WriteString("\n")
	for i, msg := range m.threadMessages {
		line := m.formatMessageLine(msg, i)
		if i == 0 {
			// Parent message
			sb.WriteString(liveNormalStyle.Render(line))
		} else {
			// Thread replies
			sb.WriteString(liveThreadStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *LiveModel) formatMessageLine(msg slack.Message, index int) string {
	// Get user name
	userName := msg.UserName
	if userName == "" {
		if msg.IsBot && msg.BotName != "" {
			userName = msg.BotName
		} else if name, ok := m.userCache[msg.User]; ok {
			userName = name
		} else {
			userName = msg.User
		}
	}
	if userName == "" && msg.IsBot {
		userName = "bot"
	}

	// Parse timestamp
	ts := m.parseTimestamp(msg.Timestamp)
	timeStr := ts.Format("01/02 15:04")

	// Thread indicator
	threadIndicator := ""
	if msg.ReplyCount > 0 {
		threadIndicator = fmt.Sprintf(" [%d replies]", msg.ReplyCount)
	}

	// Truncate text if too long
	text := msg.Text
	maxLen := m.width - 30
	if maxLen < 20 {
		maxLen = 20
	}
	if len(text) > maxLen {
		text = text[:maxLen-3] + "..."
	}

	// Replace newlines with spaces
	text = strings.ReplaceAll(text, "\n", " ")

	return fmt.Sprintf("[%s] %s: %s%s", timeStr, userName, text, threadIndicator)
}

func (m *LiveModel) parseTimestamp(ts string) time.Time {
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	return time.Unix(sec, 0)
}

func (m *LiveModel) renderHelp() string {
	var help string
	if m.inputMode != InputModeNone {
		help = "Enter: send | Esc: cancel"
	} else if m.threadVisible {
		help = "r: reply | q/Esc: back | j/k: scroll"
	} else {
		help = "i: new message | Enter: view thread | r: reply | R: reload | j/k/arrows: navigate | q: exit"
	}
	return "\n" + liveHelpStyle.Render(help)
}

// AddIncomingMessage adds a new message from realtime events
func (m *LiveModel) AddIncomingMessage(channelID, userID, userName, text, timestamp, threadTS string) {
	// Only add if it's for this channel
	if channelID != m.channelID {
		return
	}

	// Create a new message
	newMsg := slack.Message{
		Timestamp: timestamp,
		User:      userID,
		UserName:  userName,
		Text:      text,
		ThreadTS:  threadTS,
	}

	// If this is a thread reply to the currently viewed thread
	if m.threadVisible && threadTS != "" && threadTS == m.threadTS {
		m.threadMessages = append(m.threadMessages, newMsg)
		return
	}

	// If this is a main channel message (not a thread reply or it's a parent message)
	if threadTS == "" || threadTS == timestamp {
		m.messages = append(m.messages, newMsg)
		// Auto-scroll to the newest message if already at the bottom
		if m.selectedIndex == len(m.messages)-2 {
			m.selectedIndex = len(m.messages) - 1
			m.ensureVisible()
		}
	}
}

// GetChannelID returns the channel ID for this live model
func (m *LiveModel) GetChannelID() string {
	return m.channelID
}

// ShouldExit returns true if the user wants to exit live mode
func (m *LiveModel) ShouldExit(msg tea.KeyMsg) bool {
	// Only exit on 'q' when not in input mode and not in thread view
	if m.inputMode != InputModeNone || m.threadVisible {
		return false
	}
	return msg.String() == "q"
}

// IsInInputMode returns true if live model is in input mode
func (m *LiveModel) IsInInputMode() bool {
	return m.inputMode != InputModeNone
}

// IsThreadVisible returns true if thread view is visible
func (m *LiveModel) IsThreadVisible() bool {
	return m.threadVisible
}
