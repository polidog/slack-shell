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

// CachedUser represents a cached user entry
type CachedUser struct {
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name,omitempty"`
	RealName    string    `json:"real_name,omitempty"`
	CachedAt    time.Time `json:"cached_at"`
}

// UserCacheFile represents the JSON file structure
type UserCacheFile struct {
	Version   int                   `json:"version"`
	TeamID    string                `json:"team_id"`
	UpdatedAt time.Time             `json:"updated_at"`
	Users     map[string]CachedUser `json:"users"`
}

// UserCache manages user ID to name mappings with persistence
type UserCache struct {
	mu       sync.RWMutex
	data     map[string]CachedUser
	filePath string
	teamID   string
	ttl      time.Duration
	dirty    bool
}

// DefaultTTL is the default time-to-live for cached entries (24 hours)
const DefaultTTL = 24 * time.Hour

// NewUserCache creates a new UserCache instance
func NewUserCache(cacheDir, teamID string, ttl time.Duration) (*UserCache, error) {
	if teamID == "" {
		return nil, fmt.Errorf("teamID is required")
	}

	// Create team-specific cache directory
	teamCacheDir := filepath.Join(cacheDir, teamID)
	if err := os.MkdirAll(teamCacheDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	filePath := filepath.Join(teamCacheDir, "users.json")

	if ttl == 0 {
		ttl = DefaultTTL
	}

	cache := &UserCache{
		data:     make(map[string]CachedUser),
		filePath: filePath,
		teamID:   teamID,
		ttl:      ttl,
		dirty:    false,
	}

	// Load existing cache (errors are non-fatal)
	if err := cache.Load(); err != nil {
		log.Printf("Warning: failed to load user cache: %v", err)
	}

	return cache, nil
}

// Get retrieves a user name from the cache
// Returns the name and whether it was found (and not expired)
func (c *UserCache) Get(userID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[userID]
	if !ok {
		return "", false
	}

	// Check TTL - still return the value even if expired
	// This allows stale-while-revalidate behavior
	return entry.Name, true
}

// IsExpired checks if a cached entry is past its TTL
func (c *UserCache) IsExpired(userID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[userID]
	if !ok {
		return true
	}

	return time.Since(entry.CachedAt) > c.ttl
}

// Set stores a user name in the cache
func (c *UserCache) Set(userID, name string) {
	c.SetFull(userID, name, "", "")
}

// SetFull stores a user with all name fields in the cache
func (c *UserCache) SetFull(userID, name, displayName, realName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[userID] = CachedUser{
		Name:        name,
		DisplayName: displayName,
		RealName:    realName,
		CachedAt:    time.Now(),
	}
	c.dirty = true
}

// GetFull retrieves full user info from the cache
func (c *UserCache) GetFull(userID string) (CachedUser, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[userID]
	return entry, ok
}

// GetDisplayName returns the appropriate name based on format preference
// format can be: "display_name", "real_name", "username"
func (c *UserCache) GetDisplayName(userID, format string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[userID]
	if !ok {
		return "", false
	}

	return entry.GetPreferredName(format), true
}

// GetPreferredName returns the preferred name based on format
func (u CachedUser) GetPreferredName(format string) string {
	switch format {
	case "display_name":
		// Display name -> Real name -> Username
		if u.DisplayName != "" {
			return u.DisplayName
		}
		if u.RealName != "" {
			return u.RealName
		}
		return u.Name
	case "real_name":
		// Real name -> Display name -> Username
		if u.RealName != "" {
			return u.RealName
		}
		if u.DisplayName != "" {
			return u.DisplayName
		}
		return u.Name
	default: // "username" or empty
		return u.Name
	}
}

// SetBatch stores multiple user names in the cache
func (c *UserCache) SetBatch(users map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for userID, name := range users {
		c.data[userID] = CachedUser{
			Name:     name,
			CachedAt: now,
		}
	}
	if len(users) > 0 {
		c.dirty = true
	}
}

// Load reads the cache from disk
func (c *UserCache) Load() error {
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

	var cacheFile UserCacheFile
	if err := json.Unmarshal(data, &cacheFile); err != nil {
		return fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Verify team ID matches
	if cacheFile.TeamID != "" && cacheFile.TeamID != c.teamID {
		// Different team, start fresh
		c.data = make(map[string]CachedUser)
		return nil
	}

	c.data = cacheFile.Users
	if c.data == nil {
		c.data = make(map[string]CachedUser)
	}

	return nil
}

// Save writes the cache to disk
func (c *UserCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty {
		return nil
	}

	cacheFile := UserCacheFile{
		Version:   1,
		TeamID:    c.teamID,
		UpdatedAt: time.Now(),
		Users:     c.data,
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

// ToMap returns a copy of the cache data as a simple map (for compatibility)
func (c *UserCache) ToMap() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]string, len(c.data))
	for userID, entry := range c.data {
		result[userID] = entry.Name
	}
	return result
}

// Size returns the number of cached entries
func (c *UserCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// Clear removes all entries from the cache
func (c *UserCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]CachedUser)
	c.dirty = true
}

// IsDirty returns whether the cache has unsaved changes
func (c *UserCache) IsDirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty
}
