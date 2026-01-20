package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-tui/internal/keymap"
	"github.com/polidog/slack-tui/internal/slack"
	"github.com/polidog/slack-tui/internal/ui/styles"
)

type SidebarSection int

const (
	SectionChannels SidebarSection = iota
	SectionDMs
)

type SidebarModel struct {
	keymap        *keymap.Keymap
	channels      []slack.Channel
	dms           []slack.Channel
	selectedIndex int
	section       SidebarSection
	width         int
	height        int
	focused       bool
	userCache     map[string]string // userID -> userName
}

func NewSidebarModel(km *keymap.Keymap) SidebarModel {
	return SidebarModel{
		keymap:    km,
		channels:  []slack.Channel{},
		dms:       []slack.Channel{},
		userCache: make(map[string]string),
	}
}

func (m SidebarModel) Init() tea.Cmd {
	return nil
}

func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		if m.keymap.MatchKey(msg, keymap.ActionUp) {
			m.selectedIndex--
			if m.selectedIndex < 0 {
				// Move to previous section
				if m.section == SectionDMs && len(m.channels) > 0 {
					m.section = SectionChannels
					m.selectedIndex = len(m.channels) - 1
				} else {
					m.selectedIndex = 0
				}
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionDown) {
			currentList := m.getCurrentList()
			m.selectedIndex++
			if m.selectedIndex >= len(currentList) {
				// Move to next section
				if m.section == SectionChannels && len(m.dms) > 0 {
					m.section = SectionDMs
					m.selectedIndex = 0
				} else {
					m.selectedIndex = len(currentList) - 1
				}
			}
		} else if m.keymap.MatchKey(msg, keymap.ActionTop) {
			m.section = SectionChannels
			m.selectedIndex = 0
		} else if m.keymap.MatchKey(msg, keymap.ActionBottom) {
			if len(m.dms) > 0 {
				m.section = SectionDMs
				m.selectedIndex = len(m.dms) - 1
			} else if len(m.channels) > 0 {
				m.selectedIndex = len(m.channels) - 1
			}
		}
	}

	return m, nil
}

func (m SidebarModel) View() string {
	var b strings.Builder

	// Channels section
	b.WriteString(styles.SidebarHeaderStyle.Render("Channels"))
	b.WriteString("\n")

	for i, ch := range m.channels {
		icon := styles.ChannelIcon(ch.IsPrivate)
		name := fmt.Sprintf("%s %s", icon, ch.Name)

		var style lipgloss.Style
		if m.focused && m.section == SectionChannels && i == m.selectedIndex {
			style = styles.ChannelSelectedStyle
		} else {
			style = styles.ChannelStyle
		}

		b.WriteString(style.Width(m.width - 4).Render(name))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// DMs section
	b.WriteString(styles.SidebarHeaderStyle.Render("Direct Messages"))
	b.WriteString("\n")

	for i, dm := range m.dms {
		name := dm.Name
		if userName, ok := m.userCache[dm.UserID]; ok {
			name = userName
		}
		displayName := fmt.Sprintf("%s %s", styles.DMIcon(), name)

		var style lipgloss.Style
		if m.focused && m.section == SectionDMs && i == m.selectedIndex {
			style = styles.ChannelSelectedStyle
		} else {
			style = styles.ChannelStyle
		}

		b.WriteString(style.Width(m.width - 4).Render(displayName))
		b.WriteString("\n")
	}

	content := b.String()
	return styles.SidebarStyle.Width(m.width).Height(m.height).Render(content)
}

func (m SidebarModel) getCurrentList() []slack.Channel {
	if m.section == SectionChannels {
		return m.channels
	}
	return m.dms
}

func (m SidebarModel) GetSelectedChannel() *slack.Channel {
	list := m.getCurrentList()
	if m.selectedIndex >= 0 && m.selectedIndex < len(list) {
		return &list[m.selectedIndex]
	}
	return nil
}

func (m *SidebarModel) SetChannels(channels []slack.Channel) {
	m.channels = channels
}

func (m *SidebarModel) SetDMs(dms []slack.Channel) {
	m.dms = dms
}

func (m *SidebarModel) SetUserCache(cache map[string]string) {
	m.userCache = cache
}

func (m *SidebarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *SidebarModel) SetFocused(focused bool) {
	m.focused = focused
}

func (m SidebarModel) IsFocused() bool {
	return m.focused
}
