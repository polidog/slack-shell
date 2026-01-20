package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/polidog/slack-shell/internal/keymap"
	"github.com/polidog/slack-shell/internal/slack"
	"github.com/polidog/slack-shell/internal/ui/styles"
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
	scrollOffset  int
	width         int
	height        int
	focused       bool
	userCache     map[string]string // userID -> userName

	// Search
	searchMode    bool
	searchQuery   string
	filteredChans []slack.Channel
	filteredDMs   []slack.Channel
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

		// Search mode handling
		if m.searchMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.searchMode = false
				m.searchQuery = ""
				m.updateFilteredLists()
				m.selectedIndex = 0
				m.scrollOffset = 0
				return m, nil
			case tea.KeyBackspace:
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.updateFilteredLists()
					m.selectedIndex = 0
					m.scrollOffset = 0
				}
				return m, nil
			case tea.KeyEnter:
				// Exit search mode but keep filter
				m.searchMode = false
				return m, nil
			case tea.KeyRunes:
				m.searchQuery += string(msg.Runes)
				m.updateFilteredLists()
				m.selectedIndex = 0
				m.scrollOffset = 0
				return m, nil
			}

			// Navigation in search mode
			if m.keymap.MatchKey(msg, keymap.ActionUp) {
				m.moveUp()
			} else if m.keymap.MatchKey(msg, keymap.ActionDown) {
				m.moveDown()
			}
			return m, nil
		}

		// Normal mode
		if m.keymap.MatchKey(msg, keymap.ActionSearch) {
			m.searchMode = true
			m.searchQuery = ""
			m.updateFilteredLists()
			return m, nil
		}

		if m.keymap.MatchKey(msg, keymap.ActionUp) {
			m.moveUp()
		} else if m.keymap.MatchKey(msg, keymap.ActionDown) {
			m.moveDown()
		} else if m.keymap.MatchKey(msg, keymap.ActionTop) {
			m.section = SectionChannels
			m.selectedIndex = 0
			m.scrollOffset = 0
		} else if m.keymap.MatchKey(msg, keymap.ActionBottom) {
			dms := m.getDisplayDMs()
			chans := m.getDisplayChannels()
			if len(dms) > 0 {
				m.section = SectionDMs
				m.selectedIndex = len(dms) - 1
			} else if len(chans) > 0 {
				m.selectedIndex = len(chans) - 1
			}
			m.ensureVisible()
		}
	}

	return m, nil
}

func (m *SidebarModel) moveUp() {
	m.selectedIndex--
	if m.selectedIndex < 0 {
		chans := m.getDisplayChannels()
		if m.section == SectionDMs && len(chans) > 0 {
			m.section = SectionChannels
			m.selectedIndex = len(chans) - 1
		} else {
			m.selectedIndex = 0
		}
	}
	m.ensureVisible()
}

func (m *SidebarModel) moveDown() {
	currentList := m.getCurrentList()
	m.selectedIndex++
	if m.selectedIndex >= len(currentList) {
		dms := m.getDisplayDMs()
		chans := m.getDisplayChannels()
		if m.section == SectionChannels && len(dms) > 0 {
			m.section = SectionDMs
			m.selectedIndex = 0
		} else {
			if m.section == SectionChannels {
				m.selectedIndex = len(chans) - 1
			} else {
				m.selectedIndex = len(dms) - 1
			}
			if m.selectedIndex < 0 {
				m.selectedIndex = 0
			}
		}
	}
	m.ensureVisible()
}

func (m *SidebarModel) updateFilteredLists() {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		m.filteredChans = nil
		m.filteredDMs = nil
		return
	}

	m.filteredChans = []slack.Channel{}
	for _, ch := range m.channels {
		if strings.Contains(strings.ToLower(ch.Name), query) {
			m.filteredChans = append(m.filteredChans, ch)
		}
	}

	m.filteredDMs = []slack.Channel{}
	for _, dm := range m.dms {
		name := dm.Name
		if userName, ok := m.userCache[dm.UserID]; ok {
			name = userName
		}
		if strings.Contains(strings.ToLower(name), query) {
			m.filteredDMs = append(m.filteredDMs, dm)
		}
	}
}

func (m SidebarModel) getDisplayChannels() []slack.Channel {
	if m.searchQuery != "" && m.filteredChans != nil {
		return m.filteredChans
	}
	return m.channels
}

func (m SidebarModel) getDisplayDMs() []slack.Channel {
	if m.searchQuery != "" && m.filteredDMs != nil {
		return m.filteredDMs
	}
	return m.dms
}

// ensureVisible adjusts scroll offset to keep selected item visible
func (m *SidebarModel) ensureVisible() {
	// Use roughly half the height as visible lines
	// This accounts for styling, padding, borders etc.
	visibleLines := m.height / 2
	if m.searchMode || m.searchQuery != "" {
		visibleLines -= 1
	}
	if visibleLines < 5 {
		visibleLines = 5
	}

	absolutePos := m.getAbsolutePosition()

	if absolutePos < m.scrollOffset {
		m.scrollOffset = absolutePos
	} else if absolutePos >= m.scrollOffset+visibleLines {
		m.scrollOffset = absolutePos - visibleLines + 1
	}

	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m SidebarModel) getAbsolutePosition() int {
	chans := m.getDisplayChannels()
	if m.section == SectionChannels {
		return m.selectedIndex + 1 // +1 for header
	}
	return len(chans) + 3 + m.selectedIndex
}

func (m SidebarModel) View() string {
	var lines []string

	// Search bar
	if m.searchMode || m.searchQuery != "" {
		searchStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Padding(0, 1)
		cursor := ""
		if m.searchMode {
			cursor = "▏"
		}
		searchBar := searchStyle.Width(m.width - 4).Render("/" + m.searchQuery + cursor)
		lines = append(lines, searchBar)
	}

	chans := m.getDisplayChannels()
	dms := m.getDisplayDMs()

	// Channels section
	lines = append(lines, styles.SidebarHeaderStyle.Render(fmt.Sprintf("Channels (%d)", len(chans))))

	for i, ch := range chans {
		icon := styles.ChannelIcon(ch.IsPrivate)
		name := fmt.Sprintf("%s %s", icon, ch.Name)

		var style lipgloss.Style
		if m.focused && m.section == SectionChannels && i == m.selectedIndex {
			style = styles.ChannelSelectedStyle
		} else {
			style = styles.ChannelStyle
		}

		lines = append(lines, style.Width(m.width-4).Render(name))
	}

	lines = append(lines, "")

	// DMs section
	lines = append(lines, styles.SidebarHeaderStyle.Render(fmt.Sprintf("Direct Messages (%d)", len(dms))))

	for i, dm := range dms {
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

		lines = append(lines, style.Width(m.width-4).Render(displayName))
	}

	// Apply scroll offset
	// Account for: padding (2), border (1), headers (2), some margin
	visibleLines := m.height - 8
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Don't apply scroll, just limit the number of visible lines
	// The sidebar content should fit within the available height
	totalLines := len(lines)

	start := m.scrollOffset
	if start < 0 {
		start = 0
	}
	if start > totalLines-visibleLines {
		start = totalLines - visibleLines
		if start < 0 {
			start = 0
		}
	}

	end := start + visibleLines
	if end > totalLines {
		end = totalLines
	}

	var visibleContent string
	if start < end && start < len(lines) {
		visibleContent = strings.Join(lines[start:end], "\n")
	} else if len(lines) > 0 {
		visibleContent = strings.Join(lines, "\n")
	}

	// Use MaxHeight instead of Height to prevent bottom-alignment
	return styles.SidebarStyle.Width(m.width).MaxHeight(m.height).Render(visibleContent)
}

func (m SidebarModel) getCurrentList() []slack.Channel {
	if m.section == SectionChannels {
		return m.getDisplayChannels()
	}
	return m.getDisplayDMs()
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
	m.updateFilteredLists()
}

func (m *SidebarModel) SetDMs(dms []slack.Channel) {
	m.dms = dms
	m.updateFilteredLists()
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

func (m SidebarModel) IsSearchMode() bool {
	return m.searchMode
}

func (m SidebarModel) DebugInfo() string {
	absPos := m.getAbsolutePosition()
	visLines := m.height - 8
	return fmt.Sprintf("idx:%d,abs:%d,scr:%d,vis:%d", m.selectedIndex, absPos, m.scrollOffset, visLines)
}
