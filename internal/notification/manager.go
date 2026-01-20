package notification

import (
	"strings"
	"sync"
)

// Manager coordinates all notification systems
type Manager struct {
	config  *Config
	bell    *BellNotifier
	desktop *DesktopNotifier
	title   *TitleNotifier
	visual  *VisualNotifier

	unreadCount map[string]int
	mu          sync.Mutex
}

// NewManager creates a new notification manager
func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	m := &Manager{
		config:      cfg,
		unreadCount: make(map[string]int),
	}

	// Initialize notifiers
	m.bell = NewBellNotifier(&cfg.Bell)
	m.desktop = NewDesktopNotifier(&cfg.Desktop)
	m.title = NewTitleNotifier(&cfg.Title)
	m.visual = NewVisualNotifier(&cfg.Visual)

	return m
}

// HandleMessage processes an incoming message and triggers notifications
func (m *Manager) HandleMessage(msg Message, currentChannelID string, inTailMode bool) {
	// Check if notifications are enabled
	if !m.config.Enabled {
		return
	}

	// Check DND
	if m.config.DND {
		return
	}

	// Check if channel is muted
	if m.isChannelMuted(msg.ChannelID, msg.ChannelName) {
		return
	}

	// Skip if currently viewing this channel (unless in tail mode)
	if msg.ChannelID == currentChannelID && !inTailMode {
		return
	}

	// Increment unread count
	m.mu.Lock()
	m.unreadCount[msg.ChannelID]++
	totalUnread := m.getTotalUnreadLocked()
	m.mu.Unlock()

	// Check mentions_only for each notifier
	shouldBell := m.config.Bell.Enabled && (!m.config.Bell.MentionsOnly || msg.IsMention)
	shouldDesktop := m.config.Desktop.Enabled && (!m.config.Desktop.MentionsOnly || msg.IsMention)

	// Trigger notifications
	if shouldBell {
		m.bell.Notify(msg)
	}

	if shouldDesktop {
		m.desktop.Notify(msg)
	}

	if m.config.Title.Enabled {
		m.title.UpdateUnreadCount(totalUnread)
	}

	if m.config.Visual.Enabled {
		m.visual.Notify(msg)
	}
}

// ClearUnread clears the unread count for a channel
func (m *Manager) ClearUnread(channelID string) {
	m.mu.Lock()
	delete(m.unreadCount, channelID)
	totalUnread := m.getTotalUnreadLocked()
	m.mu.Unlock()

	if m.config.Title.Enabled {
		m.title.UpdateUnreadCount(totalUnread)
	}
}

// GetTotalUnread returns the total unread count
func (m *Manager) GetTotalUnread() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getTotalUnreadLocked()
}

func (m *Manager) getTotalUnreadLocked() int {
	total := 0
	for _, count := range m.unreadCount {
		total += count
	}
	return total
}

// GetUnreadForChannel returns the unread count for a specific channel
func (m *Manager) GetUnreadForChannel(channelID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.unreadCount[channelID]
}

// GetVisualNotifications returns pending visual notifications
func (m *Manager) GetVisualNotifications() []Message {
	if m.visual == nil {
		return nil
	}
	return m.visual.GetNotifications()
}

// DismissVisualNotification removes a notification from the visual queue
func (m *Manager) DismissVisualNotification(index int) {
	if m.visual != nil {
		m.visual.Dismiss(index)
	}
}

// DismissAllVisualNotifications clears all visual notifications
func (m *Manager) DismissAllVisualNotifications() {
	if m.visual != nil {
		m.visual.DismissAll()
	}
}

// SetDND sets the Do Not Disturb mode
func (m *Manager) SetDND(enabled bool) {
	m.config.DND = enabled
}

// IsDND returns whether DND mode is enabled
func (m *Manager) IsDND() bool {
	return m.config.DND
}

// MuteChannel adds a channel to the mute list
func (m *Manager) MuteChannel(channelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ch := range m.config.MuteChannels {
		if ch == channelID {
			return
		}
	}
	m.config.MuteChannels = append(m.config.MuteChannels, channelID)
}

// UnmuteChannel removes a channel from the mute list
func (m *Manager) UnmuteChannel(channelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, ch := range m.config.MuteChannels {
		if ch == channelID {
			m.config.MuteChannels = append(m.config.MuteChannels[:i], m.config.MuteChannels[i+1:]...)
			return
		}
	}
}

func (m *Manager) isChannelMuted(channelID, channelName string) bool {
	for _, ch := range m.config.MuteChannels {
		if ch == channelID || strings.EqualFold(ch, channelName) {
			return true
		}
	}
	return false
}

// Close cleans up all notifiers
func (m *Manager) Close() {
	if m.bell != nil {
		m.bell.Close()
	}
	if m.desktop != nil {
		m.desktop.Close()
	}
	if m.title != nil {
		m.title.Close()
	}
	if m.visual != nil {
		m.visual.Close()
	}
}
