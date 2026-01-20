package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/polidog/slack-tui/internal/keymap"
	"github.com/polidog/slack-tui/internal/slack"
	"github.com/polidog/slack-tui/internal/ui/styles"
)

type ThreadModel struct {
	keymap        *keymap.Keymap
	messages      []slack.Message
	selectedIndex int
	scrollOffset  int
	width         int
	height        int
	visible       bool
	focused       bool
	userCache     map[string]string
	parentMessage *slack.Message
}

func NewThreadModel(km *keymap.Keymap) ThreadModel {
	return ThreadModel{
		keymap:    km,
		messages:  []slack.Message{},
		userCache: make(map[string]string),
	}
}

func (m ThreadModel) Init() tea.Cmd {
	return nil
}

func (m ThreadModel) Update(msg tea.Msg) (ThreadModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused || !m.visible {
			return m, nil
		}

		if m.keymap.MatchKey(msg, keymap.ActionUp) {
			if m.selectedIndex > 0 {
				m.selectedIndex--
				if m.selectedIndex < m.scrollOffset {
					m.scrollOffset = m.selectedIndex
				}
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionDown) {
			if m.selectedIndex < len(m.messages)-1 {
				m.selectedIndex++
				visibleLines := m.height - 6
				if m.selectedIndex >= m.scrollOffset+visibleLines {
					m.scrollOffset = m.selectedIndex - visibleLines + 1
				}
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionTop) {
			m.selectedIndex = 0
			m.scrollOffset = 0
		} else if m.keymap.MatchKey(msg, keymap.ActionBottom) {
			m.selectedIndex = len(m.messages) - 1
			visibleLines := m.height - 6
			if len(m.messages) > visibleLines {
				m.scrollOffset = len(m.messages) - visibleLines
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionHalfUp) {
			jump := m.height / 2
			m.selectedIndex -= jump
			if m.selectedIndex < 0 {
				m.selectedIndex = 0
			}
			m.scrollOffset -= jump
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionHalfDown) {
			jump := m.height / 2
			m.selectedIndex += jump
			if m.selectedIndex >= len(m.messages) {
				m.selectedIndex = len(m.messages) - 1
			}
			visibleLines := m.height - 6
			if m.selectedIndex >= m.scrollOffset+visibleLines {
				m.scrollOffset = m.selectedIndex - visibleLines + 1
			}
		}
	}

	return m, nil
}

func (m ThreadModel) View() string {
	if !m.visible {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(styles.ThreadHeaderStyle.Render("Thread"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", m.width-4))
	b.WriteString("\n")

	if len(m.messages) == 0 {
		b.WriteString(styles.HelpStyle.Render("No replies yet"))
	} else {
		visibleLines := m.height - 8
		endIndex := m.scrollOffset + visibleLines
		if endIndex > len(m.messages) {
			endIndex = len(m.messages)
		}

		for i := m.scrollOffset; i < endIndex; i++ {
			msg := m.messages[i]
			b.WriteString(m.renderMessage(msg, i == m.selectedIndex))
			if i < endIndex-1 {
				b.WriteString("\n")
			}
		}
	}

	return styles.ThreadPanelStyle.Width(m.width).Height(m.height).Render(b.String())
}

func (m ThreadModel) renderMessage(msg slack.Message, selected bool) string {
	var b strings.Builder

	userName := msg.User
	if name, ok := m.userCache[msg.User]; ok {
		userName = name
	}

	ts := m.parseTimestamp(msg.Timestamp)
	timeStr := ts.Format("15:04")

	headerStyle := styles.MessageHeaderStyle
	if selected && m.focused {
		headerStyle = headerStyle.Background(styles.Highlight)
	}

	header := headerStyle.Render(userName) + styles.MessageTimeStyle.Render(timeStr)
	b.WriteString(header)
	b.WriteString("\n")

	textStyle := styles.MessageTextStyle
	if selected && m.focused {
		textStyle = textStyle.Background(styles.Highlight)
	}

	maxWidth := m.width - 8
	text := m.wrapText(msg.Text, maxWidth)
	b.WriteString(textStyle.Render(text))

	return b.String()
}

func (m ThreadModel) parseTimestamp(ts string) time.Time {
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	return time.Unix(sec, 0)
}

func (m ThreadModel) wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		if lineLen+wordLen+1 > maxWidth && lineLen > 0 {
			result.WriteString("\n")
			lineLen = 0
		}

		if lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}

		result.WriteString(word)
		lineLen += wordLen

		_ = i
	}

	return result.String()
}

func (m *ThreadModel) SetMessages(messages []slack.Message) {
	m.messages = messages
	m.selectedIndex = 0
	m.scrollOffset = 0
}

func (m *ThreadModel) SetParentMessage(msg *slack.Message) {
	m.parentMessage = msg
}

func (m ThreadModel) GetParentMessage() *slack.Message {
	return m.parentMessage
}

func (m *ThreadModel) SetUserCache(cache map[string]string) {
	m.userCache = cache
}

func (m *ThreadModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ThreadModel) SetVisible(visible bool) {
	m.visible = visible
}

func (m ThreadModel) IsVisible() bool {
	return m.visible
}

func (m *ThreadModel) SetFocused(focused bool) {
	m.focused = focused
}

func (m ThreadModel) IsFocused() bool {
	return m.focused
}

func (m *ThreadModel) Show(parentMsg *slack.Message, replies []slack.Message) {
	m.parentMessage = parentMsg
	m.messages = replies
	m.visible = true
	m.selectedIndex = 0
	m.scrollOffset = 0
}

func (m *ThreadModel) Hide() {
	m.visible = false
	m.focused = false
}

func (m ThreadModel) GetThreadTS() string {
	if m.parentMessage != nil {
		if m.parentMessage.ThreadTS != "" {
			return m.parentMessage.ThreadTS
		}
		return m.parentMessage.Timestamp
	}
	return ""
}

func (m *ThreadModel) AppendMessage(msg slack.Message) {
	m.messages = append(m.messages, msg)
}

func (m ThreadModel) String() string {
	return fmt.Sprintf("ThreadModel{visible: %v, messages: %d}", m.visible, len(m.messages))
}
