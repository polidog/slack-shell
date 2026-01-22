package shell

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-shell/internal/cache"
	"github.com/polidog/slack-shell/internal/config"
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
	displayConfig *config.DisplayConfig

	// Thread display
	threadMessages []slack.Message
	threadVisible  bool
	threadTS       string

	// Input mode
	inputMode InputMode
	inputText textarea.Model

	channelID   string
	channelName string

	// Loading state
	loading      bool
	loadingErr   error
	loadingOlder bool

	// Pagination
	hasMoreMessages bool
}

// NewLiveModel creates a new LiveModel
func NewLiveModel(client *slack.Client, channelID, channelName string, userCache map[string]string, displayConfig *config.DisplayConfig) *LiveModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.CharLimit = 4000
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	if displayConfig == nil {
		displayConfig = config.DefaultDisplayConfig()
	}

	return &LiveModel{
		client:        client,
		channelID:     channelID,
		channelName:   channelName,
		userCache:     userCache,
		displayConfig: displayConfig,
		inputText:     ta,
		loading:       true,
	}
}

// Init initializes the live model
func (m *LiveModel) Init() tea.Cmd {
	return m.loadMessages()
}

// LiveMessagesLoadedMsg is sent when messages are loaded in live mode
type LiveMessagesLoadedMsg struct {
	Messages []slack.Message
	HasMore  bool
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

// LiveOlderMessagesLoadedMsg is sent when older messages are loaded
type LiveOlderMessagesLoadedMsg struct {
	Messages []slack.Message
	HasMore  bool
	Err      error
}

func (m *LiveModel) loadMessages() tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.GetMessagesWithPagination(m.channelID, 50, "")
		if err != nil {
			return LiveMessagesLoadedMsg{Messages: nil, HasMore: false, Err: err}
		}
		// Resolve user names
		m.resolveUserNames(result.Messages)
		return LiveMessagesLoadedMsg{Messages: result.Messages, HasMore: result.HasMore, Err: nil}
	}
}

func (m *LiveModel) loadOlderMessages() tea.Cmd {
	if len(m.messages) == 0 {
		return nil
	}
	// Get the oldest message timestamp
	oldestTS := m.messages[0].Timestamp
	return func() tea.Msg {
		result, err := m.client.GetMessagesWithPagination(m.channelID, 50, oldestTS)
		if err != nil {
			return LiveOlderMessagesLoadedMsg{Messages: nil, HasMore: false, Err: err}
		}
		// Resolve user names
		m.resolveUserNames(result.Messages)
		return LiveOlderMessagesLoadedMsg{Messages: result.Messages, HasMore: result.HasMore, Err: nil}
	}
}

// resolveUserNames fetches and caches user names for messages
func (m *LiveModel) resolveUserNames(messages []slack.Message) {
	for _, msg := range messages {
		if msg.User != "" {
			if _, ok := m.userCache[msg.User]; !ok {
				user, err := m.client.GetUserInfo(msg.User)
				if err == nil {
					entry := cache.CachedUser{
						Name:        user.Name,
						DisplayName: user.Profile.DisplayName,
						RealName:    user.RealName,
					}
					m.userCache[msg.User] = entry.GetPreferredName(m.displayConfig.NameFormat)
				}
			}
		}
	}
}

func (m *LiveModel) loadThread(threadTS string) tea.Cmd {
	return func() tea.Msg {
		messages, err := m.client.GetThreadReplies(m.channelID, threadTS)
		if err == nil {
			m.resolveUserNames(messages)
		}
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
			m.hasMoreMessages = msg.HasMore
			// Select the last (newest) message by default
			if len(m.messages) > 0 {
				m.selectedIndex = len(m.messages) - 1
				m.ensureVisible()
			}
		}
		return m, nil

	case LiveOlderMessagesLoadedMsg:
		m.loadingOlder = false
		if msg.Err != nil {
			m.loadingErr = msg.Err
		} else if len(msg.Messages) > 0 {
			// Prepend older messages
			m.messages = append(msg.Messages, m.messages...)
			m.hasMoreMessages = msg.HasMore
			// Adjust selectedIndex to keep the same message selected
			m.selectedIndex += len(msg.Messages)
			m.scrollOffset += len(msg.Messages)
		} else {
			m.hasMoreMessages = false
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
		m.inputText.SetWidth(msg.Width - 20)
		return m, nil

	case tea.KeyMsg:
		// Handle input mode
		if m.inputMode != InputModeNone {
			// Get send key setting (default to "enter")
			sendKey := m.displayConfig.LiveSendKey
			if sendKey == "" {
				sendKey = "enter"
			}

			switch msg.Type {
			case tea.KeyEsc:
				m.inputMode = InputModeNone
				m.inputText.Blur()
				m.inputText.Reset()
				return m, nil
			case tea.KeyEnter:
				// Check for shift modifier (shift+enter always inserts newline in "enter" mode)
				if sendKey == "enter" && !msg.Alt {
					// Enter sends message (unless shift is held)
					// Note: Bubble Tea represents shift+enter differently
					text := strings.TrimSpace(m.inputText.Value())
					if text != "" {
						currentMode := m.inputMode
						m.inputMode = InputModeNone
						m.inputText.Blur()
						m.inputText.Reset()

						if currentMode == InputModeNewMessage {
							return m, m.sendMessage(text)
						} else if currentMode == InputModeReply {
							return m, m.sendReply(m.threadTS, text)
						}
					}
					return m, nil
				}
				// ctrl+enter mode: Enter inserts newline (let textarea handle it)
				m.inputText, cmd = m.inputText.Update(msg)
				return m, cmd
			case tea.KeyCtrlJ: // Ctrl+Enter is often sent as Ctrl+J
				if sendKey == "ctrl+enter" {
					text := strings.TrimSpace(m.inputText.Value())
					if text != "" {
						currentMode := m.inputMode
						m.inputMode = InputModeNone
						m.inputText.Blur()
						m.inputText.Reset()

						if currentMode == InputModeNewMessage {
							return m, m.sendMessage(text)
						} else if currentMode == InputModeReply {
							return m, m.sendReply(m.threadTS, text)
						}
					}
					return m, nil
				}
				m.inputText, cmd = m.inputText.Update(msg)
				return m, cmd
			default:
				// Check for shift+enter in "enter" mode (insert newline)
				if sendKey == "enter" && msg.String() == "shift+enter" {
					// Insert newline manually
					m.inputText.InsertString("\n")
					return m, nil
				}
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
					return m, textarea.Blink
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
			} else if m.selectedIndex == 0 && m.hasMoreMessages && !m.loadingOlder {
				// At the top, load older messages
				m.loadingOlder = true
				return m, m.loadOlderMessages()
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
			return m, textarea.Blink
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
				return m, textarea.Blink
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

	// If selected message is above the scroll offset, scroll up
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
		return
	}

	// Calculate how many lines are used from scrollOffset to selectedIndex (inclusive)
	linesUsed := m.getTotalLinesInRange(m.scrollOffset, m.selectedIndex+1)

	// If selected message doesn't fit, scroll down
	if linesUsed > visibleLines {
		// Find new scrollOffset that shows the selected message
		m.scrollOffset = m.selectedIndex
		// Try to show more context by scrolling back if possible
		linesNeeded := m.getMessageLineCount(m.selectedIndex)
		for m.scrollOffset > 0 && linesNeeded < visibleLines {
			prevLines := m.getMessageLineCount(m.scrollOffset - 1)
			if linesNeeded+prevLines <= visibleLines {
				m.scrollOffset--
				linesNeeded += prevLines
			} else {
				break
			}
		}
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

	// Show loading indicator for older messages
	if m.loadingOlder {
		sb.WriteString(liveHelpStyle.Render("Loading older messages..."))
		sb.WriteString("\n")
	}

	visibleLines := m.getVisibleLines()
	truncate := m.displayConfig.LiveTruncateMessages

	// Render messages starting from scrollOffset, counting lines
	linesRendered := 0
	endIdx := m.scrollOffset

	for i := m.scrollOffset; i < len(m.messages) && linesRendered < visibleLines; i++ {
		msg := m.messages[i]
		lines := m.formatMessageLines(msg, i, truncate)

		for _, line := range lines {
			if linesRendered >= visibleLines {
				break
			}

			if i == m.selectedIndex {
				sb.WriteString(liveSelectedStyle.Render(line))
			} else {
				sb.WriteString(liveNormalStyle.Render(line))
			}
			sb.WriteString("\n")
			linesRendered++
		}
		endIdx = i + 1
	}

	// Scroll indicator
	totalMessages := len(m.messages)
	if totalMessages > 0 {
		moreIndicator := ""
		if m.hasMoreMessages {
			moreIndicator = " (↑ for more)"
		}
		sb.WriteString(fmt.Sprintf("\n[%d-%d of %d messages]%s",
			m.scrollOffset+1, endIdx, totalMessages, moreIndicator))
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
		// Thread view always shows full text (no truncation)
		lines := m.formatMessageLines(msg, i, false)
		for _, line := range lines {
			if i == 0 {
				// Parent message
				sb.WriteString(liveNormalStyle.Render(line))
			} else {
				// Thread replies
				sb.WriteString(liveThreadStyle.Render("  " + line))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
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

// wrapText wraps text to fit within the given width (in runes, not bytes)
func (m *LiveModel) wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}

	var lines []string
	// Split by existing newlines first
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			lines = append(lines, "")
			continue
		}

		// Convert to runes for proper multi-byte character handling
		runes := []rune(para)

		// Wrap each paragraph
		for len(runes) > width {
			// Find a good break point
			breakPoint := width
			// Try to break at a space
			for i := width; i > width/2; i-- {
				if i < len(runes) && runes[i] == ' ' {
					breakPoint = i
					break
				}
			}
			lines = append(lines, string(runes[:breakPoint]))
			runes = []rune(strings.TrimLeft(string(runes[breakPoint:]), " "))
		}
		if len(runes) > 0 {
			lines = append(lines, string(runes))
		}
	}

	if len(lines) == 0 {
		lines = append(lines, "")
	}

	return lines
}

// formatMessageLines formats a message and returns multiple lines if needed
func (m *LiveModel) formatMessageLines(msg slack.Message, index int, truncate bool) []string {
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

	// Resolve mentions in text and convert emoji
	text := ConvertEmoji(ResolveMentions(msg.Text, m.userCache))

	// Header: [time] user:
	header := fmt.Sprintf("[%s] %s: ", timeStr, userName)
	headerLen := utf8.RuneCountInString(header)

	if truncate {
		maxLen := m.width - 30
		if maxLen < 20 {
			maxLen = 20
		}
		textRunes := []rune(text)
		if len(textRunes) > maxLen {
			text = string(textRunes[:maxLen-3]) + "..."
		}
		text = strings.ReplaceAll(text, "\n", " ")
		return []string{header + text + threadIndicator}
	}

	// Multi-line mode: wrap text
	availableWidth := m.width - headerLen - 2
	if availableWidth < 20 {
		availableWidth = 20
	}

	wrappedLines := m.wrapText(text, availableWidth)

	var result []string
	for i, line := range wrappedLines {
		if i == 0 {
			// First line includes header
			if len(wrappedLines) == 1 {
				result = append(result, header+line+threadIndicator)
			} else {
				result = append(result, header+line)
			}
		} else {
			// Continuation lines are indented
			indent := strings.Repeat(" ", headerLen)
			if i == len(wrappedLines)-1 {
				result = append(result, indent+line+threadIndicator)
			} else {
				result = append(result, indent+line)
			}
		}
	}

	return result
}

// getMessageLineCount returns the number of lines a message will take
func (m *LiveModel) getMessageLineCount(msgIndex int) int {
	if msgIndex < 0 || msgIndex >= len(m.messages) {
		return 1
	}
	truncate := m.displayConfig.LiveTruncateMessages
	lines := m.formatMessageLines(m.messages[msgIndex], msgIndex, truncate)
	return len(lines)
}

// getTotalLinesUpToIndex returns total lines for messages from startIdx to endIdx (exclusive)
func (m *LiveModel) getTotalLinesInRange(startIdx, endIdx int) int {
	total := 0
	for i := startIdx; i < endIdx && i < len(m.messages); i++ {
		total += m.getMessageLineCount(i)
	}
	return total
}

func (m *LiveModel) renderHelp() string {
	var help string
	if m.inputMode != InputModeNone {
		sendKey := m.displayConfig.LiveSendKey
		if sendKey == "" {
			sendKey = "enter"
		}
		if sendKey == "ctrl+enter" {
			help = "Ctrl+Enter: send | Enter: newline | Esc: cancel"
		} else {
			help = "Enter: send | Shift+Enter: newline | Esc: cancel"
		}
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
