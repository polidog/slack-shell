package notification

// Message represents a notification message
type Message struct {
	ChannelID   string
	ChannelName string
	UserName    string
	Text        string
	IsMention   bool
	IsIM        bool
}

// Notifier interface for notification implementations
type Notifier interface {
	// Notify sends a notification
	Notify(msg Message) error
	// Close cleans up resources
	Close()
}
