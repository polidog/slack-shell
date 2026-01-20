package notification

import (
	"sync"
	"time"
)

// VisualNotifier manages in-app visual notifications
type VisualNotifier struct {
	config        *VisualConfig
	notifications []notificationItem
	mu            sync.Mutex
}

type notificationItem struct {
	message   Message
	createdAt time.Time
}

// NewVisualNotifier creates a new visual notifier
func NewVisualNotifier(cfg *VisualConfig) *VisualNotifier {
	v := &VisualNotifier{
		config:        cfg,
		notifications: make([]notificationItem, 0),
	}

	// Start cleanup goroutine if dismiss_after is set
	if cfg.DismissAfter > 0 {
		go v.cleanupLoop()
	}

	return v
}

// Notify adds a notification to the visual queue
func (v *VisualNotifier) Notify(msg Message) error {
	if !v.config.Enabled {
		return nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Add new notification
	v.notifications = append(v.notifications, notificationItem{
		message:   msg,
		createdAt: time.Now(),
	})

	// Trim to max items
	if len(v.notifications) > v.config.MaxItems {
		v.notifications = v.notifications[len(v.notifications)-v.config.MaxItems:]
	}

	return nil
}

// GetNotifications returns the current notifications
func (v *VisualNotifier) GetNotifications() []Message {
	v.mu.Lock()
	defer v.mu.Unlock()

	msgs := make([]Message, len(v.notifications))
	for i, item := range v.notifications {
		msgs[i] = item.message
	}
	return msgs
}

// Dismiss removes a notification by index
func (v *VisualNotifier) Dismiss(index int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if index >= 0 && index < len(v.notifications) {
		v.notifications = append(v.notifications[:index], v.notifications[index+1:]...)
	}
}

// DismissAll clears all notifications
func (v *VisualNotifier) DismissAll() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.notifications = v.notifications[:0]
}

// Count returns the number of pending notifications
func (v *VisualNotifier) Count() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.notifications)
}

func (v *VisualNotifier) cleanupLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		v.cleanup()
	}
}

func (v *VisualNotifier) cleanup() {
	if v.config.DismissAfter <= 0 {
		return
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(v.config.DismissAfter) * time.Second)

	// Remove expired notifications
	remaining := make([]notificationItem, 0, len(v.notifications))
	for _, item := range v.notifications {
		if item.createdAt.After(cutoff) {
			remaining = append(remaining, item)
		}
	}
	v.notifications = remaining
}

// Close cleans up resources
func (v *VisualNotifier) Close() {
	v.DismissAll()
}
