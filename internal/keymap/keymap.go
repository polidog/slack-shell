package keymap

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Action represents a user action
type Action string

const (
	// Navigation
	ActionUp       Action = "up"
	ActionDown     Action = "down"
	ActionTop      Action = "top"
	ActionBottom   Action = "bottom"
	ActionPageUp   Action = "page_up"
	ActionPageDown Action = "page_down"
	ActionHalfUp   Action = "half_up"
	ActionHalfDown Action = "half_down"

	// Panel navigation
	ActionNextPanel Action = "next_panel"
	ActionPrevPanel Action = "prev_panel"

	// Actions
	ActionSelect      Action = "select"
	ActionBack        Action = "back"
	ActionInputMode   Action = "input_mode"
	ActionReply       Action = "reply"
	ActionQuit        Action = "quit"
	ActionForceQuit   Action = "force_quit"
	ActionOpenThread  Action = "open_thread"
	ActionCloseThread Action = "close_thread"

	// Input mode
	ActionSubmit Action = "submit"
	ActionCancel Action = "cancel"

	// Search (future)
	ActionSearch     Action = "search"
	ActionNextMatch  Action = "next_match"
	ActionPrevMatch  Action = "prev_match"

	// Misc
	ActionRefresh Action = "refresh"
	ActionHelp    Action = "help"
)

// KeyBindings holds all key bindings
type KeyBindings struct {
	// Navigation
	Up       []string `yaml:"up"`
	Down     []string `yaml:"down"`
	Top      []string `yaml:"top"`
	Bottom   []string `yaml:"bottom"`
	PageUp   []string `yaml:"page_up"`
	PageDown []string `yaml:"page_down"`
	HalfUp   []string `yaml:"half_up"`
	HalfDown []string `yaml:"half_down"`

	// Panel navigation
	NextPanel []string `yaml:"next_panel"`
	PrevPanel []string `yaml:"prev_panel"`

	// Actions
	Select      []string `yaml:"select"`
	Back        []string `yaml:"back"`
	InputMode   []string `yaml:"input_mode"`
	Reply       []string `yaml:"reply"`
	Quit        []string `yaml:"quit"`
	ForceQuit   []string `yaml:"force_quit"`
	OpenThread  []string `yaml:"open_thread"`
	CloseThread []string `yaml:"close_thread"`

	// Input mode
	Submit []string `yaml:"submit"`
	Cancel []string `yaml:"cancel"`

	// Search
	Search    []string `yaml:"search"`
	NextMatch []string `yaml:"next_match"`
	PrevMatch []string `yaml:"prev_match"`

	// Misc
	Refresh []string `yaml:"refresh"`
	Help    []string `yaml:"help"`
}

// DefaultKeyBindings returns vim-like default keybindings
func DefaultKeyBindings() *KeyBindings {
	return &KeyBindings{
		// Navigation - Vim style
		Up:       []string{"k", "up"},
		Down:     []string{"j", "down"},
		Top:      []string{"g", "home"},
		Bottom:   []string{"G", "end"},
		PageUp:   []string{"ctrl+b", "pgup"},
		PageDown: []string{"ctrl+f", "pgdown"},
		HalfUp:   []string{"ctrl+u"},
		HalfDown: []string{"ctrl+d"},

		// Panel navigation
		NextPanel: []string{"tab", "l"},
		PrevPanel: []string{"shift+tab", "h"},

		// Actions
		Select:      []string{"enter", "o"},
		Back:        []string{"esc", "q"},
		InputMode:   []string{"i", "a"},
		Reply:       []string{"r"},
		Quit:        []string{"q"},
		ForceQuit:   []string{"ctrl+c"},
		OpenThread:  []string{"enter", "t"},
		CloseThread: []string{"esc", "q"},

		// Input mode
		Submit: []string{"enter"},
		Cancel: []string{"esc"},

		// Search
		Search:    []string{"/"},
		NextMatch: []string{"n"},
		PrevMatch: []string{"N"},

		// Misc
		Refresh: []string{"ctrl+r", "R"},
		Help:    []string{"?"},
	}
}

// Keymap provides key matching functionality
type Keymap struct {
	bindings *KeyBindings
	actionMap map[string][]Action
}

// New creates a new Keymap with the given bindings
func New(bindings *KeyBindings) *Keymap {
	if bindings == nil {
		bindings = DefaultKeyBindings()
	}

	km := &Keymap{
		bindings:  bindings,
		actionMap: make(map[string][]Action),
	}
	km.buildActionMap()
	return km
}

// buildActionMap creates a reverse mapping from keys to actions
func (km *Keymap) buildActionMap() {
	addKeys := func(keys []string, action Action) {
		for _, key := range keys {
			km.actionMap[key] = append(km.actionMap[key], action)
		}
	}

	addKeys(km.bindings.Up, ActionUp)
	addKeys(km.bindings.Down, ActionDown)
	addKeys(km.bindings.Top, ActionTop)
	addKeys(km.bindings.Bottom, ActionBottom)
	addKeys(km.bindings.PageUp, ActionPageUp)
	addKeys(km.bindings.PageDown, ActionPageDown)
	addKeys(km.bindings.HalfUp, ActionHalfUp)
	addKeys(km.bindings.HalfDown, ActionHalfDown)

	addKeys(km.bindings.NextPanel, ActionNextPanel)
	addKeys(km.bindings.PrevPanel, ActionPrevPanel)

	addKeys(km.bindings.Select, ActionSelect)
	addKeys(km.bindings.Back, ActionBack)
	addKeys(km.bindings.InputMode, ActionInputMode)
	addKeys(km.bindings.Reply, ActionReply)
	addKeys(km.bindings.Quit, ActionQuit)
	addKeys(km.bindings.ForceQuit, ActionForceQuit)
	addKeys(km.bindings.OpenThread, ActionOpenThread)
	addKeys(km.bindings.CloseThread, ActionCloseThread)

	addKeys(km.bindings.Submit, ActionSubmit)
	addKeys(km.bindings.Cancel, ActionCancel)

	addKeys(km.bindings.Search, ActionSearch)
	addKeys(km.bindings.NextMatch, ActionNextMatch)
	addKeys(km.bindings.PrevMatch, ActionPrevMatch)

	addKeys(km.bindings.Refresh, ActionRefresh)
	addKeys(km.bindings.Help, ActionHelp)
}

// GetActions returns all actions for a given key
func (km *Keymap) GetActions(key string) []Action {
	return km.actionMap[key]
}

// HasAction checks if a key triggers a specific action
func (km *Keymap) HasAction(key string, action Action) bool {
	for _, a := range km.actionMap[key] {
		if a == action {
			return true
		}
	}
	return false
}

// MatchKey checks if a tea.KeyMsg matches any of the given actions
func (km *Keymap) MatchKey(msg tea.KeyMsg, actions ...Action) bool {
	key := msg.String()
	for _, action := range actions {
		if km.HasAction(key, action) {
			return true
		}
	}
	return false
}

// GetBindings returns the current key bindings
func (km *Keymap) GetBindings() *KeyBindings {
	return km.bindings
}

// Merge merges user bindings with defaults (user bindings take precedence)
func (km *KeyBindings) Merge(other *KeyBindings) {
	if other == nil {
		return
	}

	if len(other.Up) > 0 {
		km.Up = other.Up
	}
	if len(other.Down) > 0 {
		km.Down = other.Down
	}
	if len(other.Top) > 0 {
		km.Top = other.Top
	}
	if len(other.Bottom) > 0 {
		km.Bottom = other.Bottom
	}
	if len(other.PageUp) > 0 {
		km.PageUp = other.PageUp
	}
	if len(other.PageDown) > 0 {
		km.PageDown = other.PageDown
	}
	if len(other.HalfUp) > 0 {
		km.HalfUp = other.HalfUp
	}
	if len(other.HalfDown) > 0 {
		km.HalfDown = other.HalfDown
	}
	if len(other.NextPanel) > 0 {
		km.NextPanel = other.NextPanel
	}
	if len(other.PrevPanel) > 0 {
		km.PrevPanel = other.PrevPanel
	}
	if len(other.Select) > 0 {
		km.Select = other.Select
	}
	if len(other.Back) > 0 {
		km.Back = other.Back
	}
	if len(other.InputMode) > 0 {
		km.InputMode = other.InputMode
	}
	if len(other.Reply) > 0 {
		km.Reply = other.Reply
	}
	if len(other.Quit) > 0 {
		km.Quit = other.Quit
	}
	if len(other.ForceQuit) > 0 {
		km.ForceQuit = other.ForceQuit
	}
	if len(other.OpenThread) > 0 {
		km.OpenThread = other.OpenThread
	}
	if len(other.CloseThread) > 0 {
		km.CloseThread = other.CloseThread
	}
	if len(other.Submit) > 0 {
		km.Submit = other.Submit
	}
	if len(other.Cancel) > 0 {
		km.Cancel = other.Cancel
	}
	if len(other.Search) > 0 {
		km.Search = other.Search
	}
	if len(other.NextMatch) > 0 {
		km.NextMatch = other.NextMatch
	}
	if len(other.PrevMatch) > 0 {
		km.PrevMatch = other.PrevMatch
	}
	if len(other.Refresh) > 0 {
		km.Refresh = other.Refresh
	}
	if len(other.Help) > 0 {
		km.Help = other.Help
	}
}

// GetHelpText returns help text for a specific action
func (km *Keymap) GetHelpText(action Action) string {
	var keys []string
	switch action {
	case ActionUp:
		keys = km.bindings.Up
	case ActionDown:
		keys = km.bindings.Down
	case ActionNextPanel:
		keys = km.bindings.NextPanel
	case ActionSelect:
		keys = km.bindings.Select
	case ActionInputMode:
		keys = km.bindings.InputMode
	case ActionReply:
		keys = km.bindings.Reply
	case ActionQuit:
		keys = km.bindings.Quit
	case ActionBack:
		keys = km.bindings.Back
	case ActionHelp:
		keys = km.bindings.Help
	}

	if len(keys) > 0 {
		return keys[0]
	}
	return ""
}
