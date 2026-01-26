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
	liveNotifyBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("8"))
	liveNotifyPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("6")).
				Padding(0, 1)
	livePeekHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("5")).
				Bold(true)
)

// InputMode represents the type of input in live mode
type InputMode int

const (
	InputModeNone InputMode = iota
	InputModeNewMessage
	InputModeReply
	InputModeEdit
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

	// Delete confirmation
	deleteConfirm bool

	// Edit mode
	editTS string

	// Mention completion
	mentionActive     bool
	mentionCandidates []mentionCandidate
	mentionIndex      int
	mentionPrefix     string // The text after @ being completed
	channelMembers    []string
	membersLoaded     bool

	// Notification display
	notifications     []NotificationItem
	showNotifyPanel   bool
	notifyPanelIndex  int

	// Peek mode (read-only view of another channel)
	peekMode            bool
	peekChannelID       string
	peekChannelName     string
	peekIsIM            bool
	peekMessages        []slack.Message
	peekSelectedIndex   int
	peekScrollOffset    int
	peekThreadVisible   bool
	peekThreadMessages  []slack.Message
	peekThreadTS        string
	peekLoading         bool
	peekLoadingErr      error
	originalChannelID   string
	originalChannelName string
	originalMessages    []slack.Message
	originalScrollOffset int
	originalSelectedIndex int
}

// mentionCandidate represents a user mention candidate
type mentionCandidate struct {
	UserID   string
	UserName string
}

// NotificationItem represents a notification from another channel
type NotificationItem struct {
	ChannelID   string
	ChannelName string
	IsIM        bool
	Count       int
	LastMessage string
	LastUser    string
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
	// Load messages and channel members in parallel
	return tea.Batch(m.loadMessages(), m.loadChannelMembers())
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

// LiveMessageDeletedMsg is sent when a message is deleted
type LiveMessageDeletedMsg struct {
	Timestamp string
	Err       error
}

// LiveMessageEditedMsg is sent when a message is edited
type LiveMessageEditedMsg struct {
	Timestamp string
	NewText   string
	Err       error
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

func (m *LiveModel) deleteMessage(timestamp string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteMessage(m.channelID, timestamp)
		return LiveMessageDeletedMsg{Timestamp: timestamp, Err: err}
	}
}

func (m *LiveModel) editMessage(timestamp, text string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.UpdateMessage(m.channelID, timestamp, text)
		return LiveMessageEditedMsg{Timestamp: timestamp, NewText: text, Err: err}
	}
}

// LiveMembersLoadedMsg is sent when channel members are loaded
type LiveMembersLoadedMsg struct {
	Members   []string
	UserNames map[string]string // userID -> userName
	Err       error
}

func (m *LiveModel) loadChannelMembers() tea.Cmd {
	return func() tea.Msg {
		members, err := m.client.GetChannelMembers(m.channelID, 100)
		if err != nil {
			return LiveMembersLoadedMsg{Members: nil, Err: err}
		}
		// Fetch user info for members not in cache
		userNames := make(map[string]string)
		// Copy existing cache
		for k, v := range m.userCache {
			userNames[k] = v
		}
		// Find uncached users
		var uncached []string
		for _, userID := range members {
			if _, ok := userNames[userID]; !ok {
				uncached = append(uncached, userID)
			}
		}
		// Fetch uncached users
		if len(uncached) > 0 {
			users, err := m.client.GetUsersInfo(uncached)
			if err == nil && users != nil {
				for _, u := range *users {
					entry := cache.CachedUser{
						Name:        u.Name,
						DisplayName: u.Profile.DisplayName,
						RealName:    u.RealName,
					}
					userNames[u.ID] = entry.GetPreferredName(m.displayConfig.NameFormat)
				}
			}
		}
		return LiveMembersLoadedMsg{Members: members, UserNames: userNames, Err: nil}
	}
}

// resolveUserIDs fetches and caches user info for the given user IDs
func (m *LiveModel) resolveUserIDs(userIDs []string) {
	users, err := m.client.GetUsersInfo(userIDs)
	if err != nil || users == nil {
		return
	}
	for _, u := range *users {
		entry := cache.CachedUser{
			Name:        u.Name,
			DisplayName: u.Profile.DisplayName,
			RealName:    u.RealName,
		}
		m.userCache[u.ID] = entry.GetPreferredName(m.displayConfig.NameFormat)
	}
}

// updateMentionCompletion checks the current input and updates mention completion state
func (m *LiveModel) updateMentionCompletion() {
	text := m.inputText.Value()

	// Find the last @ that starts a mention (followed by word characters, not complete)
	mentionStart := -1
	runes := []rune(text)
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]
		if r == '@' {
			mentionStart = i
			break
		}
		// Stop if we hit whitespace (mention is complete or no mention)
		if r == ' ' || r == '\n' || r == '\t' {
			break
		}
	}

	if mentionStart == -1 {
		m.mentionActive = false
		m.mentionCandidates = nil
		return
	}

	// Extract the prefix after @
	prefix := ""
	if mentionStart+1 < len(runes) {
		prefix = string(runes[mentionStart+1:])
	}
	m.mentionPrefix = strings.ToLower(prefix)

	// Build candidates from channel members
	m.mentionCandidates = nil
	for _, userID := range m.channelMembers {
		userName, ok := m.userCache[userID]
		if !ok {
			continue
		}
		// Filter by prefix
		if m.mentionPrefix == "" || strings.HasPrefix(strings.ToLower(userName), m.mentionPrefix) {
			m.mentionCandidates = append(m.mentionCandidates, mentionCandidate{
				UserID:   userID,
				UserName: userName,
			})
		}
		// Limit to 10 candidates
		if len(m.mentionCandidates) >= 10 {
			break
		}
	}

	m.mentionActive = len(m.mentionCandidates) > 0
	if m.mentionActive && m.mentionIndex >= len(m.mentionCandidates) {
		m.mentionIndex = 0
	}
}

// completeMention inserts the selected mention candidate
func (m *LiveModel) completeMention() {
	if !m.mentionActive || len(m.mentionCandidates) == 0 {
		return
	}

	candidate := m.mentionCandidates[m.mentionIndex]
	text := m.inputText.Value()
	runes := []rune(text)

	// Find the last @ that starts a mention
	mentionStart := -1
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == '@' {
			mentionStart = i
			break
		}
		if runes[i] == ' ' || runes[i] == '\n' || runes[i] == '\t' {
			break
		}
	}

	if mentionStart == -1 {
		return
	}

	// Replace @prefix with @username
	newText := string(runes[:mentionStart]) + "@" + candidate.UserName + " "

	m.inputText.SetValue(newText)
	// Move cursor to end
	m.inputText.CursorEnd()

	m.mentionActive = false
	m.mentionCandidates = nil
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

	case LiveMessageDeletedMsg:
		if msg.Err != nil {
			m.loadingErr = msg.Err
		} else {
			// Remove the deleted message from the list
			for i, message := range m.messages {
				if message.Timestamp == msg.Timestamp {
					m.messages = append(m.messages[:i], m.messages[i+1:]...)
					// Adjust selected index if necessary
					if m.selectedIndex >= len(m.messages) && m.selectedIndex > 0 {
						m.selectedIndex--
					}
					break
				}
			}
		}
		return m, nil

	case LiveMessageEditedMsg:
		if msg.Err != nil {
			m.loadingErr = msg.Err
		} else {
			// Update the message text in the list
			for i, message := range m.messages {
				if message.Timestamp == msg.Timestamp {
					m.messages[i].Text = msg.NewText
					break
				}
			}
		}
		return m, nil

	case LiveMembersLoadedMsg:
		if msg.Err != nil {
			m.loadingErr = msg.Err
		} else {
			m.channelMembers = msg.Members
			// Merge user names into cache
			for k, v := range msg.UserNames {
				m.userCache[k] = v
			}
			m.membersLoaded = true
			// Update mention completion now that members are loaded
			if m.inputMode != InputModeNone {
				m.updateMentionCompletion()
			}
		}
		return m, nil

	case PeekMessagesLoadedMsg:
		m.peekLoading = false
		if msg.Err != nil {
			m.peekLoadingErr = msg.Err
		} else {
			m.peekMessages = msg.Messages
			// Select the last (newest) message by default
			if len(m.peekMessages) > 0 {
				m.peekSelectedIndex = len(m.peekMessages) - 1
				m.ensurePeekVisible()
			}
		}
		return m, nil

	case PeekThreadLoadedMsg:
		if msg.Err != nil {
			m.peekLoadingErr = msg.Err
			m.peekThreadVisible = false
		} else {
			m.peekThreadMessages = msg.Messages
			m.peekThreadVisible = true
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.inputText.SetWidth(msg.Width - 20)
		return m, nil

	case tea.KeyMsg:
		// Handle peek mode
		if m.peekMode {
			return m.handlePeekModeKey(msg)
		}

		// Handle notification panel
		if m.showNotifyPanel {
			return m.handleNotifyPanelKey(msg)
		}

		// Handle input mode
		if m.inputMode != InputModeNone {
			// Get send key setting (default to "enter")
			sendKey := m.displayConfig.LiveSendKey
			if sendKey == "" {
				sendKey = "enter"
			}

			// Handle mention completion keys first
			if m.mentionActive {
				switch msg.Type {
				case tea.KeyTab:
					m.completeMention()
					return m, nil
				case tea.KeyUp:
					if m.mentionIndex > 0 {
						m.mentionIndex--
					} else {
						m.mentionIndex = len(m.mentionCandidates) - 1
					}
					return m, nil
				case tea.KeyDown:
					if m.mentionIndex < len(m.mentionCandidates)-1 {
						m.mentionIndex++
					} else {
						m.mentionIndex = 0
					}
					return m, nil
				case tea.KeyEsc:
					m.mentionActive = false
					m.mentionCandidates = nil
					return m, nil
				}
			}

			switch msg.Type {
			case tea.KeyTab:
				// Start or update mention completion when Tab is pressed
				if !m.membersLoaded {
					return m, m.loadChannelMembers()
				}
				m.updateMentionCompletion()
				if m.mentionActive {
					m.completeMention()
				}
				return m, nil
			case tea.KeyEsc:
				m.inputMode = InputModeNone
				m.editTS = ""
				m.mentionActive = false
				m.mentionCandidates = nil
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
						editTS := m.editTS
						m.inputMode = InputModeNone
						m.editTS = ""
						m.inputText.Blur()
						m.inputText.Reset()

						if currentMode == InputModeNewMessage {
							return m, m.sendMessage(text)
						} else if currentMode == InputModeReply {
							return m, m.sendReply(m.threadTS, text)
						} else if currentMode == InputModeEdit {
							return m, m.editMessage(editTS, text)
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
						editTS := m.editTS
						m.inputMode = InputModeNone
						m.editTS = ""
						m.inputText.Blur()
						m.inputText.Reset()

						if currentMode == InputModeNewMessage {
							return m, m.sendMessage(text)
						} else if currentMode == InputModeReply {
							return m, m.sendReply(m.threadTS, text)
						} else if currentMode == InputModeEdit {
							return m, m.editMessage(editTS, text)
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
				// Update mention completion after text changes
				if m.membersLoaded {
					m.updateMentionCompletion()
				} else {
					// Check if @ was typed and load members
					text := m.inputText.Value()
					if strings.Contains(text, "@") {
						return m, m.loadChannelMembers()
					}
				}
				return m, cmd
			}
		}

		// Handle delete confirmation
		if m.deleteConfirm {
			switch msg.String() {
			case "y", "Y":
				m.deleteConfirm = false
				if len(m.messages) > 0 && m.selectedIndex < len(m.messages) {
					selectedMsg := m.messages[m.selectedIndex]
					return m, m.deleteMessage(selectedMsg.Timestamp)
				}
				return m, nil
			case "n", "N", "esc":
				m.deleteConfirm = false
				return m, nil
			}
			return m, nil
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
		case "d":
			// Delete selected message (show confirmation)
			if len(m.messages) > 0 && m.selectedIndex < len(m.messages) {
				selectedMsg := m.messages[m.selectedIndex]
				// Only allow deleting own messages
				if selectedMsg.User == m.client.GetUserID() {
					m.deleteConfirm = true
				}
			}
			return m, nil
		case "e":
			// Edit selected message
			if len(m.messages) > 0 && m.selectedIndex < len(m.messages) {
				selectedMsg := m.messages[m.selectedIndex]
				// Only allow editing own messages
				if selectedMsg.User == m.client.GetUserID() {
					m.editTS = selectedMsg.Timestamp
					m.inputMode = InputModeEdit
					m.inputText.Placeholder = "Edit message..."
					m.inputText.SetValue(selectedMsg.Text)
					m.inputText.Focus()
					return m, textarea.Blink
				}
			}
			return m, nil
		case "n":
			// Toggle notification panel
			if len(m.notifications) > 0 {
				m.showNotifyPanel = true
				m.notifyPanelIndex = 0
			}
			return m, nil
		}
	}

	return m, nil
}

// handlePeekModeKey handles key events in peek mode
func (m *LiveModel) handlePeekModeKey(msg tea.KeyMsg) (*LiveModel, tea.Cmd) {
	// Handle peek thread view
	if m.peekThreadVisible {
		switch msg.String() {
		case "q", "esc":
			m.peekThreadVisible = false
			m.peekThreadMessages = nil
			m.peekThreadTS = ""
			return m, nil
		}
		return m, nil
	}

	// Handle peek main view
	switch msg.String() {
	case "q", "esc":
		// Exit peek mode
		m.exitPeekMode()
		return m, nil
	case "up", "k":
		if m.peekSelectedIndex > 0 {
			m.peekSelectedIndex--
			m.ensurePeekVisible()
		}
		return m, nil
	case "down", "j":
		if m.peekSelectedIndex < len(m.peekMessages)-1 {
			m.peekSelectedIndex++
			m.ensurePeekVisible()
		}
		return m, nil
	case "enter":
		// View thread
		if len(m.peekMessages) > 0 && m.peekSelectedIndex < len(m.peekMessages) {
			selectedMsg := m.peekMessages[m.peekSelectedIndex]
			threadTS := selectedMsg.Timestamp
			if selectedMsg.ThreadTS != "" {
				threadTS = selectedMsg.ThreadTS
			}
			m.peekThreadTS = threadTS
			return m, m.loadPeekThread(threadTS)
		}
		return m, nil
	}
	return m, nil
}

// handleNotifyPanelKey handles key events in notification panel
func (m *LiveModel) handleNotifyPanelKey(msg tea.KeyMsg) (*LiveModel, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "n":
		m.showNotifyPanel = false
		return m, nil
	case "up", "k":
		if m.notifyPanelIndex > 0 {
			m.notifyPanelIndex--
		}
		return m, nil
	case "down", "j":
		if m.notifyPanelIndex < len(m.notifications)-1 {
			m.notifyPanelIndex++
		}
		return m, nil
	case "enter":
		// Enter peek mode for selected notification
		if m.notifyPanelIndex < len(m.notifications) {
			n := m.notifications[m.notifyPanelIndex]
			return m, m.enterPeekMode(n.ChannelID, n.ChannelName, n.IsIM)
		}
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Quick select by number
		idx := int(msg.String()[0] - '1')
		if idx < len(m.notifications) {
			n := m.notifications[idx]
			return m, m.enterPeekMode(n.ChannelID, n.ChannelName, n.IsIM)
		}
		return m, nil
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

func (m *LiveModel) ensurePeekVisible() {
	visibleLines := m.getVisibleLines()

	// If selected message is above the scroll offset, scroll up
	if m.peekSelectedIndex < m.peekScrollOffset {
		m.peekScrollOffset = m.peekSelectedIndex
		return
	}

	// Calculate how many lines are used from scrollOffset to selectedIndex (inclusive)
	linesUsed := m.getPeekTotalLinesInRange(m.peekScrollOffset, m.peekSelectedIndex+1)

	// If selected message doesn't fit, scroll down
	if linesUsed > visibleLines {
		m.peekScrollOffset = m.peekSelectedIndex
		linesNeeded := m.getPeekMessageLineCount(m.peekSelectedIndex)
		for m.peekScrollOffset > 0 && linesNeeded < visibleLines {
			prevLines := m.getPeekMessageLineCount(m.peekScrollOffset - 1)
			if linesNeeded+prevLines <= visibleLines {
				m.peekScrollOffset--
				linesNeeded += prevLines
			} else {
				break
			}
		}
	}
}

func (m *LiveModel) getPeekMessageLineCount(msgIndex int) int {
	if msgIndex < 0 || msgIndex >= len(m.peekMessages) {
		return 1
	}
	truncate := m.displayConfig.LiveTruncateMessages
	lines := m.formatMessageLines(m.peekMessages[msgIndex], msgIndex, truncate)
	return len(lines)
}

func (m *LiveModel) getPeekTotalLinesInRange(startIdx, endIdx int) int {
	total := 0
	for i := startIdx; i < endIdx && i < len(m.peekMessages); i++ {
		total += m.getPeekMessageLineCount(i)
	}
	return total
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
	// Peek mode has its own view
	if m.peekMode {
		return m.renderPeekView()
	}

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
		sb.WriteString(m.renderNotificationBar())
		sb.WriteString(m.renderHelp())
		return sb.String()
	}

	if m.loadingErr != nil {
		sb.WriteString(fmt.Sprintf("\nError: %v\n", m.loadingErr))
		sb.WriteString(m.renderNotificationBar())
		sb.WriteString(m.renderHelp())
		return sb.String()
	}

	if len(m.messages) == 0 {
		sb.WriteString("\nNo messages found.\n")
		sb.WriteString(m.renderNotificationBar())
		sb.WriteString(m.renderHelp())
		return sb.String()
	}

	// Notification panel overlay
	if m.showNotifyPanel {
		sb.WriteString(m.renderNotificationPanel())
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
		switch m.inputMode {
		case InputModeNewMessage:
			sb.WriteString("Message: ")
		case InputModeReply:
			sb.WriteString("Reply: ")
		case InputModeEdit:
			sb.WriteString("Edit: ")
		}
		sb.WriteString(m.inputText.View())
		sb.WriteString("\n")

		// Show mention completion candidates
		if m.mentionActive && len(m.mentionCandidates) > 0 {
			sb.WriteString(m.renderMentionCandidates())
		}
	}

	// Delete confirmation
	if m.deleteConfirm {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true).Render("Delete this message? (y/n)"))
		sb.WriteString("\n")
	}

	// Notification bar
	sb.WriteString(m.renderNotificationBar())

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

func (m *LiveModel) renderMentionCandidates() string {
	var sb strings.Builder
	sb.WriteString(liveHelpStyle.Render("Mention: "))
	for i, c := range m.mentionCandidates {
		name := "@" + c.UserName
		if i == m.mentionIndex {
			sb.WriteString(liveSelectedStyle.Render(name))
		} else {
			sb.WriteString(liveNormalStyle.Render(name))
		}
		if i < len(m.mentionCandidates)-1 {
			sb.WriteString(" ")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(liveHelpStyle.Render("Tab: complete | ↑↓: select | Esc: cancel"))
	sb.WriteString("\n")
	return sb.String()
}

func (m *LiveModel) renderNotificationBar() string {
	if len(m.notifications) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("─────────────────────────────────────────────────────\n")

	// Show the most recent notification
	n := m.notifications[len(m.notifications)-1]
	prefix := "#"
	if n.IsIM {
		prefix = "@"
	}

	// Truncate message preview (use runes for proper multi-byte support)
	preview := n.LastMessage
	previewRunes := []rune(preview)
	if len(previewRunes) > 25 {
		preview = string(previewRunes[:22]) + "..."
	}

	// Format: 📨 #channel (count) | @user: message... [n: 確認]
	totalCount := 0
	for _, notif := range m.notifications {
		totalCount += notif.Count
	}

	barText := fmt.Sprintf("📨 %s%s (%d) | @%s: %s [n: notifications]",
		prefix, n.ChannelName, totalCount, n.LastUser, preview)

	sb.WriteString(liveNotifyBarStyle.Render(barText))
	sb.WriteString("\n")
	sb.WriteString("─────────────────────────────────────────────────────")

	return sb.String()
}

func (m *LiveModel) renderNotificationPanel() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("┌─ Notifications ")
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteString("┐\n")

	for i, n := range m.notifications {
		prefix := "#"
		if n.IsIM {
			prefix = "@"
		}

		// Truncate message preview (use runes for proper multi-byte support)
		preview := n.LastMessage
		previewRunes := []rune(preview)
		if len(previewRunes) > 20 {
			preview = string(previewRunes[:17]) + "..."
		}

		line := fmt.Sprintf(" %d. %s%-12s (%d) @%s: %s",
			i+1, prefix, truncateString(n.ChannelName, 12), n.Count, truncateString(n.LastUser, 10), preview)

		if i == m.notifyPanelIndex {
			sb.WriteString("│" + liveSelectedStyle.Render(padRight(line, 55)) + "│\n")
		} else {
			sb.WriteString("│" + liveNormalStyle.Render(padRight(line, 55)) + "│\n")
		}
	}

	// Fill empty space if fewer than 5 notifications
	for i := len(m.notifications); i < 5; i++ {
		sb.WriteString("│" + strings.Repeat(" ", 55) + "│\n")
	}

	sb.WriteString("│" + strings.Repeat(" ", 55) + "│\n")
	sb.WriteString("│ " + liveHelpStyle.Render("[1-9]: peek  Enter: select  j/k: move  q/Esc: back") + " │\n")
	sb.WriteString("└")
	sb.WriteString(strings.Repeat("─", 55))
	sb.WriteString("┘")

	return sb.String()
}

func (m *LiveModel) renderPeekView() string {
	var sb strings.Builder

	// Header showing peek mode
	prefix := "#"
	if m.peekIsIM {
		prefix = "@"
	}
	header := fmt.Sprintf("Live #%s → Peek %s%s (read-only)", m.originalChannelName, prefix, m.peekChannelName)
	sb.WriteString(livePeekHeaderStyle.Render(header))
	sb.WriteString("\n")

	if m.peekLoading {
		sb.WriteString("\nLoading messages...\n")
		sb.WriteString(m.renderPeekHelp())
		return sb.String()
	}

	if m.peekLoadingErr != nil {
		sb.WriteString(fmt.Sprintf("\nError: %v\n", m.peekLoadingErr))
		sb.WriteString(m.renderPeekHelp())
		return sb.String()
	}

	if len(m.peekMessages) == 0 {
		sb.WriteString("\nNo messages found.\n")
		sb.WriteString(m.renderPeekHelp())
		return sb.String()
	}

	// Thread view in peek mode
	if m.peekThreadVisible {
		sb.WriteString(m.renderPeekThread())
	} else {
		// Main message list in peek mode
		sb.WriteString(m.renderPeekMessageList())
	}

	sb.WriteString(m.renderPeekHelp())
	return sb.String()
}

func (m *LiveModel) renderPeekMessageList() string {
	var sb strings.Builder

	visibleLines := m.getVisibleLines()
	truncate := m.displayConfig.LiveTruncateMessages

	// Render messages starting from scrollOffset, counting lines
	linesRendered := 0
	endIdx := m.peekScrollOffset

	for i := m.peekScrollOffset; i < len(m.peekMessages) && linesRendered < visibleLines; i++ {
		msg := m.peekMessages[i]
		lines := m.formatMessageLines(msg, i, truncate)

		for _, line := range lines {
			if linesRendered >= visibleLines {
				break
			}

			if i == m.peekSelectedIndex {
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
	totalMessages := len(m.peekMessages)
	if totalMessages > 0 {
		sb.WriteString(fmt.Sprintf("\n[%d-%d of %d messages]",
			m.peekScrollOffset+1, endIdx, totalMessages))
	}

	return sb.String()
}

func (m *LiveModel) renderPeekThread() string {
	var sb strings.Builder

	if len(m.peekThreadMessages) == 0 {
		sb.WriteString("\nNo replies in this thread.\n")
		return sb.String()
	}

	sb.WriteString("\n")
	for i, msg := range m.peekThreadMessages {
		lines := m.formatMessageLines(msg, i, false)
		for _, line := range lines {
			if i == 0 {
				sb.WriteString(liveNormalStyle.Render(line))
			} else {
				sb.WriteString(liveThreadStyle.Render("  " + line))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (m *LiveModel) renderPeekHelp() string {
	var help string
	if m.peekThreadVisible {
		help = "q/Esc: back to peek list"
	} else {
		help = "j/k: move | Enter: view thread | q/Esc: back to #" + m.originalChannelName
	}
	return "\n" + liveHelpStyle.Render(help)
}

// Helper functions for string formatting
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-1]) + "…"
	}
	return s
}

func padRight(s string, length int) string {
	runes := []rune(s)
	if len(runes) >= length {
		return string(runes[:length])
	}
	return s + strings.Repeat(" ", length-len(runes))
}

func (m *LiveModel) renderHelp() string {
	var help string
	if m.deleteConfirm {
		help = "y: confirm delete | n/Esc: cancel"
	} else if m.inputMode != InputModeNone {
		sendKey := m.displayConfig.LiveSendKey
		if sendKey == "" {
			sendKey = "enter"
		}
		if sendKey == "ctrl+enter" {
			help = "Ctrl+Enter: send | Enter: newline | Esc: cancel"
		} else {
			help = "Enter: send | Shift+Enter: newline | Esc: cancel"
		}
	} else if m.showNotifyPanel {
		help = "[1-9]: peek | Enter: select | j/k: move | q/Esc: close"
	} else if m.threadVisible {
		help = "r: reply | q/Esc: back | j/k: scroll"
	} else {
		help = "i: message | Enter: thread | r: reply | e: edit | d: delete | R: reload | j/k: nav"
		if len(m.notifications) > 0 {
			help += " | n: notifications"
		}
		help += " | q: exit"
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

// AddPeekIncomingMessage adds a message to the peek view if in peek mode
func (m *LiveModel) AddPeekIncomingMessage(channelID, userID, userName, text, timestamp, threadTS string) {
	// Only add if in peek mode and message is for the peek channel
	if !m.peekMode || channelID != m.peekChannelID {
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

	// If this is a thread reply to the currently viewed peek thread
	if m.peekThreadVisible && threadTS != "" && threadTS == m.peekThreadTS {
		m.peekThreadMessages = append(m.peekThreadMessages, newMsg)
		return
	}

	// If this is a main channel message
	if threadTS == "" || threadTS == timestamp {
		m.peekMessages = append(m.peekMessages, newMsg)
		// Auto-scroll to the newest message if already at the bottom
		if m.peekSelectedIndex == len(m.peekMessages)-2 {
			m.peekSelectedIndex = len(m.peekMessages) - 1
			m.ensurePeekVisible()
		}
	}
}

// GetChannelID returns the channel ID for this live model
func (m *LiveModel) GetChannelID() string {
	return m.channelID
}

// ShouldExit returns true if the user wants to exit live mode
func (m *LiveModel) ShouldExit(msg tea.KeyMsg) bool {
	// Only exit on 'q' when not in input mode, not in thread view, not confirming delete,
	// not in peek mode, and not showing notification panel
	if m.inputMode != InputModeNone || m.threadVisible || m.deleteConfirm || m.peekMode || m.showNotifyPanel {
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

// IsDeleteConfirm returns true if delete confirmation is shown
func (m *LiveModel) IsDeleteConfirm() bool {
	return m.deleteConfirm
}

// AddNotification adds or updates a notification from another channel
func (m *LiveModel) AddNotification(item NotificationItem) {
	// Check if notification for this channel already exists
	for i, n := range m.notifications {
		if n.ChannelID == item.ChannelID {
			m.notifications[i].Count++
			m.notifications[i].LastMessage = item.LastMessage
			m.notifications[i].LastUser = item.LastUser
			return
		}
	}
	// Add new notification
	item.Count = 1
	m.notifications = append(m.notifications, item)
}

// ClearNotification removes a notification for a specific channel
func (m *LiveModel) ClearNotification(channelID string) {
	for i, n := range m.notifications {
		if n.ChannelID == channelID {
			m.notifications = append(m.notifications[:i], m.notifications[i+1:]...)
			return
		}
	}
}

// HasNotifications returns true if there are pending notifications
func (m *LiveModel) HasNotifications() bool {
	return len(m.notifications) > 0
}

// IsPeekMode returns true if live model is in peek mode
func (m *LiveModel) IsPeekMode() bool {
	return m.peekMode
}

// IsNotifyPanelVisible returns true if notification panel is visible
func (m *LiveModel) IsNotifyPanelVisible() bool {
	return m.showNotifyPanel
}

// PeekMessagesLoadedMsg is sent when peek mode messages are loaded
type PeekMessagesLoadedMsg struct {
	Messages []slack.Message
	HasMore  bool
	Err      error
}

// PeekThreadLoadedMsg is sent when peek mode thread is loaded
type PeekThreadLoadedMsg struct {
	Messages []slack.Message
	Err      error
}

// PeekModeEnteredMsg is sent when entering peek mode
type PeekModeEnteredMsg struct {
	ChannelID string
}

func (m *LiveModel) loadPeekMessages() tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.GetMessagesWithPagination(m.peekChannelID, 50, "")
		if err != nil {
			return PeekMessagesLoadedMsg{Messages: nil, HasMore: false, Err: err}
		}
		// Resolve user names
		m.resolveUserNames(result.Messages)
		return PeekMessagesLoadedMsg{Messages: result.Messages, HasMore: result.HasMore, Err: nil}
	}
}

func (m *LiveModel) loadPeekThread(threadTS string) tea.Cmd {
	return func() tea.Msg {
		messages, err := m.client.GetThreadReplies(m.peekChannelID, threadTS)
		if err == nil {
			m.resolveUserNames(messages)
		}
		return PeekThreadLoadedMsg{Messages: messages, Err: err}
	}
}

// enterPeekMode enters peek mode for the specified channel
func (m *LiveModel) enterPeekMode(channelID, channelName string, isIM bool) tea.Cmd {
	// Save original state
	m.originalChannelID = m.channelID
	m.originalChannelName = m.channelName
	m.originalMessages = m.messages
	m.originalScrollOffset = m.scrollOffset
	m.originalSelectedIndex = m.selectedIndex

	// Set up peek mode
	m.peekMode = true
	m.peekChannelID = channelID
	m.peekChannelName = channelName
	m.peekIsIM = isIM
	m.peekLoading = true
	m.peekLoadingErr = nil
	m.peekMessages = nil
	m.peekSelectedIndex = 0
	m.peekScrollOffset = 0
	m.peekThreadVisible = false
	m.showNotifyPanel = false

	// Clear the notification for this channel
	m.ClearNotification(channelID)

	// Send notification to parent model to clear unread, and load messages
	return tea.Batch(
		func() tea.Msg {
			return PeekModeEnteredMsg{ChannelID: channelID}
		},
		m.loadPeekMessages(),
	)
}

// exitPeekMode exits peek mode and restores original state
func (m *LiveModel) exitPeekMode() {
	m.peekMode = false
	m.peekChannelID = ""
	m.peekChannelName = ""
	m.peekMessages = nil
	m.peekThreadVisible = false
	m.peekThreadMessages = nil
	m.peekLoading = false
	m.peekLoadingErr = nil

	// Restore original state
	m.messages = m.originalMessages
	m.scrollOffset = m.originalScrollOffset
	m.selectedIndex = m.originalSelectedIndex
	m.originalMessages = nil
}

// GetPeekChannelID returns the peek channel ID if in peek mode
func (m *LiveModel) GetPeekChannelID() string {
	if m.peekMode {
		return m.peekChannelID
	}
	return ""
}
