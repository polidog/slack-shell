package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-shell/internal/keymap"
	"github.com/polidog/slack-shell/internal/slack"
	"github.com/polidog/slack-shell/internal/ui/styles"
)

type MessagesModel struct {
	keymap        *keymap.Keymap
	messages      []slack.Message
	selectedIndex int
	scrollOffset  int
	width         int
	height        int
	focused       bool
	userCache     map[string]string // userID -> userName
	channelName   string
}

func NewMessagesModel(km *keymap.Keymap) MessagesModel {
	return MessagesModel{
		keymap:    km,
		messages:  []slack.Message{},
		userCache: make(map[string]string),
	}
}

func (m MessagesModel) Init() tea.Cmd {
	return nil
}

func (m MessagesModel) Update(msg tea.Msg) (MessagesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
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
				visibleLines := m.height - 4 // Account for borders and header
				if m.selectedIndex >= m.scrollOffset+visibleLines {
					m.scrollOffset = m.selectedIndex - visibleLines + 1
				}
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionTop) {
			m.selectedIndex = 0
			m.scrollOffset = 0
		} else if m.keymap.MatchKey(msg, keymap.ActionBottom) {
			m.selectedIndex = len(m.messages) - 1
			visibleLines := m.height - 4
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
			visibleLines := m.height - 4
			if m.selectedIndex >= m.scrollOffset+visibleLines {
				m.scrollOffset = m.selectedIndex - visibleLines + 1
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionPageUp) {
			jump := m.height - 4
			m.selectedIndex -= jump
			if m.selectedIndex < 0 {
				m.selectedIndex = 0
			}
			m.scrollOffset -= jump
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionPageDown) {
			jump := m.height - 4
			m.selectedIndex += jump
			if m.selectedIndex >= len(m.messages) {
				m.selectedIndex = len(m.messages) - 1
			}
			visibleLines := m.height - 4
			if m.selectedIndex >= m.scrollOffset+visibleLines {
				m.scrollOffset = m.selectedIndex - visibleLines + 1
			}
		}
	}

	return m, nil
}

func (m MessagesModel) View() string {
	var b strings.Builder

	// Header
	header := fmt.Sprintf("# %s", m.channelName)
	b.WriteString(styles.MessageHeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", m.width-4))
	b.WriteString("\n")

	if len(m.messages) == 0 {
		b.WriteString(styles.HelpStyle.Render("No messages yet"))
	} else {
		visibleLines := m.height - 6 // Account for header and borders
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

	return styles.MessagesStyle.Width(m.width).Height(m.height).Render(b.String())
}

func (m MessagesModel) renderMessage(msg slack.Message, selected bool) string {
	var b strings.Builder

	// User name
	userName := msg.User
	if name, ok := m.userCache[msg.User]; ok {
		userName = name
	}
	if msg.IsBot && msg.BotID != "" {
		userName = fmt.Sprintf("%s (bot)", userName)
	}

	// Timestamp
	ts := m.parseTimestamp(msg.Timestamp)
	timeStr := ts.Format("15:04")

	// Header line
	headerStyle := styles.MessageHeaderStyle
	if selected && m.focused {
		headerStyle = headerStyle.Background(styles.Highlight)
	}

	header := headerStyle.Render(userName) + styles.MessageTimeStyle.Render(timeStr)
	b.WriteString(header)
	b.WriteString("\n")

	// Message text
	textStyle := styles.MessageTextStyle
	if selected && m.focused {
		textStyle = textStyle.Background(styles.Highlight)
	}

	// Wrap text to fit width
	maxWidth := m.width - 8
	text := m.wrapText(msg.Text, maxWidth)
	b.WriteString(textStyle.Render(text))

	// Thread indicator
	if msg.ReplyCount > 0 {
		b.WriteString("\n")
		threadText := fmt.Sprintf("  💬 %d replies", msg.ReplyCount)
		b.WriteString(styles.MessageThreadStyle.Render(threadText))
	}

	// Reactions
	if len(msg.Reactions) > 0 {
		b.WriteString("\n")
		var reactions []string
		for _, r := range msg.Reactions {
			reactions = append(reactions, styles.ReactionStyle.Render(fmt.Sprintf(":%s: %d", r.Name, r.Count)))
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, reactions...))
	}

	return b.String()
}

func (m MessagesModel) parseTimestamp(ts string) time.Time {
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	return time.Unix(sec, 0)
}

func (m MessagesModel) wrapText(text string, maxWidth int) string {
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

func (m *MessagesModel) SetMessages(messages []slack.Message) {
	m.messages = messages
	if m.selectedIndex >= len(messages) {
		m.selectedIndex = len(messages) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
}

func (m *MessagesModel) AppendMessage(msg slack.Message) {
	m.messages = append(m.messages, msg)
}

func (m *MessagesModel) SetUserCache(cache map[string]string) {
	m.userCache = cache
}

func (m *MessagesModel) SetChannelName(name string) {
	m.channelName = name
}

func (m *MessagesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *MessagesModel) SetFocused(focused bool) {
	m.focused = focused
}

func (m MessagesModel) IsFocused() bool {
	return m.focused
}

func (m MessagesModel) GetSelectedMessage() *slack.Message {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.messages) {
		return &m.messages[m.selectedIndex]
	}
	return nil
}

func (m *MessagesModel) ScrollToBottom() {
	m.selectedIndex = len(m.messages) - 1
	visibleLines := m.height - 6
	if len(m.messages) > visibleLines {
		m.scrollOffset = len(m.messages) - visibleLines
	}
}
