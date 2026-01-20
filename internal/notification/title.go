package notification

import (
	"fmt"
)

// TitleNotifier updates the terminal title with unread count
type TitleNotifier struct {
	config *TitleConfig
}

// NewTitleNotifier creates a new title notifier
func NewTitleNotifier(cfg *TitleConfig) *TitleNotifier {
	return &TitleNotifier{
		config: cfg,
	}
}

// Notify updates the terminal title
func (t *TitleNotifier) Notify(msg Message) error {
	// Title notifier doesn't use Notify, use UpdateUnreadCount instead
	return nil
}

// UpdateUnreadCount updates the terminal title with the unread count
func (t *TitleNotifier) UpdateUnreadCount(count int) {
	if !t.config.Enabled {
		return
	}

	var title string
	if count > 0 {
		title = fmt.Sprintf(t.config.Format, count)
	} else {
		title = t.config.BaseTitle
	}

	// Set terminal title using ANSI escape sequence
	// OSC 0 ; title ST (where OSC = ESC ] and ST = ESC \ or BEL)
	fmt.Printf("\033]0;%s\007", title)
}

// ResetTitle resets the terminal title to the base title
func (t *TitleNotifier) ResetTitle() {
	if !t.config.Enabled {
		return
	}
	fmt.Printf("\033]0;%s\007", t.config.BaseTitle)
}

// Close cleans up resources
func (t *TitleNotifier) Close() {
	// Reset title on close
	t.ResetTitle()
}
