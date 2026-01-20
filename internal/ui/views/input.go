package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/polidog/slack-shell/internal/ui/styles"
)

type InputModel struct {
	textarea    textarea.Model
	width       int
	height      int
	focused     bool
	placeholder string
}

func NewInputModel() InputModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 4000
	ta.SetHeight(3)

	return InputModel{
		textarea:    ta,
		placeholder: "Type a message...",
	}
}

func (m InputModel) Init() tea.Cmd {
	return nil
}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var cmd tea.Cmd

	if m.focused {
		m.textarea, cmd = m.textarea.Update(msg)
	}

	return m, cmd
}

func (m InputModel) View() string {
	style := styles.InputStyle
	if m.focused {
		style = styles.InputFocusedStyle
	}

	return style.Width(m.width).Render(m.textarea.View())
}

func (m *InputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textarea.SetWidth(width - 4)
	m.textarea.SetHeight(height - 2)
}

func (m *InputModel) SetFocused(focused bool) {
	m.focused = focused
	if focused {
		m.textarea.Focus()
	} else {
		m.textarea.Blur()
	}
}

func (m InputModel) IsFocused() bool {
	return m.focused
}

func (m InputModel) Value() string {
	return strings.TrimSpace(m.textarea.Value())
}

func (m *InputModel) Reset() {
	m.textarea.Reset()
}

func (m *InputModel) SetPlaceholder(text string) {
	m.placeholder = text
	m.textarea.Placeholder = text
}
