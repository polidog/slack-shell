package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUserCache(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "usercache-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	teamID := "T12345"

	// Test NewUserCache
	cache, err := NewUserCache(tmpDir, teamID, time.Hour)
	if err != nil {
		t.Fatalf("NewUserCache failed: %v", err)
	}

	// Test Set and Get
	cache.Set("U001", "alice")
	cache.Set("U002", "bob")

	name, ok := cache.Get("U001")
	if !ok || name != "alice" {
		t.Errorf("Get(U001) = %q, %v; want alice, true", name, ok)
	}

	name, ok = cache.Get("U002")
	if !ok || name != "bob" {
		t.Errorf("Get(U002) = %q, %v; want bob, true", name, ok)
	}

	// Test missing key
	_, ok = cache.Get("U999")
	if ok {
		t.Error("Get(U999) should return false for missing key")
	}

	// Test Save
	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	cacheFile := filepath.Join(tmpDir, teamID, "users.json")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Error("cache file was not created")
	}

	// Test Load (create new cache and load from file)
	cache2, err := NewUserCache(tmpDir, teamID, time.Hour)
	if err != nil {
		t.Fatalf("NewUserCache (2) failed: %v", err)
	}

	name, ok = cache2.Get("U001")
	if !ok || name != "alice" {
		t.Errorf("After load: Get(U001) = %q, %v; want alice, true", name, ok)
	}

	name, ok = cache2.Get("U002")
	if !ok || name != "bob" {
		t.Errorf("After load: Get(U002) = %q, %v; want bob, true", name, ok)
	}

	// Test ToMap
	m := cache2.ToMap()
	if len(m) != 2 {
		t.Errorf("ToMap() returned %d entries; want 2", len(m))
	}
	if m["U001"] != "alice" {
		t.Errorf("ToMap()[U001] = %q; want alice", m["U001"])
	}

	// Test SetBatch
	cache2.SetBatch(map[string]string{
		"U003": "charlie",
		"U004": "diana",
	})

	if cache2.Size() != 4 {
		t.Errorf("Size() = %d; want 4", cache2.Size())
	}

	// Test Clear
	cache2.Clear()
	if cache2.Size() != 0 {
		t.Errorf("After Clear: Size() = %d; want 0", cache2.Size())
	}
}

func TestUserCache_EmptyTeamID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usercache-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = NewUserCache(tmpDir, "", time.Hour)
	if err == nil {
		t.Error("NewUserCache with empty teamID should return error")
	}
}

func TestUserCache_TTL(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usercache-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create cache with very short TTL
	cache, err := NewUserCache(tmpDir, "T12345", time.Millisecond)
	if err != nil {
		t.Fatalf("NewUserCache failed: %v", err)
	}

	cache.Set("U001", "alice")

	// Value should be returned immediately
	if cache.IsExpired("U001") {
		t.Error("IsExpired should return false immediately after Set")
	}

	// Wait for TTL to expire
	time.Sleep(2 * time.Millisecond)

	// Value should be expired now
	if !cache.IsExpired("U001") {
		t.Error("IsExpired should return true after TTL")
	}

	// But Get should still return the value (stale-while-revalidate)
	name, ok := cache.Get("U001")
	if !ok || name != "alice" {
		t.Errorf("Get should still return stale value, got %q, %v", name, ok)
	}
}
