package cache

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CachedChannel represents a cached channel entry
type CachedChannel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	IsPrivate   bool      `json:"is_private"`
	IsIM        bool      `json:"is_im"`
	IsExtShared bool      `json:"is_ext_shared,omitempty"`
	UserID      string    `json:"user_id,omitempty"` // For DMs
	CachedAt    time.Time `json:"cached_at"`
}

// ChannelCacheFile represents the JSON file structure
type ChannelCacheFile struct {
	Version   int             `json:"version"`
	TeamID    string          `json:"team_id"`
	UpdatedAt time.Time       `json:"updated_at"`
	Channels  []CachedChannel `json:"channels"`
	DMs       []CachedChannel `json:"dms"`
}

// ChannelCache manages channel list with persistence
type ChannelCache struct {
	mu       sync.RWMutex
	channels []CachedChannel
	dms      []CachedChannel
	filePath string
	teamID   string
	ttl      time.Duration
	dirty    bool
}

// DefaultChannelTTL is the default time-to-live for cached channel entries (1 hour)
const DefaultChannelTTL = 1 * time.Hour

// NewChannelCache creates a new ChannelCache instance
func NewChannelCache(cacheDir, teamID string, ttl time.Duration) (*ChannelCache, error) {
	if teamID == "" {
		return nil, fmt.Errorf("teamID is required")
	}

	// Create team-specific cache directory
	teamCacheDir := filepath.Join(cacheDir, teamID)
	if err := os.MkdirAll(teamCacheDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	filePath := filepath.Join(teamCacheDir, "channels.json")

	if ttl == 0 {
		ttl = DefaultChannelTTL
	}

	cache := &ChannelCache{
		channels: nil,
		dms:      nil,
		filePath: filePath,
		teamID:   teamID,
		ttl:      ttl,
		dirty:    false,
	}

	// Load existing cache (errors are non-fatal)
	if err := cache.Load(); err != nil {
		log.Printf("Warning: failed to load channel cache: %v", err)
	}

	return cache, nil
}

// GetChannels returns cached channels
// Returns nil if cache is empty or expired
func (c *ChannelCache) GetChannels() []CachedChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.channels) == 0 {
		return nil
	}

	// Check if cache is expired
	if c.isExpired(c.channels[0].CachedAt) {
		return nil
	}

	// Return a copy to prevent modification
	result := make([]CachedChannel, len(c.channels))
	copy(result, c.channels)
	return result
}

// GetDMs returns cached DMs
// Returns nil if cache is empty or expired
func (c *ChannelCache) GetDMs() []CachedChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.dms) == 0 {
		return nil
	}

	// Check if cache is expired
	if c.isExpired(c.dms[0].CachedAt) {
		return nil
	}

	// Return a copy to prevent modification
	result := make([]CachedChannel, len(c.dms))
	copy(result, c.dms)
	return result
}

// SetChannels stores channels in the cache
func (c *ChannelCache) SetChannels(channels []CachedChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for i := range channels {
		channels[i].CachedAt = now
	}

	c.channels = channels
	c.dirty = true
}

// SetDMs stores DMs in the cache
func (c *ChannelCache) SetDMs(dms []CachedChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for i := range dms {
		dms[i].CachedAt = now
	}

	c.dms = dms
	c.dirty = true
}

// isExpired checks if a cached entry is past its TTL
func (c *ChannelCache) isExpired(cachedAt time.Time) bool {
	return time.Since(cachedAt) > c.ttl
}

// IsChannelsExpired checks if the channels cache is expired
func (c *ChannelCache) IsChannelsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.channels) == 0 {
		return true
	}
	return c.isExpired(c.channels[0].CachedAt)
}

// IsDMsExpired checks if the DMs cache is expired
func (c *ChannelCache) IsDMsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.dms) == 0 {
		return true
	}
	return c.isExpired(c.dms[0].CachedAt)
}

// Load reads the cache from disk
func (c *ChannelCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No cache file yet, not an error
			return nil
		}
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	var cacheFile ChannelCacheFile
	if err := json.Unmarshal(data, &cacheFile); err != nil {
		return fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Verify team ID matches
	if cacheFile.TeamID != "" && cacheFile.TeamID != c.teamID {
		// Different team, start fresh
		c.channels = nil
		c.dms = nil
		return nil
	}

	c.channels = cacheFile.Channels
	c.dms = cacheFile.DMs

	return nil
}

// Save writes the cache to disk
func (c *ChannelCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty {
		return nil
	}

	cacheFile := ChannelCacheFile{
		Version:   1,
		TeamID:    c.teamID,
		UpdatedAt: time.Now(),
		Channels:  c.channels,
		DMs:       c.dms,
	}

	data, err := json.MarshalIndent(cacheFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tmpPath := c.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	if err := os.Rename(tmpPath, c.filePath); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	c.dirty = false
	return nil
}

// Clear removes all entries from the cache
func (c *ChannelCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.channels = nil
	c.dms = nil
	c.dirty = true
}

// IsDirty returns whether the cache has unsaved changes
func (c *ChannelCache) IsDirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty
}

// ChannelsSize returns the number of cached channels
func (c *ChannelCache) ChannelsSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.channels)
}

// DMsSize returns the number of cached DMs
func (c *ChannelCache) DMsSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.dms)
}
