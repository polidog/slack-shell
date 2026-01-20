package notification

import (
	"fmt"
)

// BellNotifier sends terminal bell notifications
type BellNotifier struct {
	config *BellConfig
}

// NewBellNotifier creates a new bell notifier
func NewBellNotifier(cfg *BellConfig) *BellNotifier {
	return &BellNotifier{
		config: cfg,
	}
}

// Notify sends a terminal bell
func (b *BellNotifier) Notify(msg Message) error {
	if !b.config.Enabled {
		return nil
	}

	// Print the bell character to trigger terminal bell
	fmt.Print("\a")
	return nil
}

// Close cleans up resources
func (b *BellNotifier) Close() {
	// No cleanup needed
}
