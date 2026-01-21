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
	browseSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("4")).
				Foreground(lipgloss.Color("15"))
	browseNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	browseThreadStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")).
				PaddingLeft(2)
	browseHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("3")).
				Bold(true)
	browseHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)

// BrowseModel represents the browse mode UI
type BrowseModel struct {
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
	inputMode bool
	replyText textinput.Model

	channelID   string
	channelName string

	// Loading state
	loading    bool
	loadingErr error
}

// NewBrowseModel creates a new BrowseModel
func NewBrowseModel(client *slack.Client, channelID, channelName string, userCache map[string]string) *BrowseModel {
	ti := textinput.New()
	ti.Placeholder = "Type your reply..."
	ti.CharLimit = 1000
	ti.Width = 60

	return &BrowseModel{
		client:      client,
		channelID:   channelID,
		channelName: channelName,
		userCache:   userCache,
		replyText:   ti,
		loading:     true,
	}
}

// Init initializes the browse model
func (m *BrowseModel) Init() tea.Cmd {
	return m.loadMessages()
}

// MessagesLoadedMsg is sent when messages are loaded
type MessagesLoadedMsg struct {
	Messages []slack.Message
	Err      error
}

// ThreadLoadedMsg is sent when thread is loaded
type ThreadLoadedMsg struct {
	Messages []slack.Message
	Err      error
}

// ReplySentMsg is sent when a reply is sent
type ReplySentMsg struct {
	Err error
}

func (m *BrowseModel) loadMessages() tea.Cmd {
	return func() tea.Msg {
		messages, err := m.client.GetMessages(m.channelID, 50)
		return MessagesLoadedMsg{Messages: messages, Err: err}
	}
}

func (m *BrowseModel) loadThread(threadTS string) tea.Cmd {
	return func() tea.Msg {
		messages, err := m.client.GetThreadReplies(m.channelID, threadTS)
		return ThreadLoadedMsg{Messages: messages, Err: err}
	}
}

func (m *BrowseModel) sendReply(threadTS, text string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.PostThreadReply(m.channelID, threadTS, text)
		return ReplySentMsg{Err: err}
	}
}

// Update handles messages
func (m *BrowseModel) Update(msg tea.Msg) (*BrowseModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case MessagesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.loadingErr = msg.Err
		} else {
			m.messages = msg.Messages
			// Select the last (newest) message by default
			if len(m.messages) > 0 {
				m.selectedIndex = len(m.messages) - 1
			}
		}
		return m, nil

	case ThreadLoadedMsg:
		if msg.Err != nil {
			m.loadingErr = msg.Err
			m.threadVisible = false
		} else {
			m.threadMessages = msg.Messages
			m.threadVisible = true
		}
		return m, nil

	case ReplySentMsg:
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
		m.replyText.Width = msg.Width - 20
		return m, nil

	case tea.KeyMsg:
		// Handle input mode
		if m.inputMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.inputMode = false
				m.replyText.Blur()
				m.replyText.SetValue("")
				return m, nil
			case tea.KeyEnter:
				text := strings.TrimSpace(m.replyText.Value())
				if text != "" {
					m.inputMode = false
					m.replyText.Blur()
					m.replyText.SetValue("")
					return m, m.sendReply(m.threadTS, text)
				}
				return m, nil
			default:
				m.replyText, cmd = m.replyText.Update(msg)
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
					m.inputMode = true
					m.replyText.Focus()
					return m, textinput.Blink
				}
				return m, nil
			}
			return m, nil
		}

		// Handle main list view
		switch msg.String() {
		case "q":
			// Signal to exit browse mode (handled by parent)
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
		case "r":
			// Reply to selected message directly (create thread or reply in existing thread)
			if len(m.messages) > 0 && m.selectedIndex < len(m.messages) {
				selectedMsg := m.messages[m.selectedIndex]
				threadTS := selectedMsg.Timestamp
				if selectedMsg.ThreadTS != "" {
					threadTS = selectedMsg.ThreadTS
				}
				m.threadTS = threadTS
				m.inputMode = true
				m.replyText.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *BrowseModel) ensureVisible() {
	visibleLines := m.getVisibleLines()
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	} else if m.selectedIndex >= m.scrollOffset+visibleLines {
		m.scrollOffset = m.selectedIndex - visibleLines + 1
	}
}

func (m *BrowseModel) getVisibleLines() int {
	// Reserve space for header (2 lines) and help (2 lines)
	available := m.height - 4
	if available < 1 {
		return 1
	}
	return available
}

// View renders the browse UI
func (m *BrowseModel) View() string {
	var sb strings.Builder

	// Header
	header := fmt.Sprintf("Browse #%s", m.channelName)
	if m.threadVisible {
		header += " (Thread View)"
	}
	sb.WriteString(browseHeaderStyle.Render(header))
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
	if m.inputMode {
		sb.WriteString("\n")
		sb.WriteString("Reply: ")
		sb.WriteString(m.replyText.View())
		sb.WriteString("\n")
	}

	sb.WriteString(m.renderHelp())

	return sb.String()
}

func (m *BrowseModel) renderMessageList() string {
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
			sb.WriteString(browseSelectedStyle.Render(line))
		} else {
			sb.WriteString(browseNormalStyle.Render(line))
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

func (m *BrowseModel) renderThread() string {
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
			sb.WriteString(browseNormalStyle.Render(line))
		} else {
			// Thread replies
			sb.WriteString(browseThreadStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *BrowseModel) formatMessageLine(msg slack.Message, index int) string {
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

func (m *BrowseModel) parseTimestamp(ts string) time.Time {
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	return time.Unix(sec, 0)
}

func (m *BrowseModel) renderHelp() string {
	var help string
	if m.inputMode {
		help = "Enter: send | Esc: cancel"
	} else if m.threadVisible {
		help = "r: reply | q/Esc: back | j/k: scroll"
	} else {
		help = "Enter: view thread | r: reply | j/k/arrows: navigate | q: exit"
	}
	return "\n" + browseHelpStyle.Render(help)
}

// AddIncomingMessage adds a new message from realtime events
func (m *BrowseModel) AddIncomingMessage(channelID, userID, userName, text, timestamp, threadTS string) {
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

// GetChannelID returns the channel ID for this browse model
func (m *BrowseModel) GetChannelID() string {
	return m.channelID
}

// ShouldExit returns true if the user wants to exit browse mode
func (m *BrowseModel) ShouldExit(msg tea.KeyMsg) bool {
	// Only exit on 'q' when not in input mode and not in thread view
	if m.inputMode || m.threadVisible {
		return false
	}
	return msg.String() == "q"
}

// IsInInputMode returns true if browse model is in input mode
func (m *BrowseModel) IsInInputMode() bool {
	return m.inputMode
}

// IsThreadVisible returns true if thread view is visible
func (m *BrowseModel) IsThreadVisible() bool {
	return m.threadVisible
}
