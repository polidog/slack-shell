package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	Primary     = lipgloss.Color("#4A154B") // Slack purple
	Secondary   = lipgloss.Color("#36C5F0") // Slack blue
	Accent      = lipgloss.Color("#2EB67D") // Slack green
	Warning     = lipgloss.Color("#ECB22E") // Slack yellow
	Error       = lipgloss.Color("#E01E5A") // Slack red
	Muted       = lipgloss.Color("#616061")
	Background  = lipgloss.Color("#1A1D21")
	Surface     = lipgloss.Color("#222529")
	Border      = lipgloss.Color("#383838")
	Text        = lipgloss.Color("#D1D2D3")
	TextMuted   = lipgloss.Color("#9B9B9B")
	Highlight   = lipgloss.Color("#1264A3")

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Background(Background).
			Foreground(Text)

	// Sidebar styles
	SidebarStyle = lipgloss.NewStyle().
			Background(Surface).
			Padding(1, 2).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(Border).
			BorderRight(true)

	SidebarHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Text).
				MarginBottom(1)

	ChannelStyle = lipgloss.NewStyle().
			Foreground(Text).
			PaddingLeft(1)

	ChannelSelectedStyle = lipgloss.NewStyle().
				Foreground(Text).
				Background(Highlight).
				Bold(true).
				PaddingLeft(1)

	ChannelUnreadStyle = lipgloss.NewStyle().
				Foreground(Text).
				Bold(true).
				PaddingLeft(1)

	// Messages styles
	MessagesStyle = lipgloss.NewStyle().
			Padding(1, 2)

	MessageHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Text)

	MessageTimeStyle = lipgloss.NewStyle().
				Foreground(TextMuted).
				MarginLeft(1)

	MessageTextStyle = lipgloss.NewStyle().
				Foreground(Text)

	MessageThreadStyle = lipgloss.NewStyle().
				Foreground(Secondary).
				Italic(true)

	ReactionStyle = lipgloss.NewStyle().
			Background(Surface).
			Foreground(Text).
			Padding(0, 1).
			MarginRight(1)

	// Input styles
	InputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(Secondary).
				Padding(0, 1)

	// Thread panel styles
	ThreadPanelStyle = lipgloss.NewStyle().
				Background(Surface).
				Padding(1, 2).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(Border).
				BorderLeft(true)

	ThreadHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Text).
				MarginBottom(1)

	// Status bar styles
	StatusBarStyle = lipgloss.NewStyle().
			Background(Primary).
			Foreground(Text).
			Padding(0, 1)

	StatusConnectedStyle = lipgloss.NewStyle().
				Foreground(Accent)

	StatusDisconnectedStyle = lipgloss.NewStyle().
				Foreground(Error)

	// Help styles
	HelpStyle = lipgloss.NewStyle().
			Foreground(TextMuted)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Secondary).
			Bold(true)
)

func ChannelIcon(isPrivate bool) string {
	if isPrivate {
		return "🔒"
	}
	return "#"
}

func DMIcon() string {
	return "💬"
}
