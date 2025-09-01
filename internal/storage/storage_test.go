package storage

import (
	"context"
	"testing"
	"time"
)

// TestStorageManager tests the basic storage manager interface
func TestStorageManager(t *testing.T) {
	ctx := context.Background()

	t.Run("Initialize storage", func(t *testing.T) {
		// This test will initially fail (RED) - no implementation yet
		store := NewMemoryStorage()

		err := store.Initialize(ctx)
		if err != nil {
			t.Errorf("Initialize() should not return error, got: %v", err)
		}

		// Verify storage is ready
		if !store.IsReady() {
			t.Error("Storage should be ready after initialization")
		}
	})

	t.Run("Store and retrieve media item", func(t *testing.T) {
		store := NewMemoryStorage()
		_ = store.Initialize(ctx)

		// Create test media item
		item := &MediaItem{
			ID:         "test-123",
			ServiceID:  "tumblr",
			ExternalID: "post-456",
			Type:       "photo",
			URL:        "https://example.com/image.jpg",
			Metadata:   map[string]interface{}{"tags": []string{"test"}},
			Checksum:   "abc123",
			SizeBytes:  1024,
			CreatedAt:  time.Now(),
		}

		// Store item
		err := store.StoreMedia(ctx, item)
		if err != nil {
			t.Errorf("StoreMedia() should not return error, got: %v", err)
		}

		// Retrieve item
		retrieved, err := store.GetMedia(ctx, "test-123")
		if err != nil {
			t.Errorf("GetMedia() should not return error, got: %v", err)
		}

		if retrieved.ID != item.ID {
			t.Errorf("Retrieved ID = %v, want %v", retrieved.ID, item.ID)
		}
	})

	t.Run("Check duplicate", func(t *testing.T) {
		store := NewMemoryStorage()
		_ = store.Initialize(ctx)

		// First item
		item1 := &MediaItem{
			ID:         "item-1",
			ServiceID:  "tumblr",
			ExternalID: "post-100",
			Checksum:   "hash123",
		}
		_ = store.StoreMedia(ctx, item1)

		// Check for duplicate by checksum
		exists, err := store.IsDuplicate(ctx, "tumblr", "hash123")
		if err != nil {
			t.Errorf("IsDuplicate() should not return error, got: %v", err)
		}

		if !exists {
			t.Error("IsDuplicate() should return true for existing checksum")
		}

		// Check for non-duplicate
		exists, err = store.IsDuplicate(ctx, "tumblr", "different-hash")
		if err != nil {
			t.Errorf("IsDuplicate() should not return error, got: %v", err)
		}

		if exists {
			t.Error("IsDuplicate() should return false for new checksum")
		}
	})

	t.Run("Query media items", func(t *testing.T) {
		store := NewMemoryStorage()
		_ = store.Initialize(ctx)

		// Add test items
		items := []*MediaItem{
			{ID: "1", ServiceID: "tumblr", Type: "photo", CreatedAt: time.Now()},
			{ID: "2", ServiceID: "tumblr", Type: "video", CreatedAt: time.Now()},
			{ID: "3", ServiceID: "instagram", Type: "photo", CreatedAt: time.Now()},
		}

		for _, item := range items {
			_ = store.StoreMedia(ctx, item)
		}

		// Query by service
		query := MediaQuery{
			ServiceID: "tumblr",
		}

		results, err := store.QueryMedia(ctx, query)
		if err != nil {
			t.Errorf("QueryMedia() should not return error, got: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("QueryMedia() returned %d items, want 2", len(results))
		}

		// Query by type
		query = MediaQuery{
			Type: "photo",
		}

		results, err = store.QueryMedia(ctx, query)
		if err != nil {
			t.Errorf("QueryMedia() should not return error, got: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("QueryMedia() returned %d items for type=photo, want 2", len(results))
		}
	})
}

// TestSyncState tests sync state management
func TestSyncState(t *testing.T) {
	ctx := context.Background()

	t.Run("Store and retrieve sync state", func(t *testing.T) {
		store := NewMemoryStorage()
		_ = store.Initialize(ctx)

		state := &SyncState{
			ServiceID:      "tumblr",
			LastSyncTime:   time.Now(),
			LastSyncCursor: "cursor-123",
			ItemsProcessed: 100,
			ItemsSuccess:   95,
			ItemsFailed:    5,
		}

		err := store.SaveSyncState(ctx, state)
		if err != nil {
			t.Errorf("SaveSyncState() should not return error, got: %v", err)
		}

		retrieved, err := store.GetSyncState(ctx, "tumblr")
		if err != nil {
			t.Errorf("GetSyncState() should not return error, got: %v", err)
		}

		if retrieved.LastSyncCursor != state.LastSyncCursor {
			t.Errorf("Retrieved cursor = %v, want %v", retrieved.LastSyncCursor, state.LastSyncCursor)
		}

		if retrieved.ItemsProcessed != state.ItemsProcessed {
			t.Errorf("Retrieved items processed = %v, want %v", retrieved.ItemsProcessed, state.ItemsProcessed)
		}
	})
}

// Minimal implementations for initial GREEN phase
type MemoryStorage struct {
	ready      bool
	media      map[string]*MediaItem
	syncStates map[string]*SyncState
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		media:      make(map[string]*MediaItem),
		syncStates: make(map[string]*SyncState),
	}
}

func (m *MemoryStorage) Initialize(ctx context.Context) error {
	m.ready = true
	return nil
}

func (m *MemoryStorage) IsReady() bool {
	return m.ready
}

func (m *MemoryStorage) StoreMedia(ctx context.Context, item *MediaItem) error {
	m.media[item.ID] = item
	return nil
}

func (m *MemoryStorage) GetMedia(ctx context.Context, id string) (*MediaItem, error) {
	return m.media[id], nil
}

func (m *MemoryStorage) IsDuplicate(ctx context.Context, serviceID, checksum string) (bool, error) {
	for _, item := range m.media {
		if item.ServiceID == serviceID && item.Checksum == checksum {
			return true, nil
		}
	}
	return false, nil
}

func (m *MemoryStorage) QueryMedia(ctx context.Context, query MediaQuery) ([]*MediaItem, error) {
	var results []*MediaItem

	for _, item := range m.media {
		// Filter by service
		if query.ServiceID != "" && item.ServiceID != query.ServiceID {
			continue
		}
		// Filter by type
		if query.Type != "" && item.Type != query.Type {
			continue
		}
		results = append(results, item)
	}

	return results, nil
}

func (m *MemoryStorage) SaveSyncState(ctx context.Context, state *SyncState) error {
	m.syncStates[state.ServiceID] = state
	return nil
}

func (m *MemoryStorage) GetSyncState(ctx context.Context, serviceID string) (*SyncState, error) {
	return m.syncStates[serviceID], nil
}
