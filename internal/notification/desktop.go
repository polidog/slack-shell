package notification

import (
	"fmt"

	"github.com/gen2brain/beeep"
)

// DesktopNotifier sends desktop notifications
type DesktopNotifier struct {
	config *DesktopConfig
}

// NewDesktopNotifier creates a new desktop notifier
func NewDesktopNotifier(cfg *DesktopConfig) *DesktopNotifier {
	return &DesktopNotifier{
		config: cfg,
	}
}

// Notify sends a desktop notification
func (d *DesktopNotifier) Notify(msg Message) error {
	if !d.config.Enabled {
		return nil
	}

	// Format title
	var title string
	if msg.IsIM {
		title = fmt.Sprintf("@%s", msg.UserName)
	} else {
		title = fmt.Sprintf("#%s", msg.ChannelName)
	}

	// Format body
	body := fmt.Sprintf("%s: %s", msg.UserName, truncateText(msg.Text, 100))

	// Send notification using beeep
	return beeep.Notify(title, body, "")
}

// Close cleans up resources
func (d *DesktopNotifier) Close() {
	// No cleanup needed
}

// truncateText truncates text to a maximum length
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}
